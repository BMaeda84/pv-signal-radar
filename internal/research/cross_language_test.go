package research

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRGoldenInterchangeMaterializesAndAnalyzesInGo(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	goldenRoot := filepath.Join(filepath.Dir(sourceFile), "..", "..", "research", "tests", "golden")
	manifest, err := LoadDatasetManifest(filepath.Join(goldenRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("load R/Go golden manifest: %v", err)
	}
	tsvPath := filepath.Join(goldenRoot, "aggregate_interchange.tsv")
	hash, err := FileSHA256(tsvPath)
	if err != nil {
		t.Fatal(err)
	}
	if hash != manifest.Artifacts[0].SHA256 {
		t.Fatalf("golden TSV hash drifted: got %s want %s", hash, manifest.Artifacts[0].SHA256)
	}

	sqlitePath := filepath.Join(t.TempDir(), "aggregate.sqlite")
	rows, err := MaterializeSQLite(context.Background(), tsvPath, sqlitePath, manifest)
	if err != nil {
		t.Fatalf("materialize R golden interchange: %v", err)
	}
	if rows != 13 {
		t.Fatalf("materialized row count = %d, want 13", rows)
	}
	sqliteHash, err := FileSHA256(sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(sqlitePath)
	if err != nil {
		t.Fatal(err)
	}
	servingManifest := manifest
	servingManifest.Artifacts = []DatasetArtifact{{
		Name: "read-only aggregate", Path: "aggregate.sqlite", MediaType: "application/vnd.sqlite3",
		SHA256: sqliteHash, Bytes: stat.Size(),
	}}
	engine, err := OpenSQLiteEngine(context.Background(), sqlitePath, servingManifest)
	if err != nil {
		t.Fatalf("open materialized R golden aggregate: %v", err)
	}
	defer engine.Close()

	record, err := engine.Analyze(context.Background(), AnalysisRequest{
		SchemaVersion: SchemaVersion, DatasetID: manifest.DatasetID,
		DrugConceptID: "faers-prod_ai:ASPIRIN", DrugRole: "primary_suspect",
		EventScope: EventScopeAllRecordedSourcePTs, Comparator: ComparatorAllOtherEligibleReports,
		Methods: []string{"fisher_exact", "prr", "ror"}, ThresholdProfileID: ThresholdProfileNone,
	})
	if err != nil {
		t.Fatalf("analyze materialized R golden aggregate: %v", err)
	}
	if len(record.Result.Rows) != 3 {
		t.Fatalf("Go returned %d event rows, want 3", len(record.Result.Rows))
	}
	var nausea *DrugEventResult
	for index := range record.Result.Rows {
		if record.Result.Rows[index].EventTerm == "NAUSEA" {
			nausea = &record.Result.Rows[index]
		}
	}
	if nausea == nil || nausea.Table != (IntegerTable{A: 1, B: 1, C: 1, D: 0, N: 3}) {
		t.Fatalf("NAUSEA table drifted: %#v", nausea)
	}
	metric := func(method string) float64 {
		for _, candidate := range nausea.Metrics {
			if candidate.Method == method {
				return candidate.Estimate
			}
		}
		t.Fatalf("missing %s metric", method)
		return 0
	}
	if math.Abs(metric("prr")-2.0/3.0) > 1e-12 || math.Abs(metric("ror")-1.0/3.0) > 1e-12 {
		t.Fatalf("Go metrics diverged from R reference: %#v", nausea.Metrics)
	}
	for _, candidate := range nausea.Metrics {
		if candidate.Method == "prr" || candidate.Method == "ror" {
			correction := candidate.Calculation.ZeroCellCorrection
			if candidate.Calculation.InputCells != "haldane_anscombe_corrected" || !correction.Applied || correction.AddedToEachCell != 0.5 {
				t.Fatalf("%s lost the R/Go zero-cell correction contract: %+v", candidate.Method, candidate.Calculation)
			}
		}
		if candidate.Method == "fisher_exact" && (candidate.Calculation.InputCells != "observed" || candidate.Calculation.ZeroCellCorrection.Applied) {
			t.Fatalf("Fisher must use the uncorrected observed R/Go table: %+v", candidate.Calculation)
		}
	}
	if !strings.Contains(string(record.Files[0].Data), "zero_cell_correction_method") || !strings.Contains(string(record.Files[0].Data), "haldane_anscombe") {
		t.Fatal("CSV export does not preserve per-metric zero-cell correction metadata")
	}
}
