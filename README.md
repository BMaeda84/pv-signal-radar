# PV Signal Radar 🛰️

[Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · **English**

[![CI](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml/badge.svg)](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)

> An open-source, **exploratory** pharmacovigilance screening dashboard. It queries public [openFDA FAERS](https://open.fda.gov/apis/drug/event/) reports and computes PRR, ROR, 95% confidence intervals, and Yates-corrected chi-square for hypothesis generation.

## What it does

For an active substance, the service retrieves the most frequently reported MedDRA Preferred Terms (PTs) and constructs a report-level 2 × 2 table for each term:

| | Target reaction (E) | Other reactions (¬E) | Total |
|---|---:|---:|---:|
| Target drug (D) | a | b | a + b |
| Other drugs (¬D) | c | d | c + d |
| Total | a + c | b + d | N |

The dashboard presents:

- `PRR = [a / (a + b)] / [c / (c + d)]`
- `ROR = (a × d) / (b × c)`
- asymptotic 95% confidence intervals, with Haldane-Anscombe correction when a cell is zero;
- Yates-corrected chi-square and an exploratory volcano plot; and
- a configurable screening label when `a ≥ 3`, `PRR ≥ 2.0`, and `χ² ≥ 4.0`.

The label is a prioritization rule implemented by this project, **not** a regulatory decision, a clinical finding, or evidence of causality.

## Languages and accessibility

The public interface supports **Português (Brasil)**, **Español**, and English. The selector persists locally in the browser, updates document language/title/metadata, localizes numerical formatting, and does not alter API payloads or cached results. MedDRA PT values and API enums remain unchanged to preserve their source semantics.

The dashboard includes a labelled search field, keyboard-operable 2 × 2 matrix controls, canvas text alternatives, visible focus states, and responsive layout behavior.

## Run locally

### Go

```powershell
git clone https://github.com/BMaeda84/pv-signal-radar.git
Set-Location pv-signal-radar
go run ./cmd/server
```

Open <http://localhost:8080>.

### Docker

```powershell
docker build --tag pv-signal-radar .
docker run --rm --publish 8080:8080 pv-signal-radar
```

The image runs as a non-root user and exposes `/api/v1/health` for container/orchestrator health checks.

## Configuration

| Variable | Default | Purpose |
|---|---:|---|
| `PORT` | `8080` | HTTP listening port. Railway supplies this automatically. |
| `OPENFDA_API_KEY` | unset | Optional openFDA API key. Keep it in the deployment secret store; never commit it. |
| `CACHE_CAPACITY` | `500` | Maximum number of completed analyses held in the in-memory LRU cache. |
| `CACHE_TTL_HOURS` | `24` | TTL for completed analysis cache entries. |

The service limits concurrent cache-miss analyses and spaces scan starts by at least 15 seconds in each process. At the current maximum of 28 upstream requests per scan, no more than five starts can occur in a 60-second window (140 openFDA requests) without an initial burst. A busy or rate-gated service returns `429` with `Retry-After`; it does not silently queue unlimited work. Upstream daily and multi-instance quotas still need operational monitoring.

## API

```http
GET /api/v1/analyze?drug=Semaglutide
GET /api/v1/health
```

`/api/v1/analyze` accepts only `GET`. Its response includes the queried substance, current FAERS universe, source counts, metrics, a stable `signal_level`, timestamp, and an exploratory-use disclaimer. Successful analyses are cached server-side, while HTTP responses use `Cache-Control: no-store`.

Expected error codes are `drug_required` (`400`), `invalid_drug` (`400`), `method_not_allowed` (`405`), `analysis_busy` (`429`), `analysis_rate_limited` (`429`), and `analysis_unavailable` (`502`). Error responses intentionally omit upstream transport details, so an `OPENFDA_API_KEY` cannot be reflected to a public client.

## Data-quality boundary

This repository is deliberately conservative about incomplete source data: it rejects an analysis when it cannot obtain the current universe or a reaction background marginal. It does **not** substitute a fixed historical universe or substitute the target count for an unavailable background count, because either behavior can fabricate an extreme PRR/ROR and a false screening signal.

It still has material methodological limits:

1. FAERS reports are spontaneous, incomplete, and subject to reporting, notoriety, duplicate, and temporal biases.
2. A report can contain multiple drugs and multiple reactions. Public openFDA data do not individually establish that a particular drug caused a particular reaction.
3. The project does not add case-level deduplication, exposure denominators, confounder adjustment, clinical adjudication, or an immutable data snapshot.
4. openFDA updates over time; the same query can legitimately return different results later.
5. This is not medical advice, a clinical decision-support system, or a validated/qualified GxP or regulatory reporting system.

Use results only as a starting point for qualified pharmacovigilance review with appropriate case-level and clinical evidence.

## Verification

```powershell
go test -race ./...
go vet ./...
go build ./cmd/server
docker build --tag pv-signal-radar:local .
```

CI runs race-enabled unit tests, static analysis, binary compilation, and a Docker build on every pull request and push to `main`.

## Dual-Base Harmonization (US FDA FAERS × Brasil ANVISA VigiMed)

Version 2.0 introduces cross-jurisdictional screening comparing global **FDA FAERS** notifications against **ANVISA VigiMed** (Brazil's official pharmacovigilance system based on WHO UMC VigiFlow).

- **Substance Harmonization**: Cross-referenced using universal **WHO-ATC** classification codes (e.g. `A10BJ06` for Semaglutide) and Brazilian DCB catalog.
- **Reaction Terminology**: Standardized via **MedDRA Preferred Terms (PT)** with bilingual Portuguese/English indexing.
- **Comparative Concordance**: Evaluates whether active signals in the global FDA dataset replicate in Brazilian epidemiological surveillance.

## Feedback & Community Research Queue

Researchers and pharmacovigilance professionals can flag questionable data points, calculation discrepancies, or suspected reporting biases directly on any metric card, table row, or 2 × 2 matrix. Flagged items accumulate in an in-memory review queue before being submitted via `POST /api/v1/feedback`.

```http
POST /api/v1/feedback
Content-Type: application/json

{
  "email": "researcher@institution.edu",
  "comments": "Observed notoriety bias post-media coverage.",
  "flagged_statistics": [
    {
      "drug": "Semaglutide",
      "reaction": "PANCREATITIS",
      "jurisdiction": "FDA",
      "metric": "PRR",
      "displayed_value": "7.12 [6.75 - 7.51]",
      "reason": "Suspected stimulated reporting"
    }
  ]
}
```

## Scientific Bibliography & Official Sources

### Primary Regulatory Guidelines
- **ANVISA (Brazil)**: [RDC nº 406/2020](https://www.gov.br/anvisa/pt-br/assuntos/medicamentos/farmacovigilancia/legislacao) & [RDC nº 967/2025](https://www.gov.br/anvisa/pt-br/assuntos/medicamentos/farmacovigilancia/legislacao) (Good Pharmacovigilance Practices & VigiMed).
- **US FDA**: [FDA Guidance for Industry: Good Pharmacovigilance Practices and Pharmacoepidemiologic Assessment (2005)](https://www.fda.gov/media/72257/download).
- **EMA (EU)**: [Guideline on good pharmacovigilance practices (GVP) – Module IX – Signal Management (Rev 1)](https://www.ema.europa.eu/en/documents/scientific-guideline/guideline-good-pharmacovigilance-practices-gvp-module-ix-signal-management-rev-1_en.pdf).
- **CIOMS**: [Practical Aspects of Signal Detection in Pharmacovigilance (Report of CIOMS Working Group VIII)](https://cioms.ch/publications/product/practical-aspects-of-signal-detection-in-pharmacovigilance-report-of-cioms-working-group-viii/). ISBN: `978-92-9036-082-7`.

### Seminal Literature
- **Evans, S. J., et al. (2001)**. *Use of proportional reporting ratios (PRRs) for signal generation from spontaneous adverse drug reaction reports*. *Pharmacoepidemiology and Drug Safety*, 10(6), 483–486. DOI: [10.1002/pds.677](https://doi.org/10.1002/pds.677)
- **van Puijenbroek, E. P., et al. (2002)**. *A comparison of statistical methods for signal detection in spontaneous reporting systems*. *Pharmacoepidemiology and Drug Safety*, 11(1), 3–10. DOI: [10.1002/pds.668](https://doi.org/10.1002/pds.668)
- **Rothman, K. J., et al. (2004)**. *The Reporting Odds Ratio and its advantages over the Proportional Reporting Ratio*. *Pharmacoepidemiology and Drug Safety*, 13(8), 519–523. DOI: [10.1002/pds.1001](https://doi.org/10.1002/pds.1001)

### Textbooks (Non-Affiliate Links)
- *Mann's Pharmacovigilance* (3rd Ed., Wiley-Blackwell). ISBN: `978-0-470-67104-7`. [Wiley](https://www.wiley.com/en-us/Mann%27s+Pharmacovigilance%2C+3rd+Edition-p-9780470671047) · [Amazon](https://www.amazon.com/dp/0470671048)
- *Pharmacoepidemiology* (6th Ed., Wiley). ISBN: `978-1-119-41342-4`. [Wiley](https://www.wiley.com/en-us/Pharmacoepidemiology%2C+6th+Edition-p-9781119413424) · [Amazon](https://www.amazon.com/dp/1119413423)

## AI Transparency & Attribution

The architecture, statistical pipelines, and cross-dataset harmonization of **PV Signal Radar** were developed with advanced Artificial Intelligence assistance from:
- **Google AI**: Google Antigravity platform with the **Gemini 3.7 Flash High** model.
- **OpenAI**: **GPT-Sol 5.6 Ultra** model.

## License

Distributed under the [MIT License](LICENSE). Developed by [Bruno Maeda](https://github.com/BMaeda84).
