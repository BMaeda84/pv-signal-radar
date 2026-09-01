# PV Signal Radar 🛰️

[![CI](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml/badge.svg)](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)

> **Open-Source Real-Time Pharmacovigilance Disproportionality & Signal Detection Engine.**  
> Mining OpenFDA FAERS adverse event reports with standard regulatory statistical data mining algorithms (PRR, ROR, Yates' $\chi^2$, 95% Confidence Intervals, and Volcano Plots).

---

## 🔬 Overview & Motivation

Spontaneous reporting systems (such as the US FDA FAERS and WHO VigiBase) are foundational to post-marketing drug safety surveillance. However, commercial signal detection suites (such as Oracle Empirica or IQVIA) are locked behind high-cost enterprise contracts, while open-source alternatives primarily exist as command-line R scripts requiring manual data engineering across gigabytes of raw ASCII records.

**PV Signal Radar** bridges this gap:
- **Zero-Latency Data Mining**: Queries the live openFDA adverse events dataset and dynamically reconstructs the $2 \times 2$ contingency matrix across the 25M+ FAERS universe.
- **Regulatory-Grade Statistics**: Calculates Proportional Reporting Ratio ($\text{PRR}$), Reporting Odds Ratio ($\text{ROR}$), 95% Confidence Intervals, and Yates' Continuity-Corrected Chi-Square ($\chi^2$).
- **Ultra-Lightweight Footprint**: Written in Go with an embedded HTML5/Canvas dashboard. Single static binary, RAM footprint $< 20\text{ MB}$, and zero external database dependencies.
- **Interactive Visualizations**: Real-time Volcano Plot ($\log_2(\text{PRR})$ vs. $\sqrt{\chi^2}$) and expandable matrix inspector.

---

## 📐 Mathematical Formulation

### 1. The $2 \times 2$ Contingency Table

For a given target active substance ($D$) and a specific adverse reaction ($E$), the reporting space is partitioned into four cells against the total database universe ($N$):

$$\begin{array}{c|c|c|c}
 & \text{Target Reaction } (E) & \text{Other Reactions } (\neg E) & \text{Total} \\
\hline
\text{Target Drug } (D) & a & b & a + b \\
\hline
\text{Other Drugs } (\neg D) & c & d & c + d \\
\hline
\text{Total} & a + c & b + d & N = a + b + c + d \\
\end{array}$$

Where:
- $a = \text{Reports with Drug } D \text{ and Reaction } E$
- $b = \text{Reports with Drug } D \text{ and other reactions } = (a + b) - a$
- $c = \text{Reports with other drugs and Reaction } E = (a + c) - a$
- $d = \text{Reports with other drugs and other reactions } = N - (a + b + c)$

---

### 2. Proportional Reporting Ratio (PRR)

Compares the proportion of reaction $E$ for drug $D$ against the background proportion for all other drugs:

$$\text{PRR} = \frac{\frac{a}{a + b}}{\frac{c}{c + d}}$$

Asymptotic Standard Error of $\ln(\text{PRR})$:
$$\text{SE}(\ln \text{PRR}) = \sqrt{\frac{1}{a} - \frac{1}{a + b} + \frac{1}{c} - \frac{1}{c + d}}$$

Two-sided 95% Confidence Interval:
$$\text{CI}_{95\%}(\text{PRR}) = \left[ \exp\left(\ln \text{PRR} - 1.96 \cdot \text{SE}\right), \; \exp\left(\ln \text{PRR} + 1.96 \cdot \text{SE}\right) \right]$$

---

### 3. Reporting Odds Ratio (ROR)

Measures the odds of reporting reaction $E$ with drug $D$ relative to all other drugs (analogous to a case-control odds ratio):

$$\text{ROR} = \frac{a \cdot d}{b \cdot c}$$

Asymptotic Standard Error of $\ln(\text{ROR})$ (with Haldane-Anscombe $0.5$ continuity correction for zero cells):
$$\text{SE}(\ln \text{ROR}) = \sqrt{\frac{1}{a} + \frac{1}{b} + \frac{1}{c} + \frac{1}{d}}$$

Two-sided 95% Confidence Interval:
$$\text{CI}_{95\%}(\text{ROR}) = \left[ \exp\left(\ln \text{ROR} - 1.96 \cdot \text{SE}\right), \; \exp\left(\ln \text{ROR} + 1.96 \cdot \text{SE}\right) \right]$$

---

### 4. Yates' Continuity-Corrected Chi-Square ($\chi^2_{\text{Yates}}$)

Corrects for the continuous approximation of the discrete 1-degree-of-freedom binomial distribution:

$$\chi^2_{\text{Yates}} = \frac{N \cdot \left( \max\left(0, |a \cdot d - b \cdot c| - \frac{N}{2}\right) \right)^2}{(a + b)(c + d)(a + c)(b + d)}$$

---

### 5. Regulatory Signal Decision Rules (Evans et al. / EMA Guidelines)

An association is classified as an **Active Statistical Signal** when all three conditions are met simultaneously:
1. $\text{Case Count } (a) \ge 3$
2. $\text{PRR} \ge 2.0$
3. $\chi^2_{\text{Yates}} \ge 4.0$ (corresponds to $p < 0.0455$)

---

## 🚀 Quick Start

### Running with Go

```bash
# Clone the repository
git clone https://github.com/BMaeda84/pv-signal-radar.git
cd pv-signal-radar

# Run server
go run ./cmd/server
```

Open [http://localhost:8080](http://localhost:8080) in your browser.

### Running with Docker

```bash
# Build Docker image
docker build -t pv-signal-radar .

# Run container
docker run -p 8080:8080 pv-signal-radar
```

---

## 📡 REST API Specifications

### 1. Analyze Drug Disproportionality
```http
GET /api/v1/analyze?drug=Semaglutide
```

#### Response Payload (`200 OK`)
```json
{
  "query_drug": "Semaglutide",
  "normalized_drug": "Semaglutide",
  "drug_total_reports": 58320,
  "database_universe_n": 26145890,
  "active_signals_count": 8,
  "total_reactions_analyzed": 25,
  "signals": [
    {
      "reaction": "PANCREATITIS",
      "count_a": 1420,
      "drug_total": 58320,
      "reaction_total": 89400,
      "prr": 7.12,
      "prr_lower_95": 6.75,
      "prr_upper_95": 7.51,
      "ror": 7.34,
      "ror_lower_95": 6.95,
      "ror_upper_95": 7.75,
      "chi_square_yates": 7854.2,
      "p_value_approx": 0.0,
      "signal_level": "ACTIVE_SIGNAL",
      "signal_score": 251.2,
      "interpretation": "Statistically significant disproportionate reporting detected (Evans/EMA criteria satisfied). Clinical review recommended."
    }
  ],
  "timestamp": "2026-09-01T22:00:00Z"
}
```

### 2. Health & Cache Status
```http
GET /api/v1/health
```

---

## ⚖️ Methodological & Epistemic Disclaimers

> [!WARNING]
> 1. **Non-Causality**: Spontaneous reports in the FAERS database reflect suspicions and observed associations submitted by healthcare professionals, pharmaceutical companies, and consumers. A report does not establish clinical causality or proof that the drug caused the reaction.
> 2. **Lack of Denominator**: Spontaneous reporting databases measure reporting ratios, not clinical incidence rates. The true number of patients exposed to the drug (prescription volume) is absent.
> 3. **Weber Effect & Reporting Biases**: Factors such as media coverage, notoriety bias, litigation publicity, and product launch timelines can substantially stimulate reporting volumes.
> 4. **Intended Use**: This tool is an exploratory, hypotheses-generating data mining instrument intended for researchers, pharmacists, and epidemiologists. It is not a clinical decision support system or medical advice.

---

## 📄 License

Distributed under the **MIT License**. See [`LICENSE`](./LICENSE) for more information.

Developed by **[Bruno Maeda](https://github.com/BMaeda84)**.
