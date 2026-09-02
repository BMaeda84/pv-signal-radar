package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/BMaeda84/pv-signal-radar/internal/cache"
	"github.com/BMaeda84/pv-signal-radar/internal/openfda"
	"github.com/BMaeda84/pv-signal-radar/internal/research"
	"github.com/BMaeda84/pv-signal-radar/internal/version"
)

const (
	maxDrugQueryRunes            = 120
	maxResearchRequestBytes      = 64 << 10
	maxConcurrentAnalyses        = 2
	analysisStartInterval        = 15 * time.Second
	researchAnalysisTimeout      = 20 * time.Second
	maxUpstreamRequestsPerScan   = 3 + openfda.MaxReactionsPerAnalysis
	maxUpstreamRequestsPerMinute = 140
	// RFC 9745 encodes Deprecation as a Structured Field Date, not a boolean.
	// The date marks the research-preview contract change; Sunset gives clients
	// a one-year migration window to the frozen-dataset v2 endpoints.
	v1DeprecationHeader = "@1788307200" // 2026-09-02T00:00:00Z
	v1SunsetHeader      = "Thu, 02 Sep 2027 00:00:00 GMT"
)

var analysisIDPattern = regexp.MustCompile("^[a-f0-9]{64}$")
var errUnsupportedResearchMediaType = errors.New("research analysis requires application/json")

// analysisStartGate spaces live openFDA scans without a burst. This controls
// operational load only; it does not make the mutable live query reproducible.
type analysisStartGate struct {
	mu          sync.Mutex
	nextAllowed time.Time
}

func (g *analysisStartGate) tryAcquire(now time.Time) (time.Duration, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if now.Before(g.nextAllowed) {
		return g.nextAllowed.Sub(now), false
	}
	g.nextAllowed = now.Add(analysisStartInterval)
	return 0, true
}

