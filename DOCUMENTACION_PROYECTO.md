# Análisis y Predicción de Alta Demanda Eléctrica Doméstica
## Sistema Concurrente y Paralelo para Procesamiento de Datos Energéticos

---

## 2. DESCRIPCIÓN DEL PROBLEMA Y MOTIVACIÓN

### 2.1 Contexto del problema

El consumo eléctrico doméstico es un componente fundamental dentro de la sostenibilidad energética global. Un uso ineficiente de la energía en los hogares genera múltiples consecuencias negativas:

- **Costos económicos elevados**: Familias pagan facturas más altas sin necesidad
- **Presión sobre la red eléctrica**: Mayor demanda en horarios pico puede comprometer la estabilidad del sistema
- **Impacto ambiental**: Mayor consumo implica mayor generación de energía, frecuentemente a través de fuentes contaminantes

Por estas razones, identificar y entender los patrones de consumo energético es esencial para promover hábitos sostenibles y eficientes en los hogares.

### 2.2 Problema identificado

El problema consiste en **analizar grandes volúmenes de registros eléctricos domésticos para identificar patrones de alta demanda energética**. Específicamente:

- Identificar qué **horarios específicos** presentan mayor consumo
- Detectar qué **equipos o electrodomésticos** generan los picos de demanda
- Reconocer **comportamientos de consumo** poco eficientes que pueden ser modificados
- Encontrar correlaciones entre variables eléctricas y patrones de uso

Estos patrones son críticos para desarrollar estrategias de ahorro y optimización energética.

### 2.3 Motivación

La motivación de este proyecto es multifacética:

1. **Aplicabilidad práctica**: El análisis de consumo eléctrico permite apoyar decisiones orientadas a **eficiencia energética**, tanto a nivel individual (usuarios domésticos) como a nivel de gestión de redes (operadores eléctricos).

2. **Justificación técnica**: El dataset contiene **más de 2 millones de registros**, lo que hace impractical procesarlos secuencialmente. Esto justifica la aplicación de técnicas de **programación concurrente y paralela** para:
   - Acelerar la limpieza de datos
   - Paralelizar transformaciones de variables
   - Distribuir cálculos de características (features)
   - Optimizar modelos de Machine Learning

3. **Aspecto educativo**: Este proyecto integra conceptos avanzados de programación concurrente con aplicaciones reales del machine learning y ciencia de datos.

### 2.4 Relación con sostenibilidad

Detectar periodos de alta demanda es un primer paso hacia:

- **Promover hábitos de consumo eficientes**: Los usuarios pueden ajustar actividades para evitar horarios pico
- **Reducir desperdicio energético**: Identificar equipos ineficientes y su impacto en la factura
- **Sistemas de recomendación y alerta**: Implementar herramientas que adviertan a usuarios cuando están próximos a períodos de alto consumo
- **Sostenibilidad ambiental**: Menor consumo = menor presión ambiental y menor huella de carbono

---

## 3. OBJETIVOS

### 3.1 Objetivo general

Desarrollar un **sistema integral de análisis y predicción de alta demanda eléctrica doméstica** mediante:
- Limpieza y transformación de datos
- Análisis exploratorio de datos (EDA)
- Diseño e implementación de modelos de Machine Learning
- Paralelización del cálculo para optimizar rendimiento

Con el fin de **apoyar la sostenibilidad energética** y proporcionar insights accionables para usuarios y gestores de redes.

### 3.2 Objetivos específicos

1. **Limpieza y preparación de datos**:
   - Unificar variables de fecha y hora
   - Detectar y tratar valores faltantes
   - Validar rangos de variables eléctricas
   - Crear variables derivadas relevantes

2. **Análisis exploratorio**:
   - Caracterizar distribuciones de consumo
   - Identificar patrones temporales (por hora, día, mes)
   - Detectar anomalías y valores atípicos
   - Analizar correlaciones entre variables

3. **Modelado predictivo**:
   - Entrenar modelos de clasificación para alta/baja demanda
   - Optimizar hiperparámetros
   - Evaluar rendimiento con métricas apropiadas
   - Generar predicciones accionables

4. **Optimización con concurrencia**:
   - Implementar procesamiento paralelo en limpieza de datos
   - Paralelizar cálculo de características
   - Optimizar tiempo de ejecución del modelo
   - Analizar speedup y eficiencia de paralelización

---

## 4. DESCRIPCIÓN DEL DATASET

### 4.1 Fuente del dataset

**Dataset**: Individual Household Electric Power Consumption

**Fuente**: UCI Machine Learning Repository

**Cita académica**:
```
Hebrail, G., & Berard, A. (2006). Individual Household Electric Power Consumption [Dataset]. 
UCI Machine Learning Repository. https://doi.org/10.24432/C58K54
```

