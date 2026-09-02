package research

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/BMaeda84/pv-signal-radar/internal/stats"
)

func TestBenjaminiHochbergMatchesRReferenceAndPreservesFamilyMembership(t *testing.T) {
	// R: p.adjust(c(0.5, 0.01, 0.04), method="BH") = 0.5, 0.03, 0.06.
	rows := []DrugEventResult{
		fisherRow("A", 0.5),
		fisherRow("B", 0.01),
		fisherRow("C", 0.04),
	}
	otherP := 0.001
	rows[0].Metrics = append(rows[0].Metrics, MetricEstimate{Method: "unrelated_test", Measure: "p", Estimate: otherP, PValue: &otherP})
	applyBenjaminiHochberg(rows)

	want := map[string]float64{"A": 0.5, "B": 0.03, "C": 0.06}
	for _, row := range rows {
		metric := row.Metrics[0]
		if metric.QValue == nil || math.Abs(*metric.QValue-want[row.EventTerm]) > 1e-15 {
			t.Fatalf("event %s q=%v, want %.17g", row.EventTerm, metric.QValue, want[row.EventTerm])
		}
	}
	if rows[0].Metrics[1].QValue != nil {
		t.Fatal("BH family must include only the declared Fisher metrics")
	}

	// Equal p-values must receive equal adjusted values irrespective of stable
	// sort order; otherwise event label ordering would affect inference.
	ties := []DrugEventResult{fisherRow("X", 0.01), fisherRow("Y", 0.04), fisherRow("Z", 0.01)}
	applyBenjaminiHochberg(ties)
	if math.Abs(*ties[0].Metrics[0].QValue-0.015) > 1e-15 || math.Abs(*ties[2].Metrics[0].QValue-0.015) > 1e-15 || math.Abs(*ties[1].Metrics[0].QValue-0.04) > 1e-15 {
		t.Fatalf("BH tie handling drifted: %#v", ties)
	}
}

func TestReconcileAggregateCountsRejectsInt64Wraparound(t *testing.T) {
	// Without checked addition, MaxInt64+MaxInt64+MaxInt64 wraps to
	// MaxInt64-2 and would appear to reconcile with the forged universe below.
	if err := reconcileAggregateCounts(
		0, math.MaxInt64, math.MaxInt64, math.MaxInt64,
		math.MaxInt64, math.MaxInt64, math.MaxInt64-2,
	); err == nil {
		t.Fatal("wrapped aggregate total must be rejected")
	}
	if err := reconcileAggregateCounts(20, 80, 40, 860, 100, 60, 1_000); err != nil {
		t.Fatalf("valid aggregate rejected: %v", err)
	}
}

func TestRequestedMetricsDiscloseCorrectionAndKeepFisherObserved(t *testing.T) {
	table, err := stats.NewContingencyTable(0, 100, 50, 100_000)
	if err != nil {
		t.Fatal(err)
	}
	metrics := requestedMetrics([]string{"prr", "ror", "fisher_exact"}, table.CalculateWithFisher("Drug", "Event"))
	if len(metrics) != 3 {
		t.Fatalf("metric count=%d, want 3", len(metrics))
	}
	for _, metric := range metrics {
		switch metric.Method {
		case "prr", "ror":
			correction := metric.Calculation.ZeroCellCorrection
			if metric.Calculation.InputCells != "haldane_anscombe_corrected" || !correction.Applied || correction.Method != "haldane_anscombe" || correction.AddedToEachCell != 0.5 {
				t.Fatalf("%s correction metadata is incomplete: %+v", metric.Method, metric.Calculation)
			}
		case "fisher_exact":
			if metric.Calculation.InputCells != "observed" || metric.Calculation.ZeroCellCorrection.Applied || metric.Calculation.ZeroCellCorrection.Method != "none" {
				t.Fatalf("Fisher must remain bound to observed integer cells: %+v", metric.Calculation)
			}
		}
	}
}

func TestResultExportsCorrectionMetadataAndNeutralizesSpreadsheetFormulas(t *testing.T) {
	unsafeDrug := "+SUM(1,1)"
	unsafeEvent := " \t=HYPERLINK(\"https://example.invalid\")"
	result := AnalysisResult{
		AnalysisID:   strings.Repeat("a", 64),
		ResultDigest: strings.Repeat("d", 64),
		RowCount:     1,
		ResultFamily: CanonicalResultFamily(),
		Request:      AnalysisRequest{Comparator: ComparatorAllOtherEligibleReports},
		Rows: []DrugEventResult{{
			DrugConceptID: unsafeDrug, EventConceptID: "meddra-pt-text:test", EventTerm: unsafeEvent, EventCategory: "unclassified_source_pt",
			Table: IntegerTable{A: 0, B: 100, C: 50, D: 99_850, N: 100_000},
			Metrics: []MetricEstimate{{
				Method: "prr", Measure: "reporting_ratio", Estimate: 9.793255563180081,
				Calculation: MetricCalculation{InputCells: "haldane_anscombe_corrected", ZeroCellCorrection: ZeroCellCorrection{Applied: true, Method: "haldane_anscombe", AddedToEachCell: 0.5}},
			}},
		}},
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	jsonText := string(encoded)
	if !strings.Contains(jsonText, `"input_cells":"haldane_anscombe_corrected"`) || !strings.Contains(jsonText, `"added_to_each_cell":0.5`) || !strings.Contains(jsonText, unsafeDrug) {
		t.Fatalf("canonical JSON lost source text or calculation metadata: %s", jsonText)
	}

	csvBytes, err := resultCSV(result)
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(csvBytes)))
	header, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	record, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	columns := make(map[string]int, len(header))
	for index, name := range header {
		columns[name] = index
	}
	if got := record[columns["drug_concept_id"]]; got != "'"+unsafeDrug {
		t.Fatalf("CSV drug formula was not neutralized: %q", got)
	}
	if got := record[columns["event_term"]]; got != "'"+unsafeEvent {
		t.Fatalf("CSV event formula after whitespace was not neutralized: %q", got)
	}
	if record[columns["result_digest"]] != result.ResultDigest || record[columns["result_family_id"]] != result.ResultFamily.FamilyID || record[columns["result_row_count"]] != "1" || record[columns["result_row_number"]] != "1" {
		t.Fatalf("CSV emitted-family metadata drifted: %#v", record)
	}
	if record[columns["zero_cell_correction_applied"]] != "true" || record[columns["zero_cell_correction_method"]] != "haldane_anscombe" || record[columns["added_to_each_cell"]] != "0.5" {
		t.Fatalf("CSV correction metadata drifted: %#v", record)
	}
	// Sanitization is an export-view transformation only. The canonical result
	// must remain lossless for hashing and scientific reproduction.
	if result.Rows[0].DrugConceptID != unsafeDrug || result.Rows[0].EventTerm != unsafeEvent {
		t.Fatal("CSV sanitization mutated the canonical analysis result")
	}
	if !strings.Contains(methodsText(result), "analysis.json remains the lossless canonical result") {
		t.Fatal("METHODS does not disclose the CSV safety transformation")
	}
}

func fisherRow(event string, p float64) DrugEventResult {
	return DrugEventResult{
		EventTerm: event,
		Metrics:   []MetricEstimate{{Method: "fisher_exact", Measure: "two_sided_probability_ordering", Estimate: p, PValue: &p}},
	}
}
