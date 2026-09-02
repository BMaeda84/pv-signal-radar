# Uso previsto y responsabilidades de los usuarios

## Uso previsto

PV Signal Radar respalda la enseñanza, la exploración transparente y el estudio académico reproducible de la notificación desproporcionada en conjuntos públicos de notificaciones espontáneas. Sus resultados son asociaciones estadísticas de notificación (`SDRs`, signals of disproportionate reporting) que requieren revisión cualificada y evidencia externa.

Los usos adecuados incluyen:

- enseñar tablas 2 × 2, PRR, ROR, comportamiento de datos dispersos, sesgo de notificación y diseño de protocolos;
- generar hipótesis documentadas para la revisión de la literatura o de series de casos;
- reproducir un análisis publicado a partir de una configuración y un manifiesto de fuente congelados;
- comparar métodos, thresholds, ventanas temporales o estratos predefinidos; y
- preparar tablas y figuras para un manuscrito cuando estén acompañadas del protocolo completo y sus limitaciones.

## Usos explícitamente excluidos

La aplicación no es asesoramiento médico, diagnóstico, estimación de riesgo individual, calculadora de exposición-incidencia, soporte para decisiones clínicas, sistema de notificación de acontecimientos adversos, sistema GxP validado o cualificado ni mecanismo automatizado de decisión regulatoria. No establece que un medicamento haya causado un acontecimiento y no debe utilizarse como única evidencia para cambiar un tratamiento, etiquetado, conclusiones de beneficio-riesgo o una acción regulatoria.

El endpoint openFDA en vivo es una facilidad exploratoria. Debido a que su fuente cambia, no proporciona un conjunto de investigación congelado y su resultado no debe citarse como análisis reproducible.

## Uso según el público

### Estudiantes

Utilice el modo guiado con las celdas 2 × 2 mostradas. Recalcule al menos una fila, explique el comparator y la corrección de celdas cero e identifique un sesgo que pueda cambiar la medida sin cambiar el riesgo biológico. No denomine señal de seguridad confirmada al cruce de un threshold.

### Investigadores

Registre un protocolo antes de inspeccionar los resultados. Congele las revisiones del dataset y del software, declare el papel del medicamento, alcance de acontecimientos, comparator, período, estratos, estrategia de multiplicidad y perfil de threshold y, después, archive el bundle exportado con los materiales del estudio. Siga las [recomendaciones READUS-PV](https://pmc.ncbi.nlm.nih.gov/articles/PMC11116242/) para la presentación de resultados; el checklist mejora la calidad del informe, pero no valida un estudio ni una afirmación causal.

### Profesionales de farmacovigilancia

Trate un SDR como un elemento de triaje. Revise casos, patrones temporales, duplicados, notificación estimulada, indicaciones, medicamentos concomitantes, plausibilidad biológica, literatura, contexto de exposición y otras fuentes de datos. Siga el sistema de calidad organizativo aplicable y el [EMA GVP Module IX](https://www.ema.europa.eu/en/documents/scientific-guideline/guideline-good-pharmacovigilance-practices-gvp-module-ix-signal-management-rev-1_en.pdf); este software no sustituye las etapas de validación, confirmación, análisis, priorización o evaluación.

## Límite mínimo de citación

Un resultado formal debe identificar el ID del dataset, la cobertura de la fuente, los checksums de fuente y salida, el ID/configuración del análisis, el commit del software, el lockfile de R, las definiciones de los métodos, las exclusiones y las desviaciones conocidas. Si falta alguno de estos elementos, describa el resultado como exploratorio y no reproducible.
