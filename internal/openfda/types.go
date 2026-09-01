package openfda

// CountResult represents a single item in an OpenFDA count aggregation response.
type CountResult struct {
	Term  string `json:"term"`
	Count int64  `json:"count"`
}

// CountResponse represents OpenFDA's response for count queries (e.g. count=patient.reaction.reactionmeddrapt.exact).
type CountResponse struct {
	Meta struct {
		Disclaimer  string `json:"disclaimer"`
		Terms       string `json:"terms"`
		License     string `json:"license"`
		LastUpdated string `json:"last_updated"`
		Results     struct {
			Skip  int `json:"skip"`
			Limit int `json:"limit"`
			Total int `json:"total"`
		} `json:"results"`
	} `json:"meta"`
	Results []CountResult `json:"results"`
}

// TotalResponse represents OpenFDA's metadata response to extract the total number of reports.
type TotalResponse struct {
	Meta struct {
		Results struct {
			Total int64 `json:"total"`
		} `json:"results"`
	} `json:"meta"`
}

// DrugEventAnalysis represents the comprehensive analysis payload for a requested drug.
type DrugEventAnalysis struct {
	QueryDrug          string          `json:"query_drug"`
	NormalizedDrug     string          `json:"normalized_drug"`
	DrugTotalReports   int64           `json:"drug_total_reports"`
	DatabaseUniverseN  int64           `json:"database_universe_n"`
	ActiveSignalsCount int             `json:"active_signals_count"`
	TotalReactions     int             `json:"total_reactions_analyzed"`
	Signals            []SignalSummary `json:"signals"`
	Timestamp          string          `json:"timestamp"`
	Disclaimer         string          `json:"disclaimer"`
}

// SignalSummary holds both the raw OpenFDA counts and calculated disproportionality metrics.
type SignalSummary struct {
	Reaction        string  `json:"reaction"`
	CountA          int64   `json:"count_a"`           // Drug + Reaction
	DrugTotal       int64   `json:"drug_total"`         // Drug total (a + b)
	ReactionTotal   int64   `json:"reaction_total"`     // Reaction total (a + c)
	PRR             float64 `json:"prr"`
	PRRLower95      float64 `json:"prr_lower_95"`
	PRRUpper95      float64 `json:"prr_upper_95"`
	ROR             float64 `json:"ror"`
	RORLower95      float64 `json:"ror_lower_95"`
	RORUpper95      float64 `json:"ror_upper_95"`
	ChiSquare       float64 `json:"chi_square_yates"`
	PValueApprox    float64 `json:"p_value_approx"`
	SignalLevel     string  `json:"signal_level"`
	SignalScore     float64 `json:"signal_score"`
	Interpretation  string  `json:"interpretation"`
}
