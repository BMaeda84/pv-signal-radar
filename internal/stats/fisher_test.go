package stats

import (
	"math"
	"testing"
)

func TestFisherExactTwoSidedKnownTable(t *testing.T) {
	table := ContingencyTable{A: 1, B: 9, C: 11, D: 3, N: 24}
	got, err := table.FisherExactTwoSided()
	if err != nil {
		t.Fatalf("FisherExactTwoSided returned error: %v", err)
	}
	const want = 0.0027594561852200836
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("Fisher p = %.16g, want %.16g", got, want)
	}
}

func TestFisherExactTwoSidedMatchesRGoldensAcrossScales(t *testing.T) {
	// R 4.6.1 stats::fisher.test(..., alternative="two.sided") goldens.
	// Relative comparison is required for very small p-values: an absolute
	// 1e-12 tolerance would incorrectly accept zero for the two large examples.
	tests := []struct {
		name       string
		table      ContingencyTable
		want       float64
		relativeTo float64
	}{
		{"moderate", ContingencyTable{A: 123, B: 456, C: 789, D: 1_011, N: 2_379}, 2.0292871311477705e-23, 2e-10},
		{"large", ContingencyTable{A: 10_000, B: 20_000, C: 30_000, D: 40_000, N: 100_000}, 6.926875748277738e-177, 2e-9},
		// At N=1e9 the lgamma normalization accumulates about 0.8 ppm relative
		// drift against R. The declared 2 ppm bound is intentionally far tighter
		// than a decision threshold; larger/unbounded workloads remain batch-only.
		{"billion-report sparse margins", ContingencyTable{A: 5, B: 995, C: 1_000, D: 999_998_000, N: 1_000_000_000}, 8.367807727781138e-18, 2e-6},
		{"sparse zero", ContingencyTable{A: 17, B: 0, C: 3, D: 99, N: 119}, 6.9933612949115424e-18, 2e-10},
		{"zero target pair", ContingencyTable{A: 0, B: 100, C: 50, D: 99_850, N: 100_000}, 1, 2e-14},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.table.FisherExactTwoSided()
			if err != nil {
				t.Fatal(err)
			}
			assertRelativeClose(t, "Fisher two-sided p", got, test.want, test.relativeTo)
		})
	}
}

func TestFisherExactTwoSidedSparseZeroAndSymmetry(t *testing.T) {
	base := ContingencyTable{A: 0, B: 5, C: 10, D: 0, N: 15}
	want, err := base.FisherExactTwoSided()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []ContingencyTable{
		{A: 5, B: 0, C: 0, D: 10, N: 15},
		{A: 10, B: 0, C: 0, D: 5, N: 15},
	} {
		got, err := table.FisherExactTwoSided()
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(got-want) > 1e-14 {
			t.Fatalf("symmetry violated: got %.16g, want %.16g", got, want)
		}
	}
}

func TestFisherExactTwoSidedIsBitStableAcrossAllTableSymmetries(t *testing.T) {
	base := ContingencyTable{A: 20, B: 80, C: 40, D: 860, N: 1_000}
	want, err := base.FisherExactTwoSided()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []ContingencyTable{
		{A: 40, B: 860, C: 20, D: 80, N: 1_000},
		{A: 80, B: 20, C: 860, D: 40, N: 1_000},
		{A: 860, B: 40, C: 80, D: 20, N: 1_000},
		{A: 20, B: 40, C: 80, D: 860, N: 1_000},
		{A: 80, B: 860, C: 20, D: 40, N: 1_000},
		{A: 40, B: 20, C: 860, D: 80, N: 1_000},
		{A: 860, B: 80, C: 40, D: 20, N: 1_000},
	} {
		got, err := table.FisherExactTwoSided()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("symmetry layout %+v produced %.17g, want bit-identical %.17g", table, got, want)
		}
	}
}

func TestFisherExactTwoSidedLargeBalancedTable(t *testing.T) {
	// A mode table with a four-billion-report universe must return immediately;
	// the implementation must not enumerate the full support.
	table := ContingencyTable{A: 1_000_000_000, B: 1_000_000_000, C: 1_000_000_000, D: 1_000_000_000, N: 4_000_000_000}
	got, err := table.FisherExactTwoSided()
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("balanced table p = %g, want 1", got)
	}
}

