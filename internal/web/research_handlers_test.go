package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BMaeda84/pv-signal-radar/internal/cache"
	"github.com/BMaeda84/pv-signal-radar/internal/research"
	"github.com/BMaeda84/pv-signal-radar/internal/stats"
)

func TestLiveAnalysisDisclosesExploratoryBoundary(t *testing.T) {
	server := NewServer(nil, cache.New(5, time.Hour))
	mux := http.NewServeMux()
	server.Routes(mux)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/analyze?drug=Example", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Deprecation") != v1DeprecationHeader || recorder.Header().Get("Sunset") != v1SunsetHeader {
		t.Fatal("v1 live endpoint must carry its RFC 9745 deprecation date and documented sunset")
	}
	var payload LiveAnalysisPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Citable || payload.Mode != "live_exploration" || payload.Source.FrozenSnapshot || payload.Source.Deduplicated {
		t.Fatalf("live boundary is not explicit: %#v", payload)
	}
	if payload.ThresholdProfile.Regulatory {
		t.Fatal("educational Evans profile must not be represented as regulatory")
	}
	if !strings.Contains(strings.Join(payload.Limitations, "\n"), "patient.drug.drugcharacterization") {
		t.Fatalf("live boundary must disclose pooled drug roles: %#v", payload.Limitations)
	}
	if strings.Contains(recorder.Body.String(), "anvisa_analysis") || strings.Contains(recorder.Body.String(), "comparative_summary") {
		t.Fatal("retired VigiMed/comparative fields leaked into v1")
	}
}

