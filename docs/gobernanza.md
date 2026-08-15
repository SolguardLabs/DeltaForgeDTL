# Gobernanza

## Operación canónica

Cada cambio declara dominio, red, destino, método, hash del payload, salt, inicio, caducidad y predecesor opcional. La codificación antepone la longitud de cada texto en big-endian; el ID es SHA-256 de esos bytes.

```mermaid
flowchart LR
    S["GovernanceSpec"] --> V["Validación"]
    V --> C["Campos con longitud"]
    C --> T["Timestamps big-endian"]
    T --> H["SHA-256"]
    H --> ID["Operation ID"]
```

Dominio y red evitan reutilizar aprobaciones entre entornos. El salt separa cambios con payload idéntico. Toda variación genera otro ID y requiere nuevas aprobaciones.

## Estados

```mermaid
stateDiagram-v2
    [*] --> Scheduled: schedule
    Scheduled --> Scheduled: approve
    Scheduled --> Ready: quorum y execute_after
    Scheduled --> Expired: expires_at
    Ready --> Executed: execute
    Ready --> Cancelled: guardian
    Scheduled --> Cancelled: guardian
    Executed --> [*]
    Cancelled --> [*]
    Expired --> [*]
```

Las aprobaciones forman un conjunto ordenado; repetir un gobernador no aumenta quorum. El guardián debe ser independiente del consejo y solo puede cancelar operaciones no terminales con una nota normalizada.

## Dependencias

```mermaid
sequenceDiagram
    participant C as Consejo
    participant G as GovernanceBook
    participant A as Adaptador
    C->>G: Programar capacidad A
    C->>G: Programar política B, predecessor=A
    C->>G: Aprobar A y B
    C->>G: Ejecutar B
    G-->>C: A aún no ejecutada
    C->>G: Ejecutar A
    G-->>A: Aplicar capacidad
    C->>G: Ejecutar B
    G-->>A: Aplicar política
```

El adaptador confirma que el registro está en `executed` y que su ID corresponde exactamente al payload aplicado.

## Política recomendada

| Cambio | Quorum | Timelock | Caducidad |
| --- | ---: | ---: | ---: |
| Umbrales de alerta | 2/3 | 6 h | 48 h |
| Capacidad y tolerancias | 3/5 | 24 h | 72 h |
| Activos y grupos | 4/7 | 48 h | 96 h |
| Cancelación de emergencia | guardián | inmediata | puntual |

Cada ejecución conserva spec, ID, aprobadores, timestamps, estado terminal, commit y reporte posterior.
