# PV Signal Radar 🛰️

[Português (Brasil)](README.pt-BR.md) · **Español** · [English](README.md)

[![CI](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml/badge.svg)](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml)
[![Licencia: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26.6+-00ADD8?logo=go)](https://go.dev/)

PV Signal Radar es una plataforma open source para enseñanza, exploración transparente e investigación académica reproducible sobre desproporcionalidad de notificación en datos públicos de farmacovigilancia.

Produce **señales de notificación desproporcionada (SDR) para revisión**. No demuestra causalidad ni incidencia y no es consejo médico, soporte a decisiones clínicas, sistema GxP validado/cualificado ni motor automático de decisiones regulatorias.

## Estado actual

- **Modo exploratorio en vivo:** consulta openFDA FAERS y calcula PRR, ROR, intervalos de confianza, chi-cuadrado corregido de Yates y corrección explícita de celdas cero. No calcula Fisher exact ni q-values ajustados por multiplicidad. La fuente cambia; el resultado no es un análisis congelado citable.
- **Modo de investigación congelado:** `research/` contiene un scaffold de ETL FAERS determinista, manifiesto de fuente/checksum, deduplicación por versión actual del caso, pares por informe, Parquet canónico más intercambio TSV con integridad verificada, QA y pruebas con fixtures. `cmd/materialize-sqlite` convierte ese intercambio en el derivado SQLite con checksum servido por Go.
- **VigiMed:** no está disponible hasta que la ingestión oficial y versionada de las tres tablas ANVISA y su armonización superen validación. No se usan cifras VigiMed escritas manualmente ni “confirmación entre fuentes”.
- **Release científico:** el `research/renv.lock` revisado congela R 4.6.1/Bioconductor 3.23 y los paquetes científicos declarados en una imagen linux/amd64 fijada por digest. El repositorio no incluye snapshot FAERS oficial, resultado científico, DOI, benchmark con reference sets ni cualificación a escala real; la reproducibilidad de dependencias no equivale a validación científica. Una publicación formal exige los gates restantes de [`docs/es/validation-and-evidence.md`](docs/es/validation-and-evidence.md).

## Qué significan las medidas

Para un medicamento objetivo `D` y evento `E`, la plataforma construye una tabla de recuentos de informes:

| | E notificado | Otros eventos | Total |
|---|---:|---:|---:|
| D notificado | a | b | a + b |
| Otros informes elegibles | c | d | c + d |
| Total | a + c | b + d | N |

- `PRR = [a/(a+b)] / [c/(c+d)]`
- `ROR = (a×d)/(b×c)`

Son medidas de asociación de notificación, no riesgos en personas expuestas. La regla guiada `a ≥ 3`, `PRR ≥ 2` y `χ² ≥ 4` se identifica como **heurística educativa de estilo Evans**, no regla EMA ni confirmación. El protocolo científico debe preespecificar dataset, roles del medicamento, alcance de eventos, comparador, periodo, estratos, política para ceros, multiplicidad, métodos y perfil de umbrales.

La FDA y [openFDA](https://open.fda.gov/apis/drug/event/) advierten que los informes espontáneos pueden estar duplicados, incompletos o sesgados y contener varios medicamentos y reacciones sin vínculo causal individual. El resultado requiere revisión de casos, contexto temporal y clínico, literatura, datos de exposición y otras fuentes.

## Ejecutar la aplicación Go

```powershell
git clone https://github.com/BMaeda84/pv-signal-radar.git
Set-Location pv-signal-radar
go run ./cmd/server
```

Abra <http://localhost:8080>. También se admite Docker:

```powershell
docker build --tag pv-signal-radar .
docker run --rm --publish 8080:8080 pv-signal-radar
```

| Variable | Predeterminado | Finalidad |
|---|---:|---|
| `PORT` | `8080` | Puerto HTTP |
| `OPENFDA_API_KEY` | sin definir | Secret opcional; nunca lo incluya en Git |
| `CACHE_CAPACITY` | `500` | Análisis en vivo completados en memoria |
| `CACHE_TTL_HOURS` | `24` | TTL de la caché de análisis en vivo |
| `RESEARCH_MANIFEST_DIR` | sin definir | Directorio que contiene solo manifiestos de serving inmediatos y estrictos; ausente desactiva el modo de investigación |
| `RESEARCH_ANALYSIS_DIR` | `data/research-analyses` | Almacén escribible de resultados inmutables, usado solo con el registro habilitado |
| `RESEARCH_SQLITE_PATH` | sin definir | Derivado SQLite con checksum; abierto con controles read-only, immutable y query-only de SQLite |
| `RESEARCH_SQLITE_DATASET_ID` | inferido con un manifiesto | Dataset vinculado al SQLite; obligatorio cuando el registro contiene múltiples datasets |
| `RESEARCH_ALLOW_ONLINE_MATERIALIZATION` | `false` | Feature flag para crear análisis ausentes; manténgalo falso en despliegues públicos/read-only sin identidad, cuotas y gobernanza de almacenamiento en el gateway |
| `PV_RADAR_APPLICATION_COMMIT` | revisión VCS limpia embebida | SHA Git minúsculo revisado, usado solo si el build no embebe su revisión; el modo de investigación rechaza una revisión ausente/sucia |

## Construir un artefacto FAERS congelado

El pipeline nunca descarga datos implícitamente. Obtenga los archivos trimestrales ASCII en la [página oficial de FDA](https://www.fda.gov/drugs/fda-adverse-event-monitoring-system-aems/fda-adverse-event-monitoring-system-aems-latest-quarterly-data-files), cree el registro de fuentes y verifique los bytes. La landing page ahora se titula FDA Adverse Event Monitoring System (AEMS) y está marcada como “Formerly FAERS”; los archivos trimestrales que publica siguen denominándose FAERS. Esto no cambia los dataset IDs ni implica una migración de datos:

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

El build del snapshot falla sin un `renv.lock` revisado, registra locale/encoding/TZ y rechaza filas relacionales huérfanas o inconsistentes. El materializador rechaza sobrescribir su salida, verifica tamaño y SHA-256 del TSV declarado en el manifiesto, valida cada marginal y el contrato canónico del dataset y publica atómicamente un nuevo directorio de runtime. Mantenga `runtime/manifests/` como registro dedicado que contenga solo manifiestos de serving; no use el propio directorio del dataset materializado como `RESEARCH_MANIFEST_DIR`, porque también preserva `parent-manifest.json` para procedencia. Los árboles de ejemplo `research/input/`, `research/output/` y `runtime/` son artefactos locales y nunca deben incluirse en un commit.

Habilite la ejecución científica local con rutas explícitas:

```powershell
$env:RESEARCH_MANIFEST_DIR = (Resolve-Path runtime/manifests).Path
$env:RESEARCH_ANALYSIS_DIR = (Resolve-Path runtime/analyses).Path
$env:RESEARCH_SQLITE_PATH = (Resolve-Path runtime/datasets/faers-2026q2-v1/aggregate.sqlite).Path
$env:RESEARCH_SQLITE_DATASET_ID = "faers-2026q2-v1"
$env:RESEARCH_ALLOW_ONLINE_MATERIALIZATION = "true" # solo local/staging tras revisar las cuotas
$env:PV_RADAR_APPLICATION_COMMIT = $softwareRevision
go run ./cmd/server
```

En Docker, monte procedencia y datos como read-only y deje solo el almacén de análisis escribible:

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

El lock versionado fue generado dos veces con bytes idénticos y restaurado en la imagen linux/amd64 fijada; el build de la imagen repite la baseline de 58 assertions. El uso normal es `bootstrap.R --restore`. `--snapshot` es una operación controlada de mantenimiento que se niega a sobrescribir el lock revisado. Esta evidencia fija la resolución de paquetes, pero no cualifica un trimestre FAERS real, el rendimiento de los métodos ni otras arquitecturas. Consulte [`research/README.md`](research/README.md) para el contrato de entrada, el contenedor, las salidas, los fallos y las licencias.

Límite actual del runtime científico: el motor SQLite en Go ofrece PRR, ROR, Fisher exacto bilateral y FDR de Benjamini-Hochberg sobre el agregado almacenado. Rechaza explícitamente solicitudes temporales/de subgrupos no soportadas y BCPNN/GPS, que siguen siendo trabajo batch. El `analysis_id` vincula el manifiesto completo del dataset, el protocolo normalizado, la versión y el object ID Git completo y limpio de la aplicación. El `result_digest`, separado, vincula una definición versionada de la familia emitida, el `row_count` y la secuencia canónica exacta de filas; la API también devuelve el manifiesto integral. Esto detecta truncamiento, reordenación o alteración stale, pero no demuestra completitud upstream ni de la familia científicamente esperada. El export repite este límite en JSON/CSV, headers, metadatos de cita/entorno con alcance, checksums y script de reproducción. No se incluyen snapshot FAERS oficial, benchmark con reference sets positivos/negativos ni cualificación a escala real.

## Límite de la API

| Ruta | Finalidad | Reproducibilidad |
|---|---|---|
| `GET /api/v1/analyze?drug=...` | Exploración openFDA en vivo deprecated | No citable; upstream cambia |
| `GET /api/v1/health` | Salud del proceso | No valida científicamente el servicio |
| `GET /api/v2/datasets` | Datasets inmutables registrados y procedencia | Depende de un artefacto verificado instalado |
| `POST /api/v2/analyses` | Configuración determinista | `analysis_id` vincula inputs; `result_digest` vincula filas emitidas |
| `GET /api/v2/analyses/{id}` | Resultado, manifiesto integral, métodos y limitaciones | Reproducible dentro del límite de integridad registrado |
| `GET /api/v2/analyses/{id}/export` | Bundle científico | Manifiesto integral, integridad del resultado, metadatos de cita con alcance, checksums y script de reproducción |

No se sustituye silenciosamente ningún dataset. Un snapshot o método no disponible hace que la API científica falle de forma explícita.

## Documentación científica y publicación

- [Uso previsto y responsabilidades](docs/es/intended-use.md)
- [Contrato de métodos y protocolo](docs/es/methods-and-protocol.md)
- [Diccionario de datos](docs/es/data-dictionary.md)
- [Sesgos y modos de fallo](docs/es/biases-and-failure-modes.md)
- [Validación y matriz de evidencias](docs/es/validation-and-evidence.md)
- [Privacidad, seguridad, licencias y gobernanza](docs/es/privacy-security-and-governance.md)
- [Selección de paquetes científicos y repositorios](docs/es/ecosystem-and-method-selection.md)
- [Checklist de publicación y DOI](docs/es/publication-checklist.md)

Informe estudios formales según [READUS-PV](https://pmc.ncbi.nlm.nih.gov/articles/PMC11116242/) y archive ID del dataset, hashes de fuente/salida, configuración/ID del análisis, commit, package lock, desviaciones y bundle exportado. READUS-PV mejora la transparencia; no valida causalidad ni aptitud regulatoria.

## Verificación

```powershell
go test -race ./...
go vet ./...
go build ./cmd/server
Rscript research/tests/run_tests.R
docker build --tag pv-signal-radar:local .
```

Las fixtures R son sintéticas y usan solo base R. No validan trimestre oficial, escala real, serialización Parquet, BCPNN/GPS, mapeo MedDRA, VigiMed, publicación S3 ni interpretación clínica.

## Cita y licencias

Use [`CITATION.cff`](CITATION.cff) para el software y cite por separado el bundle inmutable de dataset/análisis. El `CITATION.cff` generado en el bundle es explícitamente una cita de software; `citation-metadata.json` registra declaraciones de la fuente/vocabularios y deja la licencia del análisis sin afirmar. El código de la aplicación es MIT, pero esa licencia no se aplica a datos fuente, artefactos derivados, vocabularios ni al resultado del análisis. Entorno R, datos y vocabularios conservan sus propios términos; los paquetes científicos GPL no se enlazan con el binario Go. Esta separación técnica no es asesoramiento legal: revise derechos de paquetes, datos, terminologías y distribución del resultado antes de redistribuir.
