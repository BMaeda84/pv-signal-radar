package research

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BMaeda84/pv-signal-radar/internal/stats"
)

func TestAnalysisIDCanonicalizesSetLikeRequestFields(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	first := testRequest(manifest.DatasetID)
	first.Methods = []string{"ror", "prr", "ror", "fisher_exact"}
	first.Subgroups = []SubgroupSelection{
		{Dimension: "sex", Values: []string{"female", "male", "female"}},
		{Dimension: "country", Values: []string{"US", "BR"}},
	}
	second := testRequest(manifest.DatasetID)
	second.Methods = []string{" FISHER_EXACT ", "PRR", "ROR"}
	second.Subgroups = []SubgroupSelection{
		{Dimension: "COUNTRY", Values: []string{"BR", "US"}},
		{Dimension: "sex", Values: []string{"male", "female"}},
	}

	firstID, err := AnalysisID(manifest, first)
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := AnalysisID(manifest, second)
	if err != nil {
		t.Fatal(err)
	}
	if firstID != secondID {
		t.Fatalf("semantically equal requests produced different IDs:\n%s\n%s", firstID, secondID)
	}
	if len(firstID) != 64 || strings.Trim(firstID, "0123456789abcdef") != "" {
		t.Fatalf("analysis ID is not lowercase SHA-256: %q", firstID)
	}

	changed := manifest
	changed.Source.Files = append([]SourceFile(nil), manifest.Source.Files...)
	changed.Source.Files[0].SHA256 = strings.Repeat("c", 64)
	changedID, err := AnalysisID(changed, first)
	if err != nil {
		t.Fatal(err)
	}
	if changedID == firstID {
		t.Fatal("source checksum change did not change analysis ID")
	}
}

func TestAnalysisIDRejectsDatasetMismatch(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	request := testRequest("faers-2024q4")
	if _, err := AnalysisID(manifest, request); err == nil {
		t.Fatal("expected mismatched dataset to be rejected")
	}
	request = testRequest(manifest.DatasetID)
	request.Period.EndDate = "2026-01-01"
	if _, err := AnalysisID(manifest, request); err == nil {
		t.Fatal("expected out-of-coverage protocol to be rejected before ID derivation")
	}
}

func TestAnalysisIDBindsApplicationCommit(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	request := testRequest(manifest.DatasetID)
	first := DevelopmentSoftwareReference()
	first.Commit = strings.Repeat("1", 40)
	second := first
	second.Commit = strings.Repeat("2", 40)
	firstID, err := AnalysisIDForSoftware(manifest, request, first)
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := AnalysisIDForSoftware(manifest, request, second)
	if err != nil {
		t.Fatal(err)
	}
	if firstID == secondID {
		t.Fatal("application commit change did not change analysis_id")
	}
}

func TestCanonicalHashesMatchGoldenContract(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	manifestHash, err := DatasetManifestHash(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if want := "0eb405a8490df97a2c560d6e8661a18ef6d436a7429a1d99b7a97ec965ed5e30"; manifestHash != want {
		t.Fatalf("manifest hash = %s, want %s; contract changed", manifestHash, want)
	}
	analysisID, err := AnalysisID(manifest, testRequest(manifest.DatasetID))
	if err != nil {
		t.Fatal(err)
	}
	if want := "7876ce80a7dfb48659dac06242b16bd9f0365a35ce17f8150ab3f51cbbdf7069"; analysisID != want {
		t.Fatalf("analysis ID = %s, want %s; contract changed", analysisID, want)
	}
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"No causality inference."})
	if err != nil {
		t.Fatal(err)
	}
	if want := "0375e5cb4effc812414ac447e0b36afee7dcef4e8d0f848f608ad051cf8a2565"; result.ResultDigest != want {
		t.Fatalf("result digest = %s, want %s; emitted-family contract changed", result.ResultDigest, want)
	}
}

