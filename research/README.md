# Reproducible FAERS research pipeline

This directory is a separate batch-data project. It does not run inside the MIT-licensed Go service and it never downloads FAERS data implicitly. Investigators must obtain official quarterly ASCII archives from the [FDA quarterly files page](https://www.fda.gov/drugs/fda-adverse-event-monitoring-system-aems/fda-adverse-event-monitoring-system-aems-latest-quarterly-data-files), record provenance, and verify the local bytes before transformation. The landing page is now titled FDA Adverse Event Monitoring System (AEMS) and marked “Formerly FAERS”; the quarterly files it publishes remain named FAERS. This naming change does not rename the pipeline's dataset IDs or imply a data migration.

## Status and release gate

The repository contains a pinned R 4.6.1/Bioconductor 3.23 container, a committed `renv.lock`, an executable pipeline, and synthetic fixture tests. The reviewed lock currently has SHA-256 `5654bb7d2666ac37e122f639b8ee819f206733d9981005cd3f87778cae881fb5`. A clean container restore and fixture run are reproducibility evidence for the software environment; they are not evidence for a real FAERS quarter or a validated signal-detection method. A scientific dataset release remains blocked until the pipeline executes against frozen official inputs and its source, quality, numerical, licensing, and independent-review evidence is approved.

The direct method versions checked by the bootstrap are `faers 1.8.0`, `pvda 0.0.4`, and `openEBGM 0.9.1`. The generated lockfile must also pin all infrastructure and transitive packages.

## Input contract

1. Download quarterly ZIP archives or the corresponding `DEMO`, `DRUG`, and `REAC` ASCII files yourself. Never commit them.
2. Copy `config/source-register.example.csv` outside the repository or into ignored `research/input/` and add one row per ZIP, or three rows per quarter for unpacked ASCII.
3. Record an official `https://fda.gov` or `https://*.fda.gov` URL, UTC retrieval time, and the exact calendar-quarter coverage. Lookalike hosts, non-FDA mirrors, and partial or cross-quarter coverage fail closed. Leave `sha256` empty only in the draft.
4. Prepare a verified register. The output path must not already exist:

```powershell
Rscript research/R/prepare_source_register.R `
  --input research/input/source-register.draft.csv `
  --output research/input/source-register.verified.csv
```

The build rejects blank or mismatched hashes, non-official source URLs, quarter/coverage or filename/quarter disagreement, missing required tables, ambiguous headers that collide after normalization, mixed ZIP/ASCII input for a quarter, invalid or duplicate parent keys, orphan DRUG/REAC rows, PRIMARYID/CASEID disagreement, unknown role codes, unsafe ZIP paths, an absent/divergent `renv.lock`, an unavailable requested locale, an empty eligible universe, spreadsheet-formula prefixes in TSV text, or any existing output path. Before extraction, each ZIP is limited to 128 members, 16 GiB total uncompressed bytes, and a 100:1 expansion ratio. Before the DRUG×REAC merge, exact report-level components are deduplicated and each `PRIMARYID` is limited to 10,000 projected distinct drug–event pairs; the build fails instead of silently dropping pairs. Artifacts are built in a sibling staging directory and renamed only after checksums complete, so a failed run cannot expose a partial target as ready.

## Build a local verification image

The live-context command below restores and verifies the committed package lock for local development; it is not release-provenance evidence. Authoritative release images are built by CI from `git archive <verified-commit>:research`, so every context byte comes from the annotated-tag commit rather than from the runner worktree.

```powershell
$dirtyPaths = rtk git status --porcelain --untracked-files=all
if ($LASTEXITCODE -ne 0 -or $dirtyPaths) {
  throw "Build the scientific image only from a tracked, clean checkout."
}
$pipelineCommit = (rtk git rev-parse HEAD).Trim()
rtk docker build --platform linux/amd64 `
  --build-arg PIPELINE_COMMIT=$pipelineCommit `
  --file research/Dockerfile `
  --tag pv-signal-radar-research `
  research
```

That command is suitable for development verification only when its context is known. A provenance-bearing release image must be built by the protected release workflow from a clean checkout of the exact annotated-tag commit after every `research/` source is tracked. A live dirty, ignored, or untracked Docker context can contain bytes that the supplied commit does not identify; the baked commit file and OCI label cannot repair that mismatch by themselves.

Regenerating the lock is an explicit dependency-maintenance operation (`R/bootstrap.R --snapshot`) that refuses to overwrite the current lock. Normal execution uses `--restore`.

Then run the deterministic transform from a clean checkout with an explicit build timestamp and complete 40- or 64-character software revision:

```powershell
$pipelineCommit = (rtk git rev-parse HEAD).Trim()
Rscript research/R/build_snapshot.R `
  --source-register research/input/source-register.verified.csv `
  --output research/output/faers-2026q2-v1 `
  --dataset-id faers-2026q2-v1 `
  --build-timestamp 2026-09-02T00:00:00Z `
  --software-version $pipelineCommit `
  --renv-lock research/renv.lock `
  --encoding UTF-8 `
  --locale C.UTF-8
```

When `.git` metadata is available, the snapshot build requires a clean working tree and matches `--software-version` to `HEAD`. The research container intentionally excludes `.git`; its image instead bakes the complete revision into the OCI `org.opencontainers.image.revision` label and a root-owned, mode `0444` file. `PV_RADAR_PIPELINE_COMMIT` is optional corroborating input: when present it must match `--software-version`, but it cannot replace or override the baked revision. The file and label prove agreement among provenance claims, not by themselves that arbitrary local build-context bytes came from that revision. The local command rejects tracked or untracked worktree changes but still has a preflight-to-build TOCTOU window, so only the archive-based tagged CI path is revision-bound release evidence.

Outputs:

- `report_pairs/`: partitioned Parquet of unique current-version report–drug–role–PT pairs plus source demographics;
- `aggregate_interchange.tsv`: deterministic 2 × 2 counts suitable for SQLite import;
- `manifest.json`: method, coverage, environment, vocabulary treatment, and limitations;
- `source_manifest.csv`: source provenance without machine-specific absolute paths;
- `qa_summary.csv`: input, relational integrity, superseded rows, deduplication, eligibility, and output counts;
- `metadata/environment.json`: fixed build timestamp, source commit, R/platform, encoding, locale, timezone, lock hash, and package versions;
- `metadata/renv.lock`: the exact reviewed lock bound into the manifest;
- `checksums.sha256`: integrity hashes for every published artifact.

`manifest.json` implements the strict `pv-signal-radar.research/v1` contract consumed by the Go registry. Each data/support artifact is bound by path, media type, byte count, and SHA-256. Environment JSON is nested under `metadata/` so a registry scan of immediate `*.json` files cannot mistake it for a dataset manifest. The manifest itself is then covered by `checksums.sha256`.

The same source bytes, lockfile, arguments, locale, R runtime, and pipeline revision are the reproducibility boundary. Parquet byte identity across different Arrow builds is not claimed; semantic equality and recorded hashes are the release criteria.

## Materialize the read-only SQLite runtime derivative

Parquet remains the canonical research artifact. The deterministic TSV is the checked handoff to the Go materializer; do not import it manually with `sqlite3`, append it to an existing database, or register an unbound database.

Run the materializer from the repository root. Its output directory must not already exist:

```powershell
go run ./cmd/materialize-sqlite `
  --manifest research/output/faers-2026q2-v1/manifest.json `
  --tsv research/output/faers-2026q2-v1/aggregate_interchange.tsv `
  --output runtime/datasets/faers-2026q2-v1
```

Before publishing, the command validates the strict source manifest, the declared TSV byte size and SHA-256, exact TSV schema, canonical roles/comparator/event scope, row-level dataset ID, non-negative integer counts, every 2 × 2 marginal, equality of every `universe_reports` value with manifest `eligible_reports`, and the deduplication policy. The SQLite opener and result/FileStore validator repeat that denominator binding at their own trust boundaries. It creates the schema, imports inside a transaction, runs `VACUUM` and `PRAGMA integrity_check`, computes SHA-256 hashes, and atomically renames a staging directory. It refuses to overwrite an existing output or SQLite file and removes partial output after a failure.

The runtime derivative contains:

- `aggregate.sqlite`: aggregate bound to the dataset and opened by the server with read-only, immutable, and query-only controls;
- `manifest.json`: derived strict serving manifest containing the SQLite hash and the parent-manifest hash;
- `parent-manifest.json`: preserved canonical research manifest;
- `checksums.sha256`: hashes of all three files.

The server also requires a clean, traceable application revision. Release builds inject it into the binary. `PV_RADAR_APPLICATION_COMMIT` is only a reviewed fallback when a binary has no VCS metadata; it cannot mask a dirty build or replace a different embedded revision. Version and commit participate in `analysis_id`, so changing the implementation creates a distinct analysis identity even when dataset and request are unchanged. That ID covers the complete manifest, normalized protocol, and software identity, not result rows. The separate `result_digest` covers the versioned emitted-family definition, explicit `row_count`, and exact canonical row sequence; FileStore revalidates it and canonical ordering on save/load/export. This detects stale truncation, reordering, and alteration, but a coherently replaced family with recomputed metadata can still retain the same input ID. Publication therefore also requires independently attested reconciliation or deterministic reexecution against the released aggregate to establish expected-family completeness.

Keep registry files in a separate directory containing only immediate serving manifests. Do **not** set `RESEARCH_MANIFEST_DIR` to `runtime/datasets/faers-2026q2-v1`, because the registry loads every immediate `*.json` file and that directory intentionally contains both the serving and parent manifests. Source inputs, pipeline outputs, and runtime directories are local artifacts and must never be committed or copied into an image.

```powershell
New-Item -ItemType Directory -Force runtime/manifests, runtime/analyses | Out-Null
Copy-Item runtime/datasets/faers-2026q2-v1/manifest.json `
  runtime/manifests/faers-2026q2-v1.json

$env:RESEARCH_MANIFEST_DIR = (Resolve-Path runtime/manifests).Path
$env:RESEARCH_ANALYSIS_DIR = (Resolve-Path runtime/analyses).Path
$env:RESEARCH_SQLITE_PATH = (Resolve-Path runtime/datasets/faers-2026q2-v1/aggregate.sqlite).Path
$env:RESEARCH_SQLITE_DATASET_ID = "faers-2026q2-v1"
go run ./cmd/server
```

Runtime variables:

| Variable | Default | Contract |
|---|---|---|
| `RESEARCH_MANIFEST_DIR` | unset | Enables research mode and must contain at least one valid immediate manifest and no unrelated JSON |
| `RESEARCH_ANALYSIS_DIR` | `data/research-analyses` | Writable store for immutable results and deterministic export bundles |
| `RESEARCH_SQLITE_PATH` | unset | Optional serving aggregate; its bytes must match the registered manifest hash |
| `RESEARCH_SQLITE_DATASET_ID` | inferred only with one manifest | Explicit binding required when the registry has multiple datasets |
| `RESEARCH_ALLOW_ONLINE_MATERIALIZATION` | `false` | A cache miss can create a new persistent analysis only when explicitly set to `true`; public/multi-replica use also needs external identity, rate, and storage quotas |

With a registry but no SQLite path, the server can expose the dataset catalog and previously materialized result store, but a new analysis returns `analysis_not_materialized`. A malformed registry, a hash/schema mismatch, an ambiguous dataset binding, or an incompatible SQLite file fails server startup instead of falling back to live data.

For a container, mount the registry and dataset directory read-only and mount only the result store writable:

```powershell
$runtime = (Resolve-Path runtime).Path
docker run --rm --publish 8080:8080 `
  --mount "type=bind,src=$runtime/manifests,dst=/app/research/manifests,readonly" `
  --mount "type=bind,src=$runtime/datasets/faers-2026q2-v1,dst=/app/research/datasets/faers-2026q2-v1,readonly" `
  --mount "type=bind,src=$runtime/analyses,dst=/app/data/research-analyses" `
  --env RESEARCH_MANIFEST_DIR=/app/research/manifests `
  --env RESEARCH_ANALYSIS_DIR=/app/data/research-analyses `
  --env RESEARCH_SQLITE_PATH=/app/research/datasets/faers-2026q2-v1/aggregate.sqlite `
  --env RESEARCH_SQLITE_DATASET_ID=faers-2026q2-v1 `
  pv-signal-radar
```

The current Go engine computes PRR, ROR, two-sided Fisher exact, and Benjamini-Hochberg FDR from this aggregate. The fixed two-sided 95% PRR/ROR limits use the full `qnorm(0.975)` equivalent rather than the 1.96 shorthand. Every online Go metric carries strict method/measure, optional interval/probability, input-cell, and zero-correction metadata. A Fisher Go/R fixture at universe `N = 10^9` uses a 2 ppm relative tolerance and showed a maximum drift of approximately 0.8 ppm; effect estimates and limits use materially tighter deterministic fixtures. Online Fisher calculations fail closed before exceeding 100,000 enumerated support terms; an imported batch Fisher result above that work bound is not numerically recomputed by the Go validator. API results embed the complete dataset manifest and emitted-family integrity metadata. Export bundles include that manifest, normalized request, result/csv, scoped citation and serving-environment metadata, checksums, and a PowerShell reproduction script; they deliberately do not fabricate Parquet, R locks, OS/container attestations, or SBOMs that belong to separately released artifacts. `results.csv` repeats digest/count/canonical row numbers and prefixes formula-triggering text with an apostrophe for spreadsheet safety, while `analysis.json` retains original text losslessly. The engine rejects temporal/subgroup requests because this schema has no strata, and it rejects BCPNN/GPS rather than substituting an unvalidated approximation. Those methods remain planned, structurally checked but numerically unvalidated R batch work until adapters, golden outputs, reference sets, and independent review are complete.

## Fixture verification

The tests need only base R; they do not download data or install packages. The release evidence run should still use the locked container so the ZIP implementation, locale, and R runtime match the published environment:

```powershell
Rscript research/tests/run_tests.R
```

The locked-container run currently reports `PASS: 58 research fixture assertions`. Those assertions verify latest-case-version selection, deterministic tie-breaking, distinct raw/current/eligible report populations, exclusion of superseded and ineligible reports from field-missingness denominators, rejection of normalized-header collisions in DEMO/DRUG/REAC, relational parent/child integrity, pre-join DRUG/REAC deduplication with QA counts and the 10,000-pair pre-merge budget, role mapping, removal of duplicate report-level pairs, 2 × 2 invariants, sparse-cell correction, Fisher exact p-values, full-normal-quantile confidence limits, FDR, official-source and exact calendar-quarter constraints, table filename quarters, ZIP traversal and extraction budgets (including a small high-expansion ZIP), formula-injection rejection, complete commit identity, rejection of runtime-environment provenance substitution, and equality with the committed cross-language TSV. The Go suite materializes that same golden TSV and verifies its table, manifest-denominator binding, PRR/ROR values, large-count numerical tolerance, metric-specific provenance, generated-bundle namespace protection, and lossless JSON/safe-CSV export boundary.

A separate local end-to-end exercise built the complete synthetic snapshot twice with identical source bytes, lock, arguments, timestamp, locale, runtime, and revision. Both runs produced byte-identical checksum manifests and artifact hashes; a clean revision was accepted and dirty/mismatched provenance was rejected. This is useful implementation evidence, but it is not currently a committed CI test or a retained/attested release artifact. It does not validate a real FAERS quarter, real-scale performance, a different Arrow build, MedDRA version, clinical event classification, BCPNN, GPS/EBGM, or production serving.

## Unimplemented release evidence

This repository does not contain an official frozen FAERS snapshot, a published dataset/result bundle, a DOI, a positive/negative reference-set benchmark, or completed independent scientific review. The committed lock and clean restore do not establish organizational qualification. The lock pins package versions and repositories, but does not record a content hash for every downloaded package archive. The workflows build and scan the scientific image, but they do not yet publish an immutable OCI digest, generate an OCI SBOM for the R image, or attest/sign that image. Remote publication controls (branch/tag rulesets, immutable releases, and the protected `software-release` environment) are external gates and were not enabled at the time of the 2026-09-02 audit.

The project also does not establish sensitivity, PPV, time-to-detection, real-quarter throughput, byte identity across Arrow builds, MedDRA redistribution rights, or suitability for regulatory/GxP use. A point-in-time Trivy 0.74.0 scan found 0 fixable HIGH/CRITICAL findings in the Go runtime image and 0 in the R image's OS packages; that result is not a general security claim and must be repeated against the current vulnerability database for every release. Until the formal gates in `docs/validation-and-evidence.md` are evidenced, the pipeline is an executable research scaffold and teaching implementation, not a formally qualified pharmacovigilance system.

## Licensing boundary

The Go application is MIT-licensed. A generated bundle's `CITATION.cff` cites that software only; `citation-metadata.json` keeps the source/vocabulary declarations separate and leaves the analysis-result license unasserted. The R environment intentionally remains a separate process/project because `pvda`, `openEBGM`, and other scientific packages have their own licenses, including GPL terms. Generated factual data artifacts are inputs to the Go service; MIT is not inferred for source data, derived artifacts, vocabularies, or analysis results. This design is not a legal opinion. A release owner must record source-data terms, package licenses, and the distribution model before publishing an image or bundle.
