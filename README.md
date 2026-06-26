# Patch Fase 8 — Evaluación hold-out y plan de benchmark

Descomprima este contenido sobre la raíz del proyecto de Fase 7.

## Cambio funcional

Añade `POST /v1/evaluate`, que evalúa el modelo activo sobre `data/distributed/test.csv` sin entrenarlo, sin modificar pesos y sin usar la caché Redis.

La API carga ese archivo al iniciar mediante:

```powershell
-test-data data/distributed/test.csv
```

El endpoint devuelve loss de test, accuracy, precision, recall, F1, TP, TN, FP, FN, tiempo y versión de modelo.

## Validación local

```powershell
go test ./...
go test -race ./...
```

Luego inicie API como antes (el parámetro `-test-data` tiene el valor correcto por defecto) y ejecute:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/v1/evaluate | ConvertTo-Json -Depth 10
```

## Secuencia recomendada para experimentos

1. 10, 50, 100 y 300 iteraciones con 4 nodos y 4 workers por nodo.
2. Seleccionar las iteraciones usando F1 y loss de test, no solo loss de entrenamiento.
3. Recién después medir configuración de workers: 1, 2, 4 y 8 por nodo.
4. Luego comparar 1, 2 y 4 nodos regenerando shards para cada caso.
