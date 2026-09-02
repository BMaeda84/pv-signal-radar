// Command materialize-sqlite converts an integrity-checked FAERS TSV
// interchange into the immutable, read-only runtime artifact consumed by the
// Go research API. Structural and checksum checks are not scientific validation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BMaeda84/pv-signal-radar/internal/research"
	"github.com/BMaeda84/pv-signal-radar/internal/version"
)

func main() {
	var manifestPath, tsvPath, outputPath string
	flag.StringVar(&manifestPath, "manifest", "", "source DatasetManifest JSON")
	flag.StringVar(&tsvPath, "tsv", "", "integrity-checked aggregate_interchange.tsv")
	flag.StringVar(&outputPath, "output", "", "new runtime artifact directory")
	flag.Parse()
	if manifestPath == "" || tsvPath == "" || outputPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	materializerCommit, err := version.ResearchRevision(os.Getenv("PV_RADAR_APPLICATION_COMMIT"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "materialize-sqlite: authoritative clean application commit required: %v\n", err)
		os.Exit(1)
	}
	if err := run(manifestPath, tsvPath, outputPath, materializerCommit); err != nil {
		fmt.Fprintf(os.Stderr, "materialize-sqlite: %v\n", err)
		os.Exit(1)
	}
}

func run(manifestPath, tsvPath, outputPath string, optionalMaterializerCommit ...string) error {
	materializerCommit := "0000000000000000000000000000000000000000"
	if len(optionalMaterializerCommit) > 1 {
		return errors.New("only one materializer commit may be supplied")
	}
	if len(optionalMaterializerCommit) == 1 {
		materializerCommit = strings.ToLower(strings.TrimSpace(optionalMaterializerCommit[0]))
	}
	if err := research.ValidateSoftwareReference(research.SoftwareReference{Name: "pv-signal-radar", Version: version.Current, Commit: materializerCommit}); err != nil {
		return fmt.Errorf("invalid materializer identity: %w", err)
	}
	manifest, err := research.LoadDatasetManifest(manifestPath)
	if err != nil {
		return err
	}
	if err := verifyTSVArtifact(manifest, tsvPath); err != nil {
		return err
	}
	absoluteOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(absoluteOutput); err == nil {
		return errors.New("refusing to overwrite an existing output directory")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(absoluteOutput)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, "."+filepath.Base(absoluteOutput)+".staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	// #nosec G304 -- manifestPath is an explicit local CLI input already checked as a regular non-symlink file by LoadDatasetManifest.
	originalManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	parentManifestPath := filepath.Join(stage, "parent-manifest.json")
	// #nosec G703 -- stage is created by os.MkdirTemp and the filename is fixed; outputPath cannot influence a child path segment.
	if err := os.WriteFile(parentManifestPath, originalManifest, 0o600); err != nil {
		return err
	}
	sqlitePath := filepath.Join(stage, "aggregate.sqlite")
	rowCount, err := research.MaterializeSQLite(context.Background(), tsvPath, sqlitePath, manifest)
	if err != nil {
		return err
	}
	parentHash, err := research.FileSHA256(parentManifestPath)
	if err != nil {
		return err
	}
	sqliteHash, err := research.FileSHA256(sqlitePath)
	if err != nil {
		return err
	}
	parentInfo, _ := os.Stat(parentManifestPath)
	sqliteInfo, _ := os.Stat(sqlitePath)

	derived := manifest
	derived.Description = strings.TrimSpace(manifest.Description + " Runtime SQLite derivative with parent manifest bound as an artifact.")
	derived.Processing.PipelineVersion = manifest.Processing.PipelineVersion + "+sqlite-v1"
	derived.Processing.MaterializerCommit = materializerCommit
	derived.Artifacts = []research.DatasetArtifact{
		{Name: "parent research manifest", Path: "parent-manifest.json", MediaType: "application/json", SHA256: parentHash, Bytes: parentInfo.Size()},
		{Name: "read-only aggregate", Path: "aggregate.sqlite", MediaType: "application/vnd.sqlite3", SHA256: sqliteHash, Bytes: sqliteInfo.Size()},
	}
	derived.Limitations = append(append([]string(nil), manifest.Limitations...),
		"Runtime SQLite is a deterministic derivative of the parent manifest; use the canonical Parquet artifacts for independent re-analysis.",
	)
	if _, err := research.DatasetManifestHash(derived); err != nil {
		return fmt.Errorf("derived serving manifest is invalid: %w", err)
	}
	manifestBytes, err := json.MarshalIndent(derived, "", "  ")
	if err != nil {
		return err
	}
	manifestBytes = append(manifestBytes, '\n')
	servingManifestPath := filepath.Join(stage, "manifest.json")
	if err := os.WriteFile(servingManifestPath, manifestBytes, 0o600); err != nil {
		return err
	}
	manifestHash, err := research.FileSHA256(servingManifestPath)
	if err != nil {
		return err
	}
	checksums := fmt.Sprintf(
		"%s  aggregate.sqlite\n%s  manifest.json\n%s  parent-manifest.json\n",
		sqliteHash, manifestHash, parentHash,
	)
	if err := os.WriteFile(filepath.Join(stage, "checksums.sha256"), []byte(checksums), 0o600); err != nil {
		return err
	}
	if err := os.Rename(stage, absoluteOutput); err != nil {
		return fmt.Errorf("publish runtime artifact atomically: %w", err)
	}
	fmt.Printf("materialized %d aggregate row(s) in %s\n", rowCount, absoluteOutput)
	return nil
}

func verifyTSVArtifact(manifest research.DatasetManifest, tsvPath string) error {
	var declared []research.DatasetArtifact
	for _, artifact := range manifest.Artifacts {
		if artifact.MediaType == "text/tab-separated-values" {
			declared = append(declared, artifact)
		}
	}
	if len(declared) != 1 {
		return fmt.Errorf("source manifest must declare exactly one TSV interchange artifact, found %d", len(declared))
	}
	info, err := os.Lstat(tsvPath)
	if err != nil {
		return fmt.Errorf("inspect declared TSV artifact: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("declared TSV artifact must be a regular non-symlink file")
	}
	if info.Size() != declared[0].Bytes {
		return fmt.Errorf("TSV artifact size mismatch: manifest=%d actual=%d", declared[0].Bytes, info.Size())
	}
	hash, err := research.FileSHA256(tsvPath)
	if err != nil {
		return fmt.Errorf("hash declared TSV artifact: %w", err)
	}
	if hash != declared[0].SHA256 {
		return fmt.Errorf("TSV artifact checksum mismatch: manifest=%s actual=%s", declared[0].SHA256, hash)
	}
	return nil
}
