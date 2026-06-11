# Sistema Concurrente de Análisis y Predicción de Alta Demanda Eléctrica Doméstica

Proyecto académico para el curso **CC65 Programación Concurrente y Distribuida**.

La solución procesa un dataset de consumo eléctrico doméstico con más de 2 millones de registros. Para PC3 se implementa una base funcional en **Go** con carga concurrente, limpieza, generación de variables, entrenamiento paralelo de un modelo de Machine Learning y exportación de resultados en JSON.

## Tema

**Predicción de alta demanda eléctrica doméstica.**

La idea es identificar patrones de consumo elevado para apoyar decisiones de ahorro energético, eficiencia doméstica y sostenibilidad.

## Alcance técnico de PC3

Esta versión cubre:

- Lectura concurrente del dataset.
- Limpieza y validación de registros.
- Explicación de registros descartados.
- Generación de features para ML.
- Definición de variable objetivo `high_demand`.
- Entrenamiento paralelo de regresión logística.
- Métricas del modelo.
- Métricas básicas de rendimiento concurrente.

La API, el cluster distribuido, la base de datos y el frontend quedan para las siguientes entregas.

## Estructura

```txt
.
├── cmd/pc3/                         # Punto de entrada del programa
├── internal/loader/                 # Carga concurrente con goroutines/channels
├── internal/preprocessing/          # Parseo, limpieza y validación
├── internal/features/               # Feature engineering y target high_demand
├── internal/ml/                     # Regresión logística con gradientes paralelos
├── internal/metrics/                # Métricas de clasificación y rendimiento
├── data/                            # Ubicación del dataset
├── results/                         # JSON generados por la ejecución
├── docs/                            # Documentación e informe
├── docker/                          # Dockerfile inicial
├── scripts/                         # Scripts auxiliares
├── go.mod
└── README.md
```

## Dataset requerido

Colocar el archivo original en:

```txt
data/raw/household_power_consumption.txt
```

El archivo debe tener separador `;` y las columnas originales:

```txt
Date;Time;Global_active_power;Global_reactive_power;Voltage;Global_intensity;Sub_metering_1;Sub_metering_2;Sub_metering_3
```

## Ejecución local

Compilar todo el proyecto:

```bash
go build ./...
```

Ejecución rápida con límite de registros:

```bash
go run ./cmd/pc3 -input data/raw/household_power_consumption.txt -workers 4 -limit 100000 -out results -iterations 300 -lr 1.0
```

Ejecución con todo el dataset:

```bash
go run ./cmd/pc3 -input data/raw/household_power_consumption.txt -workers 8 -out results
```

Parámetros disponibles:

```txt
-input       Ruta del dataset original.
-workers     Número de workers concurrentes.
-limit       Límite opcional de registros para pruebas.
-out         Carpeta de salida.
-iterations  Iteraciones de entrenamiento del modelo.
-lr          Learning rate.
-test-ratio  Proporción del dataset limpio usada para prueba.
```

## Resultados generados

Al ejecutar, se generan tres archivos principales:

```txt
results/resumen_limpieza.json
results/metricas_modelo.json
results/benchmark_concurrencia.json
```

### `resumen_limpieza.json`

Incluye:

- Total de filas procesadas.
- Filas válidas.
- Filas descartadas.
- Razones de descarte.
- Número de workers.
- Tiempo de carga y limpieza.
- Modelo de concurrencia usado.

### `metricas_modelo.json`

Incluye:

- Features utilizadas.
- Umbral de alta demanda, calculado con percentil 75.
- Configuración del entrenamiento.
- Pérdida inicial y final.
- Accuracy, precision, recall y F1-score.
- Pesos finales del modelo.

### `benchmark_concurrencia.json`

Incluye:

- Workers usados.
- Registros procesados.
- Tiempo de carga.
- Tiempo de features.
- Tiempo de entrenamiento.
- Tiempo total.
- Registros por segundo.

Para comparar concurrencia, ejecutar el mismo comando cambiando solamente `-workers`:

```bash
go run ./cmd/pc3 -workers 1 -limit 100000
go run ./cmd/pc3 -workers 2 -limit 100000
go run ./cmd/pc3 -workers 4 -limit 100000
go run ./cmd/pc3 -workers 8 -limit 100000
```

