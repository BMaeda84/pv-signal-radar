package research

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestFileStoreRoundTripAndDeterministicExport(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"No causality or incidence inference."})
	if err != nil {
		t.Fatal(err)
	}
	record := AnalysisRecord{Dataset: manifest, Result: result, Files: []ExportFile{
		{Name: "reproduce/run.R", MediaType: "text/x-r-source", Data: []byte("# deterministic reproduction script\n")},
		{Name: "results/results.csv", MediaType: "text/csv", Data: []byte("event,ror\nExample,0.88\n")},
	}}
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(record); err != nil {
		t.Fatalf("idempotent save failed: %v", err)
	}
	loaded, err := store.Load(result.AnalysisID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, record) {
		t.Fatalf("round trip differs:\n got: %#v\nwant: %#v", loaded, record)
	}

	var first, second bytes.Buffer
	if err := store.Export(result.AnalysisID, &first); err != nil {
		t.Fatal(err)
	}
	if err := store.Export(result.AnalysisID, &second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("repeated exports are not byte-identical")
	}
	assertBundle(t, first.Bytes())
}

func TestFileStoreRejectsTraversalAndImmutableConflict(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"../escape.csv", "C:/escape.csv", `nested\escape.csv`, "/rooted.csv"} {
		record := AnalysisRecord{Dataset: manifest, Result: result, Files: []ExportFile{{Name: unsafe, MediaType: "text/csv", Data: []byte("x")}}}
		if err := store.Save(record); err == nil {
			t.Fatalf("unsafe path %q was accepted", unsafe)
		}
	}
	record := AnalysisRecord{Dataset: manifest, Result: result, Files: []ExportFile{{Name: "result.csv", MediaType: "text/csv", Data: []byte("first")}}}
	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}
	record.Files[0].Data = []byte("different")
	if err := store.Save(record); !errors.Is(err, ErrImmutableConflict) {
		t.Fatalf("immutable conflict error = %v", err)
	}
	if _, err := store.Load("../escape"); err == nil {
		t.Fatal("unsafe analysis ID was accepted")
	}
}

func TestFileStoreDetectsArtifactTampering(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	store, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	record := AnalysisRecord{Dataset: manifest, Result: result, Files: []ExportFile{{Name: "result.csv", MediaType: "text/csv", Data: []byte("original")}}}
	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, result.AnalysisID, "files", "result.csv")
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(result.AnalysisID); err == nil {
		t.Fatal("tampered artifact passed checksum validation")
	}
}