**Ubicación en proyecto**: `household_power_consumption.txt`

### 4.2 Descripción general

El dataset contiene **2,075,259 mediciones** de consumo eléctrico doméstico registradas con una **frecuencia de un minuto**, durante aproximadamente **47 meses**, entre **diciembre de 2006 y noviembre de 2010**.

Las mediciones provienen de un hogar individual en Francia e incluyen información detallada de consumo global y desglosado por circuitos específicos.

### 4.3 Variables del dataset

| Variable | Descripción | Tipo |
|----------|-------------|------|
| `Date` | Fecha de la medición (DD/MM/YYYY) | Fecha |
| `Time` | Hora de la medición (HH:MM:SS) | Tiempo |
| `Global_active_power` | Potencia activa global del hogar (kilowatios) | Continua |
| `Global_reactive_power` | Potencia reactiva global (kilovoltios-amperios reactivos) | Continua |
| `Voltage` | Voltaje promedio por minuto (voltios) | Continua |
| `Global_intensity` | Intensidad de corriente global (amperios) | Continua |
| `Sub_metering_1` | Consumo del circuito 1: cocina (vatios-hora) | Continua |
| `Sub_metering_2` | Consumo del circuito 2: lavandería (vatios-hora) | Continua |
| `Sub_metering_3` | Consumo del circuito 3: calentador de agua y aire acondicionado (vatios-hora) | Continua |

**Nota**: El dataset utiliza `;` como separador de campos.

### 4.4 Justificación de elección del dataset

Este dataset es óptimo para el proyecto por las siguientes razones:

1. **Relación directa con sostenibilidad energética**: Permite abordar un problema real y relevante para la reducción de emisiones y consumo eficiente

2. **Volumen de datos justifica paralelización**: Con más de 2 millones de registros, es impracticable procesarlos secuencialmente; esto motiva el uso de técnicas de programación concurrente

3. **Dataset real y citado académicamente**: Proviene de mediciones reales, ha sido utilizado en investigaciones académicas y tiene documentación clara

4. **Versatilidad de tareas de ML**: Permite abordar:
   - Regresión (predecir consumo futuro)
   - Clasificación (alta/baja demanda)
   - Clustering (identificar patrones de comportamiento)
   - Anomaly detection (detectar comportamientos inusuales)

5. **Variables temporales y eléctricas útiles**: Contiene dimensiones temporales (hora, día, mes) y múltiples variables eléctricas, facilitando ingeniería de features

6. **Datos reales con desafíos**: Incluye valores faltantes y ruido, requiriendo limpieza realista

---

## 5. LIMPIEZA Y ANÁLISIS DE DATOS

### 5.1 Problemas esperados en los datos

Basándose en la naturaleza del dataset y observaciones preliminares, se esperan los siguientes problemas:

| Problema | Descripción | Impacto |
|----------|-------------|---------|
| **Valores faltantes (?)** | Variables contienen "?" para valores no registrados | Invalidaría análisis y modelos si no se tratan |
| **Formato de separador** | Campos separados por `;` en lugar de `,` | Requiere especificar delimitador en lectura |
| **Tipos de datos incorrectos** | Variables numéricas pueden leerse como texto | Impide cálculos y análisis estadísticos |
| **Fecha y hora separadas** | Date y Time están en columnas distintas | Necesita unificación para análisis temporal |
| **Posibles registros inválidos** | Voltajes, intensidades o potencias fuera de rangos normales | Requiere validación y tratamiento |
| **Desajuste de decimales** | Variables numéricas pueden usar coma o punto como separador | Necesita normalización |
| **Orden temporal** | Registros pueden no estar completamente ordenados | Afecta análisis de series de tiempo |

### 5.2 Estrategia de limpieza propuesta

El procedimiento de limpieza seguirá estos pasos:

1. **Lectura de datos**:
   - Leer el archivo especificando `;` como separador
   - Manejar valores faltantes (?) en la lectura

2. **Unificación de fecha y hora**:
   - Crear columna `datetime` combinando `Date` y `Time`
   - Establecer esta columna como índice temporal

3. **Conversión de tipos de datos**:
   - Convertir variables eléctricas (`Global_active_power`, `Voltage`, etc.) a tipo numérico
   - Convertir `Global_intensity` a float
   - Manejar valores no convertibles

4. **Detección y tratamiento de valores faltantes**:
   - Contar registros con valores faltantes por variable
   - Evaluar porcentaje de datos faltantes
   - Decidir si imputar o eliminar registros incompletos

5. **Validación de rangos**:
   - Verificar que voltajes estén dentro de rangos normales (200-250V en muchos hogares europeos)
   - Validar que potencias e intensidades sean no-negativas
   - Identificar y registrar outliers extremos

