# Dicionário de dados dos artefatos de pesquisa

## Dataset Parquet `report_pairs/`

Uma linha representa uma combinação única de notificação na versão atual–texto do medicamento–papel–PT do evento. `quarter` é uma partição Parquet derivada da linha DEMO retida.

| Campo | Significado | Restrição ou ressalva |
|---|---|---|
| `primaryid` | Identificador da versão da notificação FAERS | Armazenado como texto para evitar perda de precisão numérica |
| `caseid` | Identificador longitudinal do caso FAERS | A maior versão numérica do caso é retida |
| `quarter` | Trimestre de origem da linha DEMO retida | `YYYYQ1` a `YYYYQ4` |
| `drug_text` | `PROD_AI` normalizado; caso ausente, `DRUGNAME` | Texto de origem, não um conceito canônico |
| `drug_text_source` | Campo usado em `drug_text` | `PROD_AI` ou `DRUGNAME` |
| `drug_role` | `ROLE_COD` da FDA mapeado como `PS` → `primary_suspect`, `SS` → `secondary_suspect`, `C` → `concomitant`, `I` → `interacting`; `suspect` e `all` são uniões deduplicadas | Códigos de origem desconhecidos falham de modo fechado; nenhum papel clínico é inferido |
| `event_pt` | Texto PT de origem normalizado | A versão do MedDRA não é inferida |
| `event_category` | Estado da classificação | Atualmente `UNCLASSIFIED_SOURCE_PT` |
| `event_dt`, `fda_dt` | Data de evento/recebimento da fonte | Codificação da fonte preservada; sem imputação silenciosa |
| `age`, `age_cod`, `sex` | Dados demográficos da fonte | Devem ser interpretados em conjunto; não normalizados na v1 |
| `occr_country`, `reporter_country` | Campos de país da fonte | Valores ausentes ou divergentes são possíveis |
| `serious*` | Flags de gravidade da fonte | Nenhuma composição é inferida além dos campos fornecidos |

## `aggregate_interchange.tsv`

| Campo | Significado |
|---|---|
| `dataset_id` | Identificador lógico imutável do snapshot |
| `drug_text`, `drug_text_source`, `drug_role`, `event_pt` | Chave exata de agrupamento |
| `a` | Notificações que contêm o grupo de medicamento alvo e o evento |
| `b` | Notificações que contêm o grupo de medicamento alvo sem o evento |
| `c` | Notificações que contêm o evento sem o grupo de medicamento alvo |
| `d` | Notificações que não contêm nenhum dos dois |
| `drug_reports` | `a + b` |
| `event_reports` | `a + c` |
| `universe_reports` | `a + b + c + d`; notificações elegíveis na versão atual com medicamento e PT |
| `comparator` | `all_other_eligible_reports`: notificações no universo elegível do snapshot que não pertencem ao grupo selecionado de medicamento/papel |
| `event_scope` | `all_recorded_source_pts`: todo PT de origem retido; nenhuma classificação clínica/de erro/de qualidade é inferida |
| `deduplication_policy` | Identificador estável da política usada pelo ETL |

O TSV é um artefato de intercâmbio, não uma publicação completa. Seu manifesto de fontes, resumo de QA, manifesto de dados, arquivo de checksums, lockfile, revisão do software e configuração da análise são registros de proveniência inseparáveis.

## Manifesto do dataset

`manifest.json` segue o contrato estrito `pv-signal-radar.research/v1` da API Go. Ele registra identidade/título do dataset, arquivos de origem e hashes, cobertura, políticas/commit de origem do processamento, tratamento de vocabulário, hashes/contagens de bytes dos artefatos, completude e limitações. `metadata/environment.json` registra o timestamp fixo de build, runtime R, commit de origem e versões dos pacotes; seu hash é vinculado ao manifesto do dataset. Ele fica aninhado para que uma varredura do registry de manifestos não o decodifique como manifesto de dataset. `source_manifest.csv` registra o basename original de cada arquivo local, a URL oficial da fonte, o timestamp de obtenção por arquivo, cobertura e SHA-256. Caminhos absolutos da estação de trabalho são deliberadamente excluídos.

O objeto de completude mantém quatro populações diferentes explícitas:

| Campo | População contabilizada |
|---|---|
| `source_demo_rows` | Linhas DEMO brutas, inclusive versões de caso posteriormente superseded |
| `current_case_reports` | Versões correntes retidas após a seleção de um `PRIMARYID` por `CASEID` |
| `eligible_reports` | Notificações correntes com ao menos um par medicamento–papel–evento retido; este é o denominador da análise agregada |
| `drug_event_pairs` | Pares únicos notificação–texto do medicamento–papel de origem–PT do evento antes das uniões derivadas `suspect` e `all` |

`qa_summary.csv` usa os mesmos nomes. `superseded_demo_rows` é a diferença entre linhas DEMO de origem e notificações correntes; não é uma contagem de casos distintos.

## Dados ausentes

Identificadores, papéis, texto do medicamento e PT são obrigatórios para um par elegível. Campos de origem opcionais permanecem ausentes; o ETL não os imputa. Cada entrada de completude de campo declara `population: eligible_reports` e `denominator_records`; versões superseded e notificações correntes sem par retido não entram nesse percentual. Toda análise estratificada deve relatar suas próprias contagens incluídas/ausentes, pois a seleção de casos completos muda o comparator e pode alterar a desproporcionalidade.
