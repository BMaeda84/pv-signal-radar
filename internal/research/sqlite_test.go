package research

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteEngineAnalyzesValidatedAggregateAndAddsFDR(t *testing.T) {
	path, manifest := buildSQLiteFixture(t)
	engine, err := OpenSQLiteEngine(context.Background(), path, manifest)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	defer engine.Close()

	request := AnalysisRequest{
		SchemaVersion:      SchemaVersion,
		DatasetID:          manifest.DatasetID,
		DrugConceptID:      "faers-prod_ai:SEMAGLUTIDE",
		DrugRole:           "primary_suspect",
		EventScope:         EventScopeAllRecordedSourcePTs,
		Comparator:         ComparatorAllOtherEligibleReports,
		Methods:            []string{"prr", "ror", "fisher_exact"},
		ThresholdProfileID: ThresholdProfileEvansEducational,
	}
	record, err := engine.Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("analyze aggregate: %v", err)
	}
	if len(record.Result.Rows) != 2 {
		t.Fatalf("expected two events, got %d", len(record.Result.Rows))
	}
	for _, row := range record.Result.Rows {
		if len(row.Metrics) != 3 {
			t.Fatalf("expected PRR, ROR, and Fisher metrics for %s, got %#v", row.EventTerm, row.Metrics)
		}
		for _, metric := range row.Metrics {
			if metric.Method == "fisher_exact" && (metric.PValue == nil || metric.QValue == nil || *metric.QValue < *metric.PValue || *metric.QValue > 1) {
				t.Fatalf("invalid Benjamini-Hochberg result for %s: %#v", row.EventTerm, metric)
			}
		}
		if row.EventTerm == "NAUSEA" {
			if row.Table != (IntegerTable{A: 20, B: 80, C: 40, D: 860, N: 1_000}) {
				t.Fatalf("all_other_eligible_reports cells drifted: %+v", row.Table)
			}
			for _, metric := range row.Metrics {
				switch metric.Method {
				case "prr":
					if math.Abs(metric.Estimate-4.5) > 1e-14 {
						t.Fatalf("NAUSEA PRR=%g, want 4.5", metric.Estimate)
					}
				case "ror":
					if math.Abs(metric.Estimate-5.375) > 1e-14 {
						t.Fatalf("NAUSEA ROR=%g, want 5.375", metric.Estimate)
					}
				}
			}
		}
	}
	if len(record.Files) != 2 || !strings.Contains(string(record.Files[0].Data), record.Result.AnalysisID) {
		t.Fatalf("reproducible export files are incomplete: %#v", record.Files)
	}
}

func TestSQLiteEngineRejectsHashMismatchAndBatchOnlyMethods(t *testing.T) {
	path, manifest := buildSQLiteFixture(t)
	bad := manifest
	bad.Artifacts = append([]DatasetArtifact(nil), manifest.Artifacts...)
	bad.Artifacts[0].SHA256 = strings.Repeat("0", 64)
	if _, err := OpenSQLiteEngine(context.Background(), path, bad); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	badUniverse := manifest
	badUniverse.Completeness.EligibleReports = 999
	badUniverse.Completeness.Fields = nil
	if _, err := OpenSQLiteEngine(context.Background(), path, badUniverse); err == nil || !strings.Contains(err.Error(), "does not match manifest eligible_reports") {
		t.Fatalf("expected manifest/SQLite universe mismatch, got %v", err)
	}

	engine, err := OpenSQLiteEngine(context.Background(), path, manifest)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	defer engine.Close()
	for _, methods := range [][]string{{"bcpnn_ic"}, {"gps_ebgm"}, {"prr", "gps_ebgm"}} {
		request := AnalysisRequest{
			SchemaVersion: SchemaVersion, DatasetID: manifest.DatasetID,
			DrugConceptID: "faers-prod_ai:SEMAGLUTIDE", DrugRole: "primary_suspect",
			EventScope: EventScopeAllRecordedSourcePTs, Comparator: ComparatorAllOtherEligibleReports,
			Methods: methods, ThresholdProfileID: ThresholdProfileNone,
		}
		if _, err := engine.Analyze(context.Background(), request); !errors.Is(err, ErrBatchMethodRequired) {
			t.Fatalf("methods %v must fail closed as batch-only, got %v", methods, err)
		}
	}
}