6. **Ordenamiento temporal**:
   - Asegurar que datos están ordenados por fecha y hora
   - Detectar saltos o discontinuidades temporales

7. **Eliminación de columnas innecesarias**:
   - Remover `Date` y `Time` originales tras crear `datetime`

### 5.3 Variables derivadas propuestas

Se crearán las siguientes variables para enriquecer el dataset:

| Variable | Descripción | Fórmula/Método |
|----------|-------------|-----------------|
| `hour` | Hora del día (0-23) | Extraer de `datetime` |
| `day_of_week` | Día de la semana (0=Lunes, 6=Domingo) | Extraer de `datetime` |
| `month` | Mes del año (1-12) | Extraer de `datetime` |
| `is_weekend` | Indicador binario: 1 si sábado/domingo, 0 si weekday | `day_of_week >= 5` |
| `total_sub_metering` | Suma de los tres submedidores | `Sub_metering_1 + Sub_metering_2 + Sub_metering_3` |
| `other_consumption` | Consumo no medido por submedidores | Ver fórmula en sección 5.4 |
| `high_demand` | Etiqueta objetivo: 1 si consumo alto, 0 si bajo | Basado en percentil del consumo |
| `is_peak_hour` | Indicador: 1 si hora está dentro de horas pico | Basado en análisis de EDA |
| `consumption_trend` | Tendencia de consumo en ventana temporal | Media móvil |

### 5.4 Fórmula de consumo no medido

El consumo global no se explica completamente por los tres submedidores. Se propone calcular:

```
other_consumption = (Global_active_power * 1000 / 60) 
                    - Sub_metering_1 
                    - Sub_metering_2 
                    - Sub_metering_3
```

**Explicación**:
- `Global_active_power` está en kilowatios (kW)
- Se multiplica por 1000 para convertir a vatios (W)
- Se divide por 60 porque los registros son cada minuto (vatios-minuto ÷ 60 segundos → vatios-segundo)
- Se restan los tres submedidores (en vatios-hora) para obtener consumo residual
- **Interpretación**: Representa el consumo de equipos como iluminación, electrodomésticos menores, equipos en standby, etc., que no están en los tres circuitos principales

**Validación esperada**: 
- Si `other_consumption` es siempre negativa, puede indicar inconsistencias en mediciones
- Valores razonables: generalmente entre 0 y 2000 W

### 5.5 Análisis exploratorio propuesto

Aunque los resultados aún se generarán durante la ejecución, el análisis exploratorio incluirá:

#### 5.5.1 Análisis univariado

- **Distribución de consumo eléctrico**:
  - Histogramas de `Global_active_power` por hora del día
  - Densidad de probabilidad de consumo total
  - Estadísticos descriptivos (media, mediana, desviación estándar, cuartiles)

- **Análisis de voltaje e intensidad**:
  - Rango de variación del voltaje
  - Distribución de intensidad
  - Identificación de anomalías

#### 5.5.2 Análisis temporal

- **Consumo promedio por hora del día**:
  - Identificar horas pico y horas valle
  - Visualizar perfil diario típico

- **Consumo promedio por día de la semana**:
  - Comparar patrones entre weekdays y fin de semana
  - Identificar diferencias significativas

- **Consumo promedio por mes**:
  - Detectar estacionalidad (invierno vs verano)
  - Analizar variabilidad mensual

#### 5.5.3 Análisis multivariado

- **Relación entre submedidores y consumo total**:
  - Correlación entre `Sub_metering_1/2/3` y `Global_active_power`
  - Proporción de consumo por circuito

- **Correlación entre variables eléctricas**:
  - Matriz de correlación: potencia, voltaje, intensidad
  - Detectar multicolinealidad

#### 5.5.4 Análisis de calidad

- **Cantidad y porcentaje de valores faltantes**:
  - Por variable
  - Por período temporal
  - Patrón de ausencias

- **Detección de anomalías**:
  - Valores atípicos (IQR, Z-score)
  - Picos anormales de consumo
  - Registros que se desvían significativamente del patrón

#### 5.5.5 Preparación de variable objetivo

- **Definición de "alta demanda"**:
  - Usar percentil 75 o 90 de consumo como umbral
  - Crear variable binaria `high_demand` para clasificación

---

## 6. PRÓXIMOS PASOS

1. **Implementación de limpieza**: Desarrollar scripts de preprocesamiento
2. **EDA y visualizaciones**: Generar gráficos y tablas analíticas
3. **Ingeniería de features**: Aplicar transformaciones propuestas
4. **Modelado**: Entrenar modelos de clasificación
5. **Paralelización**: Optimizar con técnicas de concurrencia
6. **Evaluación**: Medir rendimiento y speedup

---

**Documento generado**: 1 de junio de 2026
