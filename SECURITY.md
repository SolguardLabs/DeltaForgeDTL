# Security

DeltaForgeDTL modela un motor de reconciliacion local para ciclos de settlement.
La superficie de revision incluye fixtures JSON, normalizacion de politicas,
clasificacion de diferencias, correcciones directas, allocations finales y
reportes de auditoria.

## Modelo De Seguridad

- Los fixtures deben declarar cuentas, assets y politicas con ids unicos.
- Los saldos se procesan como enteros para evitar dependencias de coma flotante.
- La CLI valida estructura antes de ejecutar el ciclo.
- Los snapshots usan lineas canonicas y hash SHA-256 para detectar cambios de
  estado.
- Las correcciones directas quedan reflejadas en el reporte con trazas
  deterministas.
- Las allocations finales se serializan con ids estables y orden predecible.

## Invariantes Esperadas

- Ningun asset o account puede tener identificador vacio.
- Los totales finales deben reflejar las diferencias contabilizadas por el
  ciclo.
- Las cuentas no aptas para settlement global no deben recibir allocations.
- Las correcciones materiales deben quedar registradas como entradas directas.
- Los floors configurados deben aplicarse durante la liquidacion final.

## Validaciones Automatizadas

La suite TypeScript cubre contrato CLI, snapshots, reconciliacion, correcciones y
liquidacion. `npm run ci` ejecuta build, tests y conteo LOC de `src/`.

## Dependencias

El motor Go usa solo la libreria estandar. Node se utiliza para scripts y tests.
No se requieren servicios remotos para compilar o ejecutar el lab.

## Reporte Interno

Los hallazgos deben incluir fixture minimo, salida JSON relevante, version de Go,
version de Node y pasos para reproducir el comportamiento observado.
