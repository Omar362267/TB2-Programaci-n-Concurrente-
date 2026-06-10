# RESULTADOS DEL ANÁLISIS Y LIMPIEZA DE DATOS
## Individual Household Electric Power Consumption

**Fecha de ejecución**: 1 de junio de 2026  
**Script**: `analisis_limpieza.py`  
**Dataset original**: `household_power_consumption.txt`

---

## RESUMEN EJECUTIVO

El análisis y limpieza del dataset se completó **exitosamente**. Se procesaron **2,075,259 registros** del consumo eléctrico doméstico, generando un dataset limpio y enriquecido con **17 variables** (9 originales + 8 derivadas) listo para modelado predictivo.

### Estadísticas clave:

| Métrica | Valor |
|---------|-------|
| **Registros originales** | 2,075,259 |
| **Registros limpios** | 2,049,280 |
| **Registros eliminados** | 25,979 (1.25%) |
| **Período de datos** | 16/12/2006 - 26/11/2010 (1,441 días) |
| **Variables originales** | 9 |
| **Variables finales** | 17 |
| **Consumo promedio** | 1.092 kW |
| **Consumo máximo** | 11.122 kW |
| **Umbral de alta demanda** | 1.528 kW (percentil 75) |

---

## 1. ANÁLISIS DE LIMPIEZA

### 1.1 Valores faltantes

Se detectaron **25,979 registros (1.25%)** con valores faltantes, distribuidos equitativamente en:
- Global_active_power: 25,979 valores (1.25%)
- Global_reactive_power: 25,979 valores (1.25%)
- Voltage: 25,979 valores (1.25%)
- Global_intensity: 25,979 valores (1.25%)
- Sub_metering_1: 25,979 valores (1.25%)
- Sub_metering_2: 25,979 valores (1.25%)
- Sub_metering_3: 25,979 valores (1.25%)

**Decisión**: Eliminar registros completos con valores faltantes (estrategia conservadora)

### 1.2 Validación de tipos de datos

✓ **Todas las variables eléctricas convertidas exitosamente a float64**
- Global_active_power (kW)
- Global_reactive_power (kVar)
- Voltage (V)
- Global_intensity (A)
- Sub_metering_1-3 (Wh)

### 1.3 Consolidación temporal

✓ **Date** y **Time** unificados en índice `datetime`
- Formato: YYYY-MM-DD HH:MM:SS
- Frecuencia: 1 minuto
- Registros ordenados cronológicamente

---

## 2. ESTADÍSTICAS DESCRIPTIVAS

### 2.1 Variables eléctricas

```
                      Potencia      Potencia      Voltaje  Intensidad   Cocina  Lavandería  Cal/AC
                      Activa (kW)   Reactiva     (V)       (A)          (Wh)    (Wh)        (Wh)
Media                 1.092         0.124        240.84    4.628        1.122   1.299       6.458
Desv. Estándar        1.057         0.113        3.24      4.444        6.153   5.822       8.437
Mínimo                0.076         0.000        223.20     0.200        0.000   0.000       0.000
Q1 (25%)              0.308         0.048        238.99     1.400        0.000   0.000       0.000
Mediana               0.602         0.100        241.01     2.600        0.000   0.000       1.000
Q3 (75%)              1.528         0.194        242.89     6.400        0.000   1.000      17.000
Máximo                11.122        1.390        254.15    48.400       88.000  80.000      31.000
```

### 2.2 Observaciones importantes

- **Potencia activa global**: Altamente variable (std = 1.057 kW, similar a la media)
- **Voltaje**: Relativamente estable (240.84 ± 3.24 V, rango normal europeo)
- **Intensidad**: Proporcional a la potencia (correlación: 0.999)
- **Submedidores**: Predominio de Sub_metering_3 (calentador/AC) con media 6.458 Wh

---

## 3. ANÁLISIS TEMPORAL

### 3.1 Perfil de consumo por hora del día

