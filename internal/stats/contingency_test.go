package stats

import (
	"math"
	"testing"
)

// TestContingencyTable_StandardSignal verifies calculations against a known benchmark case.
// Scenario: Drug X has 50 reports of Reaction Y out of 1,000 total reports.
// Database universe has 5,000 reports of Reaction Y out of 1,000,000 total reports.
// a = 50, a+b = 1000, a+c = 5000, N = 1000000
// Expected:
// propTarget = 50 / 1000 = 0.05
// propBackground = 4950 / 999000 ~= 0.0049549
// PRR ~= 10.09
func TestContingencyTable_StandardSignal(t *testing.T) {
	table := NewContingencyTable(50, 1000, 5000, 1000000)

	if table.A != 50 {
		t.Fatalf("expected A=50, got %v", table.A)
	}
	if table.B != 950 {
		t.Fatalf("expected B=950, got %v", table.B)
	}
	if table.C != 4950 {
		t.Fatalf("expected C=4950, got %v", table.C)
	}
	if table.D != 994050 {
		t.Fatalf("expected D=994050, got %v", table.D)
	}

	result := table.Calculate("TestDrug", "TestReaction")

	// PRR should be ~ 10.09
	if math.Abs(result.PRR-10.09) > 0.1 {
		t.Errorf("expected PRR ~ 10.09, got %v", result.PRR)
	}

	// ROR should be (50 * 994050) / (950 * 4950) ~= 10.569
	if math.Abs(result.ROR-10.57) > 0.1 {
		t.Errorf("expected ROR ~ 10.57, got %v", result.ROR)
	}

	// Chi-square should be very large (> 100)
	if result.ChiSquare < 100 {
		t.Errorf("expected ChiSquare > 100, got %v", result.ChiSquare)
	}

	// Signal classification should be SignalActive
	if result.Signal != SignalActive {
		t.Errorf("expected SignalActive, got %v", result.Signal)
	}

	// Confidence intervals sanity check: Lower < Estimate < Upper
	if result.PRRLower95 >= result.PRR || result.PRR >= result.PRRUpper95 {
		t.Errorf("invalid PRR 95%% CI bounds: [%v, %v, %v]", result.PRRLower95, result.PRR, result.PRRUpper95)
	}

	if result.RORLower95 >= result.ROR || result.ROR >= result.RORUpper95 {
		t.Errorf("invalid ROR 95%% CI bounds: [%v, %v, %v]", result.RORLower95, result.ROR, result.RORUpper95)
	}
}

// TestContingencyTable_SmallSample verifies that samples with a < 3 do not trigger active signals.
func TestContingencyTable_SmallSample(t *testing.T) {
	table := NewContingencyTable(2, 10, 20, 10000)
	result := table.Calculate("RareDrug", "RareEvent")

	if result.Signal == SignalActive {
		t.Errorf("expected sample with a < 3 to not be SignalActive, got %v", result.Signal)
	}
}

// TestContingencyTable_ZeroCellCorrection verifies Haldane-Anscombe 0.5 correction on zero cells.
func TestContingencyTable_ZeroCellCorrection(t *testing.T) {
	table := NewContingencyTable(0, 100, 50, 100000)
	result := table.Calculate("DrugZero", "ReactionZero")

	if math.IsNaN(result.PRR) || math.IsInf(result.PRR, 0) {
		t.Errorf("PRR produced NaN or Inf for zero cell: %v", result.PRR)
	}
	if math.IsNaN(result.ROR) || math.IsInf(result.ROR, 0) {
		t.Errorf("ROR produced NaN or Inf for zero cell: %v", result.ROR)
	}
	if result.Signal != SignalNone {
		t.Errorf("expected SignalNone for zero count, got %v", result.Signal)
	}
}
