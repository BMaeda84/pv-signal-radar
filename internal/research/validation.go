package research

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BMaeda84/pv-signal-radar/internal/stats"
)

var (
	safeIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,126}[a-z0-9])?$`)
	sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	// Public provenance must use an unambiguous object ID. GitHub currently
	// serves SHA-1 repositories (40 hex); SHA-256 repositories use 64 hex.
	// Abbreviated revisions are intentionally rejected because they can become
	// ambiguous as history grows and therefore cannot identify immutable inputs.
	sourceCommitPattern = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)
)

var allowedDrugRoles = stringSet("primary_suspect", "secondary_suspect", "concomitant", "interacting", "suspect", "all")
var allowedEventScopes = stringSet(EventScopeAllRecordedSourcePTs)
var allowedComparators = stringSet(ComparatorAllOtherEligibleReports)
var allowedMethods = stringSet("prr", "ror", "fisher_exact", "bcpnn_ic", "gps_ebgm")
var allowedSubgroups = stringSet("age_group", "sex", "country", "seriousness", "calendar_period")
var allowedThresholdProfiles = stringSet(ThresholdProfileNone, ThresholdProfileEvansEducational)

const (
	// Closed-form PRR/ROR implementations agree with pvda/R materially more
	// tightly than this bound. Keeping an explicit tolerance permits equivalent
	// reference-engine serialization without accepting a changed estimate.
	effectMetricRelativeTolerance = 1e-10
	// Fisher's lgamma normalization was independently benchmarked against R up
	// to N=1e9 with <1 ppm drift; 2 ppm is the declared validation boundary.
	fisherMetricRelativeTolerance = 2e-6
	fdrRelativeTolerance          = 1e-12
)

func ValidateSoftwareReference(software SoftwareReference) error {
	var problems []error
	if software.Name != "pv-signal-radar" {
		problems = append(problems, errors.New("software name must be pv-signal-radar"))
	}
	if strings.TrimSpace(software.Version) == "" || len(software.Version) > 64 {
		problems = append(problems, errors.New("software version is required and must not exceed 64 characters"))
	}
	if !sourceCommitPattern.MatchString(software.Commit) {
		problems = append(problems, errors.New("software commit must be a lowercase hexadecimal Git revision"))
	}
	return errors.Join(problems...)
}

// ValidateDatasetManifest rejects incomplete provenance rather than allowing a
// dataset to appear research-ready without frozen source files and artifacts.
func ValidateDatasetManifest(m DatasetManifest) error {
	var problems []error
	if m.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Errorf("schema_version must be %q", SchemaVersion))
	}
	if !safeIDPattern.MatchString(m.DatasetID) {
		problems = append(problems, errors.New("dataset_id must be a lowercase path-safe identifier"))
	}
	if strings.TrimSpace(m.Title) == "" {
		problems = append(problems, errors.New("title is required"))
	}
	if strings.TrimSpace(m.Source.Name) == "" || strings.TrimSpace(m.Source.Publisher) == "" {
		problems = append(problems, errors.New("source name and publisher are required"))
	}
	if err := validateHTTPURL(m.Source.LandingPage); err != nil {
		problems = append(problems, fmt.Errorf("source landing_page: %w", err))
	}
	if _, err := time.Parse(time.RFC3339, m.Source.RetrievedAt); err != nil {
		problems = append(problems, errors.New("source retrieved_at must be RFC3339"))
	}
	if len(m.Source.Files) == 0 {
		problems = append(problems, errors.New("at least one frozen source file is required"))
	}
	for i, file := range m.Source.Files {
		if err := validateHTTPURL(file.URL); err != nil {
			problems = append(problems, fmt.Errorf("source.files[%d].url: %w", i, err))
		}
		if !sha256Pattern.MatchString(file.SHA256) {
			problems = append(problems, fmt.Errorf("source.files[%d].sha256 must be lowercase SHA-256", i))
		}
		if file.Bytes < 0 {
			problems = append(problems, fmt.Errorf("source.files[%d].bytes cannot be negative", i))
		}
	}
	start, startErr := parseDate(m.Coverage.StartDate)
	end, endErr := parseDate(m.Coverage.EndDate)
	if startErr != nil || endErr != nil {
		problems = append(problems, errors.New("coverage start_date and end_date must use YYYY-MM-DD"))
	} else if end.Before(start) {
		problems = append(problems, errors.New("coverage end_date precedes start_date"))
	}
	if strings.TrimSpace(m.Coverage.Geography) == "" || strings.TrimSpace(m.Coverage.Release) == "" {
		problems = append(problems, errors.New("coverage geography and release are required"))
	}
	if strings.TrimSpace(m.Processing.PipelineVersion) == "" || strings.TrimSpace(m.Processing.SourceCommit) == "" {
		problems = append(problems, errors.New("processing pipeline_version and source_commit are required"))
	}
	if !sourceCommitPattern.MatchString(m.Processing.SourceCommit) {
		problems = append(problems, errors.New("processing source_commit must be a lowercase hexadecimal commit identifier"))
	}
	if m.Processing.MaterializerCommit != "" && !sourceCommitPattern.MatchString(m.Processing.MaterializerCommit) {
		problems = append(problems, errors.New("processing materializer_commit must be a lowercase hexadecimal commit identifier when present"))
	}
	if strings.TrimSpace(m.Processing.DeduplicationPolicy) == "" || strings.TrimSpace(m.Processing.CountUnit) == "" || strings.TrimSpace(m.Processing.DrugRolePolicy) == "" {
		problems = append(problems, errors.New("processing policies for deduplication, count unit, and drug role are required"))
	}
	if len(m.Artifacts) == 0 {
		problems = append(problems, errors.New("at least one generated artifact is required"))
	}
	seenArtifacts := make(map[string]struct{}, len(m.Artifacts))
	for i, artifact := range m.Artifacts {
		if strings.TrimSpace(artifact.Name) == "" || strings.TrimSpace(artifact.MediaType) == "" {
			problems = append(problems, fmt.Errorf("artifacts[%d] name and media_type are required", i))
		}
		if err := ValidateRelativePath(artifact.Path); err != nil {
			problems = append(problems, fmt.Errorf("artifacts[%d].path: %w", i, err))
		}
		if _, duplicate := seenArtifacts[artifact.Path]; duplicate {
			problems = append(problems, fmt.Errorf("duplicate artifact path %q", artifact.Path))
		}
		seenArtifacts[artifact.Path] = struct{}{}
		if !sha256Pattern.MatchString(artifact.SHA256) {
			problems = append(problems, fmt.Errorf("artifacts[%d].sha256 must be lowercase SHA-256", i))
		}
		if artifact.Bytes < 0 {
			problems = append(problems, fmt.Errorf("artifacts[%d].bytes cannot be negative", i))
		}
	}
	if m.Completeness.SourceDEMORows < 0 || m.Completeness.CurrentCaseReports < 0 ||
		m.Completeness.EligibleReports < 0 || m.Completeness.DrugEventPairs < 0 {
		problems = append(problems, errors.New("completeness counts cannot be negative"))
	}
	if m.Completeness.CurrentCaseReports > m.Completeness.SourceDEMORows {
		problems = append(problems, errors.New("current_case_reports cannot exceed source_demo_rows"))
	}
	if m.Completeness.EligibleReports > m.Completeness.CurrentCaseReports {
		problems = append(problems, errors.New("eligible_reports cannot exceed current_case_reports"))
	}
	if m.Completeness.DrugEventPairs < m.Completeness.EligibleReports {
		problems = append(problems, errors.New("drug_event_pairs cannot be fewer than eligible_reports"))
	}
	for i, field := range m.Completeness.Fields {
		if strings.TrimSpace(field.Field) == "" || field.Population != "eligible_reports" ||
			field.DenominatorRecords != m.Completeness.EligibleReports || field.MissingRecords < 0 ||
			field.MissingRecords > field.DenominatorRecords || !finiteRange(field.MissingPercent, 0, 100) {
			problems = append(problems, fmt.Errorf("completeness.fields[%d] is invalid", i))
			continue
		}
		expectedPercent := 0.0
		if field.DenominatorRecords > 0 {
			expectedPercent = 100 * float64(field.MissingRecords) / float64(field.DenominatorRecords)
		}
		// The R artifact rounds percentages to six decimal places, while other
		// conforming producers may retain more precision. Half a unit at the
		// sixth decimal is therefore the largest accepted serialization drift.
		if math.Abs(field.MissingPercent-expectedPercent) > 0.0000005 {
			problems = append(problems, fmt.Errorf("completeness.fields[%d].missing_percent does not match its eligible-report denominator", i))
		}
	}
	if len(m.Limitations) == 0 {
		problems = append(problems, errors.New("at least one dataset limitation is required"))
	}
	return errors.Join(problems...)
}

// NormalizeAnalysisRequest validates and canonicalizes a request. Ordering of
// methods, subgroup dimensions, subgroup values, and duplicates cannot change
// the resulting analysis identifier.
func NormalizeAnalysisRequest(request AnalysisRequest) (AnalysisRequest, error) {
	request.SchemaVersion = strings.TrimSpace(request.SchemaVersion)
	request.DatasetID = strings.TrimSpace(request.DatasetID)
	request.DrugConceptID = strings.TrimSpace(request.DrugConceptID)
	request.DrugRole = strings.ToLower(strings.TrimSpace(request.DrugRole))
	request.EventScope = strings.ToLower(strings.TrimSpace(request.EventScope))
	request.Comparator = strings.ToLower(strings.TrimSpace(request.Comparator))
	request.ThresholdProfileID = strings.TrimSpace(request.ThresholdProfileID)

	var problems []error
	if request.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Errorf("schema_version must be %q", SchemaVersion))
	}
	if !safeIDPattern.MatchString(request.DatasetID) {
		problems = append(problems, errors.New("dataset_id must be a lowercase path-safe identifier"))
	}
	if request.DrugConceptID == "" || len(request.DrugConceptID) > 256 {
		problems = append(problems, errors.New("drug_concept_id is required and must not exceed 256 characters"))
	}
	if !allowedDrugRoles[request.DrugRole] {
		problems = append(problems, fmt.Errorf("unsupported drug_role %q", request.DrugRole))
	}
	if !allowedEventScopes[request.EventScope] {
		problems = append(problems, fmt.Errorf("unsupported event_scope %q", request.EventScope))
	}
	if !allowedComparators[request.Comparator] {
		problems = append(problems, fmt.Errorf("unsupported comparator %q", request.Comparator))
	}
	if !allowedThresholdProfiles[request.ThresholdProfileID] {
		// A named but undefined profile is not reproducible: its thresholds and
		// outcome vocabulary are absent from the request hash. New profiles must
		// therefore be registered as versioned code/contracts before acceptance.
		problems = append(problems, fmt.Errorf("unsupported threshold_profile_id %q", request.ThresholdProfileID))
	}
	if request.Period.StartDate != "" {
		if _, err := parseDate(request.Period.StartDate); err != nil {
			problems = append(problems, errors.New("period.start_date must use YYYY-MM-DD"))
		}
	}
	if request.Period.EndDate != "" {
		if _, err := parseDate(request.Period.EndDate); err != nil {
			problems = append(problems, errors.New("period.end_date must use YYYY-MM-DD"))
		}
	}
	if request.Period.StartDate != "" && request.Period.EndDate != "" {
		start, startErr := parseDate(request.Period.StartDate)
		end, endErr := parseDate(request.Period.EndDate)
		if startErr == nil && endErr == nil && end.Before(start) {
			problems = append(problems, errors.New("period.end_date precedes period.start_date"))
		}
	}

	methodSet := make(map[string]struct{}, len(request.Methods))
	for _, raw := range request.Methods {
		method := strings.ToLower(strings.TrimSpace(raw))
		if !allowedMethods[method] {
			problems = append(problems, fmt.Errorf("unsupported method %q", raw))
			continue
		}
		methodSet[method] = struct{}{}
	}
	request.Methods = sortedKeys(methodSet)
	if len(request.Methods) == 0 {
		problems = append(problems, errors.New("at least one supported method is required"))
	}

	subgroups := make(map[string]map[string]struct{}, len(request.Subgroups))
	for _, subgroup := range request.Subgroups {
		dimension := strings.ToLower(strings.TrimSpace(subgroup.Dimension))
		if !allowedSubgroups[dimension] {
			problems = append(problems, fmt.Errorf("unsupported subgroup dimension %q", subgroup.Dimension))
			continue
		}
		if subgroups[dimension] == nil {
			subgroups[dimension] = make(map[string]struct{})
		}
		for _, raw := range subgroup.Values {
			value := strings.TrimSpace(raw)
			if value == "" || len(value) > 128 {
				problems = append(problems, fmt.Errorf("subgroup %q contains an invalid value", dimension))
				continue
			}
			subgroups[dimension][value] = struct{}{}
		}
		if len(subgroups[dimension]) == 0 {
			problems = append(problems, fmt.Errorf("subgroup %q requires at least one value", dimension))
		}
	}
	normalizedSubgroups := make([]SubgroupSelection, 0, len(subgroups))
	for dimension, values := range subgroups {
		normalizedSubgroups = append(normalizedSubgroups, SubgroupSelection{Dimension: dimension, Values: sortedKeys(values)})
	}
	sort.Slice(normalizedSubgroups, func(i, j int) bool { return normalizedSubgroups[i].Dimension < normalizedSubgroups[j].Dimension })
	if len(normalizedSubgroups) == 0 {
		// Canonicalize the empty set to nil so a JSON persistence round trip does
		// not distinguish an omitted subgroup list from an explicitly empty one.
		normalizedSubgroups = nil
	}
	request.Subgroups = normalizedSubgroups

	if err := errors.Join(problems...); err != nil {
		return AnalysisRequest{}, err
	}
	return request, nil
}

// ValidateRelativePath enforces a portable slash-separated path inside a
// dataset or analysis root. Backslashes and colons are rejected so a path that
// is harmless on Unix cannot become traversal or a volume path on Windows.
func ValidateRelativePath(name string) error {
	if name == "" || strings.TrimSpace(name) != name {
		return errors.New("path is required and cannot contain surrounding whitespace")
	}
	if strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return errors.New("path must be a portable relative path")
	}
	clean := path.Clean(name)
	if clean != name || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return errors.New("path traversal is not allowed")
	}
	for _, segment := range strings.Split(clean, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("path contains an invalid segment")
		}
	}
	return nil
}

// ValidateAnalysisResult checks that the result is internally coherent and
// still bound to the manifest and normalized protocol from which it was derived.
func ValidateAnalysisResult(manifest DatasetManifest, result AnalysisResult) error {
	var problems []error
	if err := ValidateSoftwareReference(result.Software); err != nil {
		problems = append(problems, fmt.Errorf("software: %w", err))
	}
	if err := ValidateDatasetManifest(manifest); err != nil {
		problems = append(problems, fmt.Errorf("dataset: %w", err))
	}
	normalized, requestErr := NormalizeAnalysisRequest(result.Request)
	if requestErr != nil {
		problems = append(problems, fmt.Errorf("request: %w", requestErr))
	} else if !analysisRequestsEqual(normalized, result.Request) {
		problems = append(problems, errors.New("request is not in canonical normalized form"))
	} else if err := validateAnalysisRequestAgainstManifest(manifest, normalized); err != nil {
		problems = append(problems, fmt.Errorf("request does not match dataset manifest: %w", err))
	}
	if result.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Errorf("schema_version must be %q", SchemaVersion))
	}
	expectedFamily := CanonicalResultFamily()
	if result.ResultFamily != expectedFamily {
		problems = append(problems, errors.New("result_family does not match the canonical emitted-family contract"))
	}
	if result.RowCount != int64(len(result.Rows)) {
		problems = append(problems, fmt.Errorf("row_count %d does not match %d emitted rows", result.RowCount, len(result.Rows)))
	}
	if !sha256Pattern.MatchString(result.ResultDigest) {
		problems = append(problems, errors.New("result_digest must be a lowercase SHA-256"))
	} else if expectedDigest, err := CanonicalResultDigest(result.ResultFamily, result.Rows); err != nil {
		problems = append(problems, fmt.Errorf("derive result_digest: %w", err))
	} else if result.ResultDigest != expectedDigest {
		problems = append(problems, errors.New("result_digest does not match result_family, row_count, and exact ordered rows"))
	}
	if err := validateCanonicalResultRowOrder(result.Rows); err != nil {
		problems = append(problems, err)
	}
	manifestHash, manifestHashErr := DatasetManifestHash(manifest)
	if manifestHashErr == nil {
		embeddedManifestHash, err := DatasetManifestHash(result.DatasetManifest)
		if err != nil {
			problems = append(problems, fmt.Errorf("dataset_manifest: %w", err))
		} else if embeddedManifestHash != manifestHash {
			problems = append(problems, errors.New("dataset_manifest does not match the analysis manifest"))
		}
		if result.Dataset.DatasetID != manifest.DatasetID || result.Dataset.ManifestSHA256 != manifestHash {
			problems = append(problems, errors.New("dataset reference does not match manifest"))
		}
		if result.Dataset.Coverage != manifest.Coverage {
			problems = append(problems, errors.New("result coverage does not match manifest"))
		}
		if !datasetCompletenessEqual(result.Dataset.Completeness, manifest.Completeness) {
			problems = append(problems, errors.New("result completeness does not match manifest"))
		}
	}
	if requestErr == nil && manifestHashErr == nil {
		expectedID, err := AnalysisIDForSoftware(manifest, normalized, result.Software)
		if err != nil {
			// Never discard a derivation failure. In particular, a request whose
			// dataset_id differs from the manifest cannot be allowed to bypass the
			// content-address check merely because no expected ID can be produced.
			problems = append(problems, fmt.Errorf("derive analysis_id: %w", err))
		} else if result.AnalysisID != expectedID {
			problems = append(problems, errors.New("analysis_id does not match manifest and request"))
		}
	}
	if len(result.Caveats) == 0 {
		problems = append(problems, errors.New("at least one analysis caveat is required"))
	}
	if len(result.Rows) == 0 {
		problems = append(problems, errors.New("analysis result requires at least one drug-event row"))
	}
	seenPairs := make(map[string]struct{}, len(result.Rows))
	for rowIndex, row := range result.Rows {
		if !validUTF8Strings(row.DrugConceptID, row.EventConceptID, row.EventTerm, row.EventCategory) {
			problems = append(problems, fmt.Errorf("rows[%d] text fields must contain valid UTF-8", rowIndex))
		}
		if strings.TrimSpace(row.DrugConceptID) == "" || strings.TrimSpace(row.EventConceptID) == "" || strings.TrimSpace(row.EventTerm) == "" || strings.TrimSpace(row.EventCategory) == "" {
			problems = append(problems, fmt.Errorf("rows[%d] identifiers, term, and category are required", rowIndex))
		}
		pair := row.DrugConceptID + "\x00" + row.EventConceptID
		if _, duplicate := seenPairs[pair]; duplicate {
			problems = append(problems, fmt.Errorf("rows[%d] duplicates drug-event pair", rowIndex))
		}
		seenPairs[pair] = struct{}{}
		if err := validateIntegerTable(row.Table); err != nil {
			problems = append(problems, fmt.Errorf("rows[%d].contingency_table: %w", rowIndex, err))
		}
		// The manifest's eligible_reports field is the declared statistical
		// denominator for every emitted 2x2 table. Binding N here protects stored
		// records and direct exports even if they did not pass through SQLite.
		if row.Table.N != manifest.Completeness.EligibleReports {
			problems = append(problems, fmt.Errorf(
				"rows[%d].contingency_table.n %d does not match manifest eligible_reports %d",
				rowIndex,
				row.Table.N,
				manifest.Completeness.EligibleReports,
			))
		}
		if len(row.Metrics) == 0 {
			problems = append(problems, fmt.Errorf("rows[%d] requires at least one metric", rowIndex))
		}
		seenMetrics := make(map[string]struct{}, len(row.Metrics))
		for metricIndex, metric := range row.Metrics {
			if !validUTF8Strings(metric.Method, metric.Measure, metric.Calculation.InputCells, metric.Calculation.ZeroCellCorrection.Method) {
				problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] text fields must contain valid UTF-8", rowIndex, metricIndex))
			}
			key := metric.Method + "\x00" + metric.Measure
			if strings.TrimSpace(metric.Method) == "" || strings.TrimSpace(metric.Measure) == "" {
				problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] method and measure are required", rowIndex, metricIndex))
			}
			if _, duplicate := seenMetrics[key]; duplicate {
				problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] duplicates method/measure", rowIndex, metricIndex))
			}
			seenMetrics[key] = struct{}{}
			if !finite(metric.Estimate) || !optionalFinite(metric.Lower95) || !optionalFinite(metric.Upper95) {
				problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] contains a non-finite estimate or interval", rowIndex, metricIndex))
			}
			if (metric.Lower95 == nil) != (metric.Upper95 == nil) {
				problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] must provide both confidence bounds or neither", rowIndex, metricIndex))
			} else if metric.Lower95 != nil && metric.Upper95 != nil {
				if *metric.Lower95 > *metric.Upper95 {
					problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] lower interval exceeds upper interval", rowIndex, metricIndex))
				} else if metric.Estimate < *metric.Lower95 || metric.Estimate > *metric.Upper95 {
					problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] estimate lies outside its confidence interval", rowIndex, metricIndex))
				}
			}
			if !optionalProbability(metric.PValue) || !optionalProbability(metric.QValue) {
				problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] p_value and q_value must be in [0,1]", rowIndex, metricIndex))
			}
			if metric.QValue != nil && metric.PValue == nil {
				problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] q_value requires p_value", rowIndex, metricIndex))
			} else if metric.QValue != nil && *metric.QValue < *metric.PValue && !nearlyEqual(*metric.QValue, *metric.PValue, fdrRelativeTolerance, 0) {
				problems = append(problems, fmt.Errorf("rows[%d].metrics[%d] Benjamini-Hochberg q_value is below p_value", rowIndex, metricIndex))
			}
			if err := validateMetricCalculation(metric.Method, metric.Calculation); err != nil {
				problems = append(problems, fmt.Errorf("rows[%d].metrics[%d].calculation: %w", rowIndex, metricIndex, err))
			}
		}
		for flagIndex, flag := range row.ReviewFlags {
			if !validUTF8Strings(flag.ProfileID, flag.Outcome, flag.Reason) {
				problems = append(problems, fmt.Errorf("rows[%d].review_flags[%d] text fields must contain valid UTF-8", rowIndex, flagIndex))
			}
			if strings.TrimSpace(flag.ProfileID) == "" || strings.TrimSpace(flag.Outcome) == "" || strings.TrimSpace(flag.Reason) == "" {
				problems = append(problems, fmt.Errorf("rows[%d].review_flags[%d] is incomplete", rowIndex, flagIndex))
			}
		}
		if requestErr == nil {
			if err := validateRowAgainstProtocol(row, normalized); err != nil {
				problems = append(problems, fmt.Errorf("rows[%d] does not match request protocol: %w", rowIndex, err))
			}
		}
	}
	if requestErr == nil {
		if err := validateFisherFDR(result.Rows, normalized.Methods); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func validateCanonicalResultRowOrder(rows []DrugEventResult) error {
	for index := 1; index < len(rows); index++ {
		previous := rows[index-1]
		current := rows[index]
		if previous.EventConceptID > current.EventConceptID ||
			(previous.EventConceptID == current.EventConceptID && previous.DrugConceptID >= current.DrugConceptID) {
			return fmt.Errorf("rows[%d] is not in canonical %s order", index, ResultFamilyRowOrder)
		}
	}
	return nil
}

func validUTF8Strings(values ...string) bool {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return false
		}
	}
	return true
}

func validateRowAgainstProtocol(row DrugEventResult, request AnalysisRequest) error {
	var problems []error
	if row.DrugConceptID != request.DrugConceptID {
		problems = append(problems, fmt.Errorf("drug_concept_id %q does not match request %q", row.DrugConceptID, request.DrugConceptID))
	}
	if len(row.Metrics) != len(request.Methods) {
		problems = append(problems, fmt.Errorf("metric methods %v do not exactly match requested methods %v", metricMethods(row.Metrics), request.Methods))
	} else {
		for index, method := range request.Methods {
			if row.Metrics[index].Method != method {
				problems = append(problems, fmt.Errorf("metric order/methods %v do not match canonical requested methods %v", metricMethods(row.Metrics), request.Methods))
				break
			}
		}
	}
	if err := validateIntegerTable(row.Table); err != nil {
		return errors.Join(append(problems, err)...)
	}

	// These margins are safe after validateIntegerTable's checked total. The Go
	// implementation can independently recompute closed-form effects and, when
	// its online work bound permits, Fisher exact. Bayesian batch methods remain
	// structurally validated only; substituting a simplified estimator here
	// would weaken rather than strengthen the trust boundary.
	table, err := stats.NewContingencyTable(
		row.Table.A,
		row.Table.A+row.Table.B,
		row.Table.A+row.Table.C,
		row.Table.N,
	)
	if err != nil {
		return errors.Join(append(problems, fmt.Errorf("cannot reproduce contingency table: %w", err))...)
	}
	includeFisher := containsMethod(request.Methods, "fisher_exact")
	calculated := table.Calculate(row.DrugConceptID, row.EventTerm)
	if includeFisher {
		calculated = table.CalculateWithFisher(row.DrugConceptID, row.EventTerm)
	}

	for _, metric := range row.Metrics {
		switch metric.Method {
		case "prr":
			problems = append(problems, validateEffectMetric(metric, "reporting_ratio", calculated.PRR, calculated.PRRLower95, calculated.PRRUpper95, expectedEffectCalculation(calculated))...)
		case "ror":
			problems = append(problems, validateEffectMetric(metric, "reporting_odds_ratio", calculated.ROR, calculated.RORLower95, calculated.RORUpper95, expectedEffectCalculation(calculated))...)
		case "fisher_exact":
			problems = append(problems, validateFisherMetric(metric, calculated)...)
		case "bcpnn_ic", "gps_ebgm":
			// The runtime has no independently validated Go implementation for
			// these estimators. Their presence/order and finite metadata are still
			// checked, but numerical equivalence remains a batch evidence duty.
		}
	}

	if request.ThresholdProfileID == ThresholdProfileNone {
		if len(row.ReviewFlags) != 0 {
			problems = append(problems, errors.New("threshold_profile_id none forbids review_flags"))
		}
	} else {
		if len(row.ReviewFlags) != 1 {
			problems = append(problems, fmt.Errorf("threshold profile %q requires exactly one review flag", request.ThresholdProfileID))
		} else {
			flag := row.ReviewFlags[0]
			if flag.ProfileID != request.ThresholdProfileID {
				problems = append(problems, fmt.Errorf("review flag profile %q does not match request %q", flag.ProfileID, request.ThresholdProfileID))
			}
			if request.ThresholdProfileID == ThresholdProfileEvansEducational {
				expected := screeningOutcomeValue(calculated.ScreeningOutcome)
				if flag.Outcome != expected {
					problems = append(problems, fmt.Errorf("Evans review outcome %q does not match recomputed outcome %q", flag.Outcome, expected))
				}
				expectedReason := requestedReviewFlags(ThresholdProfileEvansEducational, calculated)[0].Reason
				if flag.Reason != expectedReason {
					problems = append(problems, errors.New("Evans review reason does not match the recomputed table and statistics"))
				}
			}
		}
	}
	return errors.Join(problems...)
}

func validateEffectMetric(metric MetricEstimate, measure string, estimate, lower, upper float64, calculation MetricCalculation) []error {
	var problems []error
	if metric.Measure != measure {
		problems = append(problems, fmt.Errorf("%s measure %q must be %q", metric.Method, metric.Measure, measure))
	}
	if metric.PValue != nil || metric.QValue != nil {
		problems = append(problems, fmt.Errorf("%s effect metric cannot attach Fisher/BH probabilities", metric.Method))
	}
	if metric.Lower95 == nil || metric.Upper95 == nil {
		problems = append(problems, fmt.Errorf("%s requires a two-sided 95%% confidence interval", metric.Method))
	} else {
		if !nearlyEqual(*metric.Lower95, lower, effectMetricRelativeTolerance, 0) {
			problems = append(problems, fmt.Errorf("%s lower_95 does not match recomputation", metric.Method))
		}
		if !nearlyEqual(*metric.Upper95, upper, effectMetricRelativeTolerance, 0) {
			problems = append(problems, fmt.Errorf("%s upper_95 does not match recomputation", metric.Method))
		}
	}
	if !nearlyEqual(metric.Estimate, estimate, effectMetricRelativeTolerance, 0) {
		problems = append(problems, fmt.Errorf("%s estimate does not match recomputation", metric.Method))
	}
	if metric.Calculation != calculation {
		problems = append(problems, fmt.Errorf("%s cell-correction metadata does not match the observed table", metric.Method))
	}
	return problems
}

func validateFisherMetric(metric MetricEstimate, calculated stats.DisproportionalityResult) []error {
	var problems []error
	if metric.Measure != "two_sided_probability_ordering" {
		problems = append(problems, fmt.Errorf("fisher_exact measure %q must be two_sided_probability_ordering", metric.Measure))
	}
	if metric.Lower95 != nil || metric.Upper95 != nil {
		problems = append(problems, errors.New("fisher_exact does not use confidence interval fields"))
	}
	if metric.PValue == nil {
		problems = append(problems, errors.New("fisher_exact requires p_value"))
	} else {
		if !nearlyEqual(metric.Estimate, *metric.PValue, fdrRelativeTolerance, 0) {
			problems = append(problems, errors.New("fisher_exact estimate must equal p_value"))
		}
		if calculated.FisherExactOK && !nearlyEqual(*metric.PValue, calculated.FisherExactP, fisherMetricRelativeTolerance, 0) {
			problems = append(problems, errors.New("fisher_exact p_value does not match independent Go recomputation"))
		}
	}
	if metric.QValue == nil {
		problems = append(problems, errors.New("fisher_exact requires Benjamini-Hochberg q_value across returned rows"))
	}
	if metric.Calculation != observedMetricCalculation() {
		problems = append(problems, errors.New("fisher_exact must declare observed, uncorrected input cells"))
	}
	return problems
}

func validateFisherFDR(rows []DrugEventResult, methods []string) error {
	if !containsMethod(methods, "fisher_exact") {
		return nil
	}
	type reference struct {
		row int
		p   float64
		q   *float64
	}
	values := make([]reference, 0, len(rows))
	for rowIndex, row := range rows {
		var fisher *MetricEstimate
		for metricIndex := range row.Metrics {
			if row.Metrics[metricIndex].Method == "fisher_exact" {
				fisher = &row.Metrics[metricIndex]
				break
			}
		}
		if fisher == nil || fisher.PValue == nil || fisher.QValue == nil {
			return fmt.Errorf("rows[%d] cannot validate Fisher FDR without p_value and q_value", rowIndex)
		}
		values = append(values, reference{row: rowIndex, p: *fisher.PValue, q: fisher.QValue})
	}
	sort.SliceStable(values, func(i, j int) bool { return values[i].p < values[j].p })
	expected := make(map[int]float64, len(values))
	adjusted := 1.0
	for index := len(values) - 1; index >= 0; index-- {
		candidate := values[index].p * float64(len(values)) / float64(index+1)
		adjusted = math.Min(adjusted, math.Min(1, candidate))
		expected[values[index].row] = adjusted
	}
	for _, value := range values {
		if !nearlyEqual(*value.q, expected[value.row], fdrRelativeTolerance, 0) {
			return fmt.Errorf("rows[%d] Fisher q_value does not match Benjamini-Hochberg correction across all returned rows", value.row)
		}
	}
	return nil
}

func expectedEffectCalculation(result stats.DisproportionalityResult) MetricCalculation {
	correction := result.MethodMetadata.ZeroCellCorrection
	if correction.Applied {
		return MetricCalculation{
			InputCells: MetricInputHaldaneAnscombeCorrected,
			ZeroCellCorrection: ZeroCellCorrection{
				Applied:         true,
				Method:          correction.Method,
				AddedToEachCell: correction.AddedToEachCell,
			},
		}
	}
	return observedMetricCalculation()
}

func observedMetricCalculation() MetricCalculation {
	return MetricCalculation{
		InputCells: MetricInputObserved,
		ZeroCellCorrection: ZeroCellCorrection{
			Method: ZeroCellCorrectionNone,
		},
	}
}

func screeningOutcomeValue(outcome stats.ScreeningOutcome) string {
	switch outcome {
	case stats.ScreeningMeetsProfile:
		return "meets_profile"
	case stats.ScreeningIntermediateReview:
		return "intermediate_review"
	default:
		return "below_profile"
	}
}

func metricMethods(metrics []MetricEstimate) []string {
	methods := make([]string, len(metrics))
	for index, metric := range metrics {
		methods[index] = metric.Method
	}
	return methods
}

func nearlyEqual(got, want, relativeTolerance, absoluteTolerance float64) bool {
	if got == want {
		return true
	}
	difference := math.Abs(got - want)
	if difference <= absoluteTolerance {
		return true
	}
	scale := math.Max(math.Abs(got), math.Abs(want))
	return scale > 0 && difference/scale <= relativeTolerance
}

func validateMetricCalculation(method string, calculation MetricCalculation) error {
	correction := calculation.ZeroCellCorrection
	if !finite(correction.AddedToEachCell) || correction.AddedToEachCell < 0 {
		return errors.New("added_to_each_cell must be a finite non-negative value")
	}
	switch calculation.InputCells {
	case MetricInputObserved:
		if correction.Applied || correction.Method != ZeroCellCorrectionNone || correction.AddedToEachCell != 0 {
			return errors.New("observed input cells require an explicit unapplied none correction with zero added to each cell")
		}
	case MetricInputHaldaneAnscombeCorrected:
		if !correction.Applied || correction.Method != ZeroCellCorrectionHaldaneAnscombe || correction.AddedToEachCell != 0.5 {
			return errors.New("Haldane-Anscombe input cells require applied=true, method=haldane_anscombe, and 0.5 added to every cell")
		}
	default:
		return fmt.Errorf("unsupported input_cells %q", calculation.InputCells)
	}
	if method == "fisher_exact" && calculation.InputCells != MetricInputObserved {
		return errors.New("Fisher exact must use the uncorrected observed integer cells")
	}
	return nil
}

func validateIntegerTable(table IntegerTable) error {
	if table.A < 0 || table.B < 0 || table.C < 0 || table.D < 0 || table.N < 0 {
		return errors.New("cells and n cannot be negative")
	}
	if table.A > math.MaxInt64-table.B || table.A+table.B > math.MaxInt64-table.C || table.A+table.B+table.C > math.MaxInt64-table.D {
		return errors.New("cell total overflows int64")
	}
	if table.A+table.B+table.C+table.D != table.N {
		return errors.New("n must equal a+b+c+d")
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func optionalFinite(value *float64) bool {
	return value == nil || finite(*value)
}

func optionalProbability(value *float64) bool {
	return value == nil || finiteRange(*value, 0, 1)
}

func analysisRequestsEqual(a, b AnalysisRequest) bool {
	aJSON, aErr := CanonicalJSON(a)
	bJSON, bErr := CanonicalJSON(b)
	return aErr == nil && bErr == nil && string(aJSON) == string(bJSON)
}

func datasetCompletenessEqual(a, b DatasetCompleteness) bool {
	aJSON, aErr := CanonicalJSON(a)
	bJSON, bErr := CanonicalJSON(b)
	return aErr == nil && bErr == nil && string(aJSON) == string(bJSON)
}

func validateHTTPURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return errors.New("must be an absolute HTTP(S) URL")
	}
	return nil
}

func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

func finiteRange(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}

// validateAnalysisRequestAgainstManifest is shared by registry resolution and
// persisted-result validation. A caller that bypasses Registry (for example an
// R batch importer) must not gain weaker dataset binding or period semantics.
func validateAnalysisRequestAgainstManifest(manifest DatasetManifest, request AnalysisRequest) error {
	var problems []error
	if request.DatasetID != manifest.DatasetID {
		problems = append(problems, fmt.Errorf("request dataset_id %q does not match manifest %q", request.DatasetID, manifest.DatasetID))
	}
	coverageStart, startErr := parseDate(manifest.Coverage.StartDate)
	coverageEnd, endErr := parseDate(manifest.Coverage.EndDate)
	if startErr != nil || endErr != nil {
		problems = append(problems, errors.New("manifest coverage cannot be parsed"))
		return errors.Join(problems...)
	}
	if request.Period.StartDate != "" {
		requestedStart, err := parseDate(request.Period.StartDate)
		if err != nil {
			problems = append(problems, errors.New("request period.start_date cannot be parsed"))
		} else if requestedStart.Before(coverageStart) {
			problems = append(problems, fmt.Errorf("requested period starts before dataset coverage %s", manifest.Coverage.StartDate))
		}
	}
	if request.Period.EndDate != "" {
		requestedEnd, err := parseDate(request.Period.EndDate)
		if err != nil {
			problems = append(problems, errors.New("request period.end_date cannot be parsed"))
		} else if requestedEnd.After(coverageEnd) {
			problems = append(problems, fmt.Errorf("requested period ends after dataset coverage %s", manifest.Coverage.EndDate))
		}
	}
	return errors.Join(problems...)
}

func stringSet(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func sortedKeys[V any](set map[string]V) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