func TestFileStoreRejectsDirectoryRecordAnalysisIDMismatch(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	first, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	secondRequest := testRequest(manifest.DatasetID)
	secondRequest.DrugConceptID = "rxnorm:456"
	secondRow := testRow()
	secondRow.DrugConceptID = secondRequest.DrugConceptID
	second, err := NewAnalysisResult(manifest, secondRequest, []DrugEventResult{secondRow}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	if first.AnalysisID == second.AnalysisID {
		t.Fatal("test precondition failed: distinct protocols produced the same analysis ID")
	}

	root := t.TempDir()
	store, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(AnalysisRecord{Dataset: manifest, Result: first}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(AnalysisRecord{Dataset: manifest, Result: second}); err != nil {
		t.Fatal(err)
	}
	secondRecord, err := os.ReadFile(filepath.Join(root, second.AnalysisID, "record.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, first.AnalysisID, "record.json"), secondRecord, 0o600); err != nil {
		t.Fatal(err)
	}

	for operation, load := range map[string]func() error{
		"Load": func() error {
			_, err := store.Load(first.AnalysisID)
			return err
		},
		"LoadResult": func() error {
			_, err := store.LoadResult(first.AnalysisID)
			return err
		},
	} {
		if err := load(); err == nil || !strings.Contains(err.Error(), "does not match requested analysis directory") {
			t.Fatalf("%s accepted a record copied from a different analysis directory: %v", operation, err)
		}
	}
	if _, err := store.LoadResult(second.AnalysisID); err != nil {
		t.Fatalf("source analysis directory was damaged by the mismatch test: %v", err)
	}
}

func TestFileStoreRejectsResultWhoseRequestDoesNotMatchDataset(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	result.Request.DatasetID = "faers-other"
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(AnalysisRecord{Dataset: manifest, Result: result}); err == nil || !strings.Contains(err.Error(), "does not match manifest") {
		t.Fatalf("FileStore accepted a request/manifest dataset mismatch: %v", err)
	}
}

func TestFileStoreDetectsResultRowTruncationReorderingAndAdulteration(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), testRowsTwo(), []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*storedRecord)
	}{
		{"truncation", func(stored *storedRecord) { stored.Result.Rows = stored.Result.Rows[:1] }},
		{"reordering", func(stored *storedRecord) {
			stored.Result.Rows[0], stored.Result.Rows[1] = stored.Result.Rows[1], stored.Result.Rows[0]
		}},
		{"adulteration", func(stored *storedRecord) { stored.Result.Rows[0].EventCategory = "adulterated" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			store, err := NewFileStore(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Save(AnalysisRecord{Dataset: manifest, Result: result}); err != nil {
				t.Fatal(err)
			}
			recordPath := filepath.Join(root, result.AnalysisID, "record.json")
			data, err := os.ReadFile(recordPath)
			if err != nil {
				t.Fatal(err)
			}
			var stored storedRecord
			if err := decodeStrictJSON(data, &stored); err != nil {
				t.Fatal(err)
			}
			test.mutate(&stored)
			data, err = indentedJSON(stored)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(recordPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.LoadResult(result.AnalysisID); err == nil {
				t.Fatal("tampered result family crossed the FileStore boundary")
			}
		})
	}
}

func TestExportRecordRejectsInconsistentResultDigest(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	result.ResultDigest = strings.Repeat("0", 64)
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var destination bytes.Buffer
	if err := store.ExportRecord(AnalysisRecord{Dataset: manifest, Result: result}, &destination); err == nil {
		t.Fatal("direct ExportRecord accepted an inconsistent result digest")
	}
	if destination.Len() != 0 {
		t.Fatal("invalid export wrote partial bytes")
	}
}

func TestFileStoreRejectsManifestUniverseMismatch(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}

	// Rebind every manifest-derived identity so the only remaining contradiction
	// is the table denominator versus completeness. The store must still reject
	// the coherently re-addressed record.
	badManifest := manifest
	badManifest.Completeness.EligibleReports = 999
	badManifest.Completeness.Fields = nil
	manifestHash, err := DatasetManifestHash(badManifest)
	if err != nil {
		t.Fatal(err)
	}
	result.DatasetManifest = badManifest
	result.Dataset = DatasetReference{
		DatasetID: badManifest.DatasetID, ManifestSHA256: manifestHash,
		Coverage: badManifest.Coverage, Completeness: badManifest.Completeness,
	}
	result.AnalysisID, err = AnalysisIDForSoftware(badManifest, result.Request, result.Software)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(AnalysisRecord{Dataset: badManifest, Result: result}); err == nil || !strings.Contains(err.Error(), "does not match manifest eligible_reports") {
		t.Fatalf("FileStore accepted a result with the wrong eligible universe: %v", err)
	}
}

func TestFileStoreRejectsGeneratedBundleEntryCollision(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	record := AnalysisRecord{Dataset: manifest, Result: result, Files: []ExportFile{{
		Name: generatedReproductionScriptPath, MediaType: "text/x-powershell", Data: []byte("Write-Output 'replacement'\n"),
	}}}
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(record); err == nil || !strings.Contains(err.Error(), "collides with a generated bundle entry") {
		t.Fatalf("FileStore accepted a reserved bundle path: %v", err)
	}
	var destination bytes.Buffer
	if err := store.ExportRecord(record, &destination); err == nil || !strings.Contains(err.Error(), "collides with a generated bundle entry") {
		t.Fatalf("ExportRecord accepted a reserved bundle path: %v", err)
	}
	if destination.Len() != 0 {
		t.Fatal("reserved-path export wrote partial bytes")
	}
}

func TestFileStoreRejectsCaseInsensitiveBundlePathCollisions(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		files []ExportFile
	}{
		{
			name: "generated script case alias",
			files: []ExportFile{{
				Name: "Reproduce/reproduce-request.ps1", MediaType: "text/x-powershell", Data: []byte("Write-Output 'replacement'\n"),
			}},
		},
		{
			name: "supplemental case aliases",
			files: []ExportFile{
				{Name: "results/A.csv", MediaType: "text/csv", Data: []byte("first\n")},
				{Name: "results/a.csv", MediaType: "text/csv", Data: []byte("second\n")},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := AnalysisRecord{Dataset: manifest, Result: result, Files: test.files}
			store, err := NewFileStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Save(record); err == nil || !strings.Contains(err.Error(), "collides") {
				t.Fatalf("FileStore accepted case-insensitive path aliases: %v", err)
			}
			var destination bytes.Buffer
			if err := store.ExportRecord(record, &destination); err == nil || !strings.Contains(err.Error(), "collides") {
				t.Fatalf("ExportRecord accepted case-insensitive path aliases: %v", err)
			}
			if destination.Len() != 0 {
				t.Fatal("case-collision export wrote partial bytes")
			}
		})
	}
}

