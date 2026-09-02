// Package research defines the deterministic, provenance-bearing contracts used
// by research-grade analyses. It intentionally contains no live-data fallback:
// callers must select a registered, frozen dataset snapshot.
package research

const (
	// SchemaVersion identifies the current manifest, request, and result contract.
	SchemaVersion = "pv-signal-radar.research/v1"

	// These identifiers are shared by dataset producers and API consumers. They
	// describe the actual report-level universe; they are not vocabulary mappings
	// or claims that the background consists of exposed patients.
	ComparatorAllOtherEligibleReports = "all_other_eligible_reports"
	EventScopeAllRecordedSourcePTs    = "all_recorded_source_pts"
	ThresholdProfileNone              = "none"
	ThresholdProfileEvansEducational  = "evans-educational-v1"

	MetricInputObserved                 = "observed"
	MetricInputHaldaneAnscombeCorrected = "haldane_anscombe_corrected"
	ZeroCellCorrectionNone              = "none"
	ZeroCellCorrectionHaldaneAnscombe   = "haldane_anscombe"

	// Result-family identifiers are machine-readable contract values. The
	// digest binds these declarations together with the exact ordered rows; it
	// does not by itself prove upstream dataset or scientific completeness.
	ResultFamilyEmittedProtocolRows = "normalized_protocol_emitted_rows_v1"
	ResultFamilyMembershipRule      = "all_matching_aggregate_events_emitted_without_ranking_or_top_n"
	ResultFamilyRowUnit             = "unique_drug_event_pair"
	ResultFamilyRowOrder            = "event_concept_id_asc_utf8_binary_then_drug_concept_id_asc_utf8_binary"
	ResultDigestAlgorithm           = "sha256_canonical_json_v1"
)

// DatasetManifest describes a frozen, checksummed dataset and the processing
// decisions needed to interpret results produced from it.
type DatasetManifest struct {
	SchemaVersion string                `json:"schema_version"`
	DatasetID     string                `json:"dataset_id"`
	Title         string                `json:"title"`
	Description   string                `json:"description,omitempty"`
	Source        DatasetSource         `json:"source"`
	Coverage      DatasetCoverage       `json:"coverage"`
	Processing    DatasetProcessing     `json:"processing"`
	Vocabularies  []VocabularyReference `json:"vocabularies,omitempty"`
	Artifacts     []DatasetArtifact     `json:"artifacts"`
	Completeness  DatasetCompleteness   `json:"completeness"`
	Limitations   []string              `json:"limitations"`
}

type DatasetSource struct {
	Name        string       `json:"name"`
	Publisher   string       `json:"publisher"`
	LandingPage string       `json:"landing_page"`
	RetrievedAt string       `json:"retrieved_at"`
	License     string       `json:"license,omitempty"`
	Files       []SourceFile `json:"files"`
}

type SourceFile struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type DatasetCoverage struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Geography string `json:"geography"`
	Release   string `json:"release"`
}

type DatasetProcessing struct {
	PipelineVersion     string   `json:"pipeline_version"`
	SourceCommit        string   `json:"source_commit"`
	MaterializerCommit  string   `json:"materializer_commit,omitempty"`
	DeduplicationPolicy string   `json:"deduplication_policy"`
	CountUnit           string   `json:"count_unit"`
	DrugRolePolicy      string   `json:"drug_role_policy"`
	Exclusions          []string `json:"exclusions"`
}

type VocabularyReference struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Scope   string `json:"scope"`
	License string `json:"license,omitempty"`
}

type DatasetArtifact struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	Bytes     int64  `json:"bytes"`
}

type DatasetCompleteness struct {
	// SourceDEMORows counts raw DEMO rows and therefore includes superseded
	// case versions. CurrentCaseReports is the retained one-version-per-CASEID
	// population. EligibleReports is the narrower analysis denominator: retained
	// reports with at least one drug-role/event pair.
	SourceDEMORows     int64               `json:"source_demo_rows"`
	CurrentCaseReports int64               `json:"current_case_reports"`
	EligibleReports    int64               `json:"eligible_reports"`
	DrugEventPairs     int64               `json:"drug_event_pairs"`
	Fields             []FieldCompleteness `json:"fields,omitempty"`
}

type FieldCompleteness struct {
	Field              string  `json:"field"`
	Population         string  `json:"population"`
	DenominatorRecords int64   `json:"denominator_records"`
	MissingRecords     int64   `json:"missing_records"`
	MissingPercent     float64 `json:"missing_percent"`
}

// AnalysisRequest is the complete, hashable research protocol submitted by a
// client. Methods and subgroup values are set-like and are normalized before the
// analysis identifier is computed.
type AnalysisRequest struct {
	SchemaVersion      string              `json:"schema_version"`
	DatasetID          string              `json:"dataset_id"`
	DrugConceptID      string              `json:"drug_concept_id"`
	DrugRole           string              `json:"drug_role"`
	EventScope         string              `json:"event_scope"`
	Comparator         string              `json:"comparator"`
	Period             AnalysisPeriod      `json:"period"`
	Subgroups          []SubgroupSelection `json:"subgroups,omitempty"`
	Methods            []string            `json:"methods"`
	ThresholdProfileID string              `json:"threshold_profile_id"`
}

