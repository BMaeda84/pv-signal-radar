package feedback

import "time"

// FlaggedStatistic represents a specific statistical metric or table row flagged by the researcher.
type FlaggedStatistic struct {
	Drug                 string `json:"drug"`
	Reaction             string `json:"reaction"`
	Jurisdiction         string `json:"jurisdiction"` // "FDA", "ANVISA", "COMPARATIVE"
	Metric               string `json:"metric"`       // "PRR", "ROR", "ChiSquare", "CountA", "Matrix"
	DisplayedValue       string `json:"displayed_value"`
	Reason               string `json:"reason"`
	VisualSnapshotBase64 string `json:"visual_snapshot_base64,omitempty"`
}

// FeedbackSubmission is the request payload submitted by the client.
type FeedbackSubmission struct {
	Email             string             `json:"email"`
	Comments          string             `json:"comments"`
	FlaggedStatistics []FlaggedStatistic `json:"flagged_statistics"`
}

// FeedbackRecord is the persisted entity including audit metadata.
type FeedbackRecord struct {
	ID                string             `json:"id"`
	Email             string             `json:"email"`
	Comments          string             `json:"comments"`
	FlaggedStatistics []FlaggedStatistic `json:"flagged_statistics"`
	Timestamp         time.Time          `json:"timestamp"`
	IPAddress         string             `json:"ip_address"`
	UserAgent         string             `json:"user_agent"`
}
