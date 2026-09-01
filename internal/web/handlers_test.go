package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BMaeda84/pv-signal-radar/internal/cache"
)

func newTestMux() *http.ServeMux {
	server := NewServer(nil, cache.New(5, time.Hour))
	mux := http.NewServeMux()
	server.Routes(mux)
	return mux
}

func TestAnalyzeRejectsInvalidInputWithoutCallingUpstream(t *testing.T) {
	mux := newTestMux()
	tests := []struct {
		name string
		url  string
		code string
	}{
		{"missing", "/api/v1/analyze", "drug_required"},
		{"line break", "/api/v1/analyze?drug=drug%0Aname", "invalid_drug"},
		{"too long", "/api/v1/analyze?drug=" + strings.Repeat("a", maxDrugQueryRunes+1), "invalid_drug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.url, nil))

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", recorder.Code)
			}
			var response map[string]string
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("invalid JSON response: %v", err)
			}
			if response["code"] != tt.code {
				t.Fatalf("expected code %q, got %#v", tt.code, response)
			}
		})
	}
}

func TestAPIMethodAndSecurityHeaders(t *testing.T) {
	mux := newTestMux()
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/health", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header, got %q", got)
	}
	if got := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
		t.Fatalf("expected restrictive CSP, got %q", got)
	}
}

func TestAnalyzeReturnsBusyWhenAllSlotsAreReserved(t *testing.T) {
	server := NewServer(nil, cache.New(5, time.Hour))
	for range maxConcurrentAnalyses {
		server.analysisSlots <- struct{}{}
	}
	defer func() {
		for range maxConcurrentAnalyses {
			<-server.analysisSlots
		}
	}()

	mux := http.NewServeMux()
	server.Routes(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/analyze?drug=Semaglutide", nil))

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("expected retry hint, got %q", got)
	}
}

func TestAnalyzeReturnsRateLimitedBeforeCallingUpstream(t *testing.T) {
	server := NewServer(nil, cache.New(5, time.Hour))
	server.analysisGate.mu.Lock()
	server.analysisGate.nextAllowed = time.Now().Add(3 * time.Second)
	server.analysisGate.mu.Unlock()

	mux := http.NewServeMux()
	server.Routes(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/analyze?drug=Semaglutide", nil))

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Retry-After"); got == "" || got == "0" {
		t.Fatalf("expected a positive retry hint, got %q", got)
	}
	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if response["code"] != "analysis_rate_limited" {
		t.Fatalf("expected rate-limit code, got %#v", response)
	}
}

func TestAnalysisStartPacingStaysWithinUpstreamBudget(t *testing.T) {
	// A closed 60-second window can contain a start at both endpoints. This
	// invariant makes a future increase in reaction fan-out fail tests instead
	// of silently exceeding the safety budget assumed by the rate gate.
	maxStartsPerWindow := int(time.Minute/analysisStartInterval) + 1
	if requests := maxStartsPerWindow * maxUpstreamRequestsPerScan; requests > maxUpstreamRequestsPerMinute {
		t.Fatalf("rate gate can issue %d upstream requests per 60-second window; budget is %d", requests, maxUpstreamRequestsPerMinute)
	}
}
