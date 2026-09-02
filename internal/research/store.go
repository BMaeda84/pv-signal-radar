package research

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	maxStoredRecordBytes            = 16 << 20
	maxExportFileBytes              = 32 << 20
	maxExportTotalBytes             = 64 << 20
	maxExportFileCount              = 32
	generatedReproductionScriptPath = "reproduce/reproduce-request.ps1"
)

var ErrImmutableConflict = errors.New("analysis already exists with different content")

// AnalysisStore is the immutable artifact boundary used by the HTTP runtime.
// FileStore is the local adapter; an S3-compatible adapter must preserve the
// same create-once/idempotent conflict semantics and stream exports without
// weakening validation or size limits. Provider selection and credentials are
// intentionally outside this repository revision.
type AnalysisStore interface {
	Save(record AnalysisRecord) error
	Load(analysisID string) (AnalysisRecord, error)
	LoadResult(analysisID string) (AnalysisResult, error)
	Export(analysisID string, destination io.Writer) error
	ExportRecord(record AnalysisRecord, destination io.Writer) error
}

type FileStore struct {
	root string
}

var _ AnalysisStore = (*FileStore)(nil)

type storedRecord struct {
	SchemaVersion string           `json:"schema_version"`
	Dataset       DatasetManifest  `json:"dataset"`
	Result        AnalysisResult   `json:"result"`
	Files         []storedFileMeta `json:"files"`
}

type storedFileMeta struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	Bytes     int64  `json:"bytes"`
}

func NewFileStore(root string) (*FileStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("file store root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve file store root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, fmt.Errorf("create file store root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve file store symlinks: %w", err)
	}
	return &FileStore{root: resolved}, nil
}

