package feedback

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Service manages feedback intake, rate limiting, and persistence.
type Service struct {
	mu           sync.Mutex
	storageFile  string
	rateLimitMap map[string][]time.Time
}

// NewService creates a feedback service storing to the specified JSONL file path.
func NewService(storagePath string) (*Service, error) {
	if storagePath == "" {
		storagePath = "data/feedbacks.jsonl"
	}

	dir := filepath.Dir(storagePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// Fallback to temporary directory if restricted
		storagePath = filepath.Join(os.TempDir(), "pv_feedbacks.jsonl")
	}

	return &Service{
		storageFile:  storagePath,
		rateLimitMap: make(map[string][]time.Time),
	}, nil
}

// Submit processes and saves a feedback submission.
func (s *Service) Submit(sub FeedbackSubmission, clientIP, userAgent string) (*FeedbackRecord, error) {
	email := strings.TrimSpace(sub.Email)
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("invalid email address format")
	}

	if strings.TrimSpace(sub.Comments) == "" && len(sub.FlaggedStatistics) == 0 {
		return nil, fmt.Errorf("please provide comments or at least one flagged statistic")
	}

	// Rate limiting: max 5 submissions per IP in 10 minutes
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-10 * time.Minute)

	var validTimestamps []time.Time
	for _, t := range s.rateLimitMap[clientIP] {
		if t.After(cutoff) {
			validTimestamps = append(validTimestamps, t)
		}
	}

	if len(validTimestamps) >= 5 {
		return nil, fmt.Errorf("rate limit exceeded: please wait a few minutes before submitting additional feedback")
	}

	validTimestamps = append(validTimestamps, now)
	s.rateLimitMap[clientIP] = validTimestamps

	// Generate record ID
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	recordID := hex.EncodeToString(bytes)

	record := FeedbackRecord{
		ID:                recordID,
		Email:             email,
		Comments:          strings.TrimSpace(sub.Comments),
		FlaggedStatistics: sub.FlaggedStatistics,
		Timestamp:         now,
		IPAddress:         clientIP,
		UserAgent:         userAgent,
	}

	// Append to JSONL file
	file, err := os.OpenFile(s.storageFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage file: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize record: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write record to disk: %w", err)
	}

	return &record, nil
}