**Hora pico**: 20:00 (8:00 PM) con **1.899 kW**  
**Hora valle**: 04:00 (4:00 AM) con **0.444 kW**  
**Variación**: 4.28x entre pico y valle

#### Patrones identificados:

| Período | Características |
|---------|-----------------|
| **00:00 - 05:59** | Consumo bajo (0.44-0.66 kW), con mínimo a las 04:00 |
| **06:00 - 08:59** | Aumento rápido (0.79-1.50 kW), actividad matutina |
| **09:00 - 17:59** | Meseta moderada (0.95-1.33 kW), consumo diurno |
| **18:00 - 22:00** | Picos significativos (1.33-1.90 kW), actividad vespertina/noche |
| **23:00 - 23:59** | Descenso hacia noche (0.90 kW) |

### 3.2 Consumo por día de la semana

| Día | Consumo promedio | Desv. Estándar |
|-----|------------------|-----------------|
| Lunes | 1.000 kW | 0.956 |
| Martes | 1.070 kW | 1.019 |
| Miércoles | 1.083 kW | 1.027 |
| Jueves | 0.982 kW | 0.959 |
| Viernes | 1.043 kW | 0.971 |
| **Sábado** | **1.248 kW** | 1.191 |
| **Domingo** | **1.220 kW** | 1.219 |

**Conclusión**: Fin de semana presenta **19% más consumo** que días laborales (1.23 vs 1.03 kW)

### 3.3 Estacionalidad - Consumo por mes

| Mes | Consumo promedio | Comportamiento |
|-----|------------------|-----------------|
| **Enero** | **1.462 kW** | Máximo (invierno) |
| Febrero | 1.300 kW | Alto |
| Marzo | 1.231 kW | Transición |
| Abril | 1.047 kW | Primavera |
| Mayo | 1.030 kW | Bajo |
| Junio | 0.909 kW | Bajo |
| **Julio** | **0.700 kW** | Mínimo (verano) |
| **Agosto** | **0.573 kW** | Mínimo absoluto |
| Septiembre | 0.976 kW | Recuperación |
| Octubre | 1.137 kW | Otoño |
| Noviembre | 1.292 kW | Invierno |
| Diciembre | 1.490 kW | Alto (invierno) |

**Patrón identificado**: 
- **Invierno** (Dic-Feb): Promedio 1.42 kW (calefacción)
- **Verano** (Jun-Ago): Promedio 0.73 kW (menor demanda)
- **Diferencia**: Invierno consume **1.95x** más que verano

### 3.4 Comparación Weekday vs Weekend

| Estadístico | Laborales | Fin de Semana |
|------------|-----------|---|
| Media | 1.035 kW | **1.234 kW** ↑19% |
| Mediana | 0.558 kW | 0.736 kW |
| Q1 | 0.304 kW | 0.322 kW |
| Q3 | 1.478 kW | 1.706 kW |
| Registros | 1,470,428 | 578,852 |

---

## 4. ANÁLISIS DE SUBMEDIDORES

### 4.1 Proporción de consumo

| Circuito | Consumo promedio | Proporción |
|----------|------------------|-----------|
| Cocina (Sub_1) | 1.12 W | **6.2%** |
| Lavandería (Sub_2) | 1.30 W | **7.1%** |
| Calentador/AC (Sub_3) | 6.46 W | **35.5%** |
| **Otros equipos** | **9.31 W** | **51.2%** |
| **TOTAL** | **18.19 W** | **100%** |

### 4.2 Análisis por circuito

#### Submedidor 1 - Cocina (6.2%)
- Media: 1.12 W
- Máximo: 88.0 W
- Variabilidad: Alta (horarios de comidas)
- Comportamiento: Picos en desayuno, comida y cena

#### Submedidor 2 - Lavandería (7.1%)
- Media: 1.30 W
- Máximo: 80.0 W
- Variabilidad: Moderada-Alta
- Comportamiento: Uso concentrado en ciertos días/horarios

