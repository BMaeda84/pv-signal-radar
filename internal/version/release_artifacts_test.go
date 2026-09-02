package version

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildReleaseArtifactsIsDeterministic(t *testing.T) {
	input := ReleaseArtifactInput{
		Tag:              "v" + Current,
		ReleaseDate:      "2030-04-05",
		Revision:         strings.Repeat("a", 40),
		TagObject:        strings.Repeat("b", 40),
		SourceDateEpoch:  1901577600,
		CitationTemplate: []byte("cff-version: 1.2.0\ntitle: Example\ntype: software\n"),
		ZenodoTemplate:   []byte(`{"title":"Example","upload_type":"software"}`),
	}

	first, err := BuildReleaseArtifacts(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildReleaseArtifacts(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Citation, second.Citation) || !bytes.Equal(first.Zenodo, second.Zenodo) || !bytes.Equal(first.Metadata, second.Metadata) {
		t.Fatal("identical release inputs produced different artifact bytes")
	}
	if !bytes.Contains(first.Citation, []byte(`version: "`+Current+`"`)) || !bytes.Contains(first.Citation, []byte(`date-released: "2030-04-05"`)) {
		t.Fatalf("rendered citation lacks release identity:\n%s", first.Citation)
	}

	var zenodo map[string]any
	if err := json.Unmarshal(first.Zenodo, &zenodo); err != nil {
		t.Fatal(err)
	}
	if zenodo["version"] != Current || zenodo["publication_date"] != "2030-04-05" {
		t.Fatalf("unexpected rendered Zenodo identity: %#v", zenodo)
	}
	var metadata map[string]any
	if err := json.Unmarshal(first.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["revision"] != input.Revision || metadata["tag_object"] != input.TagObject || metadata["version"] != Current {
		t.Fatalf("unexpected release metadata: %#v", metadata)
	}
}

func TestBuildReleaseArtifactsRejectsUnboundOrPrepublishedInput(t *testing.T) {
	valid := ReleaseArtifactInput{
		Tag:              "v" + Current,
		ReleaseDate:      "2030-04-05",
		Revision:         strings.Repeat("a", 40),
		TagObject:        strings.Repeat("b", 40),
		SourceDateEpoch:  1901577600,
		CitationTemplate: []byte("cff-version: 1.2.0\ntitle: Example\ntype: software\n"),
		ZenodoTemplate:   []byte(`{"title":"Example","upload_type":"software"}`),
	}
	tests := []struct {
		name   string
		mutate func(*ReleaseArtifactInput)
	}{
		{name: "different tag", mutate: func(input *ReleaseArtifactInput) { input.Tag = "v0.0.0" }},
		{name: "invalid date", mutate: func(input *ReleaseArtifactInput) { input.ReleaseDate = "2030-02-30" }},
		{name: "abbreviated revision", mutate: func(input *ReleaseArtifactInput) { input.Revision = "abcdef1" }},
		{name: "missing source epoch", mutate: func(input *ReleaseArtifactInput) { input.SourceDateEpoch = 0 }},
		{name: "published citation template", mutate: func(input *ReleaseArtifactInput) {
			input.CitationTemplate = append(input.CitationTemplate, []byte("version: \"old\"\n")...)
		}},
		{name: "published Zenodo template", mutate: func(input *ReleaseArtifactInput) { input.ZenodoTemplate = []byte(`{"version":"old"}`) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			if _, err := BuildReleaseArtifacts(input); err == nil {
				t.Fatal("invalid release input was accepted")
			}
		})
	}
}
