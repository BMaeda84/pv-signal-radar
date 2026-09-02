package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BMaeda84/pv-signal-radar/internal/research"
)

func TestRunPublishesRuntimeManifestAndReadOnlyAggregate(t *testing.T) {
	root := t.TempDir()
	tsv := filepath.Join(root, "aggregate.tsv")
	dedup := "latest case version; unique report pair"
	header := "dataset_id\tdrug_text\tdrug_text_source\tdrug_role\tevent_pt\ta\tb\tc\td\tdrug_reports\tevent_reports\tuniverse_reports\tcomparator\tevent_scope\tdeduplication_policy\n"
	row := strings.Join([]string{
		"faers-cli-test", "SEMAGLUTIDE", "PROD_AI", "primary_suspect", "NAUSEA",
		"10", "90", "40", "860", "100", "50", "1000",
		research.ComparatorAllOtherEligibleReports, research.EventScopeAllRecordedSourcePTs, dedup,
	}, "\t") + "\n"
	if err := os.WriteFile(tsv, []byte(header+row), 0o600); err != nil {
		t.Fatal(err)
	}
	tsvHash, _ := research.FileSHA256(tsv)
	tsvInfo, _ := os.Stat(tsv)
	manifest := research.DatasetManifest{
		SchemaVersion: research.SchemaVersion, DatasetID: "faers-cli-test", Title: "CLI fixture",
		Source: research.DatasetSource{
			Name: "FAERS", Publisher: "FDA", LandingPage: "https://www.fda.gov/",
			RetrievedAt: "2026-01-01T00:00:00Z",
			Files:       []research.SourceFile{{URL: "https://example.invalid/faers.zip", SHA256: strings.Repeat("a", 64), Bytes: 1}},
		},
		Coverage: research.DatasetCoverage{StartDate: "2025-01-01", EndDate: "2025-12-31", Geography: "global", Release: "2025"},
		Processing: research.DatasetProcessing{
			PipelineVersion: "test-v1", SourceCommit: strings.Repeat("a", 40), DeduplicationPolicy: dedup,
			CountUnit: "report pair", DrugRolePolicy: "explicit",
		},
		Artifacts: []research.DatasetArtifact{{
			Name: "aggregate interchange", Path: "aggregate.tsv", MediaType: "text/tab-separated-values",
			SHA256: tsvHash, Bytes: tsvInfo.Size(),
		}},
		Completeness: research.DatasetCompleteness{SourceDEMORows: 1200, CurrentCaseReports: 1100, EligibleReports: 1000, DrugEventPairs: 1000},
		Limitations:  []string{"Fixture only."},
	}
	manifestBytes, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(root, "source-manifest.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "runtime")
	materializerCommit := strings.Repeat("1", 40)
	if err := run(manifestPath, tsv, output, materializerCommit); err != nil {
		t.Fatalf("materialize command: %v", err)
	}
	derived, err := research.LoadDatasetManifest(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatalf("load derived manifest: %v", err)
	}
	if len(derived.Artifacts) != 2 || !strings.Contains(derived.Processing.PipelineVersion, "sqlite-v1") || derived.Processing.MaterializerCommit != materializerCommit {
		t.Fatalf("derived manifest does not bind runtime artifacts: %#v", derived)
	}
	engine, err := research.OpenSQLiteEngine(context.Background(), filepath.Join(output, "aggregate.sqlite"), derived)
	if err != nil {
		t.Fatalf("derived aggregate is not serveable: %v", err)
	}
	defer engine.Close()

	// A path supplied by the operator is not evidence of provenance. Changing a
	// single byte must break the manifest-bound chain before any output appears.
	if err := os.WriteFile(tsv, []byte(header+row+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tamperedOutput := filepath.Join(root, "tampered-runtime")
	if err := run(manifestPath, tsv, tamperedOutput); err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("expected manifest-bound TSV mismatch, got %v", err)
	}
	if _, err := os.Stat(tamperedOutput); !os.IsNotExist(err) {
		t.Fatalf("tampered input must not publish output, stat err=%v", err)
	}
}
