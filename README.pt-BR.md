# PV Signal Radar 🛰️

**Português (Brasil)** · [Español](README.es.md) · [English](README.md)

[![CI](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml/badge.svg)](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml)
[![Licença: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)

> Painel open source de **triagem exploratória** em farmacovigilância. Consulta relatos públicos do [openFDA FAERS](https://open.fda.gov/apis/drug/event/) e calcula PRR, ROR, intervalos de confiança de 95% e qui-quadrado corrigido de Yates para geração de hipóteses.

## O que faz

Para uma substância ativa, o serviço busca os MedDRA Preferred Terms (PTs) mais relatados e monta uma tabela 2 × 2 no nível do relato:

| | Reação-alvo (E) | Outras reações (¬E) | Total |
|---|---:|---:|---:|
| Medicamento-alvo (D) | a | b | a + b |
| Outros medicamentos (¬D) | c | d | c + d |
| Total | a + c | b + d | N |

O painel apresenta:

- `PRR = [a / (a + b)] / [c / (c + d)]`;
- `ROR = (a × d) / (b × c)`;
- intervalos de confiança assintóticos de 95%, com correção de Haldane-Anscombe quando há célula zero;
- qui-quadrado corrigido de Yates e gráfico vulcão exploratório; e
- rótulo de triagem configurado quando `a ≥ 3`, `PRR ≥ 2,0` e `χ² ≥ 4,0`.

Esse rótulo é uma regra de priorização implementada pelo projeto; **não** é decisão regulatória, achado clínico nem evidência de causalidade.

## Idiomas e acessibilidade

A interface pública suporta **Português (Brasil)**, **Español** e English. O seletor persiste no navegador, atualiza idioma/título/metadados do documento e formatação de números, sem alterar o payload da API nem o cache. Termos MedDRA e enums da API permanecem no idioma de origem para preservar a semântica da fonte.

Há campo de busca rotulado, controles de matriz 2 × 2 operáveis por teclado, alternativa textual para o canvas, foco visível e layout responsivo.

## Execução local

### Go

```powershell
git clone https://github.com/BMaeda84/pv-signal-radar.git
Set-Location pv-signal-radar
go run ./cmd/server
```

Abra <http://localhost:8080>.

### Docker

```powershell
docker build --tag pv-signal-radar .
docker run --rm --publish 8080:8080 pv-signal-radar
```

A imagem roda com usuário não-root e expõe `/api/v1/health` para health checks de container/orquestrador.

## Configuração

| Variável | Padrão | Finalidade |
|---|---:|---|
| `PORT` | `8080` | Porta HTTP. Railway injeta esse valor automaticamente. |
| `OPENFDA_API_KEY` | não definida | Chave opcional do openFDA. Use o secret store do deploy; nunca faça commit. |
| `CACHE_CAPACITY` | `500` | Máximo de análises completas no cache LRU em memória. |
| `CACHE_TTL_HOURS` | `24` | TTL, em horas, de entradas completas do cache. |

Cada análise sem cache gera várias requisições ao openFDA; por isso o serviço limita análises simultâneas e espaça inícios de scan em pelo menos 15 segundos por processo. Com o máximo atual de 28 requisições upstream por scan, não podem ocorrer mais de cinco inícios em uma janela de 60 segundos (140 chamadas openFDA), sem burst inicial. Em saturação ou limitação de ritmo responde `429` com `Retry-After`, em vez de enfileirar trabalho sem limite. Quotas diárias do upstream e múltiplas instâncias ainda exigem monitoramento operacional.

## API

```http
GET /api/v1/analyze?drug=Semaglutide
GET /api/v1/health
```

`/api/v1/analyze` aceita apenas `GET`. A resposta traz substância consultada, universo FAERS atual, contagens de origem, métricas, `signal_level` estável, timestamp e aviso de uso exploratório. Análises concluídas ficam no cache do servidor; as respostas HTTP usam `Cache-Control: no-store`.

Os códigos de erro são `drug_required` (`400`), `invalid_drug` (`400`), `method_not_allowed` (`405`), `analysis_busy` (`429`), `analysis_rate_limited` (`429`) e `analysis_unavailable` (`502`). Erros do upstream não são serializados ao cliente, evitando reflexão de `OPENFDA_API_KEY` em resposta pública.

## Limite de qualidade dos dados

O projeto falha de forma explícita quando não consegue obter o universo atual ou o background de uma reação. Não substitui o universo por constante histórica nem o background ausente pelo numerador, porque isso pode fabricar PRR/ROR extremos e falso sinal de triagem.

Ainda há limites materiais:

1. Relatos FAERS são espontâneos, incompletos e sujeitos a vieses de notificação, notoriedade, duplicidade e tempo.
2. Um relato pode conter diversos medicamentos e diversas reações; dados públicos do openFDA não estabelecem individualmente que um medicamento causou determinada reação.
3. O projeto não adiciona deduplicação por caso, denominador de exposição, ajuste de confundidores, adjudicação clínica nem snapshot imutável da fonte.
4. O openFDA é atualizado ao longo do tempo; a mesma consulta pode retornar resultado diferente depois.
5. Não é aconselhamento médico, CDS, sistema GxP validado/qualificado nem sistema de reporte regulatório.

Use o resultado apenas como ponto de partida para revisão por profissionais qualificados, com evidência clínica e de caso apropriada.

## Verificação

```powershell
go test -race ./...
go vet ./...
go build ./cmd/server
docker build --tag pv-signal-radar:local .
```

A CI executa testes com race detector, análise estática, build do binário e build Docker em todo pull request e push para `main`.

## Harmonização em Base Dupla (US FDA FAERS × Brasil ANVISA VigiMed)

A versão 2.0 introduz triagem comparativa internacional confrontando os relatos globais do **FDA FAERS** com o **ANVISA VigiMed** (sistema oficial brasileiro baseado na plataforma WHO UMC VigiFlow).

- **Harmonização de Substâncias**: Cruzamento por códigos universais **WHO-ATC** (ex: `A10BJ06` para Semaglutida) e catálogo da DCB.
- **Terminologia de Reações**: Padronização por **MedDRA Preferred Terms (PT)** com indexação bilíngue Português/Inglês.
- **Concordância Comparativa**: Avalia se sinais ativos identificados na base norte-americana replicam na vigilância epidemiológica brasileira.

## Fila de Feedback e Validação por Pesquisadores

Pesquisadores e profissionais de farmacovigilância podem capturar e reportar qualquer métrica, linha de tabela ou matriz 2 × 2 diretamente pelo botão contextual de bandeira. Os itens acumulam em uma fila de revisão antes do envio via `POST /api/v1/feedback`.

## Bibliografia Científica e Fontes Oficiais

### Diretrizes Regulatórias Primárias
- **ANVISA (Brasil)**: [RDC nº 406/2020](https://www.gov.br/anvisa/pt-br/assuntos/medicamentos/farmacovigilancia/legislacao) e [RDC nº 967/2025](https://www.gov.br/anvisa/pt-br/assuntos/medicamentos/farmacovigilancia/legislacao) (Boas Práticas de Farmacovigilância e VigiMed).
- **US FDA**: [FDA Guidance for Industry: Good Pharmacovigilance Practices and Pharmacoepidemiologic Assessment (2005)](https://www.fda.gov/media/72257/download).
- **EMA (Europa)**: [Guideline on good pharmacovigilance practices (GVP) – Module IX – Signal Management (Rev 1)](https://www.ema.europa.eu/en/documents/scientific-guideline/guideline-good-pharmacovigilance-practices-gvp-module-ix-signal-management-rev-1_en.pdf).
- **CIOMS**: [Practical Aspects of Signal Detection in Pharmacovigilance (Report of CIOMS Working Group VIII)](https://cioms.ch/publications/product/practical-aspects-of-signal-detection-in-pharmacovigilance-report-of-cioms-working-group-viii/). ISBN: `978-92-9036-082-7`.

### Literatura Seminal
- **Evans, S. J., et al. (2001)**. *Use of proportional reporting ratios (PRRs) for signal generation from spontaneous adverse drug reaction reports*. *Pharmacoepidemiology and Drug Safety*, 10(6), 483–486. DOI: [10.1002/pds.677](https://doi.org/10.1002/pds.677)
- **van Puijenbroek, E. P., et al. (2002)**. *A comparison of statistical methods for signal detection in spontaneous reporting systems*. *Pharmacoepidemiology and Drug Safety*, 11(1), 3–10. DOI: [10.1002/pds.668](https://doi.org/10.1002/pds.668)
- **Rothman, K. J., et al. (2004)**. *The Reporting Odds Ratio and its advantages over the Proportional Reporting Ratio*. *Pharmacoepidemiology and Drug Safety*, 13(8), 519–523. DOI: [10.1002/pds.1001](https://doi.org/10.1002/pds.1001)

### Livros e Tratados (Links sem Afiliados)
- *Mann's Pharmacovigilance* (3ª Edição, Wiley-Blackwell). ISBN: `978-0-470-67104-7`. [Wiley](https://www.wiley.com/en-us/Mann%27s+Pharmacovigilance%2C+3rd+Edition-p-9780470671047) · [Amazon](https://www.amazon.com/dp/0470671048)
- *Pharmacoepidemiology* (6ª Edição, Wiley). ISBN: `978-1-119-41342-4`. [Wiley](https://www.wiley.com/en-us/Pharmacoepidemiology%2C+6th+Edition-p-9781119413424) · [Amazon](https://www.amazon.com/dp/1119413423)

## Transparência e Uso de IA

A arquitetura, pipelines estatísticos e harmonização de bases do **PV Signal Radar** foram desenvolvidos com assistência direta de Inteligência Artificial de:
- **Google AI**: Plataforma Google Antigravity com o modelo **Gemini 3.7 Flash High**.
- **OpenAI**: Modelo **GPT-Sol 5.6 Ultra**.

## Licença

Distribuído sob a [Licença MIT](LICENSE). Desenvolvido por [Bruno Maeda](https://github.com/BMaeda84).
