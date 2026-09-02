# Biases, limitations, and observable failure modes

## Interpretive limitations

- **No causal attribution:** co-occurring drugs and events in one report are not individually linked. A high PRR/ROR can arise without a causal effect.
- **No incidence or individual risk:** spontaneous reports lack a reliable exposed-population denominator.
- **Reporting bias:** notoriety, stimulated reporting, media coverage, litigation, geography, reporter mix, and time can change the numerator and comparator.
- **Duplicate and follow-up behavior:** the pipeline retains the latest case version, but distinct case IDs can still describe related or duplicate clinical episodes.
- **Confounding and indication:** disease, co-medication, treatment channeling, and indication can create or suppress an association.
- **Terminology drift:** quarterly PT text does not establish the MedDRA release; source drug strings are not a canonical ingredient vocabulary.
- **Competition bias:** changing the set of drugs or events changes every comparator count.
- **Multiplicity:** screening many pairs produces chance extremes. FDR controls a declared family of tests, not causality.
- **Sparse data:** asymptotic intervals and continuity corrections can dominate small tables; always inspect cells and Fisher results.
- **Temporal instability:** live openFDA results and later FAERS revisions can change. A frozen, integrity-verified snapshot is necessary but not sufficient for reproduction; the configuration, code revision, runtime, and vocabulary assumptions must also be fixed.

The FDA describes public FAERS data as one input to a broader postmarket process, with duplicate reports and varying case definitions; the [openFDA drug-event documentation](https://open.fda.gov/apis/drug/event/) explicitly warns against inferring causality or incidence.

## Fail-closed conditions

| Condition | Why it is unsafe | Observable behavior |
|---|---|---|
| Missing or mismatched source SHA-256 | Input identity is unproven | Build exits nonzero and names the file |
| Invalid `CASEID`, `PRIMARYID`, or `CASEVERSION` | Deduplication becomes ambiguous | Build exits nonzero with invalid-row count |
| Missing or duplicate DEMO/DRUG/REAC table | A quarter is incomplete or ambiguous | Build exits nonzero and names table/quarter |
| ZIP traversal path | Extraction could escape temporary storage | Archive is rejected before extraction |
| No eligible drug-event pair | Measures have no defined universe | Build exits nonzero |
| Negative reconstructed 2 × 2 cell | Marginals are inconsistent | Aggregation exits nonzero |
| Non-empty output directory | Prior and new artifacts could be mixed | Build refuses to write |
| Missing R dependency or wrong method version | Runtime is not the declared environment | Bootstrap/build exits nonzero |
| Dirty, missing, or conflicting application revision | Analysis ID could identify code that was not executed | Research-mode startup exits nonzero |
| Exact-test or result family exceeds the online work bound | Public request could monopolize CPU or memory | Request returns a batch-method/unsupported-protocol error; no rows are truncated |
| Analysis/export concurrency or pacing limit reached | Concurrent work could exhaust CPU or memory | API returns `429` with `Retry-After` |

## Conditions that do not automatically fail

Optional demographic missingness, unmapped drug text, and unclassified PTs are retained because silently deleting or guessing them would conceal uncertainty. Their counts must be reviewed in QA and disclosed in each analysis. A scientifically unacceptable missingness level is protocol-specific and must be set before analysis.
