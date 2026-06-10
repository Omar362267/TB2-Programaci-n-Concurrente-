# Dataset

Esta carpeta contiene las instrucciones para ubicar el dataset usado en el proyecto.

## Dataset seleccionado

**Individual Household Electric Power Consumption**

El dataset contiene mediciones de consumo eléctrico doméstico tomadas por minuto durante varios años. Se utiliza porque supera el millón de registros y permite analizar patrones de demanda energética con impacto social relacionado con eficiencia, ahorro y sostenibilidad.

## Archivo requerido

El archivo original debe colocarse en:

```txt
data/raw/household_power_consumption.txt
```

No se recomienda subir el dataset completo al repositorio porque pesa más de 100 MB. Puede gestionarse mediante Git LFS o descargarse manualmente desde la fuente original usada por el equipo.

## Separador y columnas esperadas

El archivo original usa separador `;` y contiene las columnas:

```txt
Date
Time
Global_active_power
Global_reactive_power
Voltage
Global_intensity
Sub_metering_1
Sub_metering_2
Sub_metering_3
```

## Valores faltantes

Los valores faltantes están representados con `?`. Para PC3 se propone una estrategia conservadora: descartar registros con valores faltantes en variables eléctricas principales.

## Variable objetivo propuesta

```txt
high_demand = 1 si Global_active_power >= percentil 75
high_demand = 0 en caso contrario
```

## Nota importante sobre el ZIP anterior

El archivo `household_power_consumption_clean.csv` del ZIP original parece ser un puntero de Git LFS, no el dataset real. Por ello, la entrega final debe verificar que el dataset completo esté disponible localmente o documentar claramente cómo obtenerlo.