func TestDatasetCatalogFailsClosedWhenUnconfigured(t *testing.T) {
	recorder := httptest.NewRecorder()
	newTestMux().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v2/datasets", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 catalog response, got %d", recorder.Code)
	}
	var payload struct {
		Datasets                []research.DatasetManifest `json:"datasets"`
		ResearchAnalysisEnabled bool                       `json:"research_analysis_enabled"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if payload.Datasets == nil || len(payload.Datasets) != 0 || payload.ResearchAnalysisEnabled {
		t.Fatalf("unconfigured catalog must be an explicit disabled empty list: %#v", payload)
	}
}

func TestDatasetCatalogReportsRegistrationWithoutClaimingScientificValidation(t *testing.T) {
	registry, err := research.NewRegistry([]research.DatasetManifest{validWebTestManifest()})
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	server := NewServer(nil, cache.New(5, time.Hour), ResearchServices{Registry: registry})
	mux := http.NewServeMux()
	server.Routes(mux)

	catalog := httptest.NewRecorder()
	mux.ServeHTTP(catalog, httptest.NewRequest(http.MethodGet, "/api/v2/datasets", nil))
	if catalog.Code != http.StatusOK {
		t.Fatalf("catalog returned %d: %s", catalog.Code, catalog.Body.String())
	}
	var catalogPayload struct {
		Datasets []struct {
			RegistrationState string `json:"registration_state"`
		} `json:"datasets"`
	}
	if err := json.NewDecoder(catalog.Body).Decode(&catalogPayload); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(catalogPayload.Datasets) != 1 || catalogPayload.Datasets[0].RegistrationState != "integrity_checked" {
		t.Fatalf("catalog did not expose the narrow registration state: %#v", catalogPayload)
	}
	if strings.Contains(catalog.Body.String(), `"validation_status"`) || strings.Contains(catalog.Body.String(), `"validated"`) {
		t.Fatalf("catalog must not turn registry integrity checks into a scientific-validation claim: %s", catalog.Body.String())
	}

	health := httptest.NewRecorder()
	mux.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	var healthPayload map[string]any
	if err := json.NewDecoder(health.Body).Decode(&healthPayload); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if healthPayload["registered_dataset_count"] != float64(1) {
		t.Fatalf("health did not report registered dataset count: %#v", healthPayload)
	}
	if _, exists := healthPayload["validated_dataset_count"]; exists {
		t.Fatalf("health must not claim a scientific validation count: %#v", healthPayload)
	}
}

func TestResearchProtocolLookupAndDeterministicExport(t *testing.T) {
	manifest := validWebTestManifest()
	registry, err := research.NewRegistry([]research.DatasetManifest{manifest})
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	store, err := research.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	request := research.AnalysisRequest{
		SchemaVersion:      research.SchemaVersion,
		DatasetID:          manifest.DatasetID,
		DrugConceptID:      "rxnorm:1991302",
		DrugRole:           "primary_suspect",
		EventScope:         research.EventScopeAllRecordedSourcePTs,
		Comparator:         research.ComparatorAllOtherEligibleReports,
		Methods:            []string{"prr"},
		ThresholdProfileID: research.ThresholdProfileNone,
	}
	calculated := (stats.ContingencyTable{A: 10, B: 90, C: 40, D: 860, N: 1000}).Calculate("rxnorm:1991302", "meddra:10028813")
	lower95, upper95 := calculated.PRRLower95, calculated.PRRUpper95
	result, err := research.NewAnalysisResult(manifest, request, []research.DrugEventResult{{
		DrugConceptID: "rxnorm:1991302", EventConceptID: "meddra:10028813",
		EventTerm: "NAUSEA", EventCategory: "unclassified_source_pt",
		Table: research.IntegerTable{A: 10, B: 90, C: 40, D: 860, N: 1000},
		Metrics: []research.MetricEstimate{{
			Method: "prr", Measure: "reporting_ratio", Estimate: calculated.PRR,
			Lower95: &lower95, Upper95: &upper95,
			Calculation: research.MetricCalculation{
				InputCells:         research.MetricInputObserved,
				ZeroCellCorrection: research.ZeroCellCorrection{Method: research.ZeroCellCorrectionNone},
			},
		}},
	}}, []string{"Disproportionality does not establish causality or incidence."})
	if err != nil {
		t.Fatalf("create result: %v", err)
	}
	if err := store.Save(research.AnalysisRecord{
		Dataset: manifest,
		Result:  result,
		Files:   []research.ExportFile{{Name: "results.csv", MediaType: "text/csv", Data: []byte("event,prr\nNAUSEA,2.25\n")}},
	}); err != nil {
		t.Fatalf("save result: %v", err)
	}

	server := NewServer(nil, cache.New(5, time.Hour), ResearchServices{Registry: registry, Store: store})
	mux := http.NewServeMux()
	server.Routes(mux)
	requestBody, _ := json.Marshal(request)

	post := httptest.NewRecorder()
	postRequest := httptest.NewRequest(http.MethodPost, "/api/v2/analyses", bytes.NewReader(requestBody))
	postRequest.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(post, postRequest)
	if post.Code != http.StatusOK {
		t.Fatalf("expected materialized result, got %d: %s", post.Code, post.Body.String())
	}
	var posted research.AnalysisResult
	if err := json.NewDecoder(post.Body).Decode(&posted); err != nil || posted.AnalysisID != result.AnalysisID || posted.ResultDigest != result.ResultDigest || posted.RowCount != result.RowCount || posted.ResultFamily != result.ResultFamily {
		t.Fatalf("unexpected analysis response: %v %#v", err, posted)
	}
	postedManifestHash, err := research.DatasetManifestHash(posted.DatasetManifest)
	if err != nil || postedManifestHash != posted.Dataset.ManifestSHA256 {
		t.Fatalf("analysis response omitted or changed the complete dataset manifest: %v %q", err, postedManifestHash)
	}

	server.exportSlots <- struct{}{}
	busyExport := httptest.NewRecorder()
	mux.ServeHTTP(busyExport, httptest.NewRequest(http.MethodGet, "/api/v2/analyses/"+result.AnalysisID+"/export", nil))
	<-server.exportSlots
	if busyExport.Code != http.StatusTooManyRequests || busyExport.Header().Get("Retry-After") == "" {
		t.Fatalf("expected bounded export concurrency, got %d %q", busyExport.Code, busyExport.Body.String())
	}

	export := httptest.NewRecorder()
	mux.ServeHTTP(export, httptest.NewRequest(http.MethodGet, "/api/v2/analyses/"+result.AnalysisID+"/export", nil))
	if export.Code != http.StatusOK || export.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("expected ZIP export, got %d %q", export.Code, export.Header().Get("Content-Type"))
	}
	if export.Header().Get("X-Result-Digest") != result.ResultDigest || export.Header().Get("X-Result-Row-Count") != "1" {
		t.Fatalf("export integrity headers drifted: %q %q", export.Header().Get("X-Result-Digest"), export.Header().Get("X-Result-Row-Count"))
	}
	if !bytes.HasPrefix(export.Body.Bytes(), []byte("PK")) {
		t.Fatal("export is not a ZIP archive")
	}
}

func TestResearchAnalysisRejectsSimpleCrossSiteMediaType(t *testing.T) {
	registry, err := research.NewRegistry([]research.DatasetManifest{validWebTestManifest()})
	if err != nil {
		t.Fatal(err)
	}
	store, err := research.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(nil, cache.New(5, time.Hour), ResearchServices{Registry: registry, Store: store})
	mux := http.NewServeMux()
	server.Routes(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/analyses", strings.NewReader(`{"dataset_id":"known"}`))
	request.Header.Set("Content-Type", "text/plain")
	request.Header.Set("Origin", "https://attacker.invalid")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func validWebTestManifest() research.DatasetManifest {
	return research.DatasetManifest{
		SchemaVersion: research.SchemaVersion,
		DatasetID:     "faers-2025q4-test",
		Title:         "FAERS 2025 Q4 test fixture",
		Description:   "Synthetic handler fixture; not a released dataset.",
		Source: research.DatasetSource{
			Name: "FDA FAERS quarterly files", Publisher: "US FDA",
			LandingPage: "https://www.fda.gov/", RetrievedAt: "2026-01-10T12:00:00Z",
			Files: []research.SourceFile{{URL: "https://example.invalid/faers.zip", SHA256: strings.Repeat("a", 64), Bytes: 100}},
		},
		Coverage: research.DatasetCoverage{StartDate: "2025-10-01", EndDate: "2025-12-31", Geography: "global", Release: "2025Q4"},
		Processing: research.DatasetProcessing{
			PipelineVersion: "test-v1", SourceCommit: strings.Repeat("1", 40),
			DeduplicationPolicy: "latest case version", CountUnit: "unique report drug-event pair",
			DrugRolePolicy: "preserved", Exclusions: []string{"fixture only"},
		},
		Artifacts: []research.DatasetArtifact{{
			Name: "aggregate", Path: "aggregate.sqlite", MediaType: "application/vnd.sqlite3",
			SHA256: strings.Repeat("b", 64), Bytes: 200,
		}},
		Completeness: research.DatasetCompleteness{SourceDEMORows: 1200, CurrentCaseReports: 1100, EligibleReports: 1000, DrugEventPairs: 1500},
		Limitations:  []string{"Synthetic test fixture."},
	}
}
