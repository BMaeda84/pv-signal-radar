# Scientific software and repository selection record

Last reviewed: 2026-09-02. This is a dependency decision record, not an endorsement or a validation certificate. Version, license, maintenance, numerical agreement, data model, and reproducible installation are separate gates; a package implementing a named method is not automatically interchangeable with another implementation.

## Adopted in the isolated R batch environment

| Component | Frozen version | Role | Selection criterion | Boundary |
|---|---:|---|---|---|
| [`faers`](https://bioconductor.org/packages/release/bioc/html/faers.html) | 1.8.0 | FAERS-oriented ETL and independent method reference | Released in Bioconductor 3.23, documented source repository, MIT license, R 4.6 release contract | The project keeps its own explicit relational ETL; package presence does not validate a quarter or replace source-level reconciliation |
| [`pvda`](https://CRAN.R-project.org/package=pvda) | 0.0.4 | Independent PRR, ROR, and IC comparison | Published CRAN package with reference manual/tests and declared GPL >= 3 license | Used only in the separate R process; numerical adapters and declared tolerances are required before citing agreement |
| [`openEBGM`](https://CRAN.R-project.org/package=openEBGM) | 0.9.1 | GPS/EBGM, quantiles, and stratified batch work | Published CRAN implementation with method vignettes and GPL-2/GPL-3 licensing | Hyperparameter estimation, strata, convergence, and EB05 must be frozen in the protocol; it is never approximated in Go |

The transitive environment is recorded in `research/renv.lock`, not resolved from “latest” during analysis. GPL-family packages are not linked into the MIT Go binary; distribution obligations for the batch image still require a release-time license review.

## Exploratory benchmarks, not central dependencies

| Candidate | Evidence considered | Why it is not central |
|---|---|---|
| [`vigipy`](https://github.com/Shakesbeery/vigipy) | Python implementations of BCPNN, GPS, PRR, ROR, Fisher, LASSO, and longitudinal analysis; GPLv3 | Useful as a cross-language benchmark, but its own repository lists method-documentation and demonstration-dataset work as pending. It is not used to define canonical results. |
| [`faers` on PyPI](https://pypi.org/project/faers/) | Version 0.1, one 1.4 kB source release uploaded in 2015 | Name collision with the active Bioconductor project; age, scope, and release contents do not meet the pipeline requirements. |
| [`hypokrates`](https://pypi.org/project/hypokrates/) | Broad multi-source hypothesis-generation package, PyPI development status “Alpha,” AGPL-3.0-only | Different trust and product boundary: cross-database/LLM hypothesis generation rather than the frozen relational FAERS analysis contract. It may inform interoperability research only after data-lineage, numerical, license, and security review. |
| [`VigiLens`](https://github.com/firassa-ai/VigiLens) | Quarter-aware temporal safety-signal application | Product/repository comparison for temporal UX; not a numerical authority or source dependency. |
| [`PRISM-Pharmacovigilance`](https://github.com/Jehathsyed/PRISM-Pharmacovigilance) | Browser/openFDA disproportionality application | Useful UI comparator, but a live openFDA workflow cannot substitute for case-version-deduplicated frozen quarterly inputs. |

## Admission test for another package or repository

A candidate moves from benchmark to dependency only after all of the following are recorded:

1. immutable version/source hash, supported runtime, license, maintainer/release history, and reproducible clean install;
2. exact input unit and comparator semantics, duplicate/case-version handling, sparse-cell policy, interval definition, strata behavior, and failure modes;
3. golden 2 x 2 fixtures plus real-scale numerical comparison against at least one independent implementation with declared absolute/relative tolerances;
4. positive/negative reference-set behavior where the method will influence a threshold, including sensitivity, PPV, false-positive behavior, and time-to-detection;
5. memory/CPU bounds, malformed-input behavior, SBOM/vulnerability review, and deterministic output/provenance; and
6. a documented reason that the capability cannot be implemented more transparently at the current boundary.

Until that dossier exists, exploratory outputs must be labeled by implementation and version and cannot silently replace a configured method.
