package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/BMaeda84/pv-signal-radar/internal/cache"
	"github.com/BMaeda84/pv-signal-radar/internal/openfda"
	"github.com/BMaeda84/pv-signal-radar/internal/research"
	"github.com/BMaeda84/pv-signal-radar/internal/version"
	"github.com/BMaeda84/pv-signal-radar/internal/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		log.Fatal("[pv-signal-radar] PORT must be an integer from 1 through 65535")
	}
	port = strconv.Itoa(portNumber)

	apiKey := os.Getenv("OPENFDA_API_KEY")

	cacheCap := 500
	if cStr := os.Getenv("CACHE_CAPACITY"); cStr != "" {
		if cInt, err := strconv.Atoi(cStr); err == nil && cInt > 0 {
			cacheCap = cInt
		}
	}

	cacheTTL := 24 * time.Hour
	if ttlStr := os.Getenv("CACHE_TTL_HOURS"); ttlStr != "" {
		if ttlInt, err := strconv.Atoi(ttlStr); err == nil && ttlInt > 0 {
			cacheTTL = time.Duration(ttlInt) * time.Hour
		}
	}

	log.Printf("[pv-signal-radar] starting service on port :%s (cache capacity=%d, ttl=%v)", port, cacheCap, cacheTTL)

	// Initialize components
	fdaClient := openfda.NewClient(apiKey)
	lruCache := cache.New(cacheCap, cacheTTL)

	// Research mode is opt-in and fail-closed. A deployment without a registered,
	// integrity-checked manifest keeps the live teaching surface available but
	// cannot present a mutable source as a reproducible dataset. Registration is
	// not a claim of scientific validation.
	var researchServices web.ResearchServices
	if manifestDir := os.Getenv("RESEARCH_MANIFEST_DIR"); manifestDir != "" {
		applicationCommit, err := version.ResearchRevision(os.Getenv("PV_RADAR_APPLICATION_COMMIT"))
		if err != nil {
			log.Fatalf("[pv-signal-radar] research mode requires an authoritative clean application commit: %v", err)
		}
		software := research.SoftwareReference{Name: "pv-signal-radar", Version: version.Current, Commit: applicationCommit}
		if err := research.ValidateSoftwareReference(software); err != nil {
			log.Fatalf("[pv-signal-radar] research mode requires a traceable application revision: %v", err)
		}
		registry, err := research.LoadRegistry(manifestDir)
		if err != nil {
			log.Fatalf("[pv-signal-radar] invalid research manifest registry: %v", err)
		}
		analysisDir := os.Getenv("RESEARCH_ANALYSIS_DIR")
		if analysisDir == "" {
			analysisDir = "data/research-analyses"
		}
		store, err := research.NewFileStore(analysisDir)
		if err != nil {
			log.Fatalf("[pv-signal-radar] failed to initialize research result store: %v", err)
		}
		var engine *research.SQLiteEngine
		if sqlitePath := os.Getenv("RESEARCH_SQLITE_PATH"); sqlitePath != "" {
			datasetID := os.Getenv("RESEARCH_SQLITE_DATASET_ID")
			manifests := registry.List()
			if datasetID == "" && len(manifests) == 1 {
				datasetID = manifests[0].DatasetID
			}
			if datasetID == "" {
				log.Fatal("[pv-signal-radar] RESEARCH_SQLITE_DATASET_ID is required when the registry contains multiple datasets")
			}
			manifest, err := registry.Require(datasetID)
			if err != nil {
				log.Fatalf("[pv-signal-radar] SQLite dataset is not registered: %v", err)
			}
			engine, err = research.OpenSQLiteEngine(context.Background(), sqlitePath, manifest, software)
			if err != nil {
				log.Fatalf("[pv-signal-radar] invalid read-only SQLite aggregate: %v", err)
			}
			defer engine.Close()
		}
		allowMaterialization := false
		if raw := os.Getenv("RESEARCH_ALLOW_ONLINE_MATERIALIZATION"); raw != "" {
			allowMaterialization, err = strconv.ParseBool(raw)
			if err != nil {
				log.Fatal("[pv-signal-radar] RESEARCH_ALLOW_ONLINE_MATERIALIZATION must be true or false")
			}
		}
		researchServices = web.ResearchServices{
			Registry: registry, Store: store, Engine: engine, Software: software,
			AllowMaterialization: allowMaterialization,
		}
		// #nosec G706 -- ValidateSoftwareReference above restricts applicationCommit to lowercase hexadecimal Git syntax.
		log.Printf("[pv-signal-radar] research mode enabled with %d registered, integrity-checked dataset(s) at application commit %s (online materialization=%t)", registry.Len(), applicationCommit, allowMaterialization)
	} else {
		log.Printf("[pv-signal-radar] research mode disabled: RESEARCH_MANIFEST_DIR is not configured")
	}

	// Feedback intake previously persisted email/IP/user-agent in local JSONL.
	// Until retention and privacy governance exist, the service deliberately
	// exposes no PII collection path and sends contributors to GitHub instead.
	webServer := web.NewServer(fdaClient, lruCache, researchServices)

	mux := http.NewServeMux()
	webServer.Routes(mux)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[pv-signal-radar] server error: %v", err)
		}
	}()

	log.Printf("[pv-signal-radar] HTTP server listening on http://localhost:%s", port)

	<-stopChan
	log.Println("[pv-signal-radar] shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[pv-signal-radar] shutdown error: %v", err)
	}

	fmt.Println("[pv-signal-radar] stopped")
}
