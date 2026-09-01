package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/BMaeda84/pv-signal-radar/internal/cache"
	"github.com/BMaeda84/pv-signal-radar/internal/openfda"
)

//go:embed static/*
var staticFS embed.FS

// Server holds web handlers, openFDA client, and in-memory cache.
type Server struct {
	openfdaClient *openfda.Client
	cache         *cache.LRUCache
}

// NewServer initializes the HTTP router and services.
func NewServer(fdaClient *openfda.Client, cache *cache.LRUCache) *Server {
	return &Server{
		openfdaClient: fdaClient,
		cache:         cache,
	}
}

// Routes sets up all HTTP routes on the given mux.
func (s *Server) Routes(mux *http.ServeMux) {
	// 1. Static assets
	subFS, err := fs.Sub(staticFS, "static")
	if err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(subFS))))
	}

	// 2. Main Page
	mux.HandleFunc("/", s.handleIndex)

	// 3. API Endpoints
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/analyze", s.handleAnalyze)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "healthy",
		"service":     "pv-signal-radar",
		"version":     "1.0.0",
		"cached_keys": s.cache.Len(),
	})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	drugQuery := strings.TrimSpace(r.URL.Query().Get("drug"))
	if drugQuery == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "query parameter 'drug' is required (e.g. ?drug=Semaglutide)",
		})
		return
	}

	cacheKey := strings.ToLower(drugQuery)

	// Check cache
	if cachedVal, found := s.cache.Get(cacheKey); found {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache-Status", "HIT")
		_ = json.NewEncoder(w).Encode(cachedVal)
		return
	}

	// Perform live analysis
	analysis, err := s.openfdaClient.AnalyzeDrug(r.Context(), drugQuery)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to analyze openFDA records: " + err.Error(),
		})
		return
	}

	// Store in cache
	s.cache.Set(cacheKey, analysis)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache-Status", "MISS")
	_ = json.NewEncoder(w).Encode(analysis)
}