## ¿Por qué se descartan registros?

Se descartan registros cuando no son confiables para el análisis o el entrenamiento del modelo. Las razones implementadas son:

- Fila vacía.
- Valor faltante representado por `?`.
- Cantidad incorrecta de columnas.
- Fecha u hora inválida.
- Valor numérico inválido.
- Rangos físicamente inconsistentes, por ejemplo voltaje menor o igual a cero o consumos negativos.

La estrategia es conservadora: no se imputan valores en PC3 para evitar introducir datos artificiales en el entrenamiento inicial.

## Concurrencia implementada

### Carga y limpieza

El paquete `internal/loader` usa un patrón productor/consumidor:

- La goroutine principal lee el archivo.
- Envía cada fila a un `channel`.
- Varios workers reciben filas, las parsean y validan.
- Cada worker acumula resultados parciales.
- El coordinador combina los parciales.

No hay condiciones de carrera porque los workers no actualizan contadores globales compartidos. Cada worker trabaja con su propio acumulador y luego lo envía al coordinador.

### Entrenamiento ML

El paquete `internal/ml` entrena una regresión logística binaria:

- Divide las muestras entre workers.
- Cada goroutine calcula gradientes parciales.
- El coordinador reduce los gradientes.
- Luego actualiza los pesos del modelo.

El modelo predice:

```txt
high_demand = 1 si Global_active_power >= percentil 75
high_demand = 0 en caso contrario
```

## Docker

Construcción desde la raíz del proyecto:

```bash
docker build -f docker/Dockerfile -t pc3-consumo-electrico .
```

Ejecución montando la carpeta del proyecto para acceder al dataset y resultados:

```bash
docker run --rm -v "$(pwd):/app" pc3-consumo-electrico pc3 -input data/raw/household_power_consumption.txt -workers 4 -limit 100000 -out results
```

## Estado

- [x] Proyecto estructurado en Go.
- [x] Carga concurrente.
- [x] Limpieza y validación.
- [x] Razones de descarte.
- [x] Feature engineering.
- [x] Regresión logística.
- [x] Entrenamiento paralelo.
- [x] Métricas del modelo.
- [x] Métricas de rendimiento.
- [x] Exportación de resultados JSON.
- [ ] API distribuida para PC4.
- [ ] Cluster de nodos ML para PC4.
- [ ] Base de datos para PC4.
- [ ] Frontend para TB2.

## Integrantes

- Omar Junior Acuña Villegas — U201613422
- Rafael Tomas Chui Sanchez — U201925837
- Axel Yamir Pariona Rojas — U202222148

## Evidencias generadas por el codigo

La ejecucion principal genera archivos para sustentar la PC3:

- `results/resumen_limpieza.json`: registros leidos, validos, descartados y razones de descarte.
- `data/processed/features_high_demand.csv`: dataset procesado con features normalizadas y etiqueta `high_demand`.
- `results/metricas_modelo.json`: metricas de entrenamiento y prueba.
- `results/modelo_entrenado.json`: modelo entrenado persistido para reutilizarlo en la API de PC4.
- `results/predicciones_muestra.csv`: muestra de predicciones contra valores reales.
- `results/benchmark_concurrencia.json`: tiempos de la ejecucion actual.
- `results/benchmark_comparativo.json`: comparacion de rendimiento con 1, 2, 4 y 8 workers cuando se ejecuta `scripts/benchmark_pc3.sh`.

El entrenamiento puede verse directamente en `results/modelo_entrenado.json` y `results/metricas_modelo.json`, en los campos `train_report.initial_loss`, `train_report.final_loss` y `train_report.training_history`.

## Ejecucion completa

```bash
go run ./cmd/pc3 \
  -input data/raw/household_power_consumption.txt \
  -workers 8 \
  -out results \
  -processed-out data/processed \
  -iterations 300 \
  -lr 1.0
```

Para generar evidencia comparativa de concurrencia:

```bash
./scripts/benchmark_pc3.sh data/raw/household_power_consumption.txt 100000 50 1.0
```

En Windows PowerShell se puede ejecutar el mismo programa con `go run` cambiando el numero de workers manualmente.