func TestValidateDatasetManifestRejectsIncoherentCountPopulations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DatasetManifest)
	}{
		{"current reports exceed source rows", func(m *DatasetManifest) { m.Completeness.CurrentCaseReports = m.Completeness.SourceDEMORows + 1 }},
		{"eligible reports exceed current reports", func(m *DatasetManifest) { m.Completeness.EligibleReports = m.Completeness.CurrentCaseReports + 1 }},
		{"pairs fewer than eligible reports", func(m *DatasetManifest) { m.Completeness.DrugEventPairs = m.Completeness.EligibleReports - 1 }},
		{"missingness population is ambiguous", func(m *DatasetManifest) { m.Completeness.Fields[0].Population = "all_reports" }},
		{"missingness denominator differs from eligible reports", func(m *DatasetManifest) { m.Completeness.Fields[0].DenominatorRecords-- }},
		{"missingness percent differs from numerator", func(m *DatasetManifest) { m.Completeness.Fields[0].MissingPercent = 9 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := testManifest("faers-2025q4")
			test.mutate(&manifest)
			if err := ValidateDatasetManifest(manifest); err == nil {
				t.Fatal("expected incoherent count semantics to be rejected")
			}
		})
	}
}

func TestDatasetCompletenessRejectsLegacyAmbiguousNames(t *testing.T) {
	var completeness DatasetCompleteness
	legacy := []byte(`{"total_reports":5,"unique_cases":4,"drug_event_pairs":5}`)
	if err := decodeStrictJSON(legacy, &completeness); err == nil {
		t.Fatal("legacy total_reports/unique_cases names must not bypass the explicit population contract")
	}
}

func TestNormalizeAnalysisRequestRejectsInvalidConfigurations(t *testing.T) {
	base := testRequest("faers-2025q4")
	tests := []struct {
		name   string
		mutate func(*AnalysisRequest)
	}{
		{"unknown method", func(r *AnalysisRequest) { r.Methods = []string{"magic"} }},
		{"no methods", func(r *AnalysisRequest) { r.Methods = nil }},
		{"invalid period", func(r *AnalysisRequest) { r.Period = AnalysisPeriod{StartDate: "2025-12-31", EndDate: "2025-01-01"} }},
		{"unsafe dataset id", func(r *AnalysisRequest) { r.DatasetID = "../faers" }},
		{"unknown role", func(r *AnalysisRequest) { r.DrugRole = "causal" }},
		{"unknown comparator", func(r *AnalysisRequest) { r.Comparator = "selected_reports" }},
		{"unknown threshold profile", func(r *AnalysisRequest) { r.ThresholdProfileID = "unregistered-v1" }},
		{"empty subgroup", func(r *AnalysisRequest) { r.Subgroups = []SubgroupSelection{{Dimension: "sex"}} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := base
			test.mutate(&request)
			if _, err := NormalizeAnalysisRequest(request); err == nil {
				t.Fatal("expected invalid request to be rejected")
			}
		})
	}
}

func TestNewAnalysisResultRejectsNonFiniteAndIncoherentRows(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	request := testRequest(manifest.DatasetID)
	row := testRow()
	row.Table.N++
	if _, err := NewAnalysisResult(manifest, request, []DrugEventResult{row}, []string{"No causality inference."}); err == nil {
		t.Fatal("expected incoherent contingency table to be rejected")
	}
}

func TestNewAnalysisResultRejectsMissingOrInconsistentMetricCalculation(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	request := testRequest(manifest.DatasetID)
	corrected := MetricCalculation{
		InputCells: MetricInputHaldaneAnscombeCorrected,
		ZeroCellCorrection: ZeroCellCorrection{
			Applied: true, Method: ZeroCellCorrectionHaldaneAnscombe, AddedToEachCell: 0.5,
		},
	}
	tests := []struct {
		name        string
		method      string
		calculation MetricCalculation
	}{
		{"missing metadata", "ror", MetricCalculation{}},
		{"unknown input cells", "ror", MetricCalculation{InputCells: "smoothed_unknown", ZeroCellCorrection: ZeroCellCorrection{Method: ZeroCellCorrectionNone}}},
		{"observed with applied correction", "ror", MetricCalculation{InputCells: MetricInputObserved, ZeroCellCorrection: ZeroCellCorrection{Applied: true, Method: ZeroCellCorrectionHaldaneAnscombe, AddedToEachCell: 0.5}}},
		{"corrected with wrong increment", "ror", MetricCalculation{InputCells: MetricInputHaldaneAnscombeCorrected, ZeroCellCorrection: ZeroCellCorrection{Applied: true, Method: ZeroCellCorrectionHaldaneAnscombe, AddedToEachCell: 0.25}}},
		{"Fisher corrected", "fisher_exact", corrected},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := testRow()
			row.Metrics = append([]MetricEstimate(nil), row.Metrics...)
			row.Metrics[0].Method = test.method
			row.Metrics[0].Calculation = test.calculation
			if _, err := NewAnalysisResult(manifest, request, []DrugEventResult{row}, []string{"No causality inference."}); err == nil {
				t.Fatal("expected invalid metric calculation metadata to be rejected")
			}
		})
	}
}

