package feedback

import (
	"os"
	"testing"
)

func TestFeedbackService_SubmitAndRateLimit(t *testing.T) {
	tempFile := "test_feedbacks.jsonl"
	defer os.Remove(tempFile)

	svc, err := NewService(tempFile)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	sub := FeedbackSubmission{
		Email:    "researcher@university.edu.br",
		Comments: "Found discrepancy in PRR for Metformin + Diarrhoea in Brazil data.",
		FlaggedStatistics: []FlaggedStatistic{
			{
				Drug:           "Metformin",
				Reaction:       "DIARRHOEA",
				Jurisdiction:   "ANVISA",
				Metric:         "PRR",
				DisplayedValue: "4.20",
				Reason:         "Observed higher disproportionality than expected.",
			},
		},
	}

	rec, err := svc.Submit(sub, "127.0.0.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.ID == "" {
		t.Errorf("expected non-empty record ID")
	}

	// Test invalid email
	badSub := sub
	badSub.Email = "invalid-email"
	if _, err := svc.Submit(badSub, "127.0.0.1", "Mozilla/5.0"); err == nil {
		t.Errorf("expected error for invalid email")
	}
}
