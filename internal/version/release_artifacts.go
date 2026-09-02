package version

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReleaseArtifactInput binds mutable citation templates to one immutable tag.
// The commit and tag object are stored outside the source templates because a
// commit cannot contain its own object ID without a circular identity claim.
type ReleaseArtifactInput struct {
	Tag              string
	ReleaseDate      string
	Revision         string
	TagObject        string
	SourceDateEpoch  int64
	CitationTemplate []byte
	ZenodoTemplate   []byte
}

// ReleaseArtifacts contains the exact bytes checksummed and published by the
// release workflow. Rendering is deterministic for identical input bytes.
type ReleaseArtifacts struct {
	Citation []byte
	Zenodo   []byte
	Metadata []byte
}

// BuildReleaseArtifacts renders citation and provenance bytes only after the
// tag, date, and Git identities have been validated. Source templates must not
// claim a version or publication date before a release exists.
func BuildReleaseArtifacts(input ReleaseArtifactInput) (ReleaseArtifacts, error) {
	expectedTag := "v" + Current
	if input.Tag != expectedTag {
		return ReleaseArtifacts{}, fmt.Errorf("release tag %q differs from %q derived from version.Current", input.Tag, expectedTag)
	}
	if err := validateReleaseDate(input.ReleaseDate); err != nil {
		return ReleaseArtifacts{}, err
	}

	revision := normalizeRevision(input.Revision)
	if !isFullRevision(revision) {
		return ReleaseArtifacts{}, fmt.Errorf("release revision must be a full 40- or 64-character hexadecimal object ID")
	}
	tagObject := normalizeRevision(input.TagObject)
	if !isFullRevision(tagObject) {
		return ReleaseArtifacts{}, fmt.Errorf("annotated tag object must be a full 40- or 64-character hexadecimal object ID")
	}
	if input.SourceDateEpoch <= 0 {
		return ReleaseArtifacts{}, fmt.Errorf("source date epoch must be a positive Unix timestamp")
	}

	citation, err := renderCitation(input.CitationTemplate, input.Tag, input.ReleaseDate)
	if err != nil {
		return ReleaseArtifacts{}, err
	}
	zenodo, err := renderZenodo(input.ZenodoTemplate, input.Tag, input.ReleaseDate)
	if err != nil {
		return ReleaseArtifacts{}, err
	}

	metadata, err := json.MarshalIndent(struct {
		SchemaVersion   int    `json:"schema_version"`
		Software        string `json:"software"`
		Version         string `json:"version"`
		Tag             string `json:"tag"`
		TagObject       string `json:"tag_object"`
		Revision        string `json:"revision"`
		ReleaseDate     string `json:"release_date"`
		SourceDateEpoch int64  `json:"source_date_epoch"`
	}{
		SchemaVersion:   1,
		Software:        "pv-signal-radar",
		Version:         strings.TrimPrefix(input.Tag, "v"),
		Tag:             input.Tag,
		TagObject:       tagObject,
		Revision:        revision,
		ReleaseDate:     input.ReleaseDate,
		SourceDateEpoch: input.SourceDateEpoch,
	}, "", "  ")
	if err != nil {
		return ReleaseArtifacts{}, fmt.Errorf("encode release metadata: %w", err)
	}
	metadata = append(metadata, '\n')
	return ReleaseArtifacts{Citation: citation, Zenodo: zenodo, Metadata: metadata}, nil
}

func validateReleaseDate(value string) error {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return fmt.Errorf("release date %q is not an ISO 8601 calendar date", value)
	}
	return nil
}

func renderCitation(template []byte, tag, releaseDate string) ([]byte, error) {
	normalized := bytes.ReplaceAll(template, []byte("\r\n"), []byte("\n"))
	if bytes.ContainsRune(normalized, '\r') {
		return nil, fmt.Errorf("CITATION.cff template contains unsupported carriage returns")
	}
	fields, err := topLevelCFFFields(normalized)
	if err != nil {
		return nil, err
	}
	for field := range fields {
		if field == "version" || field == "date-released" {
			return nil, fmt.Errorf("unpublished CITATION.cff template must not contain %s", field)
		}
	}

	normalized = bytes.TrimRight(normalized, "\n")
	version := strings.TrimPrefix(tag, "v")
	normalized = append(normalized, []byte(fmt.Sprintf("\nversion: %q\ndate-released: %q\n", version, releaseDate))...)
	return normalized, nil
}

func topLevelCFFFields(data []byte) (map[string]struct{}, error) {
	fields := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// A bounded increase accommodates legitimate long citation descriptions
	// while still rejecting an unexpectedly large scalar as malformed input.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		key, _, found := strings.Cut(line, ":")
		if found {
			fields[strings.TrimSpace(key)] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan CITATION.cff template: %w", err)
	}
	return fields, nil
}

func renderZenodo(template []byte, tag, releaseDate string) ([]byte, error) {
	var metadata map[string]any
	if err := json.Unmarshal(template, &metadata); err != nil {
		return nil, fmt.Errorf("parse .zenodo.json template: %w", err)
	}
	for _, field := range []string{"version", "publication_date"} {
		if _, exists := metadata[field]; exists {
			return nil, fmt.Errorf("unpublished .zenodo.json template must not contain %s", field)
		}
	}
	metadata["version"] = strings.TrimPrefix(tag, "v")
	metadata["publication_date"] = releaseDate

	rendered, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode rendered Zenodo metadata: %w", err)
	}
	return append(rendered, '\n'), nil
}
