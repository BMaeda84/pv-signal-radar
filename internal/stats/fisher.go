package stats

import (
	"fmt"
	"math"
)

const (
	// R's fisher.test includes tables within a 1+1e-7 probability ratio of
	// the observed table. Matching that tolerance prevents floating-point noise
	// from breaking row/column symmetry at a probability-ordering boundary.
	fisherRelativeTolerance  = 1e-7
	maxFisherEnumeratedTerms = 100_000
)

// FisherExactTwoSided returns the probability-ordering two-sided Fisher exact
// p-value: the sum of all fixed-margin tables whose probability is no greater
// than the observed table. This definition matches the common R fisher.test
// convention; the definition is stated explicitly because alternative two-sided
// constructions (for example doubled one-sided tails) need not agree.
//
// Counts must be finite, non-negative integers and N must equal A+B+C+D. The
// implementation works in log space and chooses the shorter of the included
// tails or excluded central interval, which keeps sparse tables tractable even
// when the database universe is large.
func (t ContingencyTable) FisherExactTwoSided() (float64, error) {
	a, b, c, d, err := fisherIntegerCells(t)
	if err != nil {
		return 0, err
	}
	// Row swaps, column swaps, and transposition cannot change a two-sided
	// Fisher result. Canonicalizing the eight equivalent layouts also makes the
	// floating-point evaluation bit-stable across those symmetries; otherwise
	// algebraically equivalent lgamma expressions can round differently.
	a, b, c, d = canonicalFisherCells(a, b, c, d)

	row1, ok := checkedAddInt64(a, b)
	if !ok {
		return 0, fmt.Errorf("row total overflows int64")
	}
	col1, ok := checkedAddInt64(a, c)
	if !ok {
		return 0, fmt.Errorf("column total overflows int64")
	}
	n, ok := checkedAddInt64(row1, c)
	if !ok {
		return 0, fmt.Errorf("table total overflows int64")
	}
	n, ok = checkedAddInt64(n, d)
	if !ok {
		return 0, fmt.Errorf("table total overflows int64")
	}
	if n == 0 {
		return 1, nil
	}

	lo := maxInt64(0, row1-(n-col1))
	hi := minInt64(row1, col1)
	observedLogP := hypergeometricLogPMF(a, row1, col1, n)
	threshold := observedLogP + math.Log1p(fisherRelativeTolerance)

	// The hypergeometric PMF is unimodal. If the observed probability is at
	// the mode, every possible table belongs to the two-sided sum.
	mode := int64(math.Floor(((float64(row1) + 1) * (float64(col1) + 1)) / (float64(n) + 2)))
	if mode < lo {
		mode = lo
	}
	if mode > hi {
		mode = hi
	}
	if hypergeometricLogPMF(mode, row1, col1, n) <= threshold {
		return 1, nil
	}

	// Find the contiguous interval whose tables are strictly more probable than
	// the observed one. Everything outside it contributes to the exact p-value.
	left := firstMoreProbable(lo, mode, threshold, row1, col1, n)
	right := lastMoreProbable(mode, hi, threshold, row1, col1, n)
	outsideTerms := (left - lo) + (hi - right)
	insideTerms := right - left + 1
	var p float64
	if outsideTerms <= maxFisherEnumeratedTerms {
		// Always sum the included tails when they fit the work budget. Computing
		// p as 1-central loses every significant digit for very small p-values;
		// this occurred even on tables with only a few hundred support points.
		logTotal := math.Inf(-1)
		for x := lo; x < left; x++ {
			logTotal = logAddExp(logTotal, hypergeometricLogPMF(x, row1, col1, n))
		}
		for x := right + 1; x <= hi; x++ {
			logTotal = logAddExp(logTotal, hypergeometricLogPMF(x, row1, col1, n))
		}
		p = math.Exp(logTotal)
	} else if insideTerms <= maxFisherEnumeratedTerms {
		// A central complement is useful only near the mode, where p is large.
		// If the complement exceeds one half, subtraction may hide a small tail;
		// fail closed and defer that table to the batch implementation instead.
		central := 0.0
		compensation := 0.0
		for x := left; x <= right; x++ {
			value := math.Exp(hypergeometricLogPMF(x, row1, col1, n))
			y := value - compensation
			next := central + y
			compensation = (next - central) - y
			central = next
		}
		if central > 0.5 {
			return 0, fmt.Errorf("fisher exact included tails require %d terms; online limit is %d", outsideTerms, maxFisherEnumeratedTerms)
		}
		p = 1 - central
	} else {
		return 0, fmt.Errorf("fisher exact support requires at least %d terms; online limit is %d", minInt64(outsideTerms, insideTerms), maxFisherEnumeratedTerms)
	}

	if p < 0 && p > -1e-12 {
		p = 0
	}
	if p > 1 && p < 1+1e-12 {
		p = 1
	}
	if math.IsNaN(p) || p < 0 || p > 1 {
		return 0, fmt.Errorf("fisher exact calculation produced invalid probability %g", p)
	}
	return p, nil
}