#### Submedidor 3 - Calentador/AC (35.5%)
- Media: 6.46 W
- Máximo: 31.0 W
- Variabilidad: Alta con estacionalidad clara
- Comportamiento: Aumenta en invierno (calefacción), bajo en verano

#### Otros equipos (51.2%) - Lo más significativo
- Media: 9.31 W
- Máximo: 124.8 W
- **Este es el mayor consumidor**, representando más de la mitad del total
- Incluye: iluminación, electrónica, electrodomésticos en standby, etc.

---

## 5. ANÁLISIS DE CORRELACIÓN

### 5.1 Matriz de correlación completa

```
                       Potencia_Activa  Potencia_Reactiva  Voltaje  Intensidad  Cocina  Lavandería  Cal/AC
Potencia_Activa                 1.000                0.247   -0.400       0.999   0.484       0.435   0.639
Potencia_Reactiva               0.247                1.000   -0.112       0.266   0.123       0.139   0.090
Voltaje                        -0.400               -0.112    1.000      -0.411  -0.196      -0.167  -0.268
Intensidad                      0.999                0.266   -0.411       1.000   0.489       0.440   0.627
Cocina                          0.484                0.123   -0.196       0.489   1.000       0.055   0.103
Lavandería                      0.435                0.139   -0.167       0.440   0.055       1.000   0.081
Calentador/AC                   0.639                0.090   -0.268       0.627   0.103       0.081   1.000
```

### 5.2 Interpretación de correlaciones clave

| Pares | Correlación | Significado |
|-------|-------------|------------|
| **Potencia Activa ↔ Intensidad** | **0.999** | Perfecta (ley de Ohm): P = V × I |
| **Potencia Activa ↔ Cal/AC** | **0.639** | Fuerte: Calentador/AC es principal consumidor |
| **Potencia Activa ↔ Voltaje** | **-0.400** | Moderada negativa: Voltaje cae con alto consumo |
| Potencia Activa ↔ Cocina | 0.484 | Moderada: Consumo ligado a comidas |
| Potencia Activa ↔ Lavandería | 0.435 | Moderada: Consumo menos predecible |
| Voltaje ↔ Intensidad | -0.411 | Moderada negativa: Compensación en red |

### 5.3 Multicolinealidad

⚠️ **Alerta**: Correlación 0.999 entre Potencia Activa e Intensidad indica:
- **Implicación**: No incluir ambas variables en modelos de ML (redundancia)
- **Recomendación**: Usar Potencia Activa como target, Intensidad como predictor alternativo
- **Causa**: Relación determinística (P = V × I)

---

## 6. VARIABLE OBJETIVO - ALTA DEMANDA

### 6.1 Distribución de clases

```
Baja demanda (< 1.528 kW):  1,535,854 registros (74.95%)
Alta demanda (≥ 1.528 kW):    513,426 registros (25.05%)
```

**Balance**: Desbalance moderado (3:1) - Manejable para clasificación

### 6.2 Características de alta demanda

**Hora pico para alta demanda**: 20:00-21:00  
**Día pico**: Sábado/Domingo  
**Mes pico**: Enero, Diciembre (invierno)

---

## 7. VARIABLES DERIVADAS CREADAS

### 7.1 Componentes temporales

- **hour**: Hora del día (0-23)
- **day_of_week**: Día de la semana (0=Lunes, 6=Domingo)
- **month**: Mes del año (1-12)
- **day_of_month**: Día del mes (1-31)
- **day_name**: Nombre del día (string)
- **month_name**: Nombre del mes (string)

### 7.2 Indicadores binarios

- **is_weekend**: 1 si sábado/domingo, 0 si weekday
  - Utilidad: Clasificar patrones de fin de semana

### 7.3 Variables agregadas

- **total_sub_metering**: Suma de los 3 submedidores
  - Media: 8.88 W
  - Rango: 0 - 134 W

- **other_consumption**: Consumo no medido
  - Fórmula: (Global_active_power × 1000 / 60) - total_sub_metering
  - Media: 9.31 W
  - Rango: 0 - 124.8 W
  - **Proporción**: 51.2% del consumo total