func TestValidateAnalysisResultRejectsProtocolOutputMismatch(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	request := testRequest(manifest.DatasetID)
	valid, err := NewAnalysisResult(manifest, request, []DrugEventResult{testRow()}, []string{"No causality inference."})
	if err != nil {
		t.Fatalf("construct valid control result: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*AnalysisResult)
	}{
		{
			name: "missing requested method",
			mutate: func(result *AnalysisResult) {
				result.Rows[0].Metrics = result.Rows[0].Metrics[:len(result.Rows[0].Metrics)-1]
			},
		},
		{
			name: "noncanonical metric order",
			mutate: func(result *AnalysisResult) {
				result.Rows[0].Metrics[0], result.Rows[0].Metrics[1] = result.Rows[0].Metrics[1], result.Rows[0].Metrics[0]
			},
		},
		{
			name: "row drug differs from protocol",
			mutate: func(result *AnalysisResult) {
				result.Rows[0].DrugConceptID = "rxnorm:other"
			},
		},
		{
			name: "missing profile outcome",
			mutate: func(result *AnalysisResult) {
				result.Rows[0].ReviewFlags = nil
			},
		},
		{
			name: "wrong profile id",
			mutate: func(result *AnalysisResult) {
				result.Rows[0].ReviewFlags[0].ProfileID = ThresholdProfileNone
			},
		},
		{
			name: "wrong Evans outcome",
			mutate: func(result *AnalysisResult) {
				result.Rows[0].ReviewFlags[0].Outcome = "meets_profile"
			},
		},
		{
			name: "stale Evans reason",
			mutate: func(result *AnalysisResult) {
				result.Rows[0].ReviewFlags[0].Reason = "Configured threshold was not met."
			},
		},
		{
			name: "tampered PRR estimate",
			mutate: func(result *AnalysisResult) {
				metricByMethod(&result.Rows[0], "prr").Estimate *= 1.01
			},
		},
		{
			name: "tampered ROR interval",
			mutate: func(result *AnalysisResult) {
				upper := *metricByMethod(&result.Rows[0], "ror").Upper95 * 1.01
				metricByMethod(&result.Rows[0], "ror").Upper95 = &upper
			},
		},
		{
			name: "tampered Fisher p",
			mutate: func(result *AnalysisResult) {
				metric := metricByMethod(&result.Rows[0], "fisher_exact")
				p := *metric.PValue * 1.01
				metric.PValue = &p
				metric.Estimate = p
				metric.QValue = &p
			},
		},
		{
			name: "tampered Fisher q",
			mutate: func(result *AnalysisResult) {
				metric := metricByMethod(&result.Rows[0], "fisher_exact")
				q := *metric.QValue * 1.01
				metric.QValue = &q
			},
		},
		{
			name: "table changed without recalculation",
			mutate: func(result *AnalysisResult) {
				// Preserve N while moving one report between the target-drug cells;
				// the table remains structurally valid but all effects are stale.
				result.Rows[0].Table.A++
				result.Rows[0].Table.B--
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := cloneAnalysisResult(t, valid)
			test.mutate(&mutated)
			if err := ValidateAnalysisResult(manifest, mutated); err == nil {
				t.Fatal("protocol/output mismatch passed result validation")
			}
		})
	}
}

func TestValidateAnalysisResultBindsEmittedRowsCountDigestAndOrder(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	valid, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), testRowsTwo(), []string{"No causality inference."})
	if err != nil {
		t.Fatalf("construct two-row result: %v", err)
	}
	if valid.RowCount != 2 || !sha256Pattern.MatchString(valid.ResultDigest) || valid.ResultFamily != CanonicalResultFamily() {
		t.Fatalf("result integrity metadata is incomplete: %#v", valid)
	}

	tests := []struct {
		name   string
		mutate func(*AnalysisResult)
	}{
		{"truncated rows", func(result *AnalysisResult) { result.Rows = result.Rows[:1] }},
		{"reordered rows", func(result *AnalysisResult) { result.Rows[0], result.Rows[1] = result.Rows[1], result.Rows[0] }},
		{"adulterated row text", func(result *AnalysisResult) { result.Rows[0].EventCategory = "changed_category" }},
		{"stale row count", func(result *AnalysisResult) { result.RowCount++ }},
		{"changed family", func(result *AnalysisResult) { result.ResultFamily.FamilyID = "different_family" }},
		{"invalid digest", func(result *AnalysisResult) { result.ResultDigest = strings.Repeat("0", 64) }},
		{"adulterated embedded manifest", func(result *AnalysisResult) { result.DatasetManifest.Title = "Different snapshot" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := cloneAnalysisResult(t, valid)
			test.mutate(&mutated)
			if err := ValidateAnalysisResult(manifest, mutated); err == nil {
				t.Fatal("mutated emitted-family metadata passed validation")
			}
		})
	}

	// Recomputing a digest over a reordered sequence cannot bypass the separate
	// canonical-order contract enforced at the FileStore trust boundary.
	reordered := cloneAnalysisResult(t, valid)
	reordered.Rows[0], reordered.Rows[1] = reordered.Rows[1], reordered.Rows[0]
	reordered.ResultDigest, err = CanonicalResultDigest(reordered.ResultFamily, reordered.Rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateAnalysisResult(manifest, reordered); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("reordered family with a recomputed digest was accepted: %v", err)
	}
}

