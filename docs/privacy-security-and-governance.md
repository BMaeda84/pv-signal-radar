# Privacy, security, and governance

## Data and feedback

The public application must not accept private case uploads in v1. The previous feedback endpoint collected email, IP address, user agent, and free text without a complete retention and privacy control set; public feedback is therefore replaced by GitHub Issues/Discussions until a documented controller, purpose, notice, consent/legal basis, retention period, deletion process, access control, durable store, abuse handling, and incident process exist.

Do not paste patient information, report narratives, credentials, API keys, unpublished analyses, or institution-confidential data into issues. GitHub is a separate service with its own terms and retention behavior.

## Artifact governance

- Inputs are official frozen public files supplied by the investigator; the service never downloads them implicitly.
- Raw inputs, generated datasets, and source registers are ignored by Git in this repository.
- Publication uses immutable object storage with controlled access and an S3-compatible API, or an archival repository. Object lock/versioning and retention policy must be configured and tested by the operator.
- Checksums prove byte identity, not authenticity, scientific validity, or freedom from malicious content.
- A formal FAERS source register must point to the reviewed official FDA distribution URL and reconcile the declared quarter against filenames and coverage. Computing a hash after download freezes the observed bytes but is not an external trust anchor; retain the FDA page/response metadata and the independent review record.
- Dataset withdrawal never silently replaces an object. Publish a tombstone/correction linked to the immutable identifier.
- Secrets belong in deployment secret stores and must never enter manifests, logs, exported bundles, or Git history.

## Public API availability boundary

The application bounds new live/research analyses to two concurrent workers, spaces new research starts, applies a 20-second request deadline, fails an online result above 50,000 event rows, and bounds one export to 32 MiB per file, 64 MiB total, and 32 files. Fisher exact is opt-in and rejects an online calculation requiring more than 100,000 enumerated support terms. These controls fail the whole request; they never truncate the tested event family or silently substitute another method.

`RESEARCH_ALLOW_ONLINE_MATERIALIZATION` is false by default. In that resolve-only mode, a deterministic POST may retrieve an existing record but a cache miss cannot create permanent state. The start gate is process-local; enabling materialization in a multi-replica or public deployment still requires gateway-level identity/rate quotas, filesystem capacity alerts and quotas, request/response size limits, and retention/withdrawal operations for immutable analyses. Without those external controls, distributed callers can bypass per-process pacing or exhaust the result volume over time.

Software identity comes from the binary's embedded Go VCS metadata or release linker flag. `PV_RADAR_APPLICATION_COMMIT` is accepted only when the binary has no revision metadata, and a dirty build or mismatch is rejected; the variable is an operator attestation, not an override.

The scientific container is a batch boundary, not a network service. After dependency restore, execute transformations with networking disabled, a read-only root filesystem, no Linux capabilities, `no-new-privileges`, explicit CPU/memory/tmp/output quotas, read-only source mounts, and a fresh writable output directory. ZIP member/count/expanded-size limits protect the parser, but infrastructure quotas remain necessary for Arrow/Parquet and real-quarter workloads.

## Licensing boundary

Application source is distributed under MIT. FDA/ANVISA data, MedDRA/WHODrug terminology, scientific articles, containers, and R packages have independent terms. In particular, `pvda` and `openEBGM` use GPL-family licenses. The R batch environment is deliberately separate from the Go binary, and the Go service consumes generated data artifacts rather than linking R package code.

This separation reduces coupling but is not a legal determination. Before distribution, record each dependency version/license, data-source terms, vocabulary rights, whether data are redistributed or only reconstructed, and the intended audience/jurisdictions. Do not package proprietary MedDRA or WHODrug material without the necessary rights.

## Change control

Any change to source coverage, deduplication, eligibility, mapping, comparator, event scope, method, correction, threshold, package lock, or schema creates a new dataset/analysis version. Production publication requires a human gate after evidence reconciliation. Rollback means serving the previous immutable artifact and preserving the withdrawn artifact and reason; recovery is unproven until rehearsed in the selected store.
