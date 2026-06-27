# TB2 - Programación Concurrente y Distribuida (PC3 + PC4)

## Autor
U202222148 - Pariona Rojas, Axel Yamir
U - Acuña Villegas, Omar Junior
U - Chui Sanchez, Rafael Thomas

## Descripción del proyecto

Este proyecto implementa un sistema completo de procesamiento de datos y predicción de demanda eléctrica utilizando programación concurrente y distribuida en Go.

El sistema evoluciona desde un enfoque concurrente (PC3) hacia una arquitectura distribuida con múltiples nodos (PC4), incorporando tolerancia a fallos, cache y persistencia de datos.

## Objetivo general

Diseñar e implementar un sistema capaz de:

- Procesar grandes volúmenes de datos eléctricos
- Aplicar limpieza y transformación concurrente
- Entrenar un modelo de regresión logística desde cero en Go
- Distribuir el entrenamiento en múltiples nodos
- Evaluar rendimiento, precisión y escalabilidad del sistema

# PC3 - Programación Concurrente

## Descripción

PC3 implementa un pipeline de procesamiento de datos y entrenamiento de modelo utilizando concurrencia en Go.

## Pipeline de PC3

### 1. Limpieza de datos
- Lectura del archivo household_power_consumption.txt
- Procesamiento concurrente mediante worker pool
- Validación y parseo de registros
- Eliminación de datos inválidos con conteo de causas

### 2. Feature engineering
- Transformación de variables temporales
- Generación de features:
  - hora
  - día de la semana
  - mes
  - indicador de fin de semana

### 3. Entrenamiento del modelo
- Regresión logística implementada desde cero
- Optimización mediante descenso de gradiente
- Entrenamiento paralelo usando goroutines

## Concurrencia en PC3

- Worker pool de 8 workers
- Uso de goroutines
- Comunicación mediante channels
- Sincronización con WaitGroup
- Reducción sin condiciones de carrera

## Resultados PC3

- Registros totales: 2,075,259
- Registros válidos: 2,049,280
- Accuracy: 0.8888
- Precision: 0.8588
- Recall: 0.6647
- F1-score: 0.7493

## Benchmark de concurrencia

Workers | Speedup | Eficiencia
--------|---------|-----------
1       | 1.00    | 1.00
2       | 1.65    | 0.83
4       | 2.57    | 0.64
8       | 3.05    | 0.38

# PC4 - Sistema Distribuido

## Descripción

PC4 extiende el sistema a una arquitectura distribuida basada en múltiples nodos de entrenamiento.

## Arquitectura del sistema

- 4 nodos de entrenamiento ML
- API central coordinadora
- Redis para cache de predicciones
- MongoDB para persistencia de datos
- Docker Compose para orquestación

## Flujo del sistema distribuido

1. Preparación de shards del dataset
2. Levantamiento de cluster de 4 nodos
3. Entrenamiento distribuido del modelo
4. Evaluación centralizada
5. Predicción con cache Redis
6. Persistencia en MongoDB
7. Simulación de fallo de nodo y recuperación

## Tolerancia a fallos

El sistema soporta:

- Caída de nodos individuales
- Degradación controlada del servicio
- Recuperación automática del cluster
- Preservación del modelo entrenado

## Resultados PC4

- Nodos activos: 4/4
- Dataset total: 1,639,424 muestras
- Accuracy final: 0.8888
- F1-score final: 0.7493
- Cache Redis validado
- Persistencia en MongoDB verificada

# Concurrencia y diseño del sistema

## PC3
- Worker pools
- Goroutines
- Channels
- Reducción sin race conditions

## PC4
- Coordinación distribuida
- Locks de sincronización
- API central

# Ejecución

PC3:
.\scripts\pc3