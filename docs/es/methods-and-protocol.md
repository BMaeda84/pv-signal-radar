# Contrato de métodos y protocolo

## Unidad de análisis

Para datasets de investigación FAERS congelados, el ETL selecciona el máximo `CASEVERSION` numérico por `CASEID`; un desempate mediante el máximo `PRIMARYID` numérico hace que los duplicados ambiguos sean deterministas y visibles en QA. Una notificación elegible debe tener al menos una fila de medicamento retenida y un Preferred Term de origen no vacío. Cada `PRIMARYID × drug text × drug role × PT` aparece una sola vez.

Se prefiere `PROD_AI` como texto de origen del medicamento y `DRUGNAME` es el fallback. Se eliminan espacios en los extremos, se normaliza el whitespace y el texto se convierte a mayúsculas. Esto no constituye un mapeo a RxNorm, UNII, ATC, DCB, WHODrug ni a un concepto de ingrediente. El texto del PT se conserva de la fuente trimestral; el pipeline no infiere una versión de MedDRA.

## Preespecificación obligatoria

Antes del análisis, congele:

1. dataset y período de cobertura;
2. versión del concepto/mapeo del medicamento y nombres de origen incluidos;
3. papeles del medicamento, como sospechoso primario (`PS`) o sospechoso secundario (`SS`);
4. alcance de acontecimientos y versión de cualquier referencia DME/IME o de categoría clínica;
5. comparator y exclusiones;
6. estratos demográficos, geográficos, de gravedad y temporales;
7. métodos, política de celdas cero, nivel de confianza, procedimiento de multiplicidad y perfil de threshold; y
8. controles y medidas de evaluación al seleccionar un threshold.

Cambiar cualquiera de estos elementos después de examinar el resultado crea un análisis nuevo y debe producir un nuevo identificador de análisis.

## Medidas 2 × 2

Para un grupo de medicamento `D` y un acontecimiento `E`, `a` es el número de notificaciones elegibles que contienen ambos, `b` contiene `D` sin `E`, `c` contiene `E` sin `D` y `d` no contiene ninguno. Los recuentos son de notificaciones, no de prescripciones, personas expuestas, casos causados ni denominadores de incidencia.

- `PRR = [a/(a+b)] / [c/(c+d)]`
- `ROR = (a×d)/(b×c)`

La implementación Go y `research/R/reference_metrics.R` calculan límites de confianza asintóticos bilaterales fijos del 95% en escala log con el equivalente completo de `qnorm(0.975)`, no el atajo `1.96`. Cuando una celda es cero, la implementación de referencia actual suma 0,5 a todas las celdas y registra en cada métrica la elección de celdas de entrada/corrección. El p-value bilateral de la prueba exacta de Fisher está disponible para tablas dispersas; la ejecución online falla de forma cerrada en lugar de enumerar más de 100.000 términos del soporte. Si se muestran o utilizan múltiples p-values, es obligatorio el ajuste FDR de Benjamini-Hochberg y deben conservarse tanto los valores brutos como los ajustados.

El perfil guiado `a ≥ 3`, `PRR ≥ 2` y `χ² ≥ 4` con corrección de Yates es una heurística educativa al estilo de Evans. No es un “criterio EMA”, un threshold universal de decisión ni una prueba de señal de seguridad. El [addendum metodológico de la EMA](https://www.ema.europa.eu/en/documents/scientific-guideline/guideline-good-pharmacovigilance-practices-gvp-module-ix-addendum-i-methodological-aspects-signal_en.pdf) exige que los thresholds sean apropiados, documentados y evaluados para la base de datos y la finalidad.

## Métodos adicionales

El entorno R separado reserva versiones directas exactas de `faers 1.8.0`, `pvda 0.0.4` y `openEBGM 0.9.1` para la comparación independiente de ETL/métodos y el trabajo con BCPNN IC/IC025 y GPS EBGM/EB05. Estos métodos no se consideran validados ni habilitados por el mero hecho de que los paquetes estén enumerados. Adapters, resultados golden, reference sets y revisión de licencias siguen siendo gates de release.

No se publicará ningún threshold recomendado para investigación hasta que un reference set positivo/negativo preespecificado informe sensibilidad, valor predictivo positivo, comportamiento de falsos positivos y time-to-detection. El rendimiento de un threshold en un dataset, período o alcance de acontecimientos no se transfiere automáticamente a otro.

## Familias de acontecimientos y comparación de fuentes

Los PT de acontecimientos adversos clínicos deben analizarse por separado de las circunstancias de uso de medicamentos, problemas de calidad del producto, errores, inefectividad, abuso, uso indebido y términos de uso off-label. Hasta que exista una clasificación versionada, el ETL etiqueta cada PT como `UNCLASSIFIED_SOURCE_PT` en lugar de adivinar.

Dos fuentes de datos solo pueden compararse después de emparejar los mismos conceptos versionados de medicamento y acontecimiento. Informe en paralelo las estimaciones de efecto, intervalos, cobertura, datos ausentes y heterogeneidad. La intersección de conjuntos significa “observado en ambos análisis configurados”, nunca causalidad, replicación, validación o confirmación. FAERS global y Brasil no constituyen una comparación Estados Unidos–Brasil a menos que FAERS se filtre explícitamente para notificaciones de Estados Unidos.
