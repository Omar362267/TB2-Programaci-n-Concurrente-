# Dataset

El dataset debe ubicarse en:

```txt
data/raw/household_power_consumption.txt
```

Columnas esperadas:

```txt
Date;Time;Global_active_power;Global_reactive_power;Voltage;Global_intensity;Sub_metering_1;Sub_metering_2;Sub_metering_3
```

El proyecto genera el dataset procesado en:

```txt
data/processed/features_high_demand.csv
```

Ese archivo contiene las features normalizadas que consume el modelo:

- hour
- day_of_week
- month
- is_weekend
- voltage
- global_reactive_power
- global_intensity
- sub_metering_1
- sub_metering_2
- sub_metering_3
- other_consumption
- high_demand

## Politica de descarte

Se descartan filas cuando no son confiables para analisis o entrenamiento. Las razones quedan registradas en `results/resumen_limpieza.json`:

- `empty_line`: fila vacia.
- `missing_value`: presencia de `?`, que representa un dato faltante en el dataset.
- `invalid_column_count`: cantidad de columnas distinta a la esperada.
- `invalid_datetime`: fecha u hora no parseable.
- `invalid_number`: valor numerico invalido.
- `invalid_range`: valor fuera de rango fisicamente valido, como voltaje menor o igual a cero o consumos negativos.

Para PC3 se usa una estrategia conservadora: se descartan filas incompletas en lugar de imputarlas, para evitar entrenar el modelo con valores artificiales.
