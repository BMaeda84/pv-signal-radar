# PV Signal Radar 🛰️

[Português (Brasil)](README.pt-BR.md) · **Español** · [English](README.md)

[![CI](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml/badge.svg)](https://github.com/BMaeda84/pv-signal-radar/actions/workflows/ci.yml)
[![Licencia: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)

> Panel open source de **cribado exploratorio** de farmacovigilancia. Consulta informes públicos de [openFDA FAERS](https://open.fda.gov/apis/drug/event/) y calcula PRR, ROR, intervalos de confianza del 95 % y chi-cuadrado corregido de Yates para generar hipótesis.

## Qué hace

Para una sustancia activa, el servicio obtiene los MedDRA Preferred Terms (PTs) más informados y construye una tabla 2 × 2 a nivel de informe:

| | Reacción objetivo (E) | Otras reacciones (¬E) | Total |
|---|---:|---:|---:|
| Medicamento objetivo (D) | a | b | a + b |
| Otros medicamentos (¬D) | c | d | c + d |
| Total | a + c | b + d | N |

El panel presenta:

- `PRR = [a / (a + b)] / [c / (c + d)]`;
- `ROR = (a × d) / (b × c)`;
- intervalos de confianza asintóticos del 95 %, con corrección de Haldane-Anscombe cuando hay una celda cero;
- chi-cuadrado corregido de Yates y un gráfico volcán exploratorio; y
- una etiqueta de cribado configurada cuando `a ≥ 3`, `PRR ≥ 2,0` y `χ² ≥ 4,0`.

La etiqueta es una regla de priorización implementada por el proyecto; **no** es una decisión regulatoria, un hallazgo clínico ni evidencia de causalidad.

## Idiomas y accesibilidad

La interfaz pública admite **Português (Brasil)**, **Español** e English. El selector persiste en el navegador, actualiza idioma/título/metadatos del documento y formato numérico, sin cambiar el payload de la API ni la caché. Los términos MedDRA y los enums de la API permanecen en el idioma de origen para preservar su semántica.

Incluye campo de búsqueda etiquetado, controles de matriz 2 × 2 operables con teclado, alternativa textual para el canvas, foco visible y diseño responsivo.

## Ejecución local

### Go

```powershell
git clone https://github.com/BMaeda84/pv-signal-radar.git
Set-Location pv-signal-radar
go run ./cmd/server
```

Abra <http://localhost:8080>.

### Docker

```powershell
docker build --tag pv-signal-radar .
docker run --rm --publish 8080:8080 pv-signal-radar
```

La imagen se ejecuta como usuario no-root y expone `/api/v1/health` para health checks de contenedor/orquestador.

## Configuración

| Variable | Predeterminado | Finalidad |
|---|---:|---|
| `PORT` | `8080` | Puerto HTTP. Railway inyecta este valor automáticamente. |
| `OPENFDA_API_KEY` | sin definir | Clave opcional de openFDA. Use el secret store del despliegue; nunca la incluya en un commit. |
| `CACHE_CAPACITY` | `500` | Máximo de análisis completados en la caché LRU en memoria. |
| `CACHE_TTL_HOURS` | `24` | TTL, en horas, de entradas completas de caché. |

Cada análisis sin caché realiza varias solicitudes a openFDA; por ello el servicio limita los análisis simultáneos y separa los inicios de scan por al menos 15 segundos en cada proceso. Con el máximo actual de 28 solicitudes upstream por scan, no pueden ocurrir más de cinco inicios en una ventana de 60 segundos (140 llamadas a openFDA), sin burst inicial. Cuando está saturado o limitado por ritmo responde `429` con `Retry-After`, en vez de acumular trabajo sin límite. Las cuotas diarias del upstream y varias instancias aún requieren monitorización operativa.

## API

```http
GET /api/v1/analyze?drug=Semaglutide
GET /api/v1/health
```

`/api/v1/analyze` acepta solo `GET`. La respuesta incluye sustancia consultada, universo FAERS actual, recuentos de origen, métricas, `signal_level` estable, timestamp y advertencia de uso exploratorio. Los análisis terminados quedan en la caché del servidor; las respuestas HTTP usan `Cache-Control: no-store`.

Los códigos de error son `drug_required` (`400`), `invalid_drug` (`400`), `method_not_allowed` (`405`), `analysis_busy` (`429`), `analysis_rate_limited` (`429`) y `analysis_unavailable` (`502`). Los errores del upstream no se serializan al cliente para evitar reflejar `OPENFDA_API_KEY` en una respuesta pública.

## Límite de calidad de los datos

El proyecto falla de forma explícita cuando no puede obtener el universo actual o el background de una reacción. No sustituye el universo por una constante histórica ni el background ausente por el numerador, porque eso puede fabricar PRR/ROR extremos y una señal de cribado falsa.

Persisten límites materiales:

1. Los informes FAERS son espontáneos, incompletos y están sujetos a sesgos de notificación, notoriedad, duplicación y tiempo.
2. Un informe puede contener varios medicamentos y varias reacciones; los datos públicos de openFDA no establecen individualmente que un medicamento causó una reacción concreta.
3. El proyecto no añade deduplicación a nivel de caso, denominadores de exposición, ajuste de factores de confusión, adjudicación clínica ni un snapshot inmutable de la fuente.
4. openFDA se actualiza con el tiempo; la misma consulta puede devolver resultados diferentes posteriormente.
5. No es consejo médico, CDS, sistema GxP validado/cualificado ni sistema de reporte regulatorio.

Use los resultados únicamente como punto de partida para la revisión por profesionales cualificados, con evidencia clínica y de caso adecuada.

## Verificación

```powershell
go test -race ./...
go vet ./...
go build ./cmd/server
docker build --tag pv-signal-radar:local .
```

La CI ejecuta pruebas con race detector, análisis estático, compilación del binario y build Docker en cada pull request y push a `main`.

En Windows, `go test -race` requiere CGO y un compilador C disponible. Si el host local no proporciona ese toolchain, ejecute `go test ./...` localmente y use la ejecución Linux de CI como comprobación del race detector.

## Licencia

Distribuido bajo la [Licencia MIT](LICENSE). Desarrollado por [Bruno Maeda](https://github.com/BMaeda84).
