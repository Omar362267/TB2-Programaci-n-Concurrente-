# Estructura recomendada para la entrega PC3

El enunciado no impone una estructura exacta de carpetas para el repositorio, pero sí exige que el proyecto esté en GitHub, siga Git Flow, sea desarrollado en Go y esté orientado a programación concurrente/distribuida. Por eso, se propone una estructura estándar de proyecto Go, separando entrada, lógica interna, datos, resultados y documentación.

## Estructura propuesta

```txt
1ACC0065_PC3_U202222148/
├── README.md
├── go.mod
├── .gitignore
├── .gitattributes
├── cmd/
│   └── pc3/
│       └── main.go
├── internal/
│   ├── loader/
│   ├── preprocessing/
│   ├── features/
│   ├── ml/
│   └── metrics/
├── data/
│   ├── README.md
│   ├── raw/
│   └── processed/
├── results/
├── docs/
│   ├── 1ACC0065_PC3_202610_Informe_U202222148.md
│   ├── REPORTE_PARTICIPACION.md
│   └── legacy/
└── docker/
    └── Dockerfile
```

## Justificación

- `cmd/pc3`: contiene el punto de entrada ejecutable.
- `internal/loader`: concentra la carga concurrente del dataset.
- `internal/preprocessing`: concentra limpieza, conversión de tipos y validación.
- `internal/features`: concentra la creación de variables derivadas.
- `internal/ml`: contiene el modelo inicial y su entrenamiento paralelo.
- `internal/metrics`: calcula métricas del modelo y métricas de rendimiento.
- `data`: documenta el dataset sin obligar a subir archivos grandes.
- `results`: guarda evidencias generadas por el programa.
- `docs`: guarda informe, reporte de participación y documentos heredados.
- `docker`: prepara la transición hacia contenedores para PC4/TB2.

## Nombres de entrega

ZIP o RAR:

```txt
1ACC0065_PC3_U202222148.zip
```

Informe:

```txt
1ACC0065_PC3_202610_Informe_U202222148.md
```

Si cada integrante debe entregar individualmente, se cambia únicamente el código de alumno en el nombre del ZIP y del informe.