### 7.4 Variable objetivo

- **high_demand**: Clasificación binaria
  - 0: Baja demanda (< 1.528 kW)
  - 1: Alta demanda (≥ 1.528 kW)
  - Umbral: Percentil 75 de Global_active_power

---

## 8. VALIDACIÓN DE RANGOS

✓ **Todos los rangos son válidos**:

| Variable | Mín | Máx | Rango válido | Estatus |
|----------|-----|-----|--------------|---------|
| Potencia Activa | 0.076 kW | 11.122 kW | Típico doméstico | ✓ |
| Voltaje | 223.2 V | 254.2 V | Normal EU (230V ±10%) | ✓ |
| Intensidad | 0.2 A | 48.4 A | Razonable (típico 10-30A) | ✓ |
| Cocina | 0 W | 88 W | Normal circuito | ✓ |
| Lavandería | 0 W | 80 W | Normal circuito | ✓ |
| Calentador/AC | 0 W | 31 W | Submuestreado (típicamente mayor) | ⚠️ |

**Nota**: Sub_metering_3 usa aparentemente submuestreo (valores < 1 Wh entre muestras de 1 minuto)

---

## 9. ANOMALÍAS DETECTADAS

### 9.1 Valores cero en submedidores

| Circuito | Período con cero |
|----------|-----------------|
| Cocina | ~87% del tiempo (bajo uso base) |
| Lavandería | ~93% del tiempo (uso puntual) |
| Calentador/AC | ~27% del tiempo (más constante) |

**Implicación**: Dispositivos con uso discontinuo - normal en hogares

### 9.2 Outliers en potencia activa

- **Máximo registrado**: 11.122 kW (octubre, probablemente pico sincrónico)
- **Mínimo registrado**: 0.076 kW (consumo en standby)
- **Ratio**: 146x - Variabilidad extrema pero esperada

---

## 10. VISUALIZACIONES GENERADAS

Se generaron **5 gráficos** principales:

### 10.1 Análisis temporal
**Archivo**: `01_analisis_temporal.png`
- Histograma de distribución de consumo global
- Perfil de consumo por hora del día
- Consumo promedio por día de la semana
- Estacionalidad mensual

### 10.2 Análisis de submedidores
**Archivo**: `02_analisis_submedidores.png`
- Gráfico de pastel (proporción de consumo)
- Gráfico de barras (consumo en vatios)

### 10.3 Matriz de correlación
**Archivo**: `03_matriz_correlacion.png`
- Heatmap con valores numéricos
- Codificación de colores: azul (negativa) a rojo (positiva)

### 10.4 Heatmap temporal
**Archivo**: `04_heatmap_hora_dia.png`
- Consumo por hora del día (columnas)
- Consumo por día de la semana (filas)
- Identifica patrones de interacción

### 10.5 Variabilidad por hora
**Archivo**: `05_boxplot_horas.png`
- Distribución completa del consumo para cada hora
- Identifica horas con mayor variabilidad

---

## 11. ARCHIVOS GENERADOS

### 11.1 Datos

- **household_power_consumption_clean.csv** (1.1 GB)
  - Formato: CSV estándar
  - Separador: `,`
  - Índice: datetime
  - Registros: 2,049,280
  - Columnas: 17

- **household_power_consumption_clean.pkl** (320 MB)
  - Formato: Pickle (binario)
  - Ventaja: Carga más rápida en Python
  - Preserva tipos de datos y índice datetime

### 11.2 Metadatos

- **resumen_limpieza.json**
  - Contiene:
    - Registros originales/finales
    - Porcentaje eliminado
    - Variables creadas
    - Período temporal
    - Umbral de alta demanda

### 11.3 Gráficos

Directorio: `graficos/`
- `01_analisis_temporal.png`
- `02_analisis_submedidores.png`
- `03_matriz_correlacion.png`
- `04_heatmap_hora_dia.png`
- `05_boxplot_horas.png`

---

