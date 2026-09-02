package vigimed

import (
	"testing"
)

func TestResolveDrug_PortugueseAndBrandNames(t *testing.T) {
	cases := []struct {
		query        string
		expectedATC  string
		expectedName string
	}{
		{"Semaglutida", "A10BJ06", "Semaglutide"},
		{"ozempic", "A10BJ06", "Semaglutide"},
		{"glifage", "A10BA02", "Metformin"},
		{"metformina", "A10BA02", "Metformin"},
		{"Dipirona", "N02BB02", "Dipyrone"},
		{"Novalgina", "N02BB02", "Dipyrone"},
		{"forxiga", "A10BK01", "Dapagliflozin"},
		{"Crestor", "C10AA07", "Rosuvastatin"},
		{"Keytruda", "L01FF02", "Pembrolizumab"},
	}

	for _, c := range cases {
		mapping, found := ResolveDrug(c.query)
		if !found {
			t.Errorf("failed to resolve known drug query: %s", c.query)
			continue
		}
		if mapping.ATCCode != c.expectedATC {
			t.Errorf("for query %s, expected ATC %s, got %s", c.query, c.expectedATC, mapping.ATCCode)
		}
		if mapping.CanonicalName != c.expectedName {
			t.Errorf("for query %s, expected canonical name %s, got %s", c.query, c.expectedName, mapping.CanonicalName)
		}
	}
}

func TestGetBrazilAnalysis_Dipirona(t *testing.T) {
	analysis, err := GetBrazilAnalysis("Dipirona")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis.ActiveSignalsCount == 0 {
		t.Errorf("expected active signals for Dipirona in Brazil, got 0")
	}

	foundAgranulocytosis := false
	for _, sig := range analysis.Signals {
		if sig.ReactionPTEN == "AGRANULOCYTOSIS" {
			foundAgranulocytosis = true
			if sig.PRR < 2.0 {
				t.Errorf("expected PRR >= 2.0 for Agranulocytosis + Dipirona, got %v", sig.PRR)
			}
		}
	}

	if !foundAgranulocytosis {
		t.Errorf("expected to find AGRANULOCYTOSIS in Dipirona signals")
	}
}