func TestValidateAnalysisResultBindsRequestToManifestCoverage(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	valid, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"No causality inference."})
	if err != nil {
		t.Fatalf("construct valid control result: %v", err)
	}

	datasetMismatch := cloneAnalysisResult(t, valid)
	datasetMismatch.Request.DatasetID = "faers-other"
	// Before the explicit manifest binding, AnalysisIDForSoftware returned an
	// error for this mismatch and validation discarded that error, letting the
	// existing 64-hex ID pass through FileStore.
	if err := ValidateAnalysisResult(manifest, datasetMismatch); err == nil || !strings.Contains(err.Error(), "does not match manifest") {
		t.Fatalf("request/manifest dataset mismatch was not rejected: %v", err)
	}

	outOfCoverage := cloneAnalysisResult(t, valid)
	outOfCoverage.Request.Period.EndDate = "2026-01-01"
	if err := ValidateAnalysisResult(manifest, outOfCoverage); err == nil || !strings.Contains(err.Error(), "ends after dataset coverage") {
		t.Fatalf("out-of-coverage result was not rejected: %v", err)
	}
}

func TestValidateAnalysisResultRejectsFlagsWhenProfileIsNone(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	request := testRequest(manifest.DatasetID)
	request.ThresholdProfileID = ThresholdProfileNone
	row := testRow()
	row.ReviewFlags = nil
	result, err := NewAnalysisResult(manifest, request, []DrugEventResult{row}, []string{"No causality inference."})
	if err != nil {
		t.Fatalf("construct no-profile control result: %v", err)
	}
	result.Rows[0].ReviewFlags = []ReviewFlag{{ProfileID: ThresholdProfileEvansEducational, Outcome: "below_profile", Reason: "stale flag"}}
	if err := ValidateAnalysisResult(manifest, result); err == nil {
		t.Fatal("threshold_profile_id none accepted a review flag")
	}
}

func TestValidateAnalysisResultRejectsUniverseDifferentFromManifestEligibleReports(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	manifest.Completeness.EligibleReports = 999
	manifest.Completeness.Fields = nil
	if _, err := NewAnalysisResult(
		manifest,
		testRequest(manifest.DatasetID),
		[]DrugEventResult{testRow()},
		[]string{"No causality inference."},
	); err == nil || !strings.Contains(err.Error(), "does not match manifest eligible_reports") {
		t.Fatalf("result with a denominator different from the manifest was accepted: %v", err)
	}
}

