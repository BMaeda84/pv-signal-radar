// Package stats provides statistical routines for pharmacovigilance disproportionality analysis.
// It implements common disproportionality calculations (PRR, ROR, Yates' Chi-Square, 95% Confidence Intervals)
// based on 2x2 contingency tables derived from spontaneous adverse event reporting systems (e.g. FAERS, VigiBase).
package stats

import (
	"fmt"
	"math"
)

const (
	// qnorm(0.975). Keeping the full quantile, rather than the historical 1.96
	// shorthand, makes the log-Wald intervals numerically compatible with pvda
	// and R's default normal quantile.
	wald95Z = 1.959963984540054

	// ContingencyTable is retained as float64 for the public v1 JSON contract.
	// Every integer through 2^53 is exactly representable; accepting a larger
	// universe would silently merge adjacent report counts before any statistic
	// is calculated. Research inputs therefore fail closed at this boundary.
	maxExactContingencyCount int64 = 1 << 53
)

// ScreeningOutcome represents a protocol-specific review label. It deliberately
// avoids the word "signal": crossing a numerical threshold is not validation,
// confirmation, causality, or a regulatory signal-management decision.
type ScreeningOutcome string

const (
	// ScreeningMeetsProfile indicates that the configured educational Evans
	// threshold is met and that the pair should be reviewed.
	ScreeningMeetsProfile ScreeningOutcome = "MEETS_PROFILE"
	// ScreeningIntermediateReview indicates a secondary exploratory review rule.
	ScreeningIntermediateReview ScreeningOutcome = "INTERMEDIATE_REVIEW"
	// ScreeningBelowProfile indicates that neither configured review rule is met.
	ScreeningBelowProfile ScreeningOutcome = "BELOW_PROFILE"
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
	Drug                    string           `json:"drug"`
	Reaction                string           `json:"reaction"`
	Table                   ContingencyTable `json:"contingency_table"`
	PRR                     float64          `json:"prr"`
	PRRLower95              float64          `json:"prr_lower_95"`
	PRRUpper95              float64          `json:"prr_upper_95"`
	ROR                     float64          `json:"ror"`
	RORLower95              float64          `json:"ror_lower_95"`
	RORUpper95              float64          `json:"ror_upper_95"`
	ChiSquare               float64          `json:"chi_square_yates"`
	PValueApprox            float64          `json:"p_value_approx"`
	FisherExactP            float64          `json:"fisher_exact_two_sided_p"`
	FisherExactOK           bool             `json:"fisher_exact_available"`
	ScreeningOutcome        ScreeningOutcome `json:"screening_outcome"`
	ExploratoryRankingScore float64          `json:"exploratory_ranking_score"`
	Recommendation          string           `json:"interpretation"`
	MethodMetadata          MethodMetadata   `json:"method_metadata"`
}

// MethodMetadata makes the statistical choices behind a result machine-readable.
// The legacy numeric fields remain unchanged so existing v1 consumers can continue
// to deserialize the response while research clients can audit the calculation.
type MethodMetadata struct {
	PRREstimator          string                     `json:"prr_estimator"`
	PRRConfidenceInterval string                     `json:"prr_confidence_interval"`
	ROREstimator          string                     `json:"ror_estimator"`
	RORConfidenceInterval string                     `json:"ror_confidence_interval"`
	AssociationTests      []string                   `json:"association_tests"`
	ScreeningProfileID    string                     `json:"screening_profile_id"`
	ZeroCellCorrection    ZeroCellCorrectionMetadata `json:"zero_cell_correction"`
}

// ZeroCellCorrectionMetadata records whether the Haldane-Anscombe correction
// changed the cells used by the log-scale effect estimates and confidence intervals.
// Fisher's exact test always uses the uncorrected observed integer table.
type ZeroCellCorrectionMetadata struct {
	Applied         bool    `json:"applied"`
	Method          string  `json:"method"`
	AddedToEachCell float64 `json:"added_to_each_cell"`
}