func retryAfterSeconds(delay time.Duration) string {
	seconds := int((delay + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

//go:embed static/*
var staticFS embed.FS

// LiveAnalysisPayload keeps the v1 response compatible while making its
// non-reproducible source, selection boundary, and intended use machine-readable.
type LiveAnalysisPayload struct {
	Mode             string                     `json:"mode"`
	Citable          bool                       `json:"citable"`
	Source           LiveSource                 `json:"source"`
	ThresholdProfile ThresholdProfile           `json:"threshold_profile"`
	Limitations      []string                   `json:"limitations"`
	FDA              *openfda.DrugEventAnalysis `json:"fda_analysis"`

	// Deprecated flat fields remain during the v1 transition.
	QueryDrug         string                  `json:"query_drug"`
	NormalizedDrug    string                  `json:"normalized_drug"`
	DrugTotalReports  int64                   `json:"drug_total_reports"`
	DatabaseUniverseN int64                   `json:"database_universe_n"`
	SDRReviewCount    int                     `json:"sdr_review_count"`
	TotalReactions    int                     `json:"total_reactions_analyzed"`
	Signals           []openfda.SignalSummary `json:"signals"`
	Timestamp         string                  `json:"timestamp"`
	Disclaimer        string                  `json:"disclaimer"`
}

type LiveSource struct {
	Name           string `json:"name"`
	AccessMode     string `json:"access_mode"`
	SelectionScope string `json:"selection_scope"`
	SelectionLimit int    `json:"selection_limit"`
	Deduplicated   bool   `json:"deduplicated"`
	FrozenSnapshot bool   `json:"frozen_snapshot"`
}

type ThresholdProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IntendedUse string `json:"intended_use"`
	Rule        string `json:"rule"`
	Regulatory  bool   `json:"regulatory"`
}

type DatasetCatalogEntry struct {
	research.DatasetManifest
	ManifestSHA256    string `json:"manifest_sha256"`
	RegistrationState string `json:"registration_state"`
}

// ResearchServices are optional because a clean install has no registered
// snapshot. Registration proves schema/integrity checks only; scientific
// validation remains a separate release gate documented outside this runtime.
type ResearchServices struct {
	Registry             *research.Registry
	Store                research.AnalysisStore
	Engine               *research.SQLiteEngine
	Software             research.SoftwareReference
	AllowMaterialization bool
}

type Server struct {
	openfdaClient      *openfda.Client
	cache              *cache.LRUCache
	research           ResearchServices
	datasetCatalogJSON []byte
	datasetCatalogErr  error
	analysisSlots      chan struct{}
	analysisGate       *analysisStartGate
	researchGate       *analysisStartGate
	exportSlots        chan struct{}
}

type responseWriteTracker struct {
	http.ResponseWriter
	wrote bool
}

func (w *responseWriteTracker) Write(data []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(data)
}

func NewServer(fdaClient *openfda.Client, lruCache *cache.LRUCache, optionalResearch ...ResearchServices) *Server {
	var services ResearchServices
	if len(optionalResearch) > 0 {
		services = optionalResearch[0]
	}
	if services.Software == (research.SoftwareReference{}) {
		if services.Engine != nil {
			services.Software = services.Engine.Software()
		} else {
			services.Software = research.DevelopmentSoftwareReference()
		}
	}
	server := &Server{
		openfdaClient: fdaClient,
		cache:         lruCache,
		research:      services,
		analysisSlots: make(chan struct{}, maxConcurrentAnalyses),
		analysisGate:  &analysisStartGate{},
		researchGate:  &analysisStartGate{},
		exportSlots:   make(chan struct{}, 1),
	}
	server.datasetCatalogJSON, server.datasetCatalogErr = buildDatasetCatalog(services)
	return server
}

// buildDatasetCatalog performs manifest cloning, validation, and hashing once
// at startup. Health checks and public catalog reads must not repeatedly turn
// multi-megabyte provenance records into CPU/heap amplification primitives.
func buildDatasetCatalog(services ResearchServices) ([]byte, error) {
	datasets := make([]DatasetCatalogEntry, 0)
	if services.Registry != nil {
		for _, manifest := range services.Registry.List() {
			hash, err := research.DatasetManifestHash(manifest)
			if err != nil {
				return nil, err
			}
			datasets = append(datasets, DatasetCatalogEntry{
				DatasetManifest: manifest, ManifestSHA256: hash, RegistrationState: "integrity_checked",
			})
		}
	}
	payload := map[string]any{
		"schema_version": research.SchemaVersion, "datasets": datasets,
		"research_analysis_enabled":      len(datasets) > 0 && services.Store != nil,
		"online_materialization_enabled": len(datasets) > 0 && services.Store != nil && services.Engine != nil && services.AllowMaterialization,
	}
	if len(datasets) > 0 {
		payload["software"] = services.Software
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func (s *Server) Routes(mux *http.ServeMux) {
	subFS, err := fs.Sub(staticFS, "static")
	if err == nil {
		mux.Handle("/static/", withSecurityHeaders(http.StripPrefix("/static/", http.FileServer(http.FS(subFS)))))
	}
	mux.Handle("/", withSecurityHeaders(http.HandlerFunc(s.handleIndex)))
	mux.Handle("/methodology", withSecurityHeaders(http.HandlerFunc(s.handleIndex)))
	mux.Handle("/research", withSecurityHeaders(http.HandlerFunc(s.handleIndex)))
	mux.Handle("/api/v1/health", withSecurityHeaders(http.HandlerFunc(s.handleHealth)))
	mux.Handle("/api/v1/analyze", withSecurityHeaders(http.HandlerFunc(s.handleAnalyze)))
	mux.Handle("/api/v1/feedback", withSecurityHeaders(http.HandlerFunc(s.handleRetiredFeedback)))
	mux.Handle("/api/v2/datasets", withSecurityHeaders(http.HandlerFunc(s.handleDatasets)))
	mux.Handle("/api/v2/analyses", withSecurityHeaders(http.HandlerFunc(s.handleCreateAnalysis)))
	mux.Handle("/api/v2/analyses/", withSecurityHeaders(http.HandlerFunc(s.handleStoredAnalysis)))
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'; connect-src 'self'; img-src 'self' data:; script-src 'self'; style-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"code": code, "error": "request could not be processed"})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	allowHeader := "GET"
	if len(allowed) > 0 {
		allowHeader = strings.Join(allowed, ", ")
	}
	w.Header().Set("Allow", allowHeader)
	writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, http.MethodGet, http.MethodHead)
		return
	}
	if r.URL.Path != "/" && r.URL.Path != "/methodology" && r.URL.Path != "/research" {
		http.NotFound(w, r)
		return
	}
	indexContent, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Index file not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(indexContent)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	datasetCount := 0
	if s.research.Registry != nil {
		datasetCount = s.research.Registry.Len()
	}
	payload := map[string]any{
		"status": "healthy", "service": "pv-signal-radar", "version": version.Current,
		"cached_keys": s.cache.Len(), "live_exploration": "US FDA FAERS via openFDA",
		"registered_dataset_count":       datasetCount,
		"research_analysis_enabled":      datasetCount > 0 && s.research.Store != nil,
		"online_materialization_enabled": datasetCount > 0 && s.research.Store != nil && s.research.Engine != nil && s.research.AllowMaterialization,
	}
	if datasetCount > 0 {
		payload["research_software"] = s.research.Software
	}
	writeJSON(w, http.StatusOK, payload)
}

func setV1DeprecationHeaders(w http.ResponseWriter) {
	w.Header().Set("Deprecation", v1DeprecationHeader)
	w.Header().Set("Sunset", v1SunsetHeader)
	w.Header().Set("Link", "</api/v2/datasets>; rel=\"successor-version\"")
	w.Header().Set("Warning", "299 - \"Live openFDA exploration is mutable and not citable as a reproducible research result\"")
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	setV1DeprecationHeaders(w)
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	drugQuery := strings.TrimSpace(r.URL.Query().Get("drug"))
	if drugQuery == "" {
		writeJSONError(w, http.StatusBadRequest, "drug_required")
		return
	}
	if utf8.RuneCountInString(drugQuery) > maxDrugQueryRunes || strings.ContainsAny(drugQuery, "\r\n") {
		writeJSONError(w, http.StatusBadRequest, "invalid_drug")
		return
	}
	cacheKey := "live-openfda:" + strings.ToLower(drugQuery)
	if cachedValue, found := s.cache.Get(cacheKey); found {
		w.Header().Set("X-Cache-Status", "HIT")
		writeJSON(w, http.StatusOK, cachedValue)
		return
	}
	select {
	case s.analysisSlots <- struct{}{}:
		defer func() { <-s.analysisSlots }()
	default:
		w.Header().Set("Retry-After", "5")
		writeJSONError(w, http.StatusTooManyRequests, "analysis_busy")
		return
	}
	if retryAfter, allowed := s.analysisGate.tryAcquire(time.Now()); !allowed {
		w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
		writeJSONError(w, http.StatusTooManyRequests, "analysis_rate_limited")
		return
	}

	var analysis *openfda.DrugEventAnalysis
	var err error
	if s.openfdaClient != nil {
		analysis, err = s.openfdaClient.AnalyzeDrug(r.Context(), drugQuery)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "analysis_unavailable")
			return
		}
	} else {
		analysis = &openfda.DrugEventAnalysis{
			QueryDrug: drugQuery, NormalizedDrug: drugQuery, DatabaseUniverseN: 20_000_000,
			Signals: []openfda.SignalSummary{}, Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
	}
	payload := LiveAnalysisPayload{
		Mode: "live_exploration", Citable: false,
		Source: LiveSource{
			Name: "US FDA FAERS via openFDA Drug Event API", AccessMode: "live_api",
			SelectionScope: "most_frequently_reported_meddra_pt", SelectionLimit: openfda.MaxReactionsPerAnalysis,
			Deduplicated: false, FrozenSnapshot: false,
		},
		ThresholdProfile: ThresholdProfile{
			ID: "evans-educational-v1", Name: "Educational Evans screening profile",
			IntendedUse: "teaching_and_exploratory_review",
			Rule:        "a >= 3 AND PRR >= 2 AND Yates chi-square >= 4", Regulatory: false,
		},
		Limitations: []string{
			"Live openFDA results change over time and are not a frozen research snapshot.",
			"Reports are not case-version deduplicated by this endpoint.",
			"The live query does not filter patient.drug.drugcharacterization; suspect, concomitant, and interacting roles are pooled.",
			"Multiple drugs and reactions in one report are not individually linked.",
			"Only the most frequently reported terms are screened; rare events may be absent.",
			"The deprecated live path does not compute Fisher exact or multiplicity-adjusted q-values.",
			"Reporting disproportionality is not incidence, risk, or causality.",
		},
		FDA: analysis, QueryDrug: analysis.QueryDrug, NormalizedDrug: analysis.NormalizedDrug,
		DrugTotalReports: analysis.DrugTotalReports, DatabaseUniverseN: analysis.DatabaseUniverseN,
		SDRReviewCount: analysis.SDRReviewCount, TotalReactions: analysis.TotalReactions,
		Signals: analysis.Signals, Timestamp: analysis.Timestamp,
		Disclaimer: "Exploratory SDR screening only. Not a clinical, causal, incidence, regulatory, or reproducible research conclusion.",
	}
	s.cache.Set(cacheKey, payload)
	w.Header().Set("X-Cache-Status", "MISS")
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleRetiredFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	writeJSON(w, http.StatusGone, map[string]string{
		"code":       "feedback_retired",
		"error":      "PII feedback intake is disabled until privacy and retention governance exist",
		"issues_url": "https://github.com/BMaeda84/pv-signal-radar/issues",
	})
}

func (s *Server) handleDatasets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if s.datasetCatalogErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "dataset_registry_invalid")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(s.datasetCatalogJSON)
}

