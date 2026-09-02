# Intended use and user responsibilities

## Intended use

PV Signal Radar supports education, transparent exploration, and reproducible academic study of disproportionate reporting in public spontaneous-report datasets. Its outputs are statistical reporting associations (`SDRs`, signals of disproportionate reporting) that require qualified review and external evidence.

Suitable uses include:

- teaching 2 × 2 tables, PRR, ROR, sparse-data behavior, reporting bias, and protocol design;
- generating documented hypotheses for literature or case-series review;
- reproducing a published analysis from a frozen source manifest and configuration;
- comparing methods, thresholds, time windows, or predefined strata; and
- preparing tables and figures for a manuscript when the full protocol and limitations accompany them.

## Explicitly excluded uses

The application is not medical advice, diagnosis, individual risk estimation, an exposure-incidence calculator, clinical decision support, an adverse-event reporting system, a validated or qualified GxP system, or an automated regulatory decision engine. It does not establish that a drug caused an event and must not be used as the only evidence for changing treatment, labeling, benefit-risk conclusions, or regulatory action.

The live openFDA endpoint is an exploratory convenience. Because its source changes, it does not provide a frozen research dataset and its result must not be cited as a reproducible analysis.

## Audience-specific use

### Students

Use guided mode with the displayed 2 × 2 cells. Recalculate at least one row, explain the comparator and zero-cell correction, and identify one bias that can change the measure without changing biological risk. Do not call a threshold crossing a confirmed safety signal.

### Researchers

Register a protocol before inspecting results. Freeze the dataset and software revisions, declare drug role, event scope, comparator, period, strata, multiplicity strategy, and threshold profile, then archive the export bundle with the study materials. Follow the [READUS-PV recommendations](https://pmc.ncbi.nlm.nih.gov/articles/PMC11116242/) for reporting; the checklist improves reporting quality but does not validate a study or causal claim.

### Pharmacovigilance professionals

Treat an SDR as a triage item. Review cases, temporal patterns, duplicates, stimulated reporting, indications, concomitant drugs, biological plausibility, literature, exposure context, and other data sources. Follow the applicable organizational quality system and [EMA GVP Module IX](https://www.ema.europa.eu/en/documents/scientific-guideline/guideline-good-pharmacovigilance-practices-gvp-module-ix-signal-management-rev-1_en.pdf); this software does not replace validation, confirmation, analysis, prioritization, or assessment steps.

## Minimum citation boundary

A formal result must identify the dataset ID, source coverage, source and output checksums, analysis ID/configuration, software commit, R lockfile, method definitions, exclusions, and known deviations. If any is absent, describe the result as exploratory and non-reproducible.
