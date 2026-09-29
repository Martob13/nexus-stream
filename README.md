# nexus-stream

Pipeline de procesamiento asíncrono de telemetría de alto rendimiento diseñado con aislamiento multi-runtime (FastAPI, pool de workers en Go con CGO y núcleo de cómputo estático en Rust).



## Arquitectura del Sistema

nexus-stream combina la velocidad de desarrollo e ingesta asíncrona de Python con los modelos de concurrencia y procesamiento a bajo nivel de Go y Rust:

- **Capa de Ingesta (Python / FastAPI):** Validación estricta con Pydantic (1 <= samples <= 65535, descarte de valores NaN/Inf y marcas de tiempo positivas), autenticación obligatoria mediante token Bearer con comparación constante contra ataques de temporización (secrets.compare_digest), separación de sondas /health (liveness) y /ready (readiness con ping a Redis), y encolado no bloqueante con redis.asyncio.
- **Motor de Procesamiento (Go):** Pool concurrente de workers consumiendo desde Redis mediante canales con buffer limitado, métricas atómicas de descarte por saturación (sync/atomic), registro estructurado en formato JSON con log/slog, validación defensiva del protocolo (worker/pkg/frame) y apagado limpio sin condiciones de carrera ante señales del sistema operativo (SIGINT/SIGTERM).
- **Núcleo de Cómputo (Rust):** Enlazado estáticamente en Go vía CGO (libcore_parser.a, sin dependencias externas). Decodifica tramas binarias (0xAA55 little-endian) y calcula métricas RMS y pico mediante desenrollado de bucles en 4 vías orientado a la auto-vectorización por LLVM, garantizando cero reservas dinámicas en heap dentro del bucle de parseo.

## Justificación Técnica: ¿Por qué Rust en un Pipeline de Go?

Un micro-benchmark elemental muestra que un bucle directo en Go puro suele ser más rápido que cruzar la frontera de CGO debido a la penalización por cambio de contexto del runtime de Go (~50–100ns por llamada).

En nexus-stream, Rust se incorpora por garantías arquitectónicas concretas:
1. **Núcleo Computacional Portable:** El kernel de Rust se compila como biblioteca estática con C-ABI (libcore_parser.a). La misma rutina matemática puede integrarse sin cambios en Python FFI, C++, o microcontroladores embebidos, evitando duplicidad de lógica.
2. **Determinismo y Frontera Libre de GC:** Mientras Go depende de un recolector de basura rastreador, el parseo en Rust opera sobre buffers prestados sin reservas en memoria dinámica, garantizando tiempos de respuesta deterministas.
3. **Amortización por Lotes:** En paquetes pequeños el costo de CGO es visible; sin embargo, en flujos masivos donde las muestras se agrupan en tramas de varios kilobytes, el rendimiento del cómputo vectorial supera la sobrecarga de transición.

### Evaluación de Rendimiento (make bench)

Evaluado en entorno Linux amd64 (12 núcleos):

| Escenario de Carga | Go Puro (Línea Base) | Go -> CGO -> Rust FFI | Costo de Framing Binario |
| :--- | :--- | :--- | :--- |
| Paquete Individual (128 muestras ≈ 1 KB) | ~1,169 MSamples/seg | ~761 MSamples/seg (limitado por CGO) | ~324 ns |
| Trama por Lotes (16,384 muestras ≈ 128 KB) | ~1,369 MSamples/seg | ~1,010 MSamples/seg | ~43.4 µs |

## Variables de Entorno

| Variable | Requerida | Valor por Defecto | Descripción |
| :--- | :--- | :--- | :--- |
| NEXUS_AUTH_TOKEN | Sí | Ninguno | Token secreto Bearer para autorizar peticiones HTTP |
| REDIS_HOST | No | localhost | Dirección del servidor Redis |
| REDIS_PORT | No | 6379 | Puerto de conexión a Redis |

## Estructura del Repositorio



## Verificación y Ejecución



## Licencia

Distribuido bajo la Licencia MIT. Consulta el archivo [LICENSE](LICENSE) para más detalles.
