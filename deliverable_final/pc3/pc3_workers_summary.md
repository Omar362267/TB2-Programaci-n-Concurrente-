# Benchmark de concurrencia PC3 con medianas

Repeticiones por configuración: 3
Workers evaluados: 1,2,4,8
Iteraciones por ejecución: 300

| Workers | Repeticiones | Tiempo total mediano (ms) | Registros/s mediano | Speedup | Eficiencia |
|---:|---:|---:|---:|---:|---:|
| 1 | 3 | 44573 | 46,557.89 | 1.00 | 1.00 |
| 2 | 3 | 26747 | 77,587.18 | 1.67 | 0.83 |
| 4 | 3 | 17826 | 116,415.38 | 2.50 | 0.63 |
| 8 | 3 | 15018 | 138,177.47 | 2.97 | 0.37 |

La línea base para speedup y eficiencia es 1 worker.
Las métricas de calidad y la loss final fueron consistentes en todas las configuraciones evaluadas.
La eficiencia puede disminuir al aumentar workers por costos de coordinación, planificación, memoria, CPU y E/S.