type AnalysisPeriod struct {
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

type SubgroupSelection struct {
	Dimension string   `json:"dimension"`
	Values    []string `json:"values"`
}

// AnalysisResult binds the normalized protocol to the exact dataset manifest and
// software revision. AnalysisID addresses those inputs. ResultDigest separately
// binds the declared result family, RowCount, and exact canonical row sequence.
// The digest proves internal integrity of the emitted family, not completeness of
// upstream source data or scientific validity. A wall-clock generation time is
// deliberately omitted so equivalent executions can produce byte-identical output.
type AnalysisResult struct {
	SchemaVersion   string                 `json:"schema_version"`
	AnalysisID      string                 `json:"analysis_id"`
	ResultDigest    string                 `json:"result_digest"`
	RowCount        int64                  `json:"row_count"`
	ResultFamily    ResultFamilyDefinition `json:"result_family"`
	Software        SoftwareReference      `json:"software"`
	Dataset         DatasetReference       `json:"dataset"`
	DatasetManifest DatasetManifest        `json:"dataset_manifest"`
	Request         AnalysisRequest        `json:"request"`
	Rows            []DrugEventResult      `json:"rows"`
	Caveats         []string               `json:"caveats"`
}

// ResultFamilyDefinition makes the hypothesis-family boundary and canonical
// ordering explicit. Values are versioned enumerations rather than prose so a
// future semantic change cannot retain the same result digest accidentally.
type ResultFamilyDefinition struct {
	FamilyID        string `json:"family_id"`
	MembershipRule  string `json:"membership_rule"`
	RowUnit         string `json:"row_unit"`
	RowOrder        string `json:"row_order"`
	DigestAlgorithm string `json:"digest_algorithm"`
}

// SoftwareReference binds a result to the exact application implementation.
// A release version alone is insufficient because two commits can otherwise
// produce different bytes under the same analysis identifier.
type SoftwareReference struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

type DatasetReference struct {
	DatasetID      string              `json:"dataset_id"`
	ManifestSHA256 string              `json:"manifest_sha256"`
	Coverage       DatasetCoverage     `json:"coverage"`
	Completeness   DatasetCompleteness `json:"completeness"`
}

type DrugEventResult struct {
	DrugConceptID  string           `json:"drug_concept_id"`
	EventConceptID string           `json:"event_concept_id"`
	EventTerm      string           `json:"event_term"`
	EventCategory  string           `json:"event_category"`
	Table          IntegerTable     `json:"contingency_table"`
	Metrics        []MetricEstimate `json:"metrics"`
	ReviewFlags    []ReviewFlag     `json:"review_flags,omitempty"`
}

type IntegerTable struct {
	A int64 `json:"a"`
	B int64 `json:"b"`
	C int64 `json:"c"`
	D int64 `json:"d"`
	N int64 `json:"n"`
}

// MetricEstimate uses pointers for optional inferential quantities so an absent
// value cannot be confused with a scientifically meaningful zero.
type MetricEstimate struct {
	Method      string            `json:"method"`
	Measure     string            `json:"measure"`
	Estimate    float64           `json:"estimate"`
	Lower95     *float64          `json:"lower_95,omitempty"`
	Upper95     *float64          `json:"upper_95,omitempty"`
	PValue      *float64          `json:"p_value,omitempty"`
	QValue      *float64          `json:"q_value,omitempty"`
	Calculation MetricCalculation `json:"calculation"`
}

// MetricCalculation records which cells actually entered a metric. PRR and
// ROR can use a Haldane-Anscombe-adjusted table when any observed cell is zero;
// Fisher exact always uses the observed integer cells. Keeping this metadata on
// each metric prevents a corrected estimate from being mistaken for an
// uncorrected one after results are detached from the surrounding methods text.
type MetricCalculation struct {
	InputCells         string             `json:"input_cells"`
	ZeroCellCorrection ZeroCellCorrection `json:"zero_cell_correction"`
}

// ZeroCellCorrection is explicit even when no correction was applied. A false
// value therefore means an audited choice of observed cells, not missing data.
type ZeroCellCorrection struct {
	Applied         bool    `json:"applied"`
	Method          string  `json:"method"`
	AddedToEachCell float64 `json:"added_to_each_cell"`
}

// ReviewFlag is a protocol-specific screening outcome, not a causal, clinical,
// or regulatory conclusion.
type ReviewFlag struct {
	ProfileID string `json:"profile_id"`
	Outcome   string `json:"outcome"`
	Reason    string `json:"reason"`
}

// AnalysisRecord is the immutable unit persisted by FileStore. Its directory is
// named by the protocol AnalysisID and the embedded result must repeat that ID;
// FileStore additionally validates row invariants and artifact checksums on read.
// Extra files may contain deterministic CSV, Parquet, scripts, or notebooks
// generated elsewhere.
type AnalysisRecord struct {
	Dataset DatasetManifest `json:"dataset"`
	Result  AnalysisResult  `json:"result"`
	Files   []ExportFile    `json:"-"`
}

type ExportFile struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Data      []byte `json:"-"`
}
