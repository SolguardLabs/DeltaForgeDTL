# Operaciones

## Validación

La secuencia local y CI es idéntica:

```bash
npm ci
npm run ci
```

Incluye Prettier, `gofmt`, build reproducible, `go vet`, pruebas Go y Node, verificación documental, hash del banner y auditoría de dependencias.

```mermaid
flowchart LR
    C["Cambio"] --> L["Validación local"]
    L --> PR["Pull request"]
    PR --> CI["Linux + Windows"]
    CI --> M["main"]
    M --> P["production"]
    P --> T["tag anotado"]
    T --> R["Publicación"]
```

## Rutina de epoch

```mermaid
flowchart TB
    S["Abrir epoch"] --> CFG["Verificar política activa"]
    CFG --> I["Ingerir snapshots"]
    I --> H["Comparar hashes y totales"]
    H --> CAP["Evaluar capital"]
    CAP --> D{"Conforme"}
    D -->|no| HOLD["Mantener conciliación cerrada"]
    D -->|sí| RUN["Ejecutar ciclo"]
    RUN --> CLOSE["Sellar reporte"]
    HOLD --> GOV["Escalar al consejo"]
```

El cierre conserva input canónico, reporte, journal, parámetros, métricas, commit y responsable. El epoch del dominio prevalece sobre la hora del host.

## Promoción

```mermaid
sequenceDiagram
    participant E as Ingeniería
    participant CI as Actions
    participant G as Git refs
    participant R as Releases
    E->>CI: Candidato
    CI-->>E: Matriz verde
    E->>G: Merge a main
    CI-->>E: main verde
    E->>G: production = main
    CI-->>E: production verde
    E->>G: tag anotado
    CI-->>E: tag verde
    E->>R: Production 1.0.0
    CI-->>E: integridad verde
```

No se reconstruye código entre referencias. `main`, `production`, el commit pelado del tag y la publicación deben coincidir.

## Recuperación

Ante una inconsistencia se cierra la admisión en el adaptador, se preservan input y estado, se captura el reporte y se reconstruye el journal. No se editan balances manualmente. La reanudación requiere causa explicada, conciliación completa y, cuando cambie política, una operación de gobierno ejecutada.

Una versión anterior solo puede leer estado si conserva esquema y semántica. El rollback de binario no revierte el ledger; cualquier compensación se registra como una nueva transición.

## Checklist de entrega

1. Árbol limpio y lockfile actualizado.
2. Dos jobs candidatos verdes.
3. PR revisado y fusionado.
4. `main` y `production` verdes en el mismo SHA.
5. Tag anotado sobre ese SHA.
6. Publicación no draft ni prerelease.
7. Integridad confirmada tras el evento release.
