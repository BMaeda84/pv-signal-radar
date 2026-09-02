package research

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryLoadsValidatedManifestsInStableOrder(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, filepath.Join(dir, "z.json"), testManifest("faers-z"))
	writeManifest(t, filepath.Join(dir, "a.json"), testManifest("faers-a"))
	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Len(); got != 2 {
		t.Fatalf("registry Len() = %d; want 2", got)
	}
	listed := registry.List()
	if len(listed) != 2 || listed[0].DatasetID != "faers-a" || listed[1].DatasetID != "faers-z" {
		t.Fatalf("registry order is not deterministic: %+v", listed)
	}
	listed[0].Title = "caller mutation"
	again, _ := registry.Get("faers-a")
	if again.Title == "caller mutation" {
		t.Fatal("registry leaked mutable manifest state")
	}
	if _, err := registry.Require("unavailable-snapshot"); !errors.Is(err, ErrDatasetNotFound) {
		t.Fatalf("unavailable dataset error = %v, want ErrDatasetNotFound", err)
	}
	unavailable := testRequest("unavailable-snapshot")
	if _, _, err := registry.ResolveAnalysisRequest(unavailable); !errors.Is(err, ErrDatasetNotFound) {
		t.Fatalf("analysis resolution fabricated or substituted an unavailable dataset: %v", err)
	}
	resolvedManifest, normalized, err := registry.ResolveAnalysisRequest(testRequest("faers-a"))
	if err != nil {
		t.Fatal(err)
	}
	if resolvedManifest.DatasetID != "faers-a" || normalized.DatasetID != "faers-a" {
		t.Fatalf("unexpected resolved analysis: %q %q", resolvedManifest.DatasetID, normalized.DatasetID)
	}
	// Registry does not know the runtime commit and must not fabricate a
	// DevelopmentSoftwareReference-derived identifier. The execution boundary
	// supplies the authoritative software reference explicitly.
	software := DevelopmentSoftwareReference()
	software.Commit = strings.Repeat("9", 40)
	id, err := AnalysisIDForSoftware(resolvedManifest, normalized, software)
	if err != nil || len(id) != 64 {
		t.Fatalf("execution-bound analysis ID: id=%q err=%v", id, err)
	}
	outOfCoverage := testRequest("faers-a")
	outOfCoverage.Period.EndDate = "2026-01-01"
	if _, _, err := registry.ResolveAnalysisRequest(outOfCoverage); err == nil {
		t.Fatal("out-of-coverage request should be rejected rather than silently clamped")
	}
}

func TestRegistryRejectsUnknownFieldsAndDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	data, err := json.Marshal(testManifest("faers-a"))
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] = ','
	data = append(data, []byte(`"fabricated":true}`)...)
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(dir); err == nil {
		t.Fatal("unknown manifest field should be rejected")
	}

	manifest := testManifest("duplicate")
	if _, err := NewRegistry([]DatasetManifest{manifest, manifest}); err == nil {
		t.Fatal("duplicate dataset IDs should be rejected")
	}
}

func TestManifestRequiresFrozenProvenance(t *testing.T) {
	manifest := testManifest("faers-a")
	manifest.Source.Files = nil
	manifest.Artifacts = nil
	manifest.Limitations = nil
	if err := ValidateDatasetManifest(manifest); err == nil {
		t.Fatal("manifest without sources, artifacts, and limitations should be rejected")
	}
}

func writeManifest(t *testing.T, path string, manifest DatasetManifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
