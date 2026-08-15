# Política de seguridad

## Versiones mantenidas

| Versión | Estado | Canal |
| --- | --- | --- |
| `1.0.x` | Mantenida | `production` |
| `< 1.0.0` | Fuera de soporte | — |

## Comunicación responsable

Utiliza **Security → Report a security issue** en GitHub para comunicar de forma privada comportamientos que afecten a autorización, integridad contable, disponibilidad o confidencialidad. No publiques datos operativos en una issue.

Incluye commit, fixture mínimo, precondiciones, componente, impacto, salida JSON relevante y propiedad esperada. No adjuntes credenciales, claves, datos personales ni información de terceros. El equipo confirmará la recepción y coordinará análisis, corrección y divulgación.

## Fronteras de confianza

```mermaid
flowchart TB
    subgraph I["Integración"]
      C["Cliente institucional"]
      API["Adaptador HTTPS"]
    end
    subgraph D["Dominio"]
      E["DeltaForge Engine"]
      P["Políticas"]
    end
    subgraph S["Estado"]
      L["Ledger"]
      J["Journal"]
      H["Snapshot SHA-256"]
    end
    C --> API
    API --> E
    E --> P
    E --> L
    L --> J
    L --> H
```

```mermaid
flowchart LR
    O["Cambio administrativo"] --> C["Codificación canónica"]
    C --> H["Operation ID"]
    H --> Q["Quorum"]
    Q --> T["Timelock"]
    T --> P{"Predecesor ejecutado"}
    P -->|sí| E["Ejecución"]
    P -->|no| R["Rechazo cerrado"]
```

## Controles obligatorios

- Unicidad y normalización de cuenta, activo, grupo y operación.
- Aritmética entera con comprobación de overflow en cálculos de capital.
- Snapshot canónico y orden estable para evidencia reproducible.
- Separación entre adaptación HTTP y autoridad del motor.
- Claves de idempotencia distintas por escritura.
- Quorum, timelock, caducidad y predecesores para cambios de política.
- Conciliación de expected, executed, final, correcciones, pools y remanentes.
- Dependencias bloqueadas y revisión CODEOWNERS.

## Matriz de aseguramiento

| Área | Propiedad | Evidencia |
| --- | --- | --- |
| Ingesta | IDs únicos y política válida | validación de fixtures |
| Capital | Reserva disponible cubre demanda estresada | pruebas de `capital` |
| Gobierno | Cambio maduro y aprobado | pruebas de `governance` |
| Cliente | Precisión e idempotencia | pruebas del SDK |
| Promoción | Referencias en un commit | workflow de integridad |

## Respuesta

```mermaid
sequenceDiagram
    participant R as Remitente
    participant S as Seguridad
    participant E as Ingeniería
    participant O as Operaciones
    R->>S: Comunicación privada
    S->>S: Triage y severidad
    S->>E: Reproducción acotada
    E->>S: Cambio y evidencia
    S->>O: Autorización de promoción
    O->>O: Conciliación de referencias
    S-->>R: Resolución coordinada
```
