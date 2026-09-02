package web

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BMaeda84/pv-signal-radar/internal/version"
)

// TestOpenAPIContractCoverage is intentionally dependency-free. It does not
// replace a full OpenAPI validator; CI validates the YAML syntax separately.
// Its purpose is to make route/status/schema drift visible when handlers change.
func TestOpenAPIContractCoverage(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate OpenAPI contract test source")
	}
	specPath := filepath.Join(filepath.Dir(sourceFile), "..", "..", "docs", "openapi-v2.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	spec := string(data)
	if strings.Contains(spec, "\t") || strings.Contains(spec, "\r") {
		t.Fatal("OpenAPI YAML must use LF and spaces only")
	}
	for number, line := range strings.Split(spec, "\n") {
		if strings.TrimRight(line, " ") != line {
			t.Fatalf("OpenAPI YAML line %d has trailing whitespace", number+1)
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent%2 != 0 {
			t.Fatalf("OpenAPI YAML line %d has odd indentation", number+1)
		}
	}

	required := []string{
		"openapi: 3.1.0",
		"version: " + version.Current,
		"  /api/v1/health:",
		"  /api/v1/analyze:",
		"      deprecated: true",
		"`patient.drug.drugcharacterization`",
		"                const: '@1788307200'",
		"                const: 'Thu, 02 Sep 2027 00:00:00 GMT'",
		"  /api/v1/feedback:",
		"  /api/v2/datasets:",
		"  /api/v2/analyses:",
		"  /api/v2/analyses/{analysis_id}:",
		"  /api/v2/analyses/{analysis_id}/export:",
		"    SoftwareReference:",
		"        commit:",
		"    DatasetManifest:",
		"        registered_dataset_count:",
		"        registration_state:",
		"          const: integrity_checked",
		"    AnalysisRequest:",
		"    ResultFamilyDefinition:",
		"    AnalysisResult:",
		"        result_digest:",
		"        row_count:",
		"        result_family:",
		"        dataset_manifest:",
		"            X-Result-Digest:",
		"            X-Result-Row-Count:",
		"    DrugEventResult:",
		"    MetricEstimate:",
		"enum: [MEETS_EVANS_EDUCATIONAL_PROFILE, INTERMEDIATE_REVIEW, BELOW_PROFILE]",
		"Present only when the exact test was actually computed",
		"    ReviewFlag:",
		"    ErrorResponse:",
		"const: live_exploration",
		"const: false",
		"'201':",
		"'400':",
		"'404':",
		"'409':",
		"'422':",
		"'503':",
	}
	for _, fragment := range required {
		if !strings.Contains(spec, fragment) {
			t.Errorf("OpenAPI contract is missing %q", fragment)
		}
	}

	postAnalyses := openAPIPathBlock(t, spec, "/api/v2/analyses:")
	for _, status := range []string{"'200':", "'201':", "'400':", "'404':", "'405':", "'409':", "'415':", "'422':", "'429':", "'500':", "'503':"} {
		if !strings.Contains(postAnalyses, status) {
			t.Errorf("POST /api/v2/analyses is missing response %s", status)
		}
	}
	for _, forbidden := range []string{"vigimed", "anvisa", "confirmed safety signal", "causality established"} {
		if strings.Contains(strings.ToLower(spec), forbidden) {
			t.Errorf("OpenAPI contract contains retired or unsupported claim %q", forbidden)
		}
	}
	for _, obsolete := range []string{"validated_dataset_count", "validation_status"} {
		if strings.Contains(spec, obsolete) {
			t.Errorf("OpenAPI contract retains scientifically ambiguous field %q", obsolete)
		}
	}
}

func openAPIPathBlock(t *testing.T, spec, pathKey string) string {
	t.Helper()
	marker := "  " + pathKey + "\n"
	start := strings.Index(spec, marker)
	if start < 0 {
		t.Fatalf("OpenAPI path %s not found", pathKey)
	}
	rest := spec[start+len(marker):]
	if next := strings.Index(rest, "\n  /"); next >= 0 {
		return rest[:next]
	}
	if components := strings.Index(rest, "\ncomponents:"); components >= 0 {
		return rest[:components]
	}
	return rest
}
