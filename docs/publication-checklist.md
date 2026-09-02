# Publication, citation, and DOI checklist

## Software release

1. Reconcile the implemented commit against `docs/validation-and-evidence.md`; do not label a release “validated” while a critical row is not run.
2. Require protected-branch CI, an independent scientific review of methods/claims/residual risks, and a recorded approval in the `software-release` GitHub environment.
3. Create an annotated immutable semantic-version tag after the evidence review. Configure rulesets for the default branch and release tags, required checks/reviews, immutable releases, and required reviewers on the `software-release` environment before enabling publication.
4. Manually dispatch `.github/workflows/release.yml` with that existing tag. Its unprivileged job verifies the tag/commit, tests Go and the locked R image, scans both runtime images, builds binaries, and emits checksums, Go CycloneDX SBOMs, the R lock, and an R package/license inventory. Only the separate environment-gated job receives release/OIDC permissions, attests the verified bytes, and creates the GitHub release. Before dispatch, create the protected `software-release` environment, configure required reviewers, and add the environment-scoped secret `SOFTWARE_RELEASE_GATE` with the exact non-secret contract value `enabled-after-protection-v1`. Do not define this name as a repository- or organization-level secret: scope, rather than confidentiality, is the control. The first publication step fails closed when the value is absent or different. This source-level gate does not prove or replace required reviewers, branch/tag rulesets, or immutable releases. Treat a missing external repository control or failed/skipped gate as a blocked release.
5. If the scientific R image will be distributed, add a separate OCI release gate: publish it by immutable digest, generate an OCI SBOM that covers OS and R packages, retain the vulnerability report, and sign/attest the image provenance. The current workflow only builds/scans the R image and releases its Dockerfile, lock, and package inventory; it does not publish or attest the image itself.
6. Bind restored dependency bytes, not only names and versions. Record SHA-256 values for the exact package archives or mirror objects used by the restore; the current `renv.lock` checksum protects the lock as a whole but its package records do not contain per-archive content hashes.
7. Archive the release source, `.zenodo.json`, and `CITATION.cff`. Enable the GitHub–Zenodo repository integration only through the repository owner account, then reserve/publish a DOI for the reviewed release.
8. Add the final DOI to release metadata in a follow-up commit. Never place a reserved draft DOI in a public citation as if it were published.

Enabling branch/tag rulesets, immutable releases, the protected `software-release` environment, Zenodo, or a DOI are external publication actions and are not performed by this source change. The 2026-09-02 audit found no rulesets, an unprotected default branch, no GitHub environments, and immutable releases disabled. Source code cannot prove those repository settings remain active after configuration; recheck them immediately before each release.

## Dataset and analysis release

The software DOI does not identify data or analysis parameters. Publish a separate immutable record containing or linking:

- dataset and analysis IDs;
- official source URLs, coverage, retrieval times, and SHA-256 values;
- output `manifest.json`, `metadata/environment.json`, `source_manifest.csv`, `qa_summary.csv`, and `checksums.sha256`;
- exact source-code commit and reviewed `renv.lock`;
- machine-readable analysis configuration and method definitions;
- canonical result digest, row count/order, hypothesis-family definition, and independently attested reproduction evidence;
- result CSV/Parquet plus a human-readable methods/limitations report;
- numerical benchmark/reference-set evidence for enabled threshold profiles;
- deviations, approver, license/redistribution disposition, and contact/correction route.

If source-data terms prohibit redistribution, publish reconstruction instructions, source hashes, code, metadata, and permitted derived results instead of the restricted bytes.

## Manuscript minimum

Identify database version/coverage, case-version handling, report eligibility, drug mapping and role, PT/event scope, comparator, strata, measures, confidence intervals, sparse correction, multiplicity procedure, threshold, software/package versions, and missingness. Report effect estimates and uncertainty rather than only threshold labels. Cite the software DOI and dataset/analysis DOI independently and follow [READUS-PV](https://pmc.ncbi.nlm.nih.gov/articles/PMC11116242/).

## Correction or withdrawal

Never replace an archived artifact in place. Publish a linked correction or tombstone with reason, impact, replacement identifier, affected analyses, and date. Keep the original checksum and evidence record so readers can identify what changed.
