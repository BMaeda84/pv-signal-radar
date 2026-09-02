# Sesgos, limitaciones y modos de fallo observables

## Limitaciones interpretativas

- **Sin atribución causal:** los medicamentos y acontecimientos concurrentes en una notificación no están vinculados individualmente. Un PRR/ROR alto puede aparecer sin un efecto causal.
- **Sin incidencia ni riesgo individual:** las notificaciones espontáneas carecen de un denominador fiable de población expuesta.
- **Sesgo de notificación:** notoriedad, notificación estimulada, cobertura mediática, litigios, geografía, composición de notificadores y tiempo pueden cambiar el numerador y el comparator.
- **Comportamiento de duplicados y follow-ups:** el pipeline retiene la versión más reciente del caso, pero distintos ID de caso aún pueden describir episodios clínicos relacionados o duplicados.
- **Confusión e indicación:** enfermedad, medicación concomitante, canalización del tratamiento e indicación pueden crear o suprimir una asociación.
- **Drift terminológico:** el texto PT trimestral no establece la release de MedDRA; las strings de medicamento de origen no son un vocabulario canónico de ingredientes.
- **Sesgo de competencia:** cambiar el conjunto de medicamentos o acontecimientos modifica cada recuento del comparator.
- **Multiplicidad:** examinar muchos pares produce extremos por azar. FDR controla una familia declarada de pruebas, no la causalidad.
- **Datos dispersos:** los intervalos asintóticos y las correcciones de continuidad pueden dominar tablas pequeñas; inspeccione siempre las celdas y los resultados de Fisher.
- **Inestabilidad temporal:** los resultados openFDA en vivo y posteriores revisiones de FAERS pueden cambiar. Un snapshot congelado y verificado en cuanto a integridad es necesario, pero no suficiente para la reproducción; también deben fijarse la configuración, la revisión del código, el runtime y los supuestos de vocabulario.

La FDA describe los datos públicos de FAERS como una entrada de un proceso poscomercialización más amplio, con notificaciones duplicadas y definiciones de caso variables; la [documentación de acontecimientos de medicamentos de openFDA](https://open.fda.gov/apis/drug/event/) advierte explícitamente contra la inferencia de causalidad o incidencia.

## Condiciones de fallo cerrado

| Condición | Por qué es insegura | Comportamiento observable |
|---|---|---|
| SHA-256 de origen ausente o no coincidente | La identidad de entrada no está demostrada | El build termina con código distinto de cero y nombra el archivo |
| `CASEID`, `PRIMARYID` o `CASEVERSION` no válido | La deduplicación se vuelve ambigua | El build termina con código distinto de cero e informa el recuento de filas no válidas |
| Tabla DEMO/DRUG/REAC ausente o duplicada | Un trimestre está incompleto o es ambiguo | El build termina con código distinto de cero y nombra la tabla/trimestre |
| Ruta de traversal en ZIP | La extracción podría salir del almacenamiento temporal | El archive se rechaza antes de la extracción |
| Ningún par medicamento–acontecimiento elegible | Las medidas no tienen un universo definido | El build termina con código distinto de cero |
| Celda 2 × 2 reconstruida negativa | Las marginales son incoherentes | La agregación termina con código distinto de cero |
| Directorio de salida no vacío | Podrían mezclarse artefactos anteriores y nuevos | El build se niega a escribir |
| Dependencia R ausente o versión de método incorrecta | El runtime no es el entorno declarado | Bootstrap/build termina con código distinto de cero |
| Revisión de la aplicación sucia, ausente o conflictiva | El ID del análisis podría identificar código que no se ejecutó | El inicio en modo de investigación termina con código distinto de cero |
| La prueba exacta o la familia de resultados supera el límite de trabajo online | Una solicitud pública podría monopolizar CPU o memoria | La solicitud devuelve un error de método batch/protocolo no admitido; no se truncan filas |
| Se alcanza el límite de concurrencia o pacing de análisis/exportación | El trabajo concurrente podría agotar CPU o memoria | La API devuelve `429` con `Retry-After` |

## Condiciones que no fallan automáticamente

Los datos demográficos opcionales ausentes, el texto de medicamento no mapeado y los PT no clasificados se conservan porque eliminarlos o adivinarlos silenciosamente ocultaría la incertidumbre. Sus recuentos deben revisarse en QA y divulgarse en cada análisis. Un nivel de datos ausentes científicamente inaceptable es específico del protocolo y debe establecerse antes del análisis.
