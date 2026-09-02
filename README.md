# PV Signal Radar 🛰️

[Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · **English**

[![CI](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml/badge.svg)](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26.6+-00ADD8?logo=go)](https://go.dev/)

PV Signal Radar is an open-source platform for teaching, transparent exploration, and reproducible academic research on disproportionate reporting in public pharmacovigilance data.

It produces **signals of disproportionate reporting (SDRs) for review**. It does not establish causality or incidence and is not medical advice, clinical decision support, a validated/qualified GxP system, or an automated regulatory decision engine.

## Current status

- **Live exploratory mode:** queries openFDA FAERS and calculates report-level PRR, ROR, confidence intervals, Yates-corrected chi-square, and an explicit zero-cell correction. It does not compute Fisher exact or multiplicity-adjusted q-values. Results change with the upstream source and are not a citable frozen analysis.
- **Frozen research mode:** `research/` contains a deterministic FAERS ETL scaffold, source/checksum manifest, latest-case-version deduplication, report-level pair construction, canonical Parquet plus integrity-checked TSV interchange, QA, and fixture tests. `cmd/materialize-sqlite` converts that interchange into the checksummed SQLite derivative served by Go.
- **VigiMed:** intentionally unavailable until an official, versioned ANVISA three-table ingestion and harmonization pipeline has passed validation. No hand-written VigiMed counts or “cross-source confirmation” are used.
- **Research release:** the reviewed `research/renv.lock` freezes R 4.6.1/Bioconductor 3.23 and the declared scientific packages in a digest-pinned linux/amd64 image. No official FAERS snapshot, research result, DOI, reference-set benchmark, or real-scale qualification is bundled; dependency reproducibility alone is not scientific validation. A formal release requires the remaining evidence gates in [`docs/validation-and-evidence.md`](docs/validation-and-evidence.md).

## What the measures mean

For a target drug `D` and event `E`, the platform constructs a report-count table:

| | E reported | Other events | Total |
|---|---:|---:|---:|
| D reported | a | b | a + b |
| Other eligible reports | c | d | c + d |
| Total | a + c | b + d | N |

- `PRR = [a/(a+b)] / [c/(c+d)]`
- `ROR = (a×d)/(b×c)`

These are reporting-association measures, not exposed-population risks. The guided rule `a ≥ 3`, `PRR ≥ 2`, and `χ² ≥ 4` is identified as an educational **Evans-style heuristic**, not an EMA rule or confirmation. A research protocol must pre-specify its dataset, roles, event scope, comparator, period, strata, zero-cell policy, multiplicity handling, methods, and threshold profile.

FDA and [openFDA](https://open.fda.gov/apis/drug/event/) warn that spontaneous reports can be duplicated, incomplete, biased, and contain multiple drugs and reactions with no individual causal linkage. Results require case review, temporal and clinical context, literature, exposure evidence, and other data sources.

## Run the Go application

```powershell
git clone https://github.com/BMaeda84/pv-signal-radar.git
Set-Location pv-signal-radar
go run ./cmd/server
```

Open <http://localhost:8080>. Docker is also supported:

```powershell
docker build --tag pv-signal-radar .
docker run --rm --publish 8080:8080 pv-signal-radar
```

| Variable | Default | Purpose |
|---|---:|---|
| `PORT` | `8080` | HTTP listening port |
| `OPENFDA_API_KEY` | unset | Optional secret; never commit it |
| `CACHE_CAPACITY` | `500` | Completed live analyses in memory |
| `CACHE_TTL_HOURS` | `24` | Live-analysis cache TTL |
| `RESEARCH_MANIFEST_DIR` | unset | Directory containing only immediate, strict serving manifests; unset disables research mode |
| `RESEARCH_ANALYSIS_DIR` | `data/research-analyses` | Writable immutable-result store, used only when the manifest registry is enabled |
| `RESEARCH_SQLITE_PATH` | unset | Checksummed SQLite derivative; opened with SQLite read-only, immutable, and query-only controls |
| `RESEARCH_SQLITE_DATASET_ID` | inferred for one manifest | Dataset bound to the SQLite file; required when the registry contains multiple datasets |
| `RESEARCH_ALLOW_ONLINE_MATERIALIZATION` | `false` | Feature flag for creating cache-miss analyses; leave false on public/read-only deployments without gateway identity, quotas, and storage governance |
| `PV_RADAR_APPLICATION_COMMIT` | clean embedded VCS revision | Reviewed lowercase Git SHA used only when the build cannot embed its own revision; research mode rejects a missing/dirty revision |

## Build a frozen FAERS research artifact

The pipeline never downloads data implicitly. Obtain official quarterly ASCII archives from the [FDA quarterly files page](https://www.fda.gov/drugs/fda-adverse-event-monitoring-system-aems/fda-adverse-event-monitoring-system-aems-latest-quarterly-data-files), create a source register, and verify its bytes. The landing page is now titled FDA Adverse Event Monitoring System (AEMS) and marked “Formerly FAERS”; the quarterly files it publishes remain named FAERS. This does not rename dataset IDs or imply a data migration:

```powershell
Rscript research/R/prepare_source_register.R `
  --input research/input/source-register.draft.csv `
  --output research/input/source-register.verified.csv

Rscript research/R/bootstrap.R --restore

$softwareRevision = git rev-parse HEAD
$env:PV_RADAR_PIPELINE_COMMIT = $softwareRevision
Rscript research/R/build_snapshot.R `
  --source-register research/input/source-register.verified.csv `
  --output research/output/faers-2026q2-v1 `
  --dataset-id faers-2026q2-v1 `
  --build-timestamp 2026-09-02T00:00:00Z `
  --software-version $softwareRevision `
  --renv-lock research/renv.lock `
  --encoding UTF-8 `
  --locale C.UTF-8

go run ./cmd/materialize-sqlite `
  --manifest research/output/faers-2026q2-v1/manifest.json `
  --tsv research/output/faers-2026q2-v1/aggregate_interchange.tsv `
  --output runtime/datasets/faers-2026q2-v1

New-Item -ItemType Directory -Force runtime/manifests, runtime/analyses | Out-Null
Copy-Item runtime/datasets/faers-2026q2-v1/manifest.json `
  runtime/manifests/faers-2026q2-v1.json
```

The snapshot build fails closed without a reviewed `renv.lock`, records locale/encoding/TZ, and rejects orphan or mismatched relational rows. The materializer refuses to overwrite its output, verifies the manifest-bound TSV size and SHA-256, validates every marginal and canonical dataset contract, and publishes a new runtime directory atomically. Keep `runtime/manifests/` as a dedicated registry containing only serving manifests; do not use the materialized dataset directory itself as `RESEARCH_MANIFEST_DIR`, because it also preserves `parent-manifest.json` for provenance. The example `research/input/`, `research/output/`, and `runtime/` trees are local artifacts and must never be committed.

Enable local research execution with explicit paths:

```powershell
$env:RESEARCH_MANIFEST_DIR = (Resolve-Path runtime/manifests).Path
$env:RESEARCH_ANALYSIS_DIR = (Resolve-Path runtime/analyses).Path
$env:RESEARCH_SQLITE_PATH = (Resolve-Path runtime/datasets/faers-2026q2-v1/aggregate.sqlite).Path
$env:RESEARCH_SQLITE_DATASET_ID = "faers-2026q2-v1"
$env:RESEARCH_ALLOW_ONLINE_MATERIALIZATION = "true" # local/staging only after reviewing quotas
$env:PV_RADAR_APPLICATION_COMMIT = $softwareRevision
go run ./cmd/server
```

For Docker, mount provenance and data read-only and keep only the analysis store writable:

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
  --env RESEARCH_ALLOW_ONLINE_MATERIALIZATION=false `
  --env PV_RADAR_APPLICATION_COMMIT=$softwareRevision `
  pv-signal-radar
```

The committed lock was generated twice with identical bytes and restored in the pinned linux/amd64 image; the image build reruns the 58-assertion fixture baseline. Normal use is `bootstrap.R --restore`. `--snapshot` is a controlled maintenance operation that refuses to overwrite the reviewed lock. This evidence fixes package resolution but does not qualify a real FAERS quarter, method performance, or other architectures. See [`research/README.md`](research/README.md) for the input contract, container command, outputs, failure modes, and licensing boundary.

Current scientific-runtime boundary: the Go SQLite engine supports PRR, ROR, two-sided Fisher exact, and Benjamini-Hochberg FDR for the stored aggregate. It explicitly rejects unsupported temporal/subgroup requests and BCPNN/GPS, which remain batch-method work. The `analysis_id` binds the complete dataset manifest, normalized protocol, application version, and full clean application Git object ID. The separate `result_digest` binds a versioned emitted-family definition, `row_count`, and the exact canonical row sequence; the API also returns the complete manifest. This detects stale truncation, reordering, or alteration but does not prove upstream or scientifically expected family completeness. The export repeats this boundary in JSON/CSV, headers, scoped citation/environment metadata, checksums, and a reproduction script. No official FAERS snapshot, positive/negative reference-set benchmark, or real-scale qualification is included.

## API boundary

| Route | Purpose | Reproducibility |
|---|---|---|
| `GET /api/v1/analyze?drug=...` | Deprecated live openFDA exploration | Not citable; upstream changes |
| `GET /api/v1/health` | Process health | Not a scientific validation check |
| `GET /api/v2/datasets` | Registered immutable datasets and provenance | Depends on an installed verified artifact |
| `POST /api/v2/analyses` | Deterministic analysis configuration | `analysis_id` binds inputs; `result_digest` binds emitted rows |
| `GET /api/v2/analyses/{id}` | Result, complete manifest, methods, and limitations | Reproducible within the recorded integrity boundary |
| `GET /api/v2/analyses/{id}/export` | Research bundle | Complete manifest, result integrity, scoped citation metadata, checksums, and reproduction script |

No dataset is silently substituted. If a requested snapshot or method is unavailable, the research API must fail closed.

## Research and publication documentation

- [Intended use and user responsibilities](docs/intended-use.md)
- [Methods and protocol contract](docs/methods-and-protocol.md)
- [Data dictionary](docs/data-dictionary.md)
- [Biases and failure modes](docs/biases-and-failure-modes.md)
- [Validation and evidence matrix](docs/validation-and-evidence.md)
- [Privacy, security, licensing, and governance](docs/privacy-security-and-governance.md)
- [Scientific package and repository selection](docs/ecosystem-and-method-selection.md)
- [Publication and DOI checklist](docs/publication-checklist.md)

Report formal studies according to [READUS-PV](https://pmc.ncbi.nlm.nih.gov/articles/PMC11116242/) and archive the dataset ID, source/output hashes, analysis configuration/ID, code commit, package lock, deviations, and export bundle. READUS-PV improves reporting transparency; it does not validate causality or regulatory fitness.

## Verification

```powershell
go test -race ./...
go vet ./...
go build ./cmd/server
Rscript research/tests/run_tests.R
docker build --tag pv-signal-radar:local .
```

The R fixture suite uses synthetic data and base R only. It does not validate an official quarter, real-data scale, Parquet serialization, BCPNN/GPS, MedDRA mapping, VigiMed, S3 publication, or clinical interpretation.

## Citation and licenses

Use [`CITATION.cff`](CITATION.cff) for the software and cite the immutable dataset/analysis bundle separately. A generated bundle's `CITATION.cff` is explicitly a software citation; `citation-metadata.json` records source/vocabulary declarations and leaves the analysis license unasserted. Application source is MIT-licensed, but that license is not applied to source data, derived artifacts, vocabularies, or an analysis result. The separate R environment and data/vocabularies retain their own terms; GPL scientific packages are not linked into the Go binary. This technical separation is not legal advice—review package, data, terminology, and result-distribution rights before redistribution.
