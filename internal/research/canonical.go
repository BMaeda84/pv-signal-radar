package research

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/BMaeda84/pv-signal-radar/internal/version"
)

// DevelopmentSoftwareReference is deterministic for fixtures and callers that
// use the research package outside the server. The production server replaces
// it with the clean build's real Git revision before enabling research mode.
func DevelopmentSoftwareReference() SoftwareReference {
	return SoftwareReference{Name: "pv-signal-radar", Version: version.Current, Commit: "0000000000000000000000000000000000000000"}
}

// CanonicalJSON returns the stable compact JSON representation used for hashes
// and persisted records. Contracts intentionally avoid unordered semantic lists;
// requests must first pass NormalizeAnalysisRequest.
func CanonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode canonical JSON: %w", err)
	}
	return encoded, nil
}

func CanonicalHash(value any) (string, error) {
	encoded, err := CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

// DatasetManifestHash validates the provenance contract before hashing it.
func DatasetManifestHash(manifest DatasetManifest) (string, error) {
	if err := ValidateDatasetManifest(manifest); err != nil {
		return "", fmt.Errorf("invalid dataset manifest: %w", err)
	}
	return CanonicalHash(manifest)
}

// AnalysisID binds a normalized protocol to the complete frozen dataset
// manifest. Changing a source checksum, processing policy, or request parameter
// necessarily changes the identifier.
func AnalysisID(manifest DatasetManifest, request AnalysisRequest) (string, error) {
	return AnalysisIDForSoftware(manifest, request, DevelopmentSoftwareReference())
}

// AnalysisIDForSoftware binds the normalized protocol, complete dataset
// manifest, and exact application implementation into one input-content
// identifier. Result rows are outputs and deliberately do not participate in the
// ID; ValidateAnalysisResult and the storage boundary must verify their integrity.
func AnalysisIDForSoftware(manifest DatasetManifest, request AnalysisRequest, software SoftwareReference) (string, error) {
	manifestHash, err := DatasetManifestHash(manifest)
	if err != nil {
		return "", err
	}
	normalized, err := NormalizeAnalysisRequest(request)
	if err != nil {
		return "", fmt.Errorf("invalid analysis request: %w", err)
	}
	if err := validateAnalysisRequestAgainstManifest(manifest, normalized); err != nil {
		return "", fmt.Errorf("analysis request does not match manifest: %w", err)
	}
	if err := ValidateSoftwareReference(software); err != nil {
		return "", fmt.Errorf("invalid software reference: %w", err)
	}
	seed := struct {
		SchemaVersion  string            `json:"schema_version"`
		ManifestSHA256 string            `json:"manifest_sha256"`
		Software       SoftwareReference `json:"software"`
		Request        AnalysisRequest   `json:"request"`
	}{SchemaVersion: SchemaVersion, ManifestSHA256: manifestHash, Software: software, Request: normalized}
	return CanonicalHash(seed)
}

// CanonicalResultFamily returns the versioned emitted-family contract shared by
// the Go runtime, API, and export bundle. It describes what the engine emitted;
// it is not evidence that an upstream source or scientifically expected family
// is complete.
func CanonicalResultFamily() ResultFamilyDefinition {
	return ResultFamilyDefinition{
		FamilyID:        ResultFamilyEmittedProtocolRows,
		MembershipRule:  ResultFamilyMembershipRule,
		RowUnit:         ResultFamilyRowUnit,
		RowOrder:        ResultFamilyRowOrder,
		DigestAlgorithm: ResultDigestAlgorithm,
	}
}

// CanonicalResultDigest binds the family declaration, row count, and exact
// canonical row sequence. Reordering changes the hash even before validation;
// callers must still use ValidateAnalysisResult to enforce the declared order.
func CanonicalResultDigest(family ResultFamilyDefinition, rows []DrugEventResult) (string, error) {
	seed := struct {
		SchemaVersion string                 `json:"schema_version"`
		ResultFamily  ResultFamilyDefinition `json:"result_family"`
		RowCount      int64                  `json:"row_count"`
		Rows          []DrugEventResult      `json:"rows"`
	}{
		SchemaVersion: SchemaVersion,
		ResultFamily:  family,
		RowCount:      int64(len(rows)),
		Rows:          rows,
	}
	return CanonicalHash(seed)
}

// NewAnalysisResult constructs a deterministic result envelope. Row order is
// significant and must be selected by the analysis protocol, not changed here.
func NewAnalysisResult(manifest DatasetManifest, request AnalysisRequest, rows []DrugEventResult, caveats []string) (AnalysisResult, error) {
	return NewAnalysisResultForSoftware(manifest, request, DevelopmentSoftwareReference(), rows, caveats)
}

func NewAnalysisResultForSoftware(manifest DatasetManifest, request AnalysisRequest, software SoftwareReference, rows []DrugEventResult, caveats []string) (AnalysisResult, error) {
	normalized, err := NormalizeAnalysisRequest(request)
	if err != nil {
		return AnalysisResult{}, err
	}
	manifestHash, err := DatasetManifestHash(manifest)
	if err != nil {
		return AnalysisResult{}, err
	}
	// Embed the complete manifest without retaining aliases to caller-owned
	// slices. Dataset remains a concise summary while dataset_manifest is the
	// lossless provenance contract returned by the API.
	manifestJSON, err := CanonicalJSON(manifest)
	if err != nil {
		return AnalysisResult{}, err
	}
	var embeddedManifest DatasetManifest
	if err := json.Unmarshal(manifestJSON, &embeddedManifest); err != nil {
		return AnalysisResult{}, fmt.Errorf("clone dataset manifest: %w", err)
	}
	id, err := AnalysisIDForSoftware(manifest, normalized, software)
	if err != nil {
		return AnalysisResult{}, err
	}
	resultRows := append([]DrugEventResult(nil), rows...)
	family := CanonicalResultFamily()
	resultDigest, err := CanonicalResultDigest(family, resultRows)
	if err != nil {
		return AnalysisResult{}, err
	}
	result := AnalysisResult{
		SchemaVersion: SchemaVersion,
		AnalysisID:    id,
		ResultDigest:  resultDigest,
		RowCount:      int64(len(resultRows)),
		ResultFamily:  family,
		Software:      software,
		Dataset: DatasetReference{
			DatasetID:      manifest.DatasetID,
			ManifestSHA256: manifestHash,
			Coverage:       manifest.Coverage,
			Completeness:   manifest.Completeness,
		},
		DatasetManifest: embeddedManifest,
		Request:         normalized,
		Rows:            resultRows,
		Caveats:         append([]string(nil), caveats...),
	}
	if err := ValidateAnalysisResult(manifest, result); err != nil {
		return AnalysisResult{}, err
	}
	return result, nil
}
