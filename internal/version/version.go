// Package version is the single source of truth for the service and UI release.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// Current identifies the research-preview contract. It intentionally remains a
// pre-release until an official FAERS snapshot passes the validation matrix.
const Current = "3.0.0-research.1"

// Commit may be injected by a release build with -ldflags -X. Development
// builds fall back to Go's VCS build settings when the repository is available.
var Commit string

// Revision returns the build's Git revision and whether Go reported a modified
// working tree. Research mode must reject an absent or dirty revision because a
// result cannot be reconstructed from a commit that does not contain its code.
func Revision() (revision string, modified bool) {
	revision, modified = vcsRevision()
	if injected := normalizeRevision(Commit); injected != "" {
		return injected, modified
	}
	return revision, modified
}

func vcsRevision() (revision string, modified bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	return revision, modified
}

// ResearchRevision chooses the revision used in reproducibility hashes. Build
// metadata is authoritative. An operator-provided revision is only a fallback
// when the binary contains no VCS metadata; it can never mask a dirty checkout
// or replace a different embedded commit.
func ResearchRevision(override string) (string, error) {
	vcs, modified := vcsRevision()
	return resolveResearchRevision(Commit, vcs, modified, override)
}

func resolveResearchRevision(injected, vcs string, modified bool, override string) (string, error) {
	injected = normalizeRevision(injected)
	vcs = normalizeRevision(vcs)
	override = normalizeRevision(override)
	for label, revision := range map[string]string{"linker": injected, "embedded VCS": vcs, "operator": override} {
		if revision != "" && !isFullRevision(revision) {
			return "", fmt.Errorf("%s revision must be a full 40- or 64-character hexadecimal object ID", label)
		}
	}
	if modified {
		return "", fmt.Errorf("build VCS metadata reports a modified working tree")
	}
	if injected != "" && vcs != "" && injected != vcs {
		return "", fmt.Errorf("linker revision %q differs from embedded VCS revision %q", injected, vcs)
	}
	revision := injected
	if revision == "" {
		revision = vcs
	}
	if revision != "" {
		if override != "" && override != revision {
			return "", fmt.Errorf("PV_RADAR_APPLICATION_COMMIT %q differs from embedded revision %q", override, revision)
		}
		return revision, nil
	}
	if override == "" {
		return "", fmt.Errorf("build contains no VCS revision and PV_RADAR_APPLICATION_COMMIT is empty")
	}
	return override, nil
}

func isFullRevision(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func normalizeRevision(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "unknown" {
		return ""
	}
	return value
}