func fisherIntegerCells(t ContingencyTable) (int64, int64, int64, int64, error) {
	values := []struct {
		name  string
		value float64
	}{
		{"a", t.A}, {"b", t.B}, {"c", t.C}, {"d", t.D},
	}
	converted := make([]int64, len(values))
	for i, item := range values {
		if math.IsNaN(item.value) || math.IsInf(item.value, 0) || item.value < 0 || math.Trunc(item.value) != item.value {
			return 0, 0, 0, 0, fmt.Errorf("cell %s must be a finite non-negative integer", item.name)
		}
		// Values at or above 2^63 cannot be represented as int64; the explicit
		// power-of-two bound avoids float64 rounding around math.MaxInt64.
		if item.value >= math.Exp2(63) {
			return 0, 0, 0, 0, fmt.Errorf("cell %s exceeds int64", item.name)
		}
		converted[i] = int64(item.value)
	}

	sum := t.A + t.B + t.C + t.D
	if math.IsNaN(t.N) || math.IsInf(t.N, 0) || t.N < 0 || math.Trunc(t.N) != t.N || t.N != sum {
		return 0, 0, 0, 0, fmt.Errorf("n must be an integer equal to a+b+c+d")
	}
	return converted[0], converted[1], converted[2], converted[3], nil
}

func canonicalFisherCells(a, b, c, d int64) (int64, int64, int64, int64) {
	candidates := [][4]int64{
		{a, b, c, d}, // observed layout
		{c, d, a, b}, // swap rows
		{b, a, d, c}, // swap columns
		{d, c, b, a}, // swap rows and columns
		{a, c, b, d}, // transpose and the same swaps
		{b, d, a, c},
		{c, a, d, b},
		{d, b, c, a},
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		for index := range candidate {
			if candidate[index] < best[index] {
				best = candidate
				break
			}
			if candidate[index] > best[index] {
				break
			}
		}
	}
	return best[0], best[1], best[2], best[3]
}

func hypergeometricLogPMF(x, row1, col1, n int64) float64 {
	return logCombination(col1, x) + logCombination(n-col1, row1-x) - logCombination(n, row1)
}

func logCombination(n, k int64) float64 {
	if k < 0 || k > n {
		return math.Inf(-1)
	}
	a, _ := math.Lgamma(float64(n) + 1)
	b, _ := math.Lgamma(float64(k) + 1)
	c, _ := math.Lgamma(float64(n-k) + 1)
	return a - b - c
}

func firstMoreProbable(lo, hi int64, threshold float64, row1, col1, n int64) int64 {
	for lo < hi {
		mid := lo + (hi-lo)/2
		if hypergeometricLogPMF(mid, row1, col1, n) > threshold {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

func lastMoreProbable(lo, hi int64, threshold float64, row1, col1, n int64) int64 {
	for lo < hi {
		mid := lo + (hi-lo+1)/2
		if hypergeometricLogPMF(mid, row1, col1, n) > threshold {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func logAddExp(a, b float64) float64 {
	if math.IsInf(a, -1) {
		return b
	}
	if math.IsInf(b, -1) {
		return a
	}
	if a < b {
		a, b = b, a
	}
	return a + math.Log1p(math.Exp(b-a))
}

func checkedAddInt64(a, b int64) (int64, bool) {
	if b > math.MaxInt64-a {
		return 0, false
	}
	return a + b, true
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
