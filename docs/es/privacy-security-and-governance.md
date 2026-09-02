# Privacidad, seguridad y gobernanza

## Datos y feedback

La aplicación pública no debe aceptar cargas de casos privados en v1. El endpoint de feedback anterior recopilaba correo electrónico, dirección IP, user agent y texto libre sin un conjunto completo de controles de retención y privacidad; por tanto, el feedback público se sustituyó por GitHub Issues/Discussions hasta que existan un responsable, finalidad, aviso, consentimiento/base jurídica, período de retención, proceso de eliminación, control de acceso, almacenamiento duradero, gestión de abusos y proceso de incidentes documentados.

No pegue en issues información de pacientes, narrativas de notificaciones, credenciales, claves de API, análisis no publicados ni datos confidenciales de instituciones. GitHub es un servicio independiente con sus propios términos y comportamiento de retención.

## Gobernanza de artefactos

- Las entradas son archivos públicos oficiales y congelados suministrados por el investigador; el servicio nunca los descarga implícitamente.
- Las entradas brutas, datasets generados y registros de fuentes son ignorados por Git en este repositorio.
- La publicación utiliza object storage inmutable con acceso controlado y una API compatible con S3, o un repositorio de archivo. El object lock/versionado y la política de retención deben ser configurados y probados por el operador.
- Los checksums demuestran identidad de bytes, no autenticidad, validez científica ni ausencia de contenido malicioso.
- Un registro formal de fuentes FAERS debe apuntar a la URL oficial de distribución de la FDA revisada y reconciliar el trimestre declarado con los nombres de archivo y la cobertura. Calcular un hash después de la descarga congela los bytes observados, pero no constituye un ancla externa de confianza; conserve la página/metadatos de respuesta de la FDA y el registro de revisión independiente.
- La retirada de un dataset nunca sustituye silenciosamente un objeto. Publique un tombstone/corrección vinculado al identificador inmutable.
- Los secretos pertenecen a secret stores de deployment y nunca deben entrar en manifiestos, logs, bundles exportados ni en el historial de Git.

## Límite de disponibilidad de la API pública

La aplicación limita los nuevos análisis en vivo/de investigación a dos workers concurrentes, espacia el inicio de nuevas investigaciones, aplica un deadline de solicitud de 20 segundos, hace fallar un resultado online por encima de 50.000 filas de acontecimientos y limita una exportación a 32 MiB por archivo, 64 MiB en total y 32 archivos. La prueba exacta es opt-in y rechaza un cálculo online que requiera más de 100.000 términos enumerados del soporte. Estos controles hacen fallar toda la solicitud; nunca truncan la familia de acontecimientos probada ni sustituyen silenciosamente otro método.

`RESEARCH_ALLOW_ONLINE_MATERIALIZATION` es false de forma predeterminada. En ese modo de solo resolución, un POST determinista puede recuperar un registro existente, pero un cache miss no puede crear estado permanente. El gate de inicio es local al proceso; habilitar la materialización en un deployment público o con múltiples réplicas todavía requiere identidad/cuotas de tasa a nivel de gateway, alertas y cuotas de capacidad del filesystem, límites de tamaño de solicitud/respuesta y operaciones de retención/retirada para análisis inmutables. Sin estos controles externos, callers distribuidos pueden eludir el pacing por proceso o agotar el volumen de resultados con el tiempo.

La identidad del software procede de los metadatos Go VCS incorporados al binario o de la flag de linker de la release. `PV_RADAR_APPLICATION_COMMIT` solo se acepta cuando el binario no tiene metadatos de revisión, y se rechaza un build sucio o no coincidente; la variable es una atestación del operador, no una sustitución.

El container científico es un límite batch, no un servicio de red. Después de restaurar las dependencias, ejecute las transformaciones con la red deshabilitada, filesystem raíz read-only, sin capabilities Linux, `no-new-privileges`, cuotas explícitas de CPU/memoria/tmp/salida, mounts read-only de fuentes y un directorio de salida escribible y nuevo. Los límites de miembros/recuento/tamaño expandido de ZIP protegen el parser, pero siguen siendo necesarias cuotas de infraestructura para Arrow/Parquet y workloads de trimestres reales.

## Límite de licencias

El código fuente de la aplicación se distribuye bajo MIT. Los datos FDA/ANVISA, la terminología MedDRA/WHODrug, los artículos científicos, containers y paquetes R tienen términos independientes. En particular, `pvda` y `openEBGM` usan licencias de la familia GPL. El entorno batch R está deliberadamente separado del binario Go, y el servicio Go consume artefactos de datos generados en lugar de enlazar código de paquetes R.

Esta separación reduce el acoplamiento, pero no es una determinación jurídica. Antes de la distribución, registre la versión/licencia de cada dependencia, los términos de las fuentes de datos, derechos de vocabulario, si los datos se redistribuyen o solo se reconstruyen y el público/jurisdicciones previstos. No empaquete material propietario de MedDRA o WHODrug sin los derechos necesarios.

## Control de cambios

Cualquier cambio en la cobertura de la fuente, deduplicación, elegibilidad, mapeo, comparator, alcance de acontecimientos, método, corrección, threshold, package lock o schema crea una nueva versión de dataset/análisis. La publicación en producción requiere un gate humano después de reconciliar las evidencias. Rollback significa servir el artefacto inmutable anterior y conservar el artefacto retirado y el motivo; la recuperación no está demostrada hasta que se ensaye en el store seleccionado.
