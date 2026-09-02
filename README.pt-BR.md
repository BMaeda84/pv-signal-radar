# PV Signal Radar 🛰️

**Português (Brasil)** · [Español](README.es.md) · [English](README.md)

[![CI](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml/badge.svg)](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml)
[![Licença: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26.6+-00ADD8?logo=go)](https://go.dev/)

PV Signal Radar é uma plataforma open source para ensino, exploração transparente e pesquisa acadêmica reproduzível sobre desproporcionalidade de notificação em dados públicos de farmacovigilância.

Ela produz **sinais de notificação desproporcional (SDRs) para revisão**. Não demonstra causalidade ou incidência e não é aconselhamento médico, suporte à decisão clínica, sistema GxP validado/qualificado nem mecanismo automático de decisão regulatória.

## Estado atual

- **Modo exploratório ao vivo:** consulta o openFDA FAERS e calcula PRR, ROR, intervalos de confiança, qui-quadrado corrigido de Yates e correção explícita de células zero. Não calcula Fisher exact nem q-values ajustados por multiplicidade. A fonte muda; o resultado não é uma análise congelada citável.
- **Modo de pesquisa congelado:** `research/` contém scaffold de ETL FAERS determinístico, manifesto de fonte/checksum, deduplicação pela versão atual do caso, pares por relato, Parquet canônico mais interchange TSV verificado quanto à integridade, QA e testes com fixtures. `cmd/materialize-sqlite` converte esse interchange no derivado SQLite com checksum servido pelo Go.
- **VigiMed:** indisponível até que a ingestão oficial e versionada das três tabelas ANVISA e a harmonização passem por validação. Não são usados números VigiMed escritos à mão nem “confirmação entre fontes”.
- **Release científico:** o `research/renv.lock` revisado congela R 4.6.1/Bioconductor 3.23 e os pacotes científicos declarados em uma imagem linux/amd64 fixada por digest. O repositório não inclui snapshot FAERS oficial, resultado científico, DOI, benchmark com reference sets ou qualificação em escala real; reprodutibilidade das dependências não equivale a validação científica. Uma publicação formal exige os gates restantes de [`docs/pt-BR/validation-and-evidence.md`](docs/pt-BR/validation-and-evidence.md).

## O que as medidas significam

Para medicamento-alvo `D` e evento `E`, a plataforma monta uma tabela de contagens de relatos:

| | E notificado | Outros eventos | Total |
|---|---:|---:|---:|
| D notificado | a | b | a + b |
| Outros relatos elegíveis | c | d | c + d |
| Total | a + c | b + d | N |

- `PRR = [a/(a+b)] / [c/(c+d)]`
- `ROR = (a×d)/(b×c)`

São medidas de associação de notificação, não riscos em pessoas expostas. A regra guiada `a ≥ 3`, `PRR ≥ 2` e `χ² ≥ 4` é identificada como **heurística didática no estilo Evans**, não regra EMA ou confirmação. O protocolo científico deve pré-especificar dataset, papéis do medicamento, escopo dos eventos, comparator, período, strata, política para zeros, multiplicidade, métodos e perfil de thresholds.

A FDA e o [openFDA](https://open.fda.gov/apis/drug/event/) alertam que relatos espontâneos podem ser duplicados, incompletos, enviesados e conter diversos medicamentos e reações sem vínculo causal individual. O resultado exige revisão dos casos, contexto temporal e clínico, literatura, dados de exposição e outras fontes.

## Executar a aplicação Go

```powershell
git clone https://github.com/BMaeda84/pv-signal-radar.git
Set-Location pv-signal-radar
go run ./cmd/server
```

Abra <http://localhost:8080>. Também há suporte a Docker:

```powershell
docker build --tag pv-signal-radar .
docker run --rm --publish 8080:8080 pv-signal-radar
```

| Variável | Padrão | Finalidade |
|---|---:|---|
| `PORT` | `8080` | Porta HTTP |
| `OPENFDA_API_KEY` | não definida | Secret opcional; nunca faça commit |
| `CACHE_CAPACITY` | `500` | Análises ao vivo concluídas em memória |
| `CACHE_TTL_HOURS` | `24` | TTL do cache de análises ao vivo |
| `RESEARCH_MANIFEST_DIR` | não definida | Diretório contendo somente manifests de serving imediatos e estritos; ausente desativa o modo de pesquisa |
| `RESEARCH_ANALYSIS_DIR` | `data/research-analyses` | Store gravável de resultados imutáveis, usado apenas com o registry habilitado |
| `RESEARCH_SQLITE_PATH` | não definida | Derivado SQLite com checksum; aberto com controles read-only, immutable e query-only do SQLite |
| `RESEARCH_SQLITE_DATASET_ID` | inferido com um manifest | Dataset vinculado ao SQLite; obrigatório quando o registry contém múltiplos datasets |
| `RESEARCH_ALLOW_ONLINE_MATERIALIZATION` | `false` | Feature flag para criar análises ausentes; mantenha falso em publicação pública/read-only sem identidade, quotas e governança de armazenamento no gateway |
| `PV_RADAR_APPLICATION_COMMIT` | revisão VCS limpa embutida | SHA Git minúsculo revisado, usado somente se o build não embutir a revisão; modo de pesquisa rejeita revisão ausente/suja |

## Construir um artefato FAERS congelado

O pipeline nunca baixa dados implicitamente. Obtenha os arquivos trimestrais ASCII na [página oficial da FDA](https://www.fda.gov/drugs/fda-adverse-event-monitoring-system-aems/fda-adverse-event-monitoring-system-aems-latest-quarterly-data-files), crie o source register e verifique os bytes. A landing page agora é intitulada FDA Adverse Event Monitoring System (AEMS) e marcada como “Formerly FAERS”; os arquivos trimestrais nela publicados continuam denominados FAERS. Isso não altera os dataset IDs nem implica migração de dados:

```powershell
Rscript research/R/prepare_source_register.R `
  --input research/input/source-register.draft.csv `
  --output research/input/source-register.verified.csv

Rscript research/R/bootstrap.R --restore

$softwareRevision = git rev-parse HEAD
$env:PV_RADAR_PIPELINE_COMMIT = $softwareRevision
Rscript research/R/build_snapshot.R `
  --source-register research/input/source-register.verified.csv `
  --output research/output/faers-2026q2-v1 `
  --dataset-id faers-2026q2-v1 `
  --build-timestamp 2026-09-02T00:00:00Z `
  --software-version $softwareRevision `
  --renv-lock research/renv.lock `
  --encoding UTF-8 `
  --locale C.UTF-8

go run ./cmd/materialize-sqlite `
  --manifest research/output/faers-2026q2-v1/manifest.json `
  --tsv research/output/faers-2026q2-v1/aggregate_interchange.tsv `
  --output runtime/datasets/faers-2026q2-v1

New-Item -ItemType Directory -Force runtime/manifests, runtime/analyses | Out-Null
Copy-Item runtime/datasets/faers-2026q2-v1/manifest.json `
  runtime/manifests/faers-2026q2-v1.json
```

O build do snapshot falha sem um `renv.lock` revisado, registra locale/encoding/TZ e rejeita linhas relacionais órfãs ou inconsistentes. O materializador recusa sobrescrever a saída, verifica tamanho e SHA-256 do TSV declarado no manifest, valida cada marginal e o contrato canônico do dataset e publica atomicamente um novo diretório de runtime. Mantenha `runtime/manifests/` como registry dedicado contendo somente manifests de serving; não use o próprio diretório do dataset materializado como `RESEARCH_MANIFEST_DIR`, pois ele também preserva `parent-manifest.json` para proveniência. As árvores de exemplo `research/input/`, `research/output/` e `runtime/` são artefatos locais e nunca devem ser commitadas.

Habilite a execução científica local com caminhos explícitos:

```powershell
$env:RESEARCH_MANIFEST_DIR = (Resolve-Path runtime/manifests).Path
$env:RESEARCH_ANALYSIS_DIR = (Resolve-Path runtime/analyses).Path
$env:RESEARCH_SQLITE_PATH = (Resolve-Path runtime/datasets/faers-2026q2-v1/aggregate.sqlite).Path
$env:RESEARCH_SQLITE_DATASET_ID = "faers-2026q2-v1"
$env:RESEARCH_ALLOW_ONLINE_MATERIALIZATION = "true" # somente local/staging após revisar quotas
$env:PV_RADAR_APPLICATION_COMMIT = $softwareRevision
go run ./cmd/server
```

No Docker, monte proveniência e dados como read-only e deixe somente o store de análises gravável:

```powershell
$runtime = (Resolve-Path runtime).Path
docker run --rm --publish 8080:8080 `
  --mount "type=bind,src=$runtime/manifests,dst=/app/research/manifests,readonly" `
  --mount "type=bind,src=$runtime/datasets/faers-2026q2-v1,dst=/app/research/datasets/faers-2026q2-v1,readonly" `
  --mount "type=bind,src=$runtime/analyses,dst=/app/data/research-analyses" `
  --env RESEARCH_MANIFEST_DIR=/app/research/manifests `
  --env RESEARCH_ANALYSIS_DIR=/app/data/research-analyses `
  --env RESEARCH_SQLITE_PATH=/app/research/datasets/faers-2026q2-v1/aggregate.sqlite `
  --env RESEARCH_SQLITE_DATASET_ID=faers-2026q2-v1 `
  --env RESEARCH_ALLOW_ONLINE_MATERIALIZATION=false `
  --env PV_RADAR_APPLICATION_COMMIT=$softwareRevision `
  pv-signal-radar
```

O lock versionado foi gerado duas vezes com bytes idênticos e restaurado na imagem linux/amd64 fixada; o build da imagem repete a baseline de 58 assertions. O uso normal é `bootstrap.R --restore`. `--snapshot` é uma operação controlada de manutenção que recusa sobrescrever o lock revisado. Essa evidência fixa a resolução de pacotes, mas não qualifica um trimestre FAERS real, desempenho dos métodos ou outras arquiteturas. Consulte [`research/README.md`](research/README.md) para contrato de entrada, container, saídas, falhas e licenças.

Limite atual do runtime científico: o engine SQLite em Go oferece PRR, ROR, Fisher exact bilateral e FDR de Benjamini-Hochberg sobre o agregado armazenado. Ele rejeita explicitamente pedidos temporais/de subgrupos não suportados e BCPNN/GPS, que permanecem trabalho batch. O `analysis_id` vincula o manifest completo do dataset, protocolo normalizado, versão e object ID Git completo e limpo da aplicação. O `result_digest`, separado, vincula uma definição versionada da família emitida, o `row_count` e a sequência canônica exata de linhas; a API também retorna o manifesto integral. Isso detecta truncamento, reordenação ou alteração stale, mas não prova completude upstream nem da família cientificamente esperada. O export repete esse limite em JSON/CSV, headers, metadados de citação/ambiente com escopo, checksums e script de reprodução. Não estão incluídos snapshot FAERS oficial, benchmark com reference sets positivos/negativos nem qualificação em escala real.

## Limite da API

| Rota | Finalidade | Reprodutibilidade |
|---|---|---|
| `GET /api/v1/analyze?drug=...` | Exploração openFDA ao vivo deprecated | Não citável; upstream muda |
| `GET /api/v1/health` | Saúde do processo | Não valida cientificamente o serviço |
| `GET /api/v2/datasets` | Datasets imutáveis registrados e proveniência | Depende de artefato verificado instalado |
| `POST /api/v2/analyses` | Configuração determinística | `analysis_id` vincula inputs; `result_digest` vincula linhas emitidas |
| `GET /api/v2/analyses/{id}` | Resultado, manifesto integral, métodos e limitações | Reproduzível dentro do limite de integridade registrado |
| `GET /api/v2/analyses/{id}/export` | Bundle científico | Manifesto integral, integridade do resultado, metadados de citação com escopo, checksums e script de reprodução |

Nenhum dataset é substituído silenciosamente. Snapshot ou método indisponível faz a API científica falhar de forma explícita.

## Documentação científica e publicação

- [Uso pretendido e responsabilidades](docs/pt-BR/intended-use.md)
- [Contrato de métodos e protocolo](docs/pt-BR/methods-and-protocol.md)
- [Dicionário de dados](docs/pt-BR/data-dictionary.md)
- [Vieses e modos de falha](docs/pt-BR/biases-and-failure-modes.md)
- [Validação e matriz de evidências](docs/pt-BR/validation-and-evidence.md)
- [Privacidade, segurança, licenças e governança](docs/pt-BR/privacy-security-and-governance.md)
- [Seleção de pacotes científicos e repositórios](docs/pt-BR/ecosystem-and-method-selection.md)
- [Checklist de publicação e DOI](docs/pt-BR/publication-checklist.md)

Relate estudos formais segundo [READUS-PV](https://pmc.ncbi.nlm.nih.gov/articles/PMC11116242/) e arquive ID do dataset, hashes de fonte/saída, configuração/ID da análise, commit, package lock, desvios e bundle exportado. READUS-PV melhora transparência; não valida causalidade nem adequação regulatória.

## Verificação

```powershell
go test -race ./...
go vet ./...
go build ./cmd/server
Rscript research/tests/run_tests.R
docker build --tag pv-signal-radar:local .
```

As fixtures R são sintéticas e usam somente base R. Elas não validam trimestre oficial, escala real, serialização Parquet, BCPNN/GPS, mapeamento MedDRA, VigiMed, publicação S3 ou interpretação clínica.

## Citação e licenças

Use [`CITATION.cff`](CITATION.cff) para o software e cite separadamente o bundle imutável de dataset/análise. O `CITATION.cff` gerado no bundle é explicitamente uma citação de software; `citation-metadata.json` registra declarações da fonte/vocabulários e deixa a licença da análise não afirmada. O código da aplicação é MIT, mas essa licença não é aplicada aos dados-fonte, artefatos derivados, vocabulários ou resultado de análise. Ambiente R, dados e vocabulários mantêm seus próprios termos; pacotes científicos GPL não são linked ao binário Go. Essa separação técnica não é parecer jurídico—revise direitos de pacotes, dados, terminologias e distribuição do resultado antes de redistribuir.