// Save persists an immutable analysis record atomically. Saving byte-equivalent
// content is idempotent; attempting to reuse an analysis ID for different bytes
// returns ErrImmutableConflict.
func (s *FileStore) Save(record AnalysisRecord) error {
	stored, fileData, err := prepareRecord(record)
	if err != nil {
		return err
	}
	analysisDir, err := s.analysisDir(record.Result.AnalysisID)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(analysisDir); err == nil {
		return s.compareExisting(analysisDir, stored, fileData)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	temporary, err := os.MkdirTemp(s.root, ".analysis-")
	if err != nil {
		return fmt.Errorf("create temporary analysis directory: %w", err)
	}
	defer os.RemoveAll(temporary)
	if err := writeStoredRecord(temporary, stored, fileData); err != nil {
		return err
	}
	if err := os.Rename(temporary, analysisDir); err != nil {
		// A concurrent writer may have published the same analysis first.
		if _, statErr := os.Lstat(analysisDir); statErr == nil {
			return s.compareExisting(analysisDir, stored, fileData)
		}
		return fmt.Errorf("publish immutable analysis: %w", err)
	}
	return nil
}

func (s *FileStore) Load(analysisID string) (AnalysisRecord, error) {
	analysisDir, stored, err := s.loadStoredMetadata(analysisID)
	if err != nil {
		return AnalysisRecord{}, err
	}
	record := AnalysisRecord{Dataset: stored.Dataset, Result: stored.Result}
	for _, metadata := range stored.Files {
		filePath := filepath.Join(analysisDir, "files", filepath.FromSlash(metadata.Name))
		fileBytes, err := readRegularLimitedFile(filePath, metadata.Bytes)
		if err != nil {
			return AnalysisRecord{}, fmt.Errorf("read stored file %q: %w", metadata.Name, err)
		}
		if int64(len(fileBytes)) != metadata.Bytes || sha256Hex(fileBytes) != metadata.SHA256 {
			return AnalysisRecord{}, fmt.Errorf("stored file %q failed size or checksum validation", metadata.Name)
		}
		record.Files = append(record.Files, ExportFile{Name: metadata.Name, MediaType: metadata.MediaType, Data: fileBytes})
	}
	if _, _, err := prepareRecord(record); err != nil {
		return AnalysisRecord{}, fmt.Errorf("validate stored analysis: %w", err)
	}
	return record, nil
}

// LoadResult validates the immutable record and file metadata without loading
// export payloads. GET/POST result lookups therefore have a small fixed memory
// cost; full files are read only after the export semaphore has been acquired.
func (s *FileStore) LoadResult(analysisID string) (AnalysisResult, error) {
	_, stored, err := s.loadStoredMetadata(analysisID)
	if err != nil {
		return AnalysisResult{}, err
	}
	return stored.Result, nil
}

func (s *FileStore) loadStoredMetadata(analysisID string) (string, storedRecord, error) {
	analysisDir, err := s.analysisDir(analysisID)
	if err != nil {
		return "", storedRecord{}, err
	}
	info, err := os.Lstat(analysisDir)
	if err != nil {
		return "", storedRecord{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", storedRecord{}, errors.New("analysis path is not a regular directory")
	}
	data, err := readRegularLimitedFile(filepath.Join(analysisDir, "record.json"), maxStoredRecordBytes)
	if err != nil {
		return "", storedRecord{}, fmt.Errorf("read analysis record: %w", err)
	}
	var stored storedRecord
	if err := decodeStrictJSON(data, &stored); err != nil {
		return "", storedRecord{}, fmt.Errorf("decode analysis record: %w", err)
	}
	if stored.SchemaVersion != SchemaVersion {
		return "", storedRecord{}, fmt.Errorf("unsupported stored schema %q", stored.SchemaVersion)
	}
	// analysis_id addresses the directory but does not hash result rows. Bind
	// the decoded record back to the requested address before trusting any of
	// its metadata; otherwise a valid record copied into another analysis
	// directory could be returned under the wrong protocol identifier.
	if stored.Result.AnalysisID != analysisID {
		return "", storedRecord{}, fmt.Errorf(
			"stored result analysis_id %q does not match requested analysis directory %q",
			stored.Result.AnalysisID,
			analysisID,
		)
	}
	if err := ValidateAnalysisResult(stored.Dataset, stored.Result); err != nil {
		return "", storedRecord{}, fmt.Errorf("validate stored result: %w", err)
	}
	var totalBytes int64
	if len(stored.Files) > maxExportFileCount {
		return "", storedRecord{}, fmt.Errorf("stored analysis exceeds %d export files", maxExportFileCount)
	}
	seen := make([]string, 0, len(stored.Files))
	for _, metadata := range stored.Files {
		if err := ValidateRelativePath(metadata.Name); err != nil {
			return "", storedRecord{}, fmt.Errorf("stored file %q: %w", metadata.Name, err)
		}
		// A stored record may be moved between case-sensitive and case-insensitive
		// filesystems. Revalidate the portable identity before resolving paths so
		// case aliases cannot address the same bytes with different checksums.
		if strings.EqualFold(metadata.Name, generatedReproductionScriptPath) {
			return "", storedRecord{}, fmt.Errorf("stored file %q collides with a generated bundle entry", metadata.Name)
		}
		if existing, duplicate := findCaseInsensitivePath(seen, metadata.Name); duplicate {
			return "", storedRecord{}, fmt.Errorf("stored file %q collides with %q on a case-insensitive filesystem", metadata.Name, existing)
		}
		seen = append(seen, metadata.Name)
		if strings.TrimSpace(metadata.MediaType) == "" || !sha256Pattern.MatchString(metadata.SHA256) {
			return "", storedRecord{}, fmt.Errorf("stored file %q has invalid media type or checksum", metadata.Name)
		}
		if metadata.Bytes < 0 || metadata.Bytes > maxExportFileBytes {
			return "", storedRecord{}, fmt.Errorf("stored file %q has invalid size %d", metadata.Name, metadata.Bytes)
		}
		totalBytes += metadata.Bytes
		if totalBytes > maxExportTotalBytes {
			return "", storedRecord{}, fmt.Errorf("stored analysis exceeds %d total export bytes", maxExportTotalBytes)
		}
	}
	return analysisDir, stored, nil
}

// Export writes a deterministic ZIP bundle with fixed entry timestamps, sorted
// paths, stored compression, and a checksum manifest. No host paths are exposed.
func (s *FileStore) Export(analysisID string, destination io.Writer) error {
	record, err := s.Load(analysisID)
	if err != nil {
		return err
	}
	return s.ExportRecord(record, destination)
}

// ExportRecord writes an already loaded and checksum-validated record. HTTP
// handlers use this after Load so the same files are not read a second time.
// Size/count limits in prepareRecord bound memory before any ZIP bytes are sent.
func (s *FileStore) ExportRecord(record AnalysisRecord, destination io.Writer) error {
	// ExportRecord is public and can be called without a preceding Load. Reuse
	// the same trust-boundary validation as Save so an internally inconsistent
	// result or unsafe supplemental path can never be serialized into a bundle.
	if _, _, err := prepareRecord(record); err != nil {
		return fmt.Errorf("invalid analysis record for export: %w", err)
	}
	entries, err := bundleEntries(record)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(destination)
	names := sortedKeys(entries)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		header.SetMode(0o644)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			if closeErr := writer.Close(); closeErr != nil {
				return fmt.Errorf("create ZIP entry %q: %w; close ZIP: %v", name, err, closeErr)
			}
			return fmt.Errorf("create ZIP entry %q: %w", name, err)
		}
		if _, err := entry.Write(entries[name]); err != nil {
			if closeErr := writer.Close(); closeErr != nil {
				return fmt.Errorf("write ZIP entry %q: %w; close ZIP: %v", name, err, closeErr)
			}
			return fmt.Errorf("write ZIP entry %q: %w", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close ZIP bundle: %w", err)
	}
	return nil
}

func prepareRecord(record AnalysisRecord) (storedRecord, map[string][]byte, error) {
	if err := ValidateAnalysisResult(record.Dataset, record.Result); err != nil {
		return storedRecord{}, nil, fmt.Errorf("invalid analysis record: %w", err)
	}
	files := append([]ExportFile(nil), record.Files...)
	if len(files) > maxExportFileCount {
		return storedRecord{}, nil, fmt.Errorf("analysis exceeds %d export files", maxExportFileCount)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	stored := storedRecord{SchemaVersion: SchemaVersion, Dataset: record.Dataset, Result: record.Result}
	fileData := make(map[string][]byte, len(files))
	seenPaths := make([]string, 0, len(files))
	var totalBytes int64
	for _, file := range files {
		if err := ValidateRelativePath(file.Name); err != nil {
			return storedRecord{}, nil, fmt.Errorf("export file %q: %w", file.Name, err)
		}
		// Supplemental files live under files/. This path is generated from the
		// canonical request and result identity, so allowing caller data to reuse
		// it would replace the trusted reproduction script before checksumming.
		if strings.EqualFold(file.Name, generatedReproductionScriptPath) {
			return storedRecord{}, nil, fmt.Errorf("export file %q collides with a generated bundle entry", file.Name)
		}
		if strings.TrimSpace(file.MediaType) == "" {
			return storedRecord{}, nil, fmt.Errorf("export file %q requires media_type", file.Name)
		}
		// EqualFold models the least-permissive supported consumer: Windows and
		// common archive extractors treat case-only path variants as one file.
		// The file count is bounded at 32, so a linear check is deterministic and
		// avoids a lossy, locale-sensitive lowercasing key.
		if existing, duplicate := findCaseInsensitivePath(seenPaths, file.Name); duplicate {
			return storedRecord{}, nil, fmt.Errorf("export file %q collides with %q on a case-insensitive filesystem", file.Name, existing)
		}
		seenPaths = append(seenPaths, file.Name)
		if len(file.Data) > maxExportFileBytes {
			return storedRecord{}, nil, fmt.Errorf("export file %q exceeds %d bytes", file.Name, maxExportFileBytes)
		}
		totalBytes += int64(len(file.Data))
		if totalBytes > maxExportTotalBytes {
			return storedRecord{}, nil, fmt.Errorf("analysis exceeds %d total export bytes", maxExportTotalBytes)
		}
		copied := append([]byte(nil), file.Data...)
		fileData[file.Name] = copied
		stored.Files = append(stored.Files, storedFileMeta{Name: file.Name, MediaType: file.MediaType, SHA256: sha256Hex(copied), Bytes: int64(len(copied))})
	}
	return stored, fileData, nil
}

func writeStoredRecord(dir string, record storedRecord, fileData map[string][]byte) error {
	recordBytes, err := indentedJSON(record)
	if err != nil {
		return err
	}
	if len(recordBytes) > maxStoredRecordBytes {
		return fmt.Errorf("analysis record exceeds %d bytes", maxStoredRecordBytes)
	}
	if err := os.WriteFile(filepath.Join(dir, "record.json"), recordBytes, 0o600); err != nil {
		return fmt.Errorf("write analysis record: %w", err)
	}
	for name, data := range fileData {
		path := filepath.Join(dir, "files", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("write analysis file %q: %w", name, err)
		}
	}
	return nil
}

func (s *FileStore) compareExisting(dir string, expected storedRecord, expectedFiles map[string][]byte) error {
	existing, err := s.Load(expected.Result.AnalysisID)
	if err != nil {
		return fmt.Errorf("existing immutable analysis is invalid: %w", err)
	}
	actual, actualFiles, err := prepareRecord(existing)
	if err != nil {
		return err
	}
	expectedJSON, _ := CanonicalJSON(expected)
	actualJSON, _ := CanonicalJSON(actual)
	if !bytes.Equal(expectedJSON, actualJSON) || len(expectedFiles) != len(actualFiles) {
		return ErrImmutableConflict
	}
	for name, data := range expectedFiles {
		if !bytes.Equal(data, actualFiles[name]) {
			return ErrImmutableConflict
		}
	}
	return nil
}

func (s *FileStore) analysisDir(analysisID string) (string, error) {
	if !sha256Pattern.MatchString(analysisID) {
		return "", errors.New("analysis_id must be a lowercase SHA-256")
	}
	return filepath.Join(s.root, analysisID), nil
}

func bundleEntries(record AnalysisRecord) (map[string][]byte, error) {
	analysisJSON, err := indentedJSON(record.Result)
	if err != nil {
		return nil, err
	}
	requestJSON, err := indentedJSON(record.Result.Request)
	if err != nil {
		return nil, err
	}
	manifestJSON, err := indentedJSON(record.Dataset)
	if err != nil {
		return nil, err
	}
	softwareJSON, err := indentedJSON(record.Result.Software)
	if err != nil {
		return nil, err
	}
	citationJSON, err := indentedJSON(bundleCitationMetadata(record))
	if err != nil {
		return nil, err
	}
	environmentJSON, err := indentedJSON(bundleEnvironmentMetadata(record))
	if err != nil {
		return nil, err
	}
	readme := fmt.Sprintf("PV Signal Radar reproducible analysis bundle\nAnalysis ID: %s\nResult digest: %s\nEmitted result rows: %d\nResult family: %s\nCanonical row order: %s\nDataset ID: %s\nApplication: %s %s (%s)\n\nThe result digest binds the declared family metadata, row count, and exact ordered rows emitted in analysis.json. It does not prove upstream dataset completeness, scientific validity, causality, or incidence. Follow REPRODUCE.md and verify every checksum before use.\n", record.Result.AnalysisID, record.Result.ResultDigest, record.Result.RowCount, record.Result.ResultFamily.FamilyID, record.Result.ResultFamily.RowOrder, record.Dataset.DatasetID, record.Result.Software.Name, record.Result.Software.Version, record.Result.Software.Commit)
	entries := map[string][]byte{
		"README.txt":                               []byte(readme),
		"REPRODUCE.md":                             []byte(reproductionGuide(record)),
		"analysis.json":                            analysisJSON,
		"CITATION.cff":                             []byte(bundleCitation(record)),
		"citation-metadata.json":                   citationJSON,
		"dataset-manifest.json":                    manifestJSON,
		"execution-environment.json":               environmentJSON,
		"files/" + generatedReproductionScriptPath: []byte(reproductionPowerShell(record)),
		"request.json":                             requestJSON,
		"software.json":                            softwareJSON,
	}
	// bundleEntries is a defensive boundary even though callers already use
	// prepareRecord. Track portable identities for every generated and supplied
	// entry so future generated paths cannot introduce case-only aliases.
	entryNames := sortedKeys(entries)
	for _, file := range record.Files {
		entryName := "files/" + file.Name
		if existing, collision := findCaseInsensitivePath(entryNames, entryName); collision {
			return nil, fmt.Errorf("supplemental export file %q collides with bundle entry %q on a case-insensitive filesystem", file.Name, existing)
		}
		entries[entryName] = append([]byte(nil), file.Data...)
		entryNames = append(entryNames, entryName)
	}
	names := sortedKeys(entries)
	var checksums strings.Builder
	for _, name := range names {
		fmt.Fprintf(&checksums, "%s  %s\n", sha256Hex(entries[name]), name)
	}
	entries["checksums.sha256"] = []byte(checksums.String())
	return entries, nil
}

// findCaseInsensitivePath returns the original spelling that shares a portable
// case-insensitive identity with candidate. strings.EqualFold is used instead
// of lowercasing so the comparison follows Unicode case-folding semantics
// without making locale-dependent transformations part of the bundle format.
func findCaseInsensitivePath(existing []string, candidate string) (string, bool) {
	for _, name := range existing {
		if strings.EqualFold(name, candidate) {
			return name, true
		}
	}
	return "", false
}

func reproductionGuide(record AnalysisRecord) string {
	return fmt.Sprintf(`# Reproduce this analysis

1. Verify every entry against checksums.sha256.
2. Check out https://github.com/BMaeda84/pv-signal-radar at commit %s and verify that the tree is clean.
3. Obtain the dataset identified by %s and verify its complete dataset-manifest.json, including source and artifact hashes.
4. Start that application revision with the verified read-only SQLite derivative and submit request.json to POST /api/v2/analyses.
5. Run files/reproduce/reproduce-request.ps1 against that local service, or submit request.json yourself.
6. Require analysis_id=%s, result_digest=%s, and row_count=%d. Compare analysis.json and files/results.csv byte-for-byte.

The result digest is SHA-256 over canonical JSON containing the result-family definition, declared row count, and exact ordered result rows. It detects row mutation, omission without metadata update, and reordering, but it does not independently prove that the upstream dataset or scientifically expected hypothesis family is complete. The dataset's canonical Parquet, source register, environment.json, and renv.lock remain governed by its own manifest/release. execution-environment.json records only the deterministic serving contract available to this bundle; it is not an OS/container/SBOM attestation. This analysis bundle does not redistribute source records. Reporting disproportionality is not causality or incidence.
`, record.Result.Software.Commit, record.Dataset.DatasetID, record.Result.AnalysisID, record.Result.ResultDigest, record.Result.RowCount)
}

func bundleCitation(record AnalysisRecord) string {
	return fmt.Sprintf(`cff-version: 1.2.0
message: "This file cites the PV Signal Radar software only. Cite the dataset and analysis identifiers separately; no dataset or analysis license is asserted here."
title: "PV Signal Radar"
type: software
version: %s
authors:
  - family-names: "Maeda"
    given-names: "Bruno"
repository-code: "https://github.com/BMaeda84/pv-signal-radar"
license: MIT
identifiers:
  - type: other
    value: %s
    description: "Analysis SHA-256 identifier"
  - type: other
    value: %s
    description: "Canonical emitted-result SHA-256 digest"
  - type: other
    value: %s
    description: "Application Git commit"
  - type: other
    value: %s
    description: "Dataset identifier"
`, yamlQuoted(record.Result.Software.Version), yamlQuoted(record.Result.AnalysisID), yamlQuoted(record.Result.ResultDigest), yamlQuoted(record.Result.Software.Commit), yamlQuoted(record.Dataset.DatasetID))
}

type citationLicenseMetadata struct {
	Value  string `json:"value,omitempty"`
	Status string `json:"status"`
}

type vocabularyCitationMetadata struct {
	Name    string                  `json:"name"`
	Version string                  `json:"version"`
	Scope   string                  `json:"scope"`
	License citationLicenseMetadata `json:"license"`
}

type analysisCitationMetadata struct {
	AnalysisID   string                  `json:"analysis_id"`
	ResultDigest string                  `json:"result_digest"`
	RowCount     int64                   `json:"row_count"`
	ResultFamily ResultFamilyDefinition  `json:"result_family"`
	License      citationLicenseMetadata `json:"license"`
}

type datasetCitationMetadata struct {
	DatasetID      string                       `json:"dataset_id"`
	ManifestSHA256 string                       `json:"manifest_sha256"`
	SourceLicense  citationLicenseMetadata      `json:"source_license"`
	Vocabularies   []vocabularyCitationMetadata `json:"vocabularies"`
}

type softwareCitationMetadata struct {
	Software SoftwareReference       `json:"software"`
	License  citationLicenseMetadata `json:"license"`
}

type citationMetadata struct {
	SchemaVersion string                   `json:"schema_version"`
	Software      softwareCitationMetadata `json:"software"`
	Dataset       datasetCitationMetadata  `json:"dataset"`
	Analysis      analysisCitationMetadata `json:"analysis"`
	Limitations   []string                 `json:"limitations"`
}

func bundleCitationMetadata(record AnalysisRecord) citationMetadata {
	sourceLicense := citationLicenseMetadata{Status: "not_declared_in_manifest"}
	if strings.TrimSpace(record.Dataset.Source.License) != "" {
		sourceLicense = citationLicenseMetadata{Value: record.Dataset.Source.License, Status: "declared_by_source_manifest"}
	}
	vocabularies := make([]vocabularyCitationMetadata, 0, len(record.Dataset.Vocabularies))
	for _, vocabulary := range record.Dataset.Vocabularies {
		license := citationLicenseMetadata{Status: "not_declared_in_manifest"}
		if strings.TrimSpace(vocabulary.License) != "" {
			license = citationLicenseMetadata{Value: vocabulary.License, Status: "declared_by_source_manifest"}
		}
		vocabularies = append(vocabularies, vocabularyCitationMetadata{
			Name: vocabulary.Name, Version: vocabulary.Version, Scope: vocabulary.Scope, License: license,
		})
	}
	return citationMetadata{
		SchemaVersion: SchemaVersion,
		Software:      softwareCitationMetadata{Software: record.Result.Software, License: citationLicenseMetadata{Value: "MIT", Status: "declared_for_application_source_only"}},
		Dataset:       datasetCitationMetadata{DatasetID: record.Dataset.DatasetID, ManifestSHA256: record.Result.Dataset.ManifestSHA256, SourceLicense: sourceLicense, Vocabularies: vocabularies},
		Analysis:      analysisCitationMetadata{AnalysisID: record.Result.AnalysisID, ResultDigest: record.Result.ResultDigest, RowCount: record.Result.RowCount, ResultFamily: record.Result.ResultFamily, License: citationLicenseMetadata{Status: "not_asserted"}},
		Limitations: []string{
			"The MIT declaration applies only to the PV Signal Radar application source.",
			"No license for source data, vocabularies, derived dataset artifacts, or this analysis result is granted or inferred by this metadata.",
		},
	}
}

type executionEnvironmentMetadata struct {
	SchemaVersion         string            `json:"schema_version"`
	AnalysisID            string            `json:"analysis_id"`
	ResultDigest          string            `json:"result_digest"`
	Application           SoftwareReference `json:"application"`
	ServingRuntime        string            `json:"serving_runtime"`
	CanonicalResultFormat string            `json:"canonical_result_format"`
	DatasetEnvironment    string            `json:"dataset_environment"`
	AttestationScope      string            `json:"attestation_scope"`
}

func bundleEnvironmentMetadata(record AnalysisRecord) executionEnvironmentMetadata {
	return executionEnvironmentMetadata{
		SchemaVersion:         SchemaVersion,
		AnalysisID:            record.Result.AnalysisID,
		ResultDigest:          record.Result.ResultDigest,
		Application:           record.Result.Software,
		ServingRuntime:        "go_sqlite_read_only",
		CanonicalResultFormat: "analysis.json",
		DatasetEnvironment:    "governed_by_dataset_manifest_and_release",
		AttestationScope:      "software_identity_and_deterministic_serving_contract_only; not an OS, container, compiler, R environment, or SBOM attestation",
	}
}

func reproductionPowerShell(record AnalysisRecord) string {
	return fmt.Sprintf(`param(
    [Parameter(Mandatory = $true)]
    [string]$BaseUrl
)

$ErrorActionPreference = "Stop"
$requestPath = Join-Path $PSScriptRoot "..\..\request.json"
$uri = $BaseUrl.TrimEnd("/") + "/api/v2/analyses"
$response = Invoke-RestMethod -Method Post -Uri $uri -ContentType "application/json" -InFile $requestPath

if ($response.analysis_id -ne "%s") {
    throw "analysis_id mismatch"
}
if ($response.result_digest -ne "%s") {
    throw "result_digest mismatch"
}
if ([int64]$response.row_count -ne %d) {
    throw "row_count mismatch"
}

$response
`, record.Result.AnalysisID, record.Result.ResultDigest, record.Result.RowCount)
}

func yamlQuoted(value string) string {
	// JSON string literals are valid YAML double-quoted scalars and prevent a
	// manifest-derived value from injecting additional CFF keys.
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func indentedJSON(value any) ([]byte, error) {
	compact, err := CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := jsonIndent(&output, compact); err != nil {
		return nil, err
	}
	output.WriteByte('\n')
	return output.Bytes(), nil
}

func jsonIndent(destination *bytes.Buffer, compact []byte) error {
	// Kept behind a tiny helper so all persisted and exported JSON uses exactly
	// the same indentation and trailing-newline policy.
	if err := json.Indent(destination, compact, "", "  "); err != nil {
		return fmt.Errorf("indent JSON: %w", err)
	}
	return nil
}

func readRegularLimitedFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("path is not a regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file exceeds declared limit of %d bytes", limit)
	}
	return readLimitedFile(path, limit)
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
