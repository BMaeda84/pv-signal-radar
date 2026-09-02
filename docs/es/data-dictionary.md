# Diccionario de datos de los artefactos de investigación

## Dataset Parquet `report_pairs/`

Una fila representa una combinación única de notificación en su versión actual–texto del medicamento–papel–PT del acontecimiento. `quarter` es una partición Parquet derivada de la fila DEMO retenida.

| Campo | Significado | Restricción o salvedad |
|---|---|---|
| `primaryid` | Identificador de versión de la notificación FAERS | Se almacena como texto para evitar pérdida de precisión numérica |
| `caseid` | Identificador longitudinal del caso FAERS | Se retiene la máxima versión numérica del caso |
| `quarter` | Trimestre de origen de la fila DEMO retenida | `YYYYQ1` a `YYYYQ4` |
| `drug_text` | `PROD_AI` normalizado; en su defecto, `DRUGNAME` | Texto de origen, no un concepto canónico |
| `drug_text_source` | Campo utilizado para `drug_text` | `PROD_AI` o `DRUGNAME` |
| `drug_role` | `ROLE_COD` de la FDA mapeado como `PS` → `primary_suspect`, `SS` → `secondary_suspect`, `C` → `concomitant`, `I` → `interacting`; `suspect` y `all` son uniones deduplicadas | Los códigos de origen desconocidos fallan de forma cerrada; no se infiere ningún papel clínico |
| `event_pt` | Texto PT de origen normalizado | No se infiere la versión de MedDRA |
| `event_category` | Estado de clasificación | Actualmente `UNCLASSIFIED_SOURCE_PT` |
| `event_dt`, `fda_dt` | Fecha de acontecimiento/recepción de la fuente | Se conserva la codificación de origen; no se imputa silenciosamente |
| `age`, `age_cod`, `sex` | Datos demográficos de origen | Deben interpretarse conjuntamente; no se normalizan en v1 |
| `occr_country`, `reporter_country` | Campos de país de origen | Son posibles valores ausentes o distintos |
| `serious*` | Flags de gravedad de origen | No se infiere ninguna composición más allá de los campos suministrados |

## `aggregate_interchange.tsv`

| Campo | Significado |
|---|---|
| `dataset_id` | Identificador lógico inmutable del snapshot |
| `drug_text`, `drug_text_source`, `drug_role`, `event_pt` | Clave exacta de agrupación |
| `a` | Notificaciones que contienen el grupo de medicamento objetivo y el acontecimiento |
| `b` | Notificaciones que contienen el grupo de medicamento objetivo sin el acontecimiento |
| `c` | Notificaciones que contienen el acontecimiento sin el grupo de medicamento objetivo |
| `d` | Notificaciones que no contienen ninguno de los dos |
| `drug_reports` | `a + b` |
| `event_reports` | `a + c` |
| `universe_reports` | `a + b + c + d`; notificaciones elegibles en su versión actual con medicamento y PT |
| `comparator` | `all_other_eligible_reports`: notificaciones del universo elegible del snapshot que no pertenecen al grupo seleccionado de medicamento/papel |
| `event_scope` | `all_recorded_source_pts`: cada PT de origen retenido; no se infiere una clasificación clínica/de error/de calidad |
| `deduplication_policy` | Identificador estable de la política utilizada por el ETL |

El TSV es un artefacto de intercambio, no una publicación completa. Su manifiesto de fuentes, resumen de QA, manifiesto de datos, archivo de checksums, lockfile, revisión del software y configuración del análisis son registros de procedencia inseparables.

## Manifiesto del dataset

`manifest.json` sigue el contrato estricto `pv-signal-radar.research/v1` de la API Go. Registra identidad/título del dataset, archivos de origen y hashes, cobertura, políticas/commit de origen del procesamiento, tratamiento del vocabulario, hashes/recuentos de bytes de artefactos, completitud y limitaciones. `metadata/environment.json` registra el timestamp fijo de build, runtime R, commit de origen y versiones de paquetes; su hash queda vinculado al manifiesto del dataset. Está anidado para que un escaneo del registry de manifiestos no lo decodifique como manifiesto de dataset. `source_manifest.csv` registra el basename original de cada archivo local, la URL oficial de origen, el timestamp de obtención por archivo, la cobertura y el SHA-256. Se excluyen deliberadamente las rutas absolutas de la estación de trabajo.

El objeto de completitud mantiene explícitas cuatro poblaciones diferentes:

| Campo | Población contabilizada |
|---|---|
| `source_demo_rows` | Filas DEMO brutas, incluidas las versiones de caso posteriormente sustituidas |
| `current_case_reports` | Versiones actuales retenidas tras seleccionar un `PRIMARYID` por `CASEID` |
| `eligible_reports` | Notificaciones actuales con al menos un par medicamento–papel–acontecimiento retenido; este es el denominador del análisis agregado |
| `drug_event_pairs` | Pares únicos notificación–texto del medicamento–papel de origen–PT del acontecimiento antes de las uniones derivadas `suspect` y `all` |

`qa_summary.csv` utiliza los mismos nombres. `superseded_demo_rows` es la diferencia entre las filas DEMO de origen y las notificaciones actuales; no es un recuento de casos distintos.

## Datos ausentes

Los identificadores, papeles, texto del medicamento y PT son obligatorios para un par elegible. Los campos de origen opcionales permanecen ausentes; el ETL no los imputa. Cada entrada de completitud de campo declara `population: eligible_reports` y `denominator_records`; las versiones sustituidas y las notificaciones actuales sin un par retenido no entran en ese porcentaje. Todo análisis estratificado debe informar sus propios recuentos incluidos/ausentes porque la selección de casos completos cambia el comparator y puede cambiar la desproporcionalidad.
