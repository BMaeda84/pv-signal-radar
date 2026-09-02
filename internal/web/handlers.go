package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/BMaeda84/pv-signal-radar/internal/cache"
	"github.com/BMaeda84/pv-signal-radar/internal/feedback"
	"github.com/BMaeda84/pv-signal-radar/internal/openfda"
	"github.com/BMaeda84/pv-signal-radar/internal/vigimed"
)

const (
	maxDrugQueryRunes            = 120
	maxConcurrentAnalyses        = 2
	analysisStartInterval        = 15 * time.Second
	maxUpstreamRequestsPerScan   = 3 + openfda.MaxReactionsPerAnalysis
	maxUpstreamRequestsPerMinute = 140
)

// analysisStartGate spaces cache-miss scans without a burst. A scan makes
// three setup queries plus at most 25 reaction-background queries. A 15-second
// interval allows no more than five starts in any 60-second window (140 calls),
// leaving headroom below openFDA's documented unauthenticated 240/min limit.
type analysisStartGate struct {
	mu          sync.Mutex
	nextAllowed time.Time
}

func (g *analysisStartGate) tryAcquire(now time.Time) (retryAfter time.Duration, allowed bool) {
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

// ConsolidatedAnalysisPayload is the unified response returned by /api/v1/analyze.
type ConsolidatedAnalysisPayload struct {
	QueryDrug          string                      `json:"query_drug"`
	NormalizedDrug     string                      `json:"normalized_drug"`
	ATCCode            string                      `json:"atc_code"`
	FDA                *openfda.DrugEventAnalysis  `json:"fda_analysis"`
	Anvisa             *vigimed.BrazilAnalysis     `json:"anvisa_analysis"`
	ComparativeSummary vigimed.ComparativeSummary  `json:"comparative_summary"`
	Timestamp          string                      `json:"timestamp"`
	Disclaimer         string                      `json:"disclaimer"`

	// Backwards compatibility fields mapping directly to FDA analysis
	DrugTotalReports   int64                       `json:"drug_total_reports"`
	DatabaseUniverseN  int64                       `json:"database_universe_n"`
	ActiveSignalsCount int                         `json:"active_signals_count"`
	TotalReactions     int                         `json:"total_reactions_analyzed"`
	Signals            []openfda.SignalSummary     `json:"signals"`
}

// Server holds web handlers, openFDA client, in-memory cache, and feedback service.
type Server struct {
	openfdaClient   *openfda.Client
	cache           *cache.LRUCache
	feedbackService *feedback.Service
	analysisSlots   chan struct{}
	analysisGate    *analysisStartGate
}

// NewServer initializes the HTTP router and services.
func NewServer(fdaClient *openfda.Client, cache *cache.LRUCache, optionalFbService ...*feedback.Service) *Server {
	var fbService *feedback.Service
	if len(optionalFbService) > 0 {
		fbService = optionalFbService[0]
	}

	return &Server{
		openfdaClient:   fdaClient,
		cache:           cache,
		feedbackService: fbService,
		analysisSlots:   make(chan struct{}, maxConcurrentAnalyses),
		analysisGate:    &analysisStartGate{},
	}
}

// Routes sets up all HTTP routes on the given mux.
func (s *Server) Routes(mux *http.ServeMux) {
	// 1. Static assets
	subFS, err := fs.Sub(staticFS, "static")
	if err == nil {
		mux.Handle("/static/", withSecurityHeaders(http.StripPrefix("/static/", http.FileServer(http.FS(subFS)))))
	}

	// 2. Main Page & View Paths
	mux.Handle("/", withSecurityHeaders(http.HandlerFunc(s.handleIndex)))
	mux.Handle("/methodology", withSecurityHeaders(http.HandlerFunc(s.handleIndex)))
	mux.Handle("/feedback", withSecurityHeaders(http.HandlerFunc(s.handleIndex)))

	// 3. API Endpoints
	mux.Handle("/api/v1/health", withSecurityHeaders(http.HandlerFunc(s.handleHealth)))
	mux.Handle("/api/v1/analyze", withSecurityHeaders(http.HandlerFunc(s.handleAnalyze)))
	mux.Handle("/api/v1/feedback", withSecurityHeaders(http.HandlerFunc(s.handleFeedback)))
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; connect-src 'self'; img-src 'self' data:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":  code,
		"error": "request could not be processed",
	})
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
	if r.URL.Path != "/" && r.URL.Path != "/methodology" && r.URL.Path != "/feedback" {
		http.NotFound(w, r)
		return
	}

	indexContent, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Index file not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexContent)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "healthy",
		"service":     "pv-signal-radar",
		"version":     "2.0.0",
		"cached_keys": s.cache.Len(),
		"databases": []string{
			"US FDA FAERS (OpenFDA Live API)",
			"ANVISA VigiMed (Microdados Abertos Brasil)",
		},
	})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
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

	cacheKey := "consolidated:" + strings.ToLower(drugQuery)

	// Check cache
	if cachedVal, found := s.cache.Get(cacheKey); found {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Cache-Status", "HIT")
		_ = json.NewEncoder(w).Encode(cachedVal)
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

	// 1. Resolve canonical substance mapping
	mapping, _ := vigimed.ResolveDrug(drugQuery)
	canonicalSearchDrug := drugQuery
	atcCode := "N/A"
	if mapping != nil {
		canonicalSearchDrug = mapping.CanonicalName
		atcCode = mapping.ATCCode
	}

	// 2. Perform FDA live analysis
	var fdaAnalysis *openfda.DrugEventAnalysis
	var err error
	if s.openfdaClient != nil {
		fdaAnalysis, err = s.openfdaClient.AnalyzeDrug(r.Context(), canonicalSearchDrug)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "analysis_unavailable")
			return
		}
	} else {
		// Mock empty for test servers without client
		fdaAnalysis = &openfda.DrugEventAnalysis{
			QueryDrug:         canonicalSearchDrug,
			NormalizedDrug:    canonicalSearchDrug,
			DrugTotalReports:  0,
			DatabaseUniverseN: 20000000,
			Signals:           []openfda.SignalSummary{},
			Timestamp:         time.Now().UTC().Format(time.RFC3339),
		}
	}

	// 3. Query ANVISA VigiMed dataset
	anvisaAnalysis, _ := vigimed.GetBrazilAnalysis(drugQuery)

	// 4. Generate Comparative Summary
	comparative := vigimed.GenerateComparativeSummary(
		canonicalSearchDrug,
		atcCode,
		fdaAnalysis.ActiveSignalsCount,
		fdaAnalysis.DrugTotalReports,
		anvisaAnalysis,
	)

	payload := ConsolidatedAnalysisPayload{
		QueryDrug:          drugQuery,
		NormalizedDrug:     canonicalSearchDrug,
		ATCCode:            atcCode,
		FDA:                fdaAnalysis,
		Anvisa:             anvisaAnalysis,
		ComparativeSummary: comparative,
		Timestamp:          fdaAnalysis.Timestamp,
		Disclaimer:         "FAERS and VigiMed databases aggregate spontaneous adverse event reports. Disproportionality metrics (PRR/ROR) indicate statistical reporting associations, not proven biological causality.",

		// Backwards compatibility mappings
		DrugTotalReports:   fdaAnalysis.DrugTotalReports,
		DatabaseUniverseN:  fdaAnalysis.DatabaseUniverseN,
		ActiveSignalsCount: fdaAnalysis.ActiveSignalsCount,
		TotalReactions:     fdaAnalysis.TotalReactions,
		Signals:            fdaAnalysis.Signals,
	}

	// Store in cache
	s.cache.Set(cacheKey, payload)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Cache-Status", "MISS")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	if s.feedbackService == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "feedback_unavailable")
		return
	}

	var req feedback.FeedbackSubmission
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = strings.Split(forwarded, ",")[0]
	}

	record, err := s.feedbackService.Submit(req, clientIP, r.UserAgent())
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":  "submission_failed",
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"message":     "Feedback successfully received and recorded for review.",
		"feedback_id": record.ID,
		"timestamp":   record.Timestamp,
	})
}
