// Command release-metadata binds unpublished citation templates to one
// validated annotated tag and writes deterministic release artifacts.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BMaeda84/pv-signal-radar/internal/version"
)

func main() {
	tag := flag.String("tag", "", "annotated release tag")
	releaseDate := flag.String("date", "", "annotated tag date in UTC (YYYY-MM-DD)")
	revision := flag.String("revision", "", "full release commit object ID")
	tagObject := flag.String("tag-object", "", "full annotated tag object ID")
	sourceDateEpoch := flag.Int64("source-date-epoch", 0, "release commit timestamp")
	citationPath := flag.String("citation-template", "CITATION.cff", "unpublished CFF template")
	zenodoPath := flag.String("zenodo-template", ".zenodo.json", "unpublished Zenodo template")
	outputDirectory := flag.String("output-dir", "dist", "artifact output directory")
	flag.Parse()

	citation, err := os.ReadFile(*citationPath)
	check(err, "read citation template")
	zenodo, err := os.ReadFile(*zenodoPath)
	check(err, "read Zenodo template")
	artifacts, err := version.BuildReleaseArtifacts(version.ReleaseArtifactInput{
		Tag:              *tag,
		ReleaseDate:      *releaseDate,
		Revision:         *revision,
		TagObject:        *tagObject,
		SourceDateEpoch:  *sourceDateEpoch,
		CitationTemplate: citation,
		ZenodoTemplate:   zenodo,
	})
	check(err, "build release metadata")
	// Release packaging runs under one CI principal; group/other write or read
	// access is unnecessary before the artifact uploader publishes reviewed bytes.
	check(os.MkdirAll(*outputDirectory, 0o750), "create output directory")
	write(*outputDirectory, "CITATION.cff", artifacts.Citation)
	write(*outputDirectory, "zenodo.json", artifacts.Zenodo)
	write(*outputDirectory, "release-metadata.json", artifacts.Metadata)
}

func write(directory, name string, data []byte) {
	check(os.WriteFile(filepath.Join(directory, name), data, 0o600), "write "+name)
}

func check(err error, operation string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", operation, err)
		os.Exit(1)
	}
}
