// Package stats provides statistical routines for pharmacovigilance disproportionality analysis.
// It implements standard regulatory data mining algorithms (PRR, ROR, Yates' Chi-Square, 95% Confidence Intervals)
// based on 2x2 contingency tables derived from spontaneous adverse event reporting systems (e.g. FAERS, VigiBase).
package stats

import (
	"math"
)

// SignalLevel represents the regulatory classification of a drug-event association.
type SignalLevel string

const (
	// SignalActive indicates a strong statistical signal meeting Evans et al. / EMA criteria (a >= 3, PRR >= 2.0, Chi2 >= 4.0).
	SignalActive SignalLevel = "ACTIVE_SIGNAL"
	// SignalPotential indicates moderate disproportionality (a >= 3, lower ROR CI > 1.0 or PRR >= 1.5).
	SignalPotential SignalLevel = "POTENTIAL_SIGNAL"
	// SignalNone indicates background noise or insufficient disproportionality.
	SignalNone SignalLevel = "NO_SIGNAL"
)

// ContingencyTable represents the classic 2x2 matrix for pharmacovigilance signal detection:
//
//	                   Target Reaction (E)    Other Reactions (~E)    Total
//	Target Drug (D)             a                      b              a + b
//	Other Drugs (~D)            c                      d              c + d
//	Total                     a + c                  b + d              N
type ContingencyTable struct {
	A float64 `json:"a"` // Reports with Target Drug and Target Reaction
	B float64 `json:"b"` // Reports with Target Drug and Other Reactions
	C float64 `json:"c"` // Reports with Other Drugs and Target Reaction
	D float64 `json:"d"` // Reports with Other Drugs and Other Reactions
	N float64 `json:"n"` // Total reports in database universe
}

// DisproportionalityResult holds the computed metrics for a drug-event pair.
type DisproportionalityResult struct {
	Drug          string           `json:"drug"`
	Reaction      string           `json:"reaction"`
	Table         ContingencyTable `json:"contingency_table"`
	PRR           float64          `json:"prr"`
	PRRLower95    float64          `json:"prr_lower_95"`
	PRRUpper95    float64          `json:"prr_upper_95"`
	ROR           float64          `json:"ror"`
	RORLower95    float64          `json:"ror_lower_95"`
	RORUpper95    float64          `json:"ror_upper_95"`
	ChiSquare     float64          `json:"chi_square_yates"`
	PValueApprox  float64          `json:"p_value_approx"`
	Signal        SignalLevel      `json:"signal_level"`
	SignalScore   float64          `json:"signal_score"`
	Recommendation string          `json:"interpretation"`
}

// NewContingencyTable builds a 2x2 matrix given target counts and database background.
// drugReactionCount: a (Target Drug + Target Reaction)
// drugTotalCount: a + b (Target Drug total adverse event reports)
// reactionTotalCount: a + c (Target Reaction total reports across all drugs)
// databaseTotalCount: N (Total reports in FAERS universe)
func NewContingencyTable(drugReactionCount, drugTotalCount, reactionTotalCount, databaseTotalCount int64) ContingencyTable {
	a := float64(drugReactionCount)
	drugTotal := float64(drugTotalCount)
	reactionTotal := float64(reactionTotalCount)
	n := float64(databaseTotalCount)

	// Ensure boundary consistency
	if drugTotal < a {
		drugTotal = a
	}
	if reactionTotal < a {
		reactionTotal = a
	}
	if n < (drugTotal + reactionTotal - a) {
		n = drugTotal + reactionTotal - a
	}

	b := drugTotal - a
	c := reactionTotal - a
	d := n - (a + b + c)
	if d < 0 {
		d = 0
	}

	return ContingencyTable{
		A: a,
		B: b,
		C: c,
		D: d,
		N: n,
	}
}