func TestMaterializeSQLiteValidatesTSVAndRemovesPartialOutput(t *testing.T) {
	manifest := testManifest("faers-materialize-test")
	dedup := manifest.Processing.DeduplicationPolicy
	header := strings.Join(aggregateColumns, "\t") + "\n"
	validRow := strings.Join([]string{
		manifest.DatasetID, "SEMAGLUTIDE", "PROD_AI", "primary_suspect", "NAUSEA",
		"20", "80", "40", "860", "100", "60", "1000",
		ComparatorAllOtherEligibleReports, EventScopeAllRecordedSourcePTs, dedup,
	}, "\t") + "\n"
	root := t.TempDir()
	tsv := filepath.Join(root, "aggregate.tsv")
	if err := os.WriteFile(tsv, []byte(header+validRow), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "aggregate.sqlite")
	count, err := MaterializeSQLite(context.Background(), tsv, output, manifest)
	if err != nil || count != 1 {
		t.Fatalf("materialize valid aggregate: count=%d err=%v", count, err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("materialized database missing: %v", err)
	}

	invalidTSV := filepath.Join(root, "invalid.tsv")
	invalidRow := strings.Replace(validRow, "\t100\t60\t1000\t", "\t999\t60\t1000\t", 1)
	if err := os.WriteFile(invalidTSV, []byte(header+invalidRow), 0o600); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(root, "partial.sqlite")
	if _, err := MaterializeSQLite(context.Background(), invalidTSV, partial, manifest); err == nil {
		t.Fatal("expected invalid marginals to be rejected")
	}
	if _, err := os.Stat(partial); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial database must be removed, stat err=%v", err)
	}

	// A row can be internally coherent while describing a different report
	// universe from the source manifest. That contradiction must fail before a
	// runtime artifact is published.
	mismatchedUniverseTSV := filepath.Join(root, "mismatched-universe.tsv")
	mismatchedUniverseRow := strings.Join([]string{
		manifest.DatasetID, "SEMAGLUTIDE", "PROD_AI", "primary_suspect", "NAUSEA",
		"20", "80", "40", "859", "100", "60", "999",
		ComparatorAllOtherEligibleReports, EventScopeAllRecordedSourcePTs, dedup,
	}, "\t") + "\n"
	if err := os.WriteFile(mismatchedUniverseTSV, []byte(header+mismatchedUniverseRow), 0o600); err != nil {
		t.Fatal(err)
	}
	mismatchedUniverseOutput := filepath.Join(root, "mismatched-universe.sqlite")
	if _, err := MaterializeSQLite(context.Background(), mismatchedUniverseTSV, mismatchedUniverseOutput, manifest); err == nil || !strings.Contains(err.Error(), "does not match manifest eligible_reports") {
		t.Fatalf("expected manifest/TSV universe mismatch, got %v", err)
	}
	if _, err := os.Stat(mismatchedUniverseOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("universe-mismatched aggregate left a partial database: %v", err)
	}

	// The Go statistics contract stores v1 cells as float64. Although SQLite
	// can hold the following int64 universe, 2^53+1 cannot be represented
	// exactly and must be rejected before a serving artifact is published.
	tooLargeTSV := filepath.Join(root, "too-large.tsv")
	tooLargeRow := strings.Join([]string{
		manifest.DatasetID, "SEMAGLUTIDE", "PROD_AI", "primary_suspect", "RARE EVENT",
		"1", "0", "0", "9007199254740992", "1", "1", "9007199254740993",
		ComparatorAllOtherEligibleReports, EventScopeAllRecordedSourcePTs, dedup,
	}, "\t") + "\n"
	if err := os.WriteFile(tooLargeTSV, []byte(header+tooLargeRow), 0o600); err != nil {
		t.Fatal(err)
	}
	tooLargeOutput := filepath.Join(root, "too-large.sqlite")
	tooLargeManifest := manifest
	tooLargeManifest.Completeness = DatasetCompleteness{
		SourceDEMORows:     9007199254740993,
		CurrentCaseReports: 9007199254740993,
		EligibleReports:    9007199254740993,
		DrugEventPairs:     9007199254740993,
	}
	if _, err := MaterializeSQLite(context.Background(), tooLargeTSV, tooLargeOutput, tooLargeManifest); err == nil || !strings.Contains(err.Error(), "exact float64 count limit") {
		t.Fatalf("expected non-representable universe rejection, got %v", err)
	}
	if _, err := os.Stat(tooLargeOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-representable aggregate left a partial database: %v", err)
	}
}

func buildSQLiteFixture(t *testing.T) (string, DatasetManifest) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aggregate.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("create SQLite fixture: %v", err)
	}
	schema := `
CREATE TABLE disproportionality_cells (
  dataset_id TEXT NOT NULL,
  drug_text TEXT NOT NULL,
  drug_text_source TEXT NOT NULL,
  drug_role TEXT NOT NULL,
  event_pt TEXT NOT NULL,
  a INTEGER NOT NULL CHECK (a >= 0),
  b INTEGER NOT NULL CHECK (b >= 0),
  c INTEGER NOT NULL CHECK (c >= 0),
  d INTEGER NOT NULL CHECK (d >= 0),
  drug_reports INTEGER NOT NULL,
  event_reports INTEGER NOT NULL,
  universe_reports INTEGER NOT NULL,
  comparator TEXT NOT NULL,
  event_scope TEXT NOT NULL,
  deduplication_policy TEXT NOT NULL,
  PRIMARY KEY (dataset_id, drug_text, drug_text_source, drug_role, event_pt)
);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	manifest := testManifest("faers-2025q4-sqlite-test")
	dedup := manifest.Processing.DeduplicationPolicy
	insert := "INSERT INTO disproportionality_cells VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	rows := [][]any{
		{manifest.DatasetID, "SEMAGLUTIDE", "PROD_AI", "primary_suspect", "NAUSEA", 20, 80, 40, 860, 100, 60, 1000, ComparatorAllOtherEligibleReports, EventScopeAllRecordedSourcePTs, dedup},
		{manifest.DatasetID, "SEMAGLUTIDE", "PROD_AI", "primary_suspect", "PANCREATITIS", 5, 95, 5, 895, 100, 10, 1000, ComparatorAllOtherEligibleReports, EventScopeAllRecordedSourcePTs, dedup},
	}
	for _, row := range rows {
		if _, err := db.Exec(insert, row...); err != nil {
			t.Fatalf("insert fixture row: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}
	hash, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}
	manifest.Artifacts = []DatasetArtifact{{
		Name: "read-only aggregate", Path: "aggregate.sqlite",
		MediaType: "application/vnd.sqlite3", SHA256: hash, Bytes: info.Size(),
	}}
	return path, manifest
}
