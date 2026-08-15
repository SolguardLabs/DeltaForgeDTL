# Integración

## SDK

`sdk/deltaForgeClient.ts` expone el modelo de capital y un cliente HTTP. Los importes son `bigint`; el JSON canónico los emite como literales enteros y ordena claves. El cliente exige HTTPS, limita timeout y rechaza redirecciones.

```mermaid
flowchart LR
    A["Aplicación"] --> V["Validación local"]
    V --> J["JSON canónico"]
    J --> H["HTTPS"]
    H --> API["Adaptador"]
    API --> E["DeltaForge Engine"]
```

```ts
import { DeltaForgeClient } from "./sdk/deltaForgeClient.ts";

const client = new DeltaForgeClient({
  baseUrl: "https://api.deltaforge.example/settlement/",
  timeoutMs: 8_000,
});

const accepted = await client.submitSnapshot(
  { book: "df-main", epoch: 7001, expectedTotal: 900_000n },
  { idempotencyKey: "snapshot-20260815-0001" },
);
```

## Endpoints de referencia

| Método | Ruta | Función |
| --- | --- | --- |
| `GET` | `/v1/cycles/{id}` | Consultar estado y reporte |
| `POST` | `/v1/snapshots` | Registrar snapshot de entrada |
| `POST` | `/v1/cycles/{id}/reconciliation` | Ejecutar un ciclo aceptado |

Los errores usan `code` y `message`. El código es estable para automatización. Se recomienda `409` para conflicto de estado, `422` para semántica inválida y `503` si no puede confirmarse una escritura.

## Idempotencia

```mermaid
sequenceDiagram
    participant C as Cliente
    participant A as Adaptador
    participant E as Engine
    C->>A: POST + Idempotency-Key
    A->>A: Guardar clave y hash del cuerpo
    A->>E: Ejecutar una vez
    E-->>A: Reporte
    A-->>C: Respuesta aceptada
    C->>A: Reintento idéntico
    A-->>C: Mismo resultado
```

Una clave con cuerpo distinto responde conflicto. El timeout no demuestra fallo: el cliente consulta por ciclo y reutiliza la misma clave antes de cualquier reintento.

## Reintentos

```mermaid
flowchart TB
    R["Respuesta"] --> S{"Código"}
    S -->|2xx| OK["Confirmar"]
    S -->|409 o 422| STOP["Corregir estado o entrada"]
    S -->|429 o 503| B["Backoff con jitter"]
    S -->|timeout| Q["Consultar ciclo"]
    B --> P{"Presupuesto"}
    P -->|disponible| RETRY["Misma clave y cuerpo"]
    P -->|agotado| ESC["Escalar"]
```

## Compatibilidad

La serie `1.0.x` mantiene semántica de estados, direcciones de redondeo y campos existentes. Añadir propiedades de respuesta es compatible si el consumidor ignora campos desconocidos. Cambiar una unidad, fórmula o estado requiere versión mayor.
