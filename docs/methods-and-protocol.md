# Methods and protocol contract

## Unit of analysis

For frozen FAERS research datasets, the ETL selects the maximum numeric `CASEVERSION` per `CASEID`; a maximum numeric `PRIMARYID` tie-break makes ambiguous duplicates deterministic and visible in QA. An eligible report must have at least one retained drug row and one non-empty source Preferred Term. Each `PRIMARYID × drug text × drug role × PT` appears once.

`PROD_AI` is preferred as source drug text and `DRUGNAME` is the fallback. Text is trimmed, whitespace-normalized, and uppercased. It is not an RxNorm, UNII, ATC, DCB, WHODrug, or ingredient-concept mapping. PT text is retained from the quarterly source; the pipeline does not infer a MedDRA version.

## Required pre-specification

Before analysis, freeze:

1. dataset and coverage period;
2. drug concept/mapping version and included source names;
3. drug roles, such as primary suspect (`PS`) or secondary suspect (`SS`);
4. event scope and any DME/IME or clinical-category reference version;
5. comparator and exclusions;
6. demographic, geographic, seriousness, and temporal strata;
7. methods, zero-cell policy, confidence level, multiplicity procedure, and threshold profile; and
8. controls and evaluation measures when selecting a threshold.

Changing one of these after looking at the result creates a new analysis and must produce a new analysis identifier.

## 2 × 2 measures

For one drug group `D` and event `E`, `a` is the number of eligible reports containing both, `b` contains `D` without `E`, `c` contains `E` without `D`, and `d` contains neither. Counts are report counts, not prescriptions, exposed people, cases caused, or incidence denominators.

- `PRR = [a/(a+b)] / [c/(c+d)]`
- `ROR = (a×d)/(b×c)`

The Go implementation and `research/R/reference_metrics.R` calculate fixed two-sided 95% asymptotic confidence limits on the log scale with the full `qnorm(0.975)` equivalent, not the `1.96` shorthand. When a cell is zero, the current reference implementation adds 0.5 to all cells and records the input-cell/correction choice on each metric. Fisher's exact two-sided p-value is available for sparse tables; online execution fails closed rather than enumerating more than 100,000 support terms. If multiple p-values are displayed or used, Benjamini-Hochberg FDR adjustment is required and both raw and adjusted values must be retained.

The guided profile `a ≥ 3`, `PRR ≥ 2`, and Yates-corrected `χ² ≥ 4` is an educational Evans-style heuristic. It is not an “EMA criterion,” universal decision threshold, or proof of a safety signal. The [EMA methodological addendum](https://www.ema.europa.eu/en/documents/scientific-guideline/guideline-good-pharmacovigilance-practices-gvp-module-ix-addendum-i-methodological-aspects-signal_en.pdf) requires thresholds to be appropriate, documented, and evaluated for the database and purpose.

## Additional methods

The separated R environment reserves exact direct versions of `faers 1.8.0`, `pvda 0.0.4`, and `openEBGM 0.9.1` for independent ETL/method comparison, BCPNN IC/IC025, and GPS EBGM/EB05 work. These methods are not considered validated or enabled merely because packages are listed. Adapters, golden outputs, reference sets, and license review remain release gates.

No recommended research threshold will be published until a prespecified positive/negative reference set reports sensitivity, positive predictive value, false-positive behavior, and time-to-detection. Threshold performance from one dataset, period, or event scope does not automatically transfer to another.

## Event families and source comparison

Clinical adverse-event PTs must be analyzed separately from medication-use circumstances, product-quality issues, errors, ineffectiveness, abuse, misuse, and off-label-use terms. Until a versioned classification is available, the ETL labels every PT `UNCLASSIFIED_SOURCE_PT` rather than guessing.

Two data sources may be compared only after the same versioned drug and event concepts are matched. Report effect estimates, intervals, coverage, missingness, and heterogeneity side by side. Set intersection means “observed in both configured analyses,” never causality, replication, validation, or confirmation. FAERS global and Brazil are not a US-versus-Brazil comparison unless FAERS is explicitly filtered to US reports.