// NewContingencyTable builds a 2x2 matrix given target counts and database background.
// drugReactionCount: a (Target Drug + Target Reaction)
// drugTotalCount: a + b (Target Drug total adverse event reports)
// reactionTotalCount: a + c (Target Reaction total reports across all drugs)
// databaseTotalCount: N (Total reports in FAERS universe)
func NewContingencyTable(drugReactionCount, drugTotalCount, reactionTotalCount, databaseTotalCount int64) (ContingencyTable, error) {
	// A malformed set of margins must not be silently "repaired": changing N or
	// a cell can manufacture a precise-looking disproportionality signal from an
	// upstream timeout, stale dataset, or incompatible count query.
	if drugReactionCount < 0 || drugTotalCount < 0 || reactionTotalCount < 0 || databaseTotalCount < 0 {
		return ContingencyTable{}, fmt.Errorf("contingency counts cannot be negative")
	}
	if databaseTotalCount > maxExactContingencyCount {
		return ContingencyTable{}, fmt.Errorf(
			"database universe (%d) exceeds the exact float64 count limit (%d)",
			databaseTotalCount,
			maxExactContingencyCount,
		)
	}
	if drugReactionCount > drugTotalCount {
		return ContingencyTable{}, fmt.Errorf("drug-reaction count (%d) exceeds drug total (%d)", drugReactionCount, drugTotalCount)
	}
	if drugReactionCount > reactionTotalCount {
		return ContingencyTable{}, fmt.Errorf("drug-reaction count (%d) exceeds reaction total (%d)", drugReactionCount, reactionTotalCount)
	}
	if databaseTotalCount < drugTotalCount || databaseTotalCount < reactionTotalCount {
		return ContingencyTable{}, fmt.Errorf("database universe (%d) is smaller than a marginal total", databaseTotalCount)
	}

	// d = (N - drugTotal) - (reactionTotal - a) must remain non-negative.
	// This form avoids constructing an invalid table while preserving the exact
	// marginal totals supplied by the upstream dataset.
	if reactionTotalCount-drugReactionCount > databaseTotalCount-drugTotalCount {
		return ContingencyTable{}, fmt.Errorf("contingency margins imply a negative d cell")
	}

	a := float64(drugReactionCount)
	drugTotal := float64(drugTotalCount)
	reactionTotal := float64(reactionTotalCount)
	n := float64(databaseTotalCount)

	b := drugTotal - a
	c := reactionTotal - a
	d := n - (a + b + c)

	return ContingencyTable{
		A: a,
		B: b,
		C: c,
		D: d,
		N: n,
	}, nil
}

// Calculate computes the bounded, closed-form disproportionality metrics for the
// contingency table. Fisher is opt-in through CalculateWithFisher because an
// exact tail enumeration has a materially different CPU cost.
// Mathematical rationales:
//
//  1. PRR (Proportional Reporting Ratio): Compares the proportion of the specific reaction for the drug against all other drugs.
//     PRR = (a / (a + b)) / (c / (c + d))
//     SE(ln PRR) = sqrt(1/a - 1/(a+b) + 1/c - 1/(c+d))
//
//  2. ROR (Reporting Odds Ratio): Odds of reporting the target reaction with target drug vs other drugs.
//     ROR = (a * d) / (b * c)
//     SE(ln ROR) = sqrt(1/a + 1/b + 1/c + 1/d)
//
//  3. Yates' Chi-Square: Corrects for continuity in 1 degree-of-freedom discrete 2x2 tables.
//     Chi2_Yates = (N * (max(0, |a*d - b*c| - N/2))^2) / ((a+b)*(c+d)*(a+c)*(b+d))
func (t ContingencyTable) Calculate(drug, reaction string) DisproportionalityResult {
	return t.calculate(drug, reaction, false)
}

// CalculateWithFisher additionally evaluates the probability-ordering exact
// test. FisherExactOK remains false when the pre-specified work bound is exceeded;
// callers requesting that method must fail closed or send it to batch execution.
func (t ContingencyTable) CalculateWithFisher(drug, reaction string) DisproportionalityResult {
	return t.calculate(drug, reaction, true)
}

