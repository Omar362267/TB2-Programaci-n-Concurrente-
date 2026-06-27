# Benchmark de concurrencia PC3 con medianas

Repeticiones por configuración: 3
Workers evaluados: 1,2,4,8
Iteraciones por ejecución: 300

| Workers | Repeticiones | Tiempo total mediano (ms) | Registros/s mediano | Speedup | Eficiencia |
|---:|---:|---:|---:|---:|---:|
| 1 | 3 | 48623 | 42,679.83 | 1.00 | 1.00 |
| 2 | 3 | 28646 | 72,443.22 | 1.70 | 0.85 |
| 4 | 3 | 21039 | 98,635.59 | 2.31 | 0.58 |
| 8 | 3 | 16554 | 125,358.52 | 2.94 | 0.37 |

La línea base para speedup y eficiencia es 1 worker.
Las métricas de calidad y la loss final fueron consistentes en todas las configuraciones evaluadas.
La eficiencia puede disminuir al aumentar workers por costos de coordinación, planificación, memoria, CPU y E/S.
