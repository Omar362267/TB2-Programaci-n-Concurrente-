package distributed

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
)

func ReadShardCSV(path string) ([]features.Sample, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("leyendo cabecera del shard: %w", err)
	}
	if len(header) < 2 || header[len(header)-1] != "high_demand" {
		return nil, nil, fmt.Errorf("shard invalido: se esperaba columna final high_demand")
	}
	names := append([]string(nil), header[:len(header)-1]...)
	samples := make([]features.Sample, 0)
	row := 1
	for {
		row++
		values, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("leyendo fila %d: %w", row, err)
		}
		if len(values) != len(header) {
			return nil, nil, fmt.Errorf("fila %d tiene %d columnas; se esperaban %d", row, len(values), len(header))
		}
		x := make([]float64, len(names))
		for j := range names {
			x[j], err = strconv.ParseFloat(strings.TrimSpace(values[j]), 64)
			if err != nil {
				return nil, nil, fmt.Errorf("fila %d feature %s: %w", row, names[j], err)
			}
		}
		label, err := strconv.Atoi(strings.TrimSpace(values[len(values)-1]))
		if err != nil || (label != 0 && label != 1) {
			return nil, nil, fmt.Errorf("fila %d high_demand invalido", row)
		}
		samples = append(samples, features.Sample{X: x, Y: label})
	}
	if len(samples) == 0 {
		return nil, nil, fmt.Errorf("shard sin muestras: %s", path)
	}
	return samples, names, nil
}
