package version

import (
	"strings"
	"testing"
)

func TestResolveResearchRevisionDoesNotMaskBuildIdentity(t *testing.T) {
	revisionA := "abcdef1234567890abcdef1234567890abcdef12"
	revisionB := "0123456789abcdef0123456789abcdef01234567"
	tests := []struct {
		name      string
		injected  string
		vcs       string
		modified  bool
		override  string
		want      string
		wantError bool
	}{
		{name: "embedded VCS", vcs: strings.ToUpper(revisionA), override: "", want: revisionA},
		{name: "matching linker and VCS", injected: revisionA, vcs: strings.ToUpper(revisionA), want: revisionA},
		{name: "matching operator attestation", vcs: revisionA, override: strings.ToUpper(revisionA), want: revisionA},
		{name: "fallback without metadata", override: strings.ToUpper(revisionA), want: revisionA},
		{name: "divergent linker", injected: revisionB, vcs: revisionA, wantError: true},
		{name: "divergent override", vcs: revisionA, override: revisionB, wantError: true},
		{name: "dirty cannot be masked", vcs: revisionA, modified: true, override: revisionA, wantError: true},
		{name: "abbreviated revision", vcs: "abcdef1", wantError: true},
		{name: "missing", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveResearchRevision(test.injected, test.vcs, test.modified, test.override)
			if (err != nil) != test.wantError || got != test.want {
				t.Fatalf("got revision=%q error=%v; want revision=%q error=%v", got, err, test.want, test.wantError)
			}
		})
	}
}
