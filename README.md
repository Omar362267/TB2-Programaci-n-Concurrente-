# Sistema Concurrente de Análisis y Predicción de Alta Demanda Eléctrica Doméstica

Proyecto académico para el curso **CC65 Programación Concurrente y Distribuida**.

La solución analiza un dataset de consumo eléctrico doméstico con más de 2 millones de registros y construye una base concurrente en Go para limpieza, análisis y entrenamiento inicial de un modelo de Machine Learning orientado a predecir momentos de alta demanda energética.

## Objetivo del proyecto

Desarrollar una solución en Go que procese registros de consumo eléctrico doméstico de forma concurrente, genere variables relevantes para el análisis energético y entrene un modelo predictivo inicial para identificar situaciones de alta demanda.

## Pregunta de impacto social

¿Cómo se pueden identificar horarios y patrones de alta demanda eléctrica doméstica para apoyar decisiones de ahorro energético, reducir costos familiares y promover un uso más sostenible de la energía?

## Alcance de PC3

Esta entrega se enfoca en la base concurrente del sistema:

1. Presentación del caso a resolver.
2. Limpieza y análisis de datos.
3. Diseño del modelo ML.
4. Paralelización del cálculo con goroutines y channels.
5. Evidencias iniciales de implementación.
6. Reporte de participación.

La distribución mediante cluster de nodos ML, API, base de datos y frontend quedan planificadas para PC4 y TB2.

## Dataset

Dataset base: **Individual Household Electric Power Consumption**.

Características principales documentadas en la exploración inicial:

- Registros originales: 2,075,259.
- Registros limpios estimados: 2,049,280.
- Periodo: 16/12/2006 al 26/11/2010.
- Variable objetivo propuesta: `high_demand`.
- Umbral inicial: percentil 75 de `Global_active_power`.

El dataset completo no se incluye directamente en el repositorio por tamaño. Debe colocarse manualmente en:

```txt
/data/raw/household_power_consumption.txt
```

Ver instrucciones en [`data/README.md`](data/README.md).

## Arquitectura inicial para PC3

```txt
Archivo CSV/TXT grande
        |
        v
Cargador concurrente en Go
        |
        v
Workers de limpieza y validación
        |
        v
Generación de features
        |
        v
Entrenamiento paralelo del modelo ML
        |
        v
Métricas y resultados en /results
```

## Estructura del repositorio

```txt
.
├── cmd/pc3/                         # Punto de entrada de la aplicación Go
├── internal/loader/                 # Carga concurrente de datos
├── internal/preprocessing/          # Limpieza y validación
├── internal/features/               # Variables derivadas
├── internal/ml/                     # Modelo ML inicial
├── internal/metrics/                # Métricas del modelo y rendimiento
├── data/                            # Instrucciones y ubicación del dataset
├── results/                         # Resultados generados
├── docs/                            # Informe PC3 y documentación
├── docker/                          # Dockerfile inicial
├── go.mod
└── README.md
```

## Ejecución prevista

```bash
go run ./cmd/pc3 -input data/raw/household_power_consumption.txt -workers 8
```

Parámetros esperados:

```txt
-input      Ruta del dataset original
-workers    Número de workers concurrentes
-limit      Límite opcional de registros para pruebas rápidas
-out        Carpeta de salida para resultados
```

## Evidencias esperadas para PC3

Los resultados de ejecución deben guardarse en `/results`:

```txt
resumen_limpieza.json
metricas_modelo.json
benchmark_concurrencia.json
```

Además, se deben incluir capturas o logs en el informe PC3.

## Tecnologías

- Go
- Goroutines
- Channels
- WaitGroup
- Docker, previsto para despliegue
- GitHub y Git Flow

## Estado actual

- [x] Caso social definido.
- [x] Dataset elegido y justificado.
- [x] Documentación exploratoria inicial.
- [ ] Cargador concurrente completo en Go.
- [ ] Limpieza y análisis ejecutados desde Go.
- [ ] Entrenamiento paralelo del modelo ML.
- [ ] Evidencias finales de PC3.
- [ ] Reporte de participación final.

## Integrantes

- Omar Junior Acuña Villegas — U201613422
- Rafael Tomas Chui Sanchez — U201925837
- Axel Yamir Pariona Rojas — U202222148