func (t ContingencyTable) calculate(drug, reaction string, includeFisher bool) DisproportionalityResult {
	a, b, c, d, n := t.A, t.B, t.C, t.D, t.N

	res := DisproportionalityResult{
		Drug:         drug,
		Reaction:     reaction,
		Table:        t,
		PValueApprox: 1.0,
		FisherExactP: 1.0,
		MethodMetadata: MethodMetadata{
			PRREstimator:          "proportional_reporting_ratio",
			PRRConfidenceInterval: "wald_log_95",
			ROREstimator:          "reporting_odds_ratio",
			RORConfidenceInterval: "wald_log_95",
			AssociationTests:      []string{"yates_chi_square_1df"},
			ScreeningProfileID:    "evans-educational-v1",
			ZeroCellCorrection: ZeroCellCorrectionMetadata{
				Method: "none",
			},
		},
	}

	// Small-sample guard: if any cell is 0, apply Haldane-Anscombe 0.5 correction for stable log calculations
	adjA, adjB, adjC, adjD := a, b, c, d
	if a == 0 || b == 0 || c == 0 || d == 0 {
		adjA += 0.5
		adjB += 0.5
		adjC += 0.5
		adjD += 0.5
		res.MethodMetadata.ZeroCellCorrection = ZeroCellCorrectionMetadata{
			Applied:         true,
			Method:          "haldane_anscombe",
			AddedToEachCell: 0.5,
		}
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
			res.PRRLower95 = math.Exp(math.Log(res.PRR) - wald95Z*seLnPRR)
			res.PRRUpper95 = math.Exp(math.Log(res.PRR) + wald95Z*seLnPRR)
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
		res.RORLower95 = math.Exp(math.Log(res.ROR) - wald95Z*seLnROR)
		res.RORUpper95 = math.Exp(math.Log(res.ROR) + wald95Z*seLnROR)
	}

	// 3. Yates' Corrected Chi-Square
	// A direct a*d-b*c subtraction can erase a small association when both
	// products are large and nearly equal. The compensated FMA formulation
	// preserves the rounding residual of b*c before applying the correction.
	determinant := differenceOfProducts(a, d, b, c)
	numerator := math.Abs(determinant) - (n / 2.0)
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

	if includeFisher {
		// Fisher uses the observed integer table, never Haldane-Anscombe cells.
		// Its implementation rejects a support that exceeds the online work
		// budget instead of monopolizing a public request goroutine.
		if fisherP, err := t.FisherExactTwoSided(); err == nil {
			res.FisherExactP = fisherP
			res.FisherExactOK = true
			res.MethodMetadata.AssociationTests = append(res.MethodMetadata.AssociationTests, "fisher_exact_probability_ordering_two_sided")
		}
	}

	// 4. Configurable screening classification. The threshold is a prioritization
	// heuristic for exploratory review, not a regulatory or clinical conclusion.
	if a >= 3 && res.PRR >= 2.0 && res.ChiSquare >= 4.0 {
		res.ScreeningOutcome = ScreeningMeetsProfile
		res.Recommendation = "Configured screening threshold met. Scientific and clinical review are required before any safety conclusion."
	} else if a >= 3 && (res.PRR >= 1.5 || res.RORLower95 > 1.0) {
		res.ScreeningOutcome = ScreeningIntermediateReview
		res.Recommendation = "Potential reporting disproportionality detected. Monitor and review the underlying reports."
	} else {
		res.ScreeningOutcome = ScreeningBelowProfile
		res.Recommendation = "No configured screening threshold was met in this exploratory analysis."
	}

	// Legacy composite score retained for v1 ordering only. It is not a volcano
	// coordinate, p-value, q-value, calibrated risk score, or research endpoint.
	if res.PRR > 0 && res.ChiSquare >= 0 {
		res.ExploratoryRankingScore = math.Log2(res.PRR) * math.Sqrt(res.ChiSquare)
	}

	return res
}

// differenceOfProducts evaluates a*b-c*d with a compensated product residual.
// This matters at large report counts: the two rounded products can be equal
// even when the exact determinant is not. math.FMA performs each multiply-add
// with one final rounding, recovering the error discarded by the c*d product.
func differenceOfProducts(a, b, c, d float64) float64 {
	cd := c * d
	return math.FMA(a, b, -cd) + math.FMA(-c, d, cd)
}
