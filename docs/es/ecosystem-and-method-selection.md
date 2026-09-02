# Registro de selección de software científico y repositorios

Última revisión: 2026-09-02. Este es un registro de decisión sobre dependencias, no un respaldo ni un certificado de validación. Versión, licencia, mantenimiento, concordancia numérica, modelo de datos e instalación reproducible son gates independientes; un paquete que implementa un método con nombre no es automáticamente intercambiable con otra implementación.

## Adoptados en el entorno batch R aislado

| Componente | Versión congelada | Papel | Criterio de selección | Límite |
|---|---:|---|---|---|
| [`faers`](https://bioconductor.org/packages/release/bioc/html/faers.html) | 1.8.0 | ETL orientado a FAERS y referencia independiente de métodos | Publicado en Bioconductor 3.23, repositorio de código documentado, licencia MIT, contrato de release R 4.6 | El proyecto mantiene su propio ETL relacional explícito; la presencia del paquete no valida un trimestre ni sustituye la reconciliación en el nivel de la fuente |
| [`pvda`](https://CRAN.R-project.org/package=pvda) | 0.0.4 | Comparación independiente de PRR, ROR e IC | Paquete publicado en CRAN con manual/pruebas de referencia y licencia declarada GPL >= 3 | Se utiliza solo en el proceso R separado; se requieren adapters numéricos y tolerancias declaradas antes de citar concordancia |
| [`openEBGM`](https://CRAN.R-project.org/package=openEBGM) | 0.9.1 | GPS/EBGM, cuantiles y trabajo batch estratificado | Implementación publicada en CRAN con vignettes de métodos y licencias GPL-2/GPL-3 | La estimación de hiperparámetros, estratos, convergencia y EB05 deben congelarse en el protocolo; nunca se aproxima en Go |

El entorno transitivo se registra en `research/renv.lock`, no se resuelve a partir de “latest” durante el análisis. Los paquetes de la familia GPL no se enlazan con el binario Go MIT; las obligaciones de distribución de la imagen batch aún requieren una revisión de licencias en el momento de la release.

## Benchmarks exploratorios, no dependencias centrales

| Candidato | Evidencia considerada | Por qué no es central |
|---|---|---|
| [`vigipy`](https://github.com/Shakesbeery/vigipy) | Implementaciones Python de BCPNN, GPS, PRR, ROR, Fisher, LASSO y análisis longitudinal; GPLv3 | Útil como benchmark entre lenguajes, pero su propio repositorio enumera como pendientes la documentación de métodos y un dataset de demostración. No se utiliza para definir resultados canónicos. |
| [`faers` en PyPI](https://pypi.org/project/faers/) | Versión 0.1, una release de código fuente de 1,4 kB subida en 2015 | Colisión de nombre con el proyecto activo de Bioconductor; antigüedad, alcance y contenido de la release no cumplen los requisitos del pipeline. |
| [`hypokrates`](https://pypi.org/project/hypokrates/) | Paquete amplio de generación de hipótesis en múltiples fuentes, estado de desarrollo “Alpha” en PyPI, solo AGPL-3.0 | Límite de confianza y producto diferente: generación de hipótesis entre bases/LLM, no el contrato de análisis relacional FAERS congelado. Solo puede informar investigación de interoperabilidad después de una revisión de linaje de datos, numérica, de licencias y de seguridad. |
| [`VigiLens`](https://github.com/firassa-ai/VigiLens) | Aplicación de señales de seguridad temporales consciente de los trimestres | Comparación de producto/repositorio para UX temporal; no es autoridad numérica ni dependencia de fuente. |
| [`PRISM-Pharmacovigilance`](https://github.com/Jehathsyed/PRISM-Pharmacovigilance) | Aplicación browser/openFDA de desproporcionalidad | Comparador útil de UI, pero un workflow openFDA en vivo no puede sustituir entradas trimestrales congeladas y deduplicadas por versión de caso. |

## Prueba de admisión para otro paquete o repositorio

Un candidato pasa de benchmark a dependencia solo después de registrar todo lo siguiente:

1. versión/hash de origen inmutable, runtime compatible, licencia, historial de mantenedor/releases e instalación limpia reproducible;
2. unidad exacta de entrada y semántica del comparator, tratamiento de duplicados/versión de caso, política de celdas dispersas, definición de intervalo, comportamiento de estratos y modos de fallo;
3. fixtures golden 2 x 2 y comparación numérica a escala real contra al menos una implementación independiente, con tolerancias absolutas/relativas declaradas;
4. comportamiento en reference sets positivos/negativos cuando el método influya en un threshold, incluidos sensibilidad, PPV, comportamiento de falsos positivos y time-to-detection;
5. límites de memoria/CPU, comportamiento con entradas malformadas, revisión de SBOM/vulnerabilidades y salida/procedencia deterministas; y
6. una razón documentada de por qué la capacidad no puede implementarse de forma más transparente en el límite actual.

Hasta que exista ese expediente, los resultados exploratorios deben etiquetarse por implementación y versión y no pueden sustituir silenciosamente un método configurado.