func TestFileStoreLoadMetadataRejectsCaseInsensitiveAliases(t *testing.T) {
	manifest := testManifest("faers-2025q4")
	result, err := NewAnalysisResult(manifest, testRequest(manifest.DatasetID), []DrugEventResult{testRow()}, []string{"Limitation."})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	store, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(AnalysisRecord{Dataset: manifest, Result: result}); err != nil {
		t.Fatal(err)
	}

	recordPath := filepath.Join(root, result.AnalysisID, "record.json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	var stored storedRecord
	if err := decodeStrictJSON(data, &stored); err != nil {
		t.Fatal(err)
	}
	stored.Files = []storedFileMeta{
		{Name: "results/A.csv", MediaType: "text/csv", SHA256: strings.Repeat("a", 64), Bytes: 1},
		{Name: "results/a.csv", MediaType: "text/csv", SHA256: strings.Repeat("b", 64), Bytes: 1},
	}
	data, err = indentedJSON(stored)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recordPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadResult(result.AnalysisID); err == nil || !strings.Contains(err.Error(), "case-insensitive filesystem") {
		t.Fatalf("LoadResult accepted case-insensitive stored aliases: %v", err)
	}
}

func assertBundle(t *testing.T, data []byte) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	var checksums string
	contentsByName := make(map[string][]byte)
	for _, file := range reader.File {
		names = append(names, file.Name)
		opened, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		contents, err := io.ReadAll(opened)
		opened.Close()
		if err != nil {
			t.Fatal(err)
		}
		if file.Name == "checksums.sha256" {
			checksums = string(contents)
		}
		contentsByName[file.Name] = contents
		if strings.Contains(file.Name, "..") || strings.Contains(file.Name, "\\") {
			t.Fatalf("unsafe ZIP entry %q", file.Name)
		}
	}
	want := []string{"CITATION.cff", "README.txt", "REPRODUCE.md", "analysis.json", "checksums.sha256", "citation-metadata.json", "dataset-manifest.json", "execution-environment.json", "files/reproduce/reproduce-request.ps1", "files/reproduce/run.R", "files/results/results.csv", "request.json", "software.json"}
	sort.Strings(want)
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("ZIP entries = %v, want %v", names, want)
	}
	for _, required := range []string{"CITATION.cff", "README.txt", "REPRODUCE.md", "analysis.json", "citation-metadata.json", "dataset-manifest.json", "execution-environment.json", "files/reproduce/reproduce-request.ps1", "request.json", "software.json", "files/results/results.csv"} {
		if !strings.Contains(checksums, "  "+required+"\n") {
			t.Fatalf("checksum manifest missing %q: %s", required, checksums)
		}
	}
	cff := string(contentsByName["CITATION.cff"])
	if !strings.Contains(cff, "type: software") || strings.Contains(cff, "type: dataset") {
		t.Fatalf("bundle CFF must cite software rather than classify the analysis as a dataset:\n%s", cff)
	}
	if !strings.Contains(cff, "no dataset or analysis license is asserted") {
		t.Fatalf("bundle CFF does not delimit its MIT software license:\n%s", cff)
	}
	var citation citationMetadata
	if err := json.Unmarshal(contentsByName["citation-metadata.json"], &citation); err != nil {
		t.Fatalf("decode citation metadata: %v", err)
	}
	if citation.Software.License.Value != "MIT" || citation.Software.License.Status != "declared_for_application_source_only" {
		t.Fatalf("software license scope drifted: %#v", citation.Software.License)
	}
	if citation.Analysis.License.Value != "" || citation.Analysis.License.Status != "not_asserted" {
		t.Fatalf("analysis acquired an unsupported license: %#v", citation.Analysis.License)
	}
	if citation.Dataset.SourceLicense.Status != "declared_by_source_manifest" {
		t.Fatalf("dataset source-license provenance was not preserved: %#v", citation.Dataset.SourceLicense)
	}
	if !strings.Contains(string(contentsByName["files/reproduce/reproduce-request.ps1"]), citation.Analysis.ResultDigest) {
		t.Fatal("reproduction script does not verify result_digest")
	}
}
