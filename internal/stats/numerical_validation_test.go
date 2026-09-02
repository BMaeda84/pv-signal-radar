package stats

import (
	"math"
	"testing"
)

// These values were independently generated with pvda 0.0.4 for PRR/ROR and
// their log-Wald intervals, and with R's chisq.test(correct=TRUE) for Yates'
// statistic. They are numerical contracts, not merely broad sanity bounds.
func TestDisproportionalityMetricsMatchRReference(t *testing.T) {
	table, err := NewContingencyTable(50, 1_000, 5_000, 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	got := table.Calculate("Drug", "Event")

	assertRelativeClose(t, "PRR", got.PRR, 10.090909090909092, 2e-14)
	assertRelativeClose(t, "PRR lower 95", got.PRRLower95, 7.690971775220735, 2e-14)
	assertRelativeClose(t, "PRR upper 95", got.PRRUpper95, 13.239737351405022, 2e-14)
	assertRelativeClose(t, "ROR", got.ROR, 10.569377990430622, 2e-14)
	assertRelativeClose(t, "ROR lower 95", got.RORLower95, 7.942368465883525, 2e-14)
	assertRelativeClose(t, "ROR upper 95", got.RORUpper95, 14.065294450195502, 2e-14)
	assertRelativeClose(t, "Yates chi-square", got.ChiSquare, 398.43863964466982, 2e-14)
	assertRelativeClose(t, "Yates p-value", got.PValueApprox, 1.2045499525976615e-88, 2e-13)
}

func TestHaldaneAnscombeMetricsMatchRReference(t *testing.T) {
	// The zero a cell forces +0.5 on all four cells. This can yield an estimate
	// above one even with zero observed target pairs, so the correction metadata
	// and the original a count remain essential for interpretation.
	table, err := NewContingencyTable(0, 100, 50, 100_000)
	if err != nil {
		t.Fatal(err)
	}
	got := table.Calculate("Drug", "Event")

	assertRelativeClose(t, "corrected PRR", got.PRR, 9.793255563180081, 2e-14)
	assertRelativeClose(t, "corrected PRR lower 95", got.PRRLower95, 0.6083777879352199, 2e-14)
	assertRelativeClose(t, "corrected PRR upper 95", got.PRRUpper95, 157.64522707388826, 2e-14)
	assertRelativeClose(t, "corrected ROR", got.ROR, 9.837003103295404, 2e-14)
	assertRelativeClose(t, "corrected ROR lower 95", got.RORLower95, 0.6027792775366385, 2e-14)
	assertRelativeClose(t, "corrected ROR upper 95", got.RORUpper95, 160.5341020509148, 2e-14)
	if !got.MethodMetadata.ZeroCellCorrection.Applied || got.MethodMetadata.ZeroCellCorrection.AddedToEachCell != 0.5 {
		t.Fatalf("zero-cell correction was not disclosed: %+v", got.MethodMetadata.ZeroCellCorrection)
	}
}

func TestEffectEstimatesAreInvariantToRowScaling(t *testing.T) {
	base := ContingencyTable{A: 20, B: 80, C: 40, D: 860, N: 1_000}.Calculate("Drug", "Event")
	scaled := ContingencyTable{A: 2_000, B: 8_000, C: 4_000, D: 86_000, N: 100_000}.Calculate("Drug", "Event")

	assertRelativeClose(t, "scaled PRR", scaled.PRR, base.PRR, 2e-14)
	assertRelativeClose(t, "scaled ROR", scaled.ROR, base.ROR, 2e-14)
	if !(scaled.PRRLower95 > base.PRRLower95 && scaled.PRRUpper95 < base.PRRUpper95) {
		t.Fatalf("larger counts should narrow the PRR interval: base=[%g,%g] scaled=[%g,%g]", base.PRRLower95, base.PRRUpper95, scaled.PRRLower95, scaled.PRRUpper95)
	}
}

func TestRowSwapReciprocatesEffectEstimatesAndPreservesAssociationTests(t *testing.T) {
	base := ContingencyTable{A: 20, B: 80, C: 40, D: 860, N: 1_000}.CalculateWithFisher("Drug", "Event")
	swapped := ContingencyTable{A: 40, B: 860, C: 20, D: 80, N: 1_000}.CalculateWithFisher("Other", "Event")

	assertRelativeClose(t, "reciprocal PRR", swapped.PRR, 1/base.PRR, 2e-14)
	assertRelativeClose(t, "reciprocal ROR", swapped.ROR, 1/base.ROR, 2e-14)
	assertRelativeClose(t, "Yates row symmetry", swapped.ChiSquare, base.ChiSquare, 2e-14)
	assertRelativeClose(t, "Fisher row symmetry", swapped.FisherExactP, base.FisherExactP, 2e-12)
	assertRelativeClose(t, "reciprocal PRR lower", swapped.PRRLower95, 1/base.PRRUpper95, 2e-14)
	assertRelativeClose(t, "reciprocal PRR upper", swapped.PRRUpper95, 1/base.PRRLower95, 2e-14)
}

func TestNewContingencyTableEnforcesExactFloatCountBoundary(t *testing.T) {
	if _, err := NewContingencyTable(1, 1, 1, maxExactContingencyCount); err != nil {
		t.Fatalf("2^53 is exactly representable and should be accepted: %v", err)
	}
	if _, err := NewContingencyTable(1, 1, 1, maxExactContingencyCount+1); err == nil {
		t.Fatal("a universe above 2^53 must be rejected before float64 merges adjacent counts")
	}
}

func TestDifferenceOfProductsRetainsNearCancellation(t *testing.T) {
	// Each product is about 2^102, where ordinary float multiplication has an
	// ulp around 2^50. Their exact difference is nevertheless -1.
	m := float64(int64(1) << 51)
	if naive := (m+1)*(m-1) - m*m; naive != 0 {
		t.Fatalf("test precondition changed: naive determinant=%g, want rounded zero", naive)
	}
	if got := differenceOfProducts(m+1, m-1, m, m); got != -1 {
		t.Fatalf("compensated determinant=%g, want -1", got)
	}
}

func assertRelativeClose(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("%s is non-finite: %g", name, got)
	}
	scale := math.Max(math.Abs(want), math.SmallestNonzeroFloat64)
	if relative := math.Abs(got-want) / scale; relative > tolerance {
		t.Fatalf("%s=%0.17g, want %0.17g (relative error %.3g > %.3g)", name, got, want, relative, tolerance)
	}
}