func decodeResearchRequest(w http.ResponseWriter, r *http.Request) (research.AnalysisRequest, error) {
	var request research.AnalysisRequest
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return request, errUnsupportedResearchMediaType
	}
	body := http.MaxBytesReader(w, r.Body, maxResearchRequestBytes)
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return request, errors.New("multiple JSON values are not allowed")
		}
		return request, err
	}
	return request, nil
}

func (s *Server) handleCreateAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if s.research.Registry == nil || s.research.Store == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "research_dataset_unavailable")
		return
	}
	request, err := decodeResearchRequest(w, r)
	if err != nil {
		if errors.Is(err, errUnsupportedResearchMediaType) {
			writeJSONError(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid_analysis_request")
		return
	}
	manifest, err := s.research.Registry.Require(request.DatasetID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "dataset_not_found")
		return
	}
	normalized, err := research.NormalizeAnalysisRequest(request)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, "invalid_analysis_protocol")
		return
	}
	analysisID, err := research.AnalysisIDForSoftware(manifest, normalized, s.research.Software)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, "invalid_analysis_protocol")
		return
	}
	storedResult, err := s.research.Store.LoadResult(analysisID)
	if errors.Is(err, os.ErrNotExist) {
		if !s.research.AllowMaterialization || s.research.Engine == nil || s.research.Engine.DatasetID() != manifest.DatasetID {
			writeJSON(w, http.StatusConflict, map[string]string{
				"code": "analysis_not_materialized", "analysis_id": analysisID,
				"error": "the deterministic protocol is valid, but its batch result has not been materialized",
			})
			return
		}
		// A new protocol consumes CPU and one immutable store entry. Bound both
		// concurrency and start rate; an already materialized analysis bypasses
		// this gate and remains cheap/idempotent. No event rows are truncated.
		select {
		case s.analysisSlots <- struct{}{}:
			defer func() { <-s.analysisSlots }()
		default:
			w.Header().Set("Retry-After", "5")
			writeJSONError(w, http.StatusTooManyRequests, "analysis_busy")
			return
		}
		if retryAfter, allowed := s.researchGate.tryAcquire(time.Now()); !allowed {
			w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
			writeJSONError(w, http.StatusTooManyRequests, "analysis_rate_limited")
			return
		}
		analysisContext, cancel := context.WithTimeout(r.Context(), researchAnalysisTimeout)
		defer cancel()
		record, analyzeErr := s.research.Engine.Analyze(analysisContext, normalized)
		err = analyzeErr
		if errors.Is(err, research.ErrNoMatchingRows) {
			writeJSONError(w, http.StatusNotFound, "analysis_has_no_matching_rows")
			return
		}
		if errors.Is(err, research.ErrBatchMethodRequired) || errors.Is(err, research.ErrUnknownThresholdProfile) || errors.Is(err, research.ErrOnlineAnalysisTooLarge) {
			writeJSONError(w, http.StatusUnprocessableEntity, "analysis_requires_validated_batch_method")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, "analysis_protocol_not_supported_by_dataset")
			return
		}
		if err := s.research.Store.Save(record); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "analysis_store_failed")
			return
		}
		writeJSON(w, http.StatusCreated, record.Result)
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "analysis_store_invalid")
		return
	}
	writeJSON(w, http.StatusOK, storedResult)
}

