package version

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var semanticVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

// TestReleaseMetadataTemplatesDoNotClaimPublication keeps repository metadata
// usable as an unpublished template. Version and release date are added only
// to artifacts generated from an immutable annotated tag.
func TestReleaseMetadataTemplatesDoNotClaimPublication(t *testing.T) {
	if !semanticVersionPattern.MatchString(Current) {
		t.Fatalf("version.Current %q is not a valid semantic version", Current)
	}

	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	citation := readTopLevelCFF(t, filepath.Join(repositoryRoot, "CITATION.cff"))
	for _, field := range []string{"version", "date-released"} {
		if _, exists := citation[field]; exists {
			t.Fatalf("unpublished CITATION.cff template must not contain %s", field)
		}
	}

	zenodo := readJSONObject(t, filepath.Join(repositoryRoot, ".zenodo.json"))
	for _, field := range []string{"version", "publication_date"} {
		if _, exists := zenodo[field]; exists {
			t.Fatalf("unpublished .zenodo.json template must not contain %s", field)
		}
	}
}

// TestWorkflowReleaseIdentity is activated by the release workflow. It fails
// before packaging when the requested tag does not exactly match Current or
// when the annotated tag did not yield a valid UTC calendar date.
func TestWorkflowReleaseIdentity(t *testing.T) {
	releaseTag := strings.TrimSpace(os.Getenv("PV_RADAR_RELEASE_TAG"))
	releaseDate := strings.TrimSpace(os.Getenv("PV_RADAR_RELEASE_DATE"))
	if releaseTag == "" && releaseDate == "" {
		t.Skip("release identity is supplied only by the release workflow")
	}
	if releaseTag == "" || releaseDate == "" {
		t.Fatal("release workflow must supply both tag and annotated-tag UTC date")
	}

	expectedTag := "v" + Current
	if releaseTag != expectedTag {
		t.Fatalf("release tag %q differs from tag %q derived from version.Current", releaseTag, expectedTag)
	}
	requireISODate(t, "annotated tag release date", releaseDate)
}

func readTopLevelCFF(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Release identity fields must be top-level CFF scalars. Ignoring
		// indented content prevents a nested field from satisfying the check.
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		key, rawValue, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if _, duplicate := values[key]; duplicate {
			t.Fatalf("%s contains duplicate top-level field %s", path, key)
		}
		value := strings.TrimSpace(rawValue)
		if strings.HasPrefix(value, `"`) {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				t.Fatalf("parse %s field %s: %v", path, key, err)
			}
			value = unquoted
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return values
}

func readJSONObject(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return object
}

func requireISODate(t *testing.T, label, value string) string {
	t.Helper()
	value = strings.TrimSpace(value)
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		t.Fatalf("%s %q is not an ISO 8601 calendar date", label, value)
	}
	return value
}