// Calculate computes all disproportionality metrics for the contingency table.
// Mathematical rationales:
// 1. PRR (Proportional Reporting Ratio): Compares the proportion of the specific reaction for the drug against all other drugs.
//    PRR = (a / (a + b)) / (c / (c + d))
//    SE(ln PRR) = sqrt(1/a - 1/(a+b) + 1/c - 1/(c+d))
//
// 2. ROR (Reporting Odds Ratio): Odds of reporting the target reaction with target drug vs other drugs.
//    ROR = (a * d) / (b * c)
//    SE(ln ROR) = sqrt(1/a + 1/b + 1/c + 1/d)
//
// 3. Yates' Chi-Square: Corrects for continuity in 1 degree-of-freedom discrete 2x2 tables.
//    Chi2_Yates = (N * (max(0, |a*d - b*c| - N/2))^2) / ((a+b)*(c+d)*(a+c)*(b+d))
func (t ContingencyTable) Calculate(drug, reaction string) DisproportionalityResult {
	a, b, c, d, n := t.A, t.B, t.C, t.D, t.N

	res := DisproportionalityResult{
		Drug:     drug,
		Reaction: reaction,
		Table:    t,
	}

	// Small-sample guard: if any cell is 0, apply Haldane-Anscombe 0.5 correction for stable log calculations
	adjA, adjB, adjC, adjD := a, b, c, d
	if a == 0 || b == 0 || c == 0 || d == 0 {
		adjA += 0.5
		adjB += 0.5
		adjC += 0.5
		adjD += 0.5
	}

	// 1. PRR Calculation
	propTarget := adjA / (adjA + adjB)
	propBackground := adjC / (adjC + adjD)
	if propBackground > 0 {
		res.PRR = propTarget / propBackground
		// Standard error of ln(PRR)
		varianceLnPRR := (1.0 / adjA) - (1.0 / (adjA + adjB)) + (1.0 / adjC) - (1.0 / (adjC + adjD))
		if varianceLnPRR > 0 {
			seLnPRR := math.Sqrt(varianceLnPRR)
			res.PRRLower95 = math.Exp(math.Log(res.PRR) - 1.95996*seLnPRR)
			res.PRRUpper95 = math.Exp(math.Log(res.PRR) + 1.95996*seLnPRR)
		} else {
			res.PRRLower95 = res.PRR
			res.PRRUpper95 = res.PRR
		}
	}

	// 2. ROR Calculation
	if (adjB * adjC) > 0 {
		res.ROR = (adjA * adjD) / (adjB * adjC)
		varianceLnROR := (1.0 / adjA) + (1.0 / adjB) + (1.0 / adjC) + (1.0 / adjD)
		seLnROR := math.Sqrt(varianceLnROR)
		res.RORLower95 = math.Exp(math.Log(res.ROR) - 1.95996*seLnROR)
		res.RORUpper95 = math.Exp(math.Log(res.ROR) + 1.95996*seLnROR)
	}

	// 3. Yates' Corrected Chi-Square
	numerator := math.Abs(a*d-b*c) - (n / 2.0)
	if numerator < 0 {
		numerator = 0
	}
	denominator := (a + b) * (c + d) * (a + c) * (b + d)
	if denominator > 0 {
		res.ChiSquare = (n * math.Pow(numerator, 2)) / denominator
		// Chi-square with 1 degree of freedom p-value approximation via complementary error function
		if res.ChiSquare > 0 {
			res.PValueApprox = math.Erfc(math.Sqrt(res.ChiSquare / 2.0))
		} else {
			res.PValueApprox = 1.0
		}
	}

	// 4. Regulatory Signal Classification (Evans et al. / EMA Guidelines)
	// Active Signal threshold: a >= 3, PRR >= 2.0, and Yates Chi2 >= 4.0 (corresponds to p < 0.05)
	if a >= 3 && res.PRR >= 2.0 && res.ChiSquare >= 4.0 {
		res.Signal = SignalActive
		res.Recommendation = "Statistically significant disproportionate reporting detected (Evans/EMA criteria satisfied). Clinical review recommended."
	} else if a >= 3 && (res.PRR >= 1.5 || res.RORLower95 > 1.0) {
		res.Signal = SignalPotential
		res.Recommendation = "Moderate reporting disproportionality. Lower bound of 95% CI > 1.0. Monitor for emergence."
	} else {
		res.Signal = SignalNone
		res.Recommendation = "No statistically significant signal detected over baseline reporting."
	}

	// Composite Signal Score: log2(PRR) * sqrt(ChiSquare) for volcano plotting and ranking
	if res.PRR > 0 && res.ChiSquare >= 0 {
		res.SignalScore = math.Log2(res.PRR) * math.Sqrt(res.ChiSquare)
	}

	return res
}