func (s *Server) handleStoredAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if s.research.Store == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "research_dataset_unavailable")
		return
	}
	remainder := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v2/analyses/"), "/")
	parts := strings.Split(remainder, "/")
	if len(parts) == 0 || !analysisIDPattern.MatchString(parts[0]) || (len(parts) == 2 && parts[1] != "export") || len(parts) > 2 {
		writeJSONError(w, http.StatusBadRequest, "invalid_analysis_id")
		return
	}
	if len(parts) == 1 {
		result, err := s.research.Store.LoadResult(parts[0])
		if errors.Is(err, os.ErrNotExist) {
			writeJSONError(w, http.StatusNotFound, "analysis_not_found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "analysis_store_invalid")
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	select {
	case s.exportSlots <- struct{}{}:
		defer func() { <-s.exportSlots }()
	default:
		w.Header().Set("Retry-After", "5")
		writeJSONError(w, http.StatusTooManyRequests, "export_busy")
		return
	}
	record, err := s.research.Store.Load(parts[0])
	if errors.Is(err, os.ErrNotExist) {
		writeJSONError(w, http.StatusNotFound, "analysis_not_found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "analysis_store_invalid")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"pv-signal-radar-"+parts[0]+".zip\"")
	w.Header().Set("Cache-Control", "public, immutable")
	w.Header().Set("X-Result-Digest", record.Result.ResultDigest)
	w.Header().Set("X-Result-Row-Count", strconv.FormatInt(record.Result.RowCount, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	tracked := &responseWriteTracker{ResponseWriter: w}
	if err := s.research.Store.ExportRecord(record, tracked); err != nil {
		// ExportRecord validates and builds deterministic entries before the ZIP
		// writer emits bytes. A downstream disconnect after emission cannot be
		// converted into a JSON error, so only pre-write failures are reported.
		if !tracked.wrote {
			w.Header().Del("Content-Disposition")
			writeJSONError(w, http.StatusInternalServerError, "analysis_export_failed")
		}
		return
	}
}
