package vigimed

import (
	"github.com/BMaeda84/pv-signal-radar/internal/stats"
)

// DrugMapping defines the canonical harmonization between Brazilian DCB / brand names,
// WHO-ATC classification codes, and international generic names.
type DrugMapping struct {
	CanonicalName string   `json:"canonical_name"` // e.g. "Semaglutide"
	DCBName       string   `json:"dcb_name"`       // e.g. "Semaglutida"
	ATCCode       string   `json:"atc_code"`       // e.g. "A10BJ06"
	Aliases       []string `json:"aliases"`        // e.g. ["ozempic", "wegovy", "rybelsus", "semaglutida"]
}

// BrazilReactionRecord stores the reporting counts for an adverse drug reaction in the Brazilian VigiMed dataset.
type BrazilReactionRecord struct {
	MedDRACode    int64   `json:"meddra_code"`
	ReactionPTBR  string  `json:"reaction_pt_br"`
	ReactionPTEN  string  `json:"reaction_pt_en"`
	CountA        int64   `json:"count_a"`        // Drug + Reaction in Brazil
	DrugTotal     int64   `json:"drug_total"`    // Total reports for this drug in Brazil (a + b)
	ReactionTotal int64   `json:"reaction_total"`// Total reports for this reaction across all drugs in Brazil (a + c)
}

// BrazilSignalSummary holds disproportionality metrics computed for Brazil VigiMed.
type BrazilSignalSummary struct {
	MedDRACode     int64             `json:"meddra_code"`
	ReactionPTBR   string            `json:"reaction_pt_br"`
	ReactionPTEN   string            `json:"reaction_pt_en"`
	CountA         int64             `json:"count_a"`
	DrugTotal      int64             `json:"drug_total"`
	ReactionTotal  int64             `json:"reaction_total"`
	PRR            float64           `json:"prr"`
	PRRLower95     float64           `json:"prr_lower_95"`
	PRRUpper95     float64           `json:"prr_upper_95"`
	ROR            float64           `json:"ror"`
	RORLower95     float64           `json:"ror_lower_95"`
	RORUpper95     float64           `json:"ror_upper_95"`
	ChiSquare      float64           `json:"chi_square_yates"`
	PValueApprox   float64           `json:"p_value_approx"`
	SignalLevel    stats.SignalLevel `json:"signal_level"`
	SignalScore    float64           `json:"signal_score"`
	Interpretation string            `json:"interpretation"`
}

// BrazilAnalysis represents the complete disproportionality profile in Brazil (ANVISA VigiMed).
type BrazilAnalysis struct {
	SubstanceName      string                `json:"substance_name"`
	DCBName            string                `json:"dcb_name"`
	ATCCode            string                `json:"atc_code"`
	TotalReportsBR     int64                 `json:"total_reports_br"`
	DatabaseUniverseN  int64                 `json:"database_universe_n_br"`
	ActiveSignalsCount int                   `json:"active_signals_count_br"`
	Signals            []BrazilSignalSummary `json:"signals"`
	DataOrigin         string                `json:"data_origin"`
	Disclaimer         string                `json:"disclaimer"`
}

// ComparativeSummary correlates FDA FAERS (US/Global) and ANVISA VigiMed (Brazil) results.
type ComparativeSummary struct {
	DrugNormalized       string `json:"drug_normalized"`
	ATCCode              string `json:"atc_code"`
	FDAActiveSignals     int    `json:"fda_active_signals"`
	AnvisaActiveSignals  int    `json:"anvisa_active_signals"`
	ConcordantSignals    int    `json:"concordant_signals"`
	DivergentSignals     int    `json:"divergent_signals"`
	ReportingRatioFDAvsBR float64 `json:"reporting_ratio_fda_vs_br"`
	KeyInsights          []string `json:"key_insights"`
}