## 12. RECOMENDACIONES PARA MODELADO

### 12.1 Variables a usar como predictores

**Prioritarias**:
1. `hour` - Fuerte predictor temporal
2. `is_weekend` - Diferencia clara en patrones
3. `month` - Estacionalidad significativa
4. `Global_active_power` (lag 1-10) - Autocorrelación temporal
5. `Voltage` - Correlación con demanda

**Secundarias**:
6. `day_of_week` - Refinamiento de is_weekend
7. `Sub_metering_3` - Principal componente
8. `Global_intensity` - Correlacionada con potencia

**A evitar**:
- ❌ `Global_intensity` (colineal con potencia)
- ❌ `Global_reactive_power` (baja correlación)

### 12.2 Algoritmos recomendados

**Clasificación (high_demand)**:
1. **Gradient Boosting** (XGBoost, LightGBM)
   - Maneja bien desequilibrio de clases
   - Captura patrones temporales
   
2. **Redes Neuronales Recurrentes** (LSTM)
   - Para series temporales
   - Captura autocorrelación

3. **Random Forest**
   - Robustez ante outliers
   - Fácil interpretación

**Regresión (predicción de consumo)**:
1. **ARIMA/SARIMA** - Series temporales estándar
2. **Prophet** - Maneja estacionalidad
3. **Gradient Boosting** - Mejor precisión

### 12.3 Ingeniería de features adicional

Consideraciones:
- **Lags**: Incluir valores pasados (1h, 24h, 7d)
- **Media móvil**: Suavizado temporal
- **Estacionalidad**: Variables trigonométricas para ciclos
- **Interacciones**: hour × is_weekend, month × voltage
- **Normalization**: Escalar predictores para redes neuronales

### 12.4 Validación del modelo

- **Validación temporal**: Entrenamiento histórico → predicción futura
- **Métrica principal**: AUC-ROC para clasificación desequilibrada
- **Métrica secundaria**: F1-score (balance precision/recall)
- **Cross-validation**: Time-series split (no aleatorio)

---

## 13. INSIGHTS PRINCIPALES

### 13.1 Resumen de hallazgos

1. **Consumo altamente predecible**: Patrones claros por hora, día y mes
2. **Estacionalidad fuerte**: Consumo invierno ~2x verano
3. **Picos vespertinos**: Máximo a las 20:00 (hogar ocupado, cena, entretenimiento)
4. **Fin de semana diferente**: 19% más consumo que laborales
5. **Calentador/AC dominante**: 35.5% del consumo medido (probablemente submuestra)
6. **Consumo misterioso**: 51.2% no explicado por 3 submedidores principales
7. **Datos de buena calidad**: Solo 1.25% valores faltantes, sin outliers extremos

### 13.2 Oportunidades de ahorro

- **Pico vespertino (20:00)**: 1.9 kW - Posible desplazamiento a horas valle
- **Calentador/AC**: Mayor consumidor - Optimizar temperatura/programación
- **Equipos en standby**: 51% del consumo - Apagar dispositivos innecesarios
- **Fin de semana**: 19% adicional - Cambio de hábitos

---

## 14. PRÓXIMOS PASOS

1. **Modelado predictivo**
   - Entrenar clasificadores de alta demanda
   - Evaluar diferentes algoritmos
   - Optimizar hiperparámetros

2. **Análisis de series temporales**
   - Descomposición STL (Seasonal-Trend)
   - Detección de cambios estructurales
   - Forecasting a corto/largo plazo

3. **Paralelización**
   - Implementar procesamiento concurrente
   - Medir speedup vs secuencial
   - Optimizar para datasets mayores

4. **Generación de reportes**
   - Dashboard interactivo
   - Recomendaciones personalizadas
   - Alertas de anomalías

---

**Documento generado**: 1 de junio de 2026  
**Tiempo de procesamiento**: ~2-3 minutos  
**Registros procesados**: 2,075,259 → 2,049,280  
**Estado**: ✓ COMPLETADO EXITOSAMENTE
