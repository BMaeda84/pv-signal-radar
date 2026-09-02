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
	"github.com/BMaeda84/pv-signal-radar/internal/feedback"
	"github.com/BMaeda84/pv-signal-radar/internal/openfda"
	"github.com/BMaeda84/pv-signal-radar/internal/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

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

	feedbackStorage := os.Getenv("FEEDBACK_STORAGE_FILE")
	if feedbackStorage == "" {
		feedbackStorage = "data/feedbacks.jsonl"
	}

	log.Printf("[pv-signal-radar] starting service on port :%s (cache capacity=%d, ttl=%v)", port, cacheCap, cacheTTL)

	// Initialize components
	fdaClient := openfda.NewClient(apiKey)
	lruCache := cache.New(cacheCap, cacheTTL)

	fbService, err := feedback.NewService(feedbackStorage)
	if err != nil {
		log.Fatalf("[pv-signal-radar] failed to initialize feedback service: %v", err)
	}

	webServer := web.NewServer(fdaClient, lruCache, fbService)

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