func TestFisherExactTwoSidedLargeNearModeUsesSafeComplement(t *testing.T) {
	// The included tails contain about two million support points, while the
	// only more-probable central table has small mass. The bounded online path
	// may safely compute 1-central here because the resulting p-value is near 1.
	table := ContingencyTable{A: 999_999, B: 1_000_001, C: 1_000_001, D: 999_999, N: 4_000_000}
	got, err := table.FisherExactTwoSided()
	if err != nil {
		t.Fatal(err)
	}
	assertRelativeClose(t, "large near-mode Fisher p", got, 0.99920211558880045, 2e-9)
}

func TestFisherExactTwoSidedMatchesBruteForceSmallTables(t *testing.T) {
	for a := int64(0); a <= 6; a++ {
		for b := int64(0); b <= 6; b++ {
			for c := int64(0); c <= 6; c++ {
				for d := int64(0); d <= 6; d++ {
					n := a + b + c + d
					table := ContingencyTable{A: float64(a), B: float64(b), C: float64(c), D: float64(d), N: float64(n)}
					got, err := table.FisherExactTwoSided()
					if err != nil {
						t.Fatalf("table %d,%d,%d,%d: %v", a, b, c, d, err)
					}
					want := bruteFisher(a, b, c, d)
					if math.Abs(got-want) > 1e-10 {
						t.Fatalf("table %d,%d,%d,%d: got %.16g, want %.16g", a, b, c, d, got, want)
					}
				}
			}
		}
	}
}

func TestFisherExactTwoSidedRejectsMalformedTable(t *testing.T) {
	for _, table := range []ContingencyTable{
		{A: 0.5, B: 1, C: 1, D: 1, N: 3.5},
		{A: -1, B: 1, C: 1, D: 1, N: 2},
		{A: 1, B: 1, C: 1, D: 1, N: 5},
	} {
		if _, err := table.FisherExactTwoSided(); err == nil {
			t.Fatalf("expected malformed table %+v to be rejected", table)
		}
	}
}

func TestFisherExactTwoSidedRejectsUnboundedOnlineWork(t *testing.T) {
	// Symmetric dense margins with an observation far from the mode require
	// hundreds of thousands of terms. The online implementation must reject
	// that workload instead of tying up a public request goroutine.
	table := ContingencyTable{A: 800_000, B: 1_200_000, C: 1_200_000, D: 800_000, N: 4_000_000}
	if _, err := table.FisherExactTwoSided(); err == nil {
		t.Fatal("expected exact test to reject support above its online work bound")
	}
	result := table.CalculateWithFisher("Drug", "Event")
	if result.FisherExactOK {
		t.Fatal("bounded calculation must not advertise an unavailable exact result")
	}
}

func TestCalculateReportsCorrectionAndExactMethod(t *testing.T) {
	table, err := NewContingencyTable(0, 100, 50, 100_000)
	if err != nil {
		t.Fatal(err)
	}
	result := table.CalculateWithFisher("Drug", "Event")
	if !result.MethodMetadata.ZeroCellCorrection.Applied || result.MethodMetadata.ZeroCellCorrection.Method != "haldane_anscombe" {
		t.Fatalf("missing zero-cell metadata: %+v", result.MethodMetadata.ZeroCellCorrection)
	}
	if !result.FisherExactOK || result.FisherExactP < 0 || result.FisherExactP > 1 {
		t.Fatalf("invalid Fisher result: available=%v p=%g", result.FisherExactOK, result.FisherExactP)
	}
	if result.MethodMetadata.ScreeningProfileID != "evans-educational-v1" {
		t.Fatalf("unexpected screening profile %q", result.MethodMetadata.ScreeningProfileID)
	}
}

func bruteFisher(a, b, c, d int64) float64 {
	row1, col1, n := a+b, a+c, a+b+c+d
	if n == 0 {
		return 1
	}
	probability := func(x int64) float64 {
		return bruteCombination(col1, x) * bruteCombination(n-col1, row1-x) / bruteCombination(n, row1)
	}
	observed := probability(a)
	lo := maxInt64(0, row1-(n-col1))
	hi := minInt64(row1, col1)
	total := 0.0
	for x := lo; x <= hi; x++ {
		p := probability(x)
		if p <= observed*(1+1e-10) {
			total += p
		}
	}
	return total
}

func bruteCombination(n, k int64) float64 {
	if k < 0 || k > n {
		return 0
	}
	if k > n-k {
		k = n - k
	}
	result := 1.0
	for i := int64(1); i <= k; i++ {
		result *= float64(n-k+i) / float64(i)
	}
	return result
}
