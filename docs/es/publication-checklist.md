# Checklist de publicación, citación y DOI

## Release de software

1. Reconcilie el commit implementado con `docs/validation-and-evidence.md`; no etiquete una release como “validada” mientras no se haya ejecutado una fila crítica.
2. Exija CI de branch protegida, revisión científica independiente de métodos/afirmaciones/riesgos residuales y una aprobación registrada en el entorno GitHub `software-release`.
3. Cree una tag de versión semántica anotada e inmutable después de la revisión de evidencias. Configure rulesets para la default branch y las tags de release, checks/reviews obligatorios, releases inmutables y reviewers obligatorios en el entorno `software-release` antes de habilitar la publicación.
4. Ejecute manualmente `.github/workflows/release.yml` con esa tag existente. Su job sin privilegios verifica la tag/commit, prueba Go y la imagen R bloqueada, escanea ambas imágenes de runtime, construye binarios y emite checksums, SBOMs CycloneDX de Go, el lock de R y un inventario de paquetes/licencias R. Solo el job separado protegido por el entorno recibe permisos de release/OIDC, atesta los bytes verificados y crea la release de GitHub. Antes de ejecutarlo, cree el entorno protegido `software-release`, configure required reviewers y añada el secret con alcance exclusivo del entorno `SOFTWARE_RELEASE_GATE` con el valor contractual exacto y no secreto `enabled-after-protection-v1`. No defina este nombre como secret de repositorio u organización: el control es el alcance, no la confidencialidad. El primer step de publicación falla de forma cerrada cuando el valor falta o es diferente. Este gate en el código fuente no demuestra ni sustituye required reviewers, rulesets de branch/tag ni releases inmutables. Trate como release bloqueada la ausencia de un control externo del repositorio o un gate fallido/omitido.
5. Si se va a distribuir la imagen científica R, añada un gate de release OCI independiente: publíquela por digest inmutable, genere un SBOM OCI que cubra OS y paquetes R, conserve el informe de vulnerabilidades y firme/ateste la procedencia de la imagen. El workflow actual solo construye/escanea la imagen R y publica su Dockerfile, lock e inventario de paquetes; no publica ni atesta la propia imagen.
6. Vincule los bytes restaurados de dependencias, no solo nombres y versiones. Registre valores SHA-256 de los archives exactos de paquetes u objetos del mirror utilizados por el restore; el checksum actual de `renv.lock` protege el lock en su conjunto, pero sus registros de paquetes no contienen hashes de contenido por archive.
7. Archive el código fuente de la release, `.zenodo.json` y `CITATION.cff`. Habilite la integración GitHub–Zenodo únicamente mediante la cuenta del propietario del repositorio y, después, reserve/publique un DOI para la release revisada.
8. Añada el DOI final a los metadatos de la release en un commit posterior. Nunca presente en una citación pública un DOI de borrador reservado como si estuviera publicado.

Habilitar rulesets de branch/tag, releases inmutables, el entorno protegido `software-release`, Zenodo o un DOI son acciones externas de publicación y no son realizadas por este cambio de código fuente. La auditoría del 2026-09-02 encontró ausencia de rulesets, default branch desprotegida, ausencia de entornos GitHub y releases inmutables deshabilitadas. El código fuente no puede demostrar que esas configuraciones permanezcan activas después de configurarlas; vuelva a comprobarlas inmediatamente antes de cada release.

## Release de dataset y análisis

El DOI del software no identifica datos ni parámetros de análisis. Publique un registro inmutable separado que contenga o enlace:

- ID del dataset y del análisis;
- URLs oficiales de fuentes, cobertura, timestamps de obtención y valores SHA-256;
- `manifest.json`, `metadata/environment.json`, `source_manifest.csv`, `qa_summary.csv` y `checksums.sha256` de salida;
- commit exacto del código fuente y `renv.lock` revisado;
- configuración del análisis y definiciones de métodos legibles por máquina;
- digest canónico del resultado, recuento/orden de filas, definición de la familia de hipótesis y evidencia de reproducción atestada de forma independiente;
- resultado CSV/Parquet y un informe de métodos/limitaciones legible por personas;
- evidencia de benchmark numérico/reference set para perfiles de threshold habilitados;
- desviaciones, aprobador, disposición de licencia/redistribución y ruta de contacto/corrección.

Si los términos de los datos de origen prohíben la redistribución, publique instrucciones de reconstrucción, hashes de origen, código, metadatos y resultados derivados permitidos en lugar de los bytes restringidos.

## Mínimo para el manuscrito

Identifique versión/cobertura de la base de datos, tratamiento de versión del caso, elegibilidad de notificaciones, mapeo y papel del medicamento, alcance de PT/acontecimientos, comparator, estratos, medidas, intervalos de confianza, corrección de datos dispersos, procedimiento de multiplicidad, threshold, versiones de software/paquetes y datos ausentes. Informe estimaciones de efecto e incertidumbre, no solo etiquetas de threshold. Cite por separado el DOI del software y el DOI del dataset/análisis y siga [READUS-PV](https://pmc.ncbi.nlm.nih.gov/articles/PMC11116242/).

## Corrección o retirada

Nunca sustituya en el mismo lugar un artefacto archivado. Publique una corrección vinculada o tombstone con motivo, impacto, identificador de reemplazo, análisis afectados y fecha. Conserve el checksum original y el registro de evidencias para que los lectores puedan identificar qué cambió.
