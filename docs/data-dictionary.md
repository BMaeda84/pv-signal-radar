# Research artifact data dictionary

## `report_pairs/` Parquet dataset

One row is a unique current-version report–drug-text–role–event-PT combination. `quarter` is a Parquet partition derived from the retained DEMO row.

| Field | Meaning | Constraint or caveat |
|---|---|---|
| `primaryid` | FAERS report-version identifier | Stored as text to avoid numeric precision loss |
| `caseid` | Longitudinal FAERS case identifier | Maximum numeric case version is retained |
| `quarter` | Source quarter of the retained DEMO row | `YYYYQ1` through `YYYYQ4` |
| `drug_text` | Normalized `PROD_AI`, otherwise `DRUGNAME` | Source text, not a canonical concept |
| `drug_text_source` | Field used for `drug_text` | `PROD_AI` or `DRUGNAME` |
| `drug_role` | FDA `ROLE_COD` mapped as `PS` → `primary_suspect`, `SS` → `secondary_suspect`, `C` → `concomitant`, `I` → `interacting`; `suspect` and `all` are deduplicated unions | Unknown source codes fail closed; no clinical role is inferred |
| `event_pt` | Normalized source PT text | MedDRA version is not inferred |
| `event_category` | Classification state | Currently `UNCLASSIFIED_SOURCE_PT` |
| `event_dt`, `fda_dt` | Source event/receipt date | Preserved source encoding; not silently imputed |
| `age`, `age_cod`, `sex` | Source demographics | Must be interpreted together; not normalized in v1 |
| `occr_country`, `reporter_country` | Source country fields | Missing or differing values are possible |
| `serious*` | Source seriousness flags | No composite is inferred beyond supplied fields |

## `aggregate_interchange.tsv`

| Field | Meaning |
|---|---|
| `dataset_id` | Immutable logical snapshot identifier |
| `drug_text`, `drug_text_source`, `drug_role`, `event_pt` | Exact grouping key |
| `a` | Reports containing target drug group and event |
| `b` | Reports containing target drug group without event |
| `c` | Reports containing event without target drug group |
| `d` | Reports containing neither |
| `drug_reports` | `a + b` |
| `event_reports` | `a + c` |
| `universe_reports` | `a + b + c + d`; eligible current-version reports with a drug and PT |
| `comparator` | `all_other_eligible_reports`: reports in the eligible snapshot universe that are not in the selected drug/role group |
| `event_scope` | `all_recorded_source_pts`: every retained source PT; no clinical/error/quality classification is inferred |
| `deduplication_policy` | Stable policy identifier used by ETL |

The TSV is an interchange artifact, not a complete publication. Its source manifest, QA summary, data manifest, checksum file, lockfile, software revision, and analysis configuration are inseparable provenance records.

## Dataset manifest

`manifest.json` follows the strict `pv-signal-radar.research/v1` Go API contract. It records dataset identity/title, source files and hashes, coverage, processing policies/source commit, vocabulary treatment, artifact hashes/byte counts, completeness, and limitations. `metadata/environment.json` records the fixed build timestamp, R runtime, source commit, and package versions; its hash is bound into the dataset manifest. It is nested so a manifest-registry scan does not decode it as a dataset manifest. `source_manifest.csv` records each local file's original basename, official source URL, per-file retrieval timestamp, coverage, and SHA-256. Absolute workstation paths are deliberately excluded.

The completeness object keeps four different populations explicit:

| Field | Population counted |
|---|---|
| `source_demo_rows` | Raw DEMO rows, including case versions later superseded |
| `current_case_reports` | Retained current report versions after one `PRIMARYID` is selected per `CASEID` |
| `eligible_reports` | Current reports with at least one retained drug-role/event pair; this is the aggregate analysis denominator |
| `drug_event_pairs` | Unique report–drug-text–source-role–event-PT pairs before the derived `suspect` and `all` role unions |

`qa_summary.csv` uses the same names. `superseded_demo_rows` is the difference between source DEMO rows and current case reports; it is not a count of distinct cases.

## Missingness

Identifiers, roles, drug text, and PT are required for an eligible pair. Optional source fields remain missing; the ETL does not impute them. Every field completeness entry declares `population: eligible_reports` and `denominator_records`; superseded versions and current reports without a retained pair do not enter that percentage. Every stratified analysis must report its own included/missing counts because complete-case selection changes the comparator and can change disproportionality.
