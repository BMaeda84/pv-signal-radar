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
	"github.com/BMaeda84/pv-signal-radar/internal/openfda"
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
// leaving headroom below openFDA's documented unauthenticated 240/min limit;
// deployment-wide and daily limits still require operational monitoring.
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

// Server holds web handlers, openFDA client, and in-memory cache.
type Server struct {
	openfdaClient *openfda.Client
	cache         *cache.LRUCache
	analysisSlots chan struct{}
	analysisGate  *analysisStartGate
}

// NewServer initializes the HTTP router and services.
func NewServer(fdaClient *openfda.Client, cache *cache.LRUCache) *Server {
	return &Server{
		openfdaClient: fdaClient,
		cache:         cache,
		// A cache miss fans out into up to 28 upstream requests. A small global
		// bound prevents cache-busting traffic from creating unbounded overlapping
		// work; the start gate below separately constrains aggregate request pace.
		analysisSlots: make(chan struct{}, maxConcurrentAnalyses),
		analysisGate:  &analysisStartGate{},
	}
}

// Routes sets up all HTTP routes on the given mux.
func (s *Server) Routes(mux *http.ServeMux) {
	// 1. Static assets
	subFS, err := fs.Sub(staticFS, "static")
	if err == nil {
		mux.Handle("/static/", withSecurityHeaders(http.StripPrefix("/static/", http.FileServer(http.FS(subFS)))))
	}

	// 2. Main Page
	mux.Handle("/", withSecurityHeaders(http.HandlerFunc(s.handleIndex)))

	// 3. API Endpoints
	mux.Handle("/api/v1/health", withSecurityHeaders(http.HandlerFunc(s.handleHealth)))
	mux.Handle("/api/v1/analyze", withSecurityHeaders(http.HandlerFunc(s.handleAnalyze)))
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		// The dashboard is self-contained. Constraining executable, network, and
		// frame origins makes a future accidental third-party script or embed fail
		// closed; inline style is retained only for the current static markup.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; connect-src 'self'; img-src 'self' data:; script-src 'self'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// Codes are a stable public contract for the localized UI. Deliberately avoid
	// serializing upstream errors because a transport error can include a request
	// URL and its OPENFDA_API_KEY query parameter.
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":  code,
		"error": "request could not be processed",
	})
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", http.MethodGet)
	writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	if r.URL.Path != "/" {
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
		methodNotAllowed(w)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "healthy",
		"service":     "pv-signal-radar",
		"version":     "1.1.0",
		"cached_keys": s.cache.Len(),
	})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
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

	cacheKey := strings.ToLower(drugQuery)

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

	// Perform live analysis
	analysis, err := s.openfdaClient.AnalyzeDrug(r.Context(), drugQuery)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "analysis_unavailable")
		return
	}

	// Store in cache
	s.cache.Set(cacheKey, analysis)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Cache-Status", "MISS")
	_ = json.NewEncoder(w).Encode(analysis)
}