func testManifest(id string) DatasetManifest {
	return DatasetManifest{
		SchemaVersion: SchemaVersion,
		DatasetID:     id,
		Title:         "FAERS 2025 Q4 frozen research snapshot",
		Description:   "Fixture with complete provenance.",
		Source: DatasetSource{
			Name:        "FDA Adverse Event Reporting System",
			Publisher:   "U.S. Food and Drug Administration",
			LandingPage: "https://www.fda.gov/faers",
			RetrievedAt: "2026-01-15T12:00:00Z",
			License:     "United States government work; source terms apply",
			Files: []SourceFile{{
				URL: "https://download.example.test/faers-2025q4.zip", SHA256: strings.Repeat("a", 64), Bytes: 123456,
			}},
		},
		Coverage: DatasetCoverage{StartDate: "2004-01-01", EndDate: "2025-12-31", Geography: "global", Release: "2025Q4"},
		Processing: DatasetProcessing{
			PipelineVersion: "1.0.0", SourceCommit: strings.Repeat("1", 40), DeduplicationPolicy: "latest CASEID version; unique PRIMARYID drug-event pairs", CountUnit: "unique report-level drug-event pair", DrugRolePolicy: "role retained; request selects role", Exclusions: []string{"invalid case versions"},
		},
		Vocabularies: []VocabularyReference{{Name: "MedDRA", Version: "28.1", Scope: "preferred terms", License: "source distribution terms apply"}},
		Artifacts:    []DatasetArtifact{{Name: "read-only aggregate", Path: "aggregate/faers.sqlite", MediaType: "application/vnd.sqlite3", SHA256: strings.Repeat("b", 64), Bytes: 654321}},
		Completeness: DatasetCompleteness{
			SourceDEMORows: 1200, CurrentCaseReports: 1100, EligibleReports: 1000, DrugEventPairs: 1500,
			Fields: []FieldCompleteness{{Field: "sex", Population: "eligible_reports", DenominatorRecords: 1000, MissingRecords: 100, MissingPercent: 10}},
		},
		Limitations: []string{"Spontaneous reports do not establish causality or incidence."},
	}
}

func testRequest(datasetID string) AnalysisRequest {
	return AnalysisRequest{
		SchemaVersion: SchemaVersion, DatasetID: datasetID, DrugConceptID: "rxnorm:123", DrugRole: "primary_suspect", EventScope: EventScopeAllRecordedSourcePTs, Comparator: ComparatorAllOtherEligibleReports, Period: AnalysisPeriod{StartDate: "2020-01-01", EndDate: "2025-12-31"}, Methods: []string{"prr", "ror", "fisher_exact"}, ThresholdProfileID: ThresholdProfileEvansEducational,
	}
}

func testRow() DrugEventResult {
	table, err := stats.NewContingencyTable(10, 100, 110, 1000)
	if err != nil {
		panic(err)
	}
	calculated := table.CalculateWithFisher("rxnorm:123", "Example event")
	if !calculated.FisherExactOK {
		panic("small fixture unexpectedly exceeded Fisher work bound")
	}
	row := DrugEventResult{
		DrugConceptID: "rxnorm:123", EventConceptID: "meddra:10000001", EventTerm: "Example event", EventCategory: "clinical_reaction",
		Table: IntegerTable{A: 10, B: 90, C: 100, D: 800, N: 1000},
		// Normalized requests store methods as a set in lexical order; keeping the
		// same order in every row makes the persisted JSON byte-deterministic.
		Metrics:     requestedMetrics([]string{"fisher_exact", "prr", "ror"}, calculated),
		ReviewFlags: requestedReviewFlags(ThresholdProfileEvansEducational, calculated),
	}
	rows := []DrugEventResult{row}
	applyBenjaminiHochberg(rows)
	return rows[0]
}

func testRowsTwo() []DrugEventResult {
	first := testRow()
	second := testRow()
	second.EventConceptID = "meddra:10000002"
	second.EventTerm = "Second example event"
	return []DrugEventResult{first, second}
}

func cloneAnalysisResult(t *testing.T, result AnalysisResult) AnalysisResult {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var clone AnalysisResult
	if err := json.Unmarshal(encoded, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func metricByMethod(row *DrugEventResult, method string) *MetricEstimate {
	for index := range row.Metrics {
		if row.Metrics[index].Method == method {
			return &row.Metrics[index]
		}
	}
	panic("fixture does not contain metric " + method)
}
