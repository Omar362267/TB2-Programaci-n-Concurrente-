package analysis

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/preprocessing"
)

var NumericNames = []string{
	"global_active_power", "global_reactive_power", "voltage", "global_intensity",
	"sub_metering_1", "sub_metering_2", "sub_metering_3", "other_consumption",
}

type VariableStats struct {
	Name   string  `json:"name"`
	Count  int     `json:"count"`
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	P25    float64 `json:"p25"`
	Median float64 `json:"median"`
	P75    float64 `json:"p75"`
	Max    float64 `json:"max"`
}

type GroupStat struct {
	Group                    string  `json:"group"`
	Count                    int     `json:"count"`
	AverageGlobalActivePower float64 `json:"average_global_active_power"`
	HighDemandCount          int     `json:"high_demand_count"`
	HighDemandRate           float64 `json:"high_demand_rate"`
}

type DistributionBin struct {
	From  float64 `json:"from"`
	To    float64 `json:"to"`
	Count int     `json:"count"`
}

type HighDemandDistribution struct {
	ThresholdP75    float64 `json:"threshold_p75"`
	NormalCount     int     `json:"normal_count"`
	HighDemandCount int     `json:"high_demand_count"`
	NormalRate      float64 `json:"normal_rate"`
	HighDemandRate  float64 `json:"high_demand_rate"`
	Definition      string  `json:"definition"`
}

type DatasetAnalysis struct {
	TotalRecords         int                    `json:"total_records"`
	VariableStats        []VariableStats        `json:"variable_stats"`
	Hourly               []GroupStat            `json:"hourly"`
	Weekday              []GroupStat            `json:"weekday"`
	Monthly              []GroupStat            `json:"monthly"`
	Histogram            []DistributionBin      `json:"histogram_global_active_power"`
	Correlation          [][]float64            `json:"correlation"`
	CorrelationVariables []string               `json:"correlation_variables"`
	HighDemand           HighDemandDistribution `json:"high_demand"`
	Summary              []string               `json:"summary"`
}

type accumulator struct {
	count int
	sum   float64
	high  int
}

func AnalyzeRecords(records []preprocessing.PowerRecord) DatasetAnalysis {
	threshold := percentile(records, 0.75, func(r preprocessing.PowerRecord) float64 { return r.GlobalActivePower })
	vars := make([][]float64, len(NumericNames))
	for i := range vars {
		vars[i] = make([]float64, 0, len(records))
	}
	hourly := make(map[int]*accumulator)
	weekday := make(map[int]*accumulator)
	monthly := make(map[int]*accumulator)
	high := 0
	for _, r := range records {
		vals := rawValues(r)
		for i, v := range vals {
			vars[i] = append(vars[i], v)
		}
		y := 0
		if r.GlobalActivePower >= threshold {
			y = 1
			high++
		}
		addAccum(hourly, r.Timestamp.Hour(), r.GlobalActivePower, y)
		addAccum(weekday, int(r.Timestamp.Weekday()), r.GlobalActivePower, y)
		addAccum(monthly, int(r.Timestamp.Month()), r.GlobalActivePower, y)
	}
	stats := make([]VariableStats, 0, len(NumericNames))
	for i, name := range NumericNames {
		stats = append(stats, Stats(name, vars[i]))
	}
	corrVars := []string{"global_active_power", "global_reactive_power", "voltage", "global_intensity", "sub_metering_1", "sub_metering_2", "sub_metering_3", "other_consumption"}
	corr := CorrelationMatrix(vars)
	normal := len(records) - high
	analysis := DatasetAnalysis{
		TotalRecords:         len(records),
		VariableStats:        stats,
		Hourly:               groupStatsFromMap(hourly, hourLabels),
		Weekday:              groupStatsFromMap(weekday, weekdayLabels),
		Monthly:              groupStatsFromMap(monthly, monthLabels),
		Histogram:            Histogram(vars[0], 20),
		Correlation:          corr,
		CorrelationVariables: corrVars,
		HighDemand:           HighDemandDistribution{ThresholdP75: threshold, NormalCount: normal, HighDemandCount: high, NormalRate: safeDiv(float64(normal), float64(len(records))), HighDemandRate: safeDiv(float64(high), float64(len(records))), Definition: "high_demand = 1 si Global_active_power >= percentil 75"},
	}
	analysis.Summary = []string{
		fmt.Sprintf("Se analizaron %d registros limpios del dataset.", len(records)),
		fmt.Sprintf("El umbral de alta demanda corresponde al percentil 75: %.6f kW.", threshold),
		fmt.Sprintf("La clase high_demand representa %.2f%% de los registros validos.", analysis.HighDemand.HighDemandRate*100),
		"Las tablas y graficos fueron generados por modulos Go del proyecto para mantener trazabilidad con la restriccion del curso.",
	}
	return analysis
}

func SaveAnalysis(outDir string, a DatasetAnalysis) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "estadisticas_descriptivas.json"), a.VariableStats); err != nil {
		return err
	}
	if err := writeStatsCSV(filepath.Join(outDir, "estadisticas_descriptivas.csv"), a.VariableStats); err != nil {
		return err
	}
	if err := writeGroupCSV(filepath.Join(outDir, "consumo_por_hora.csv"), a.Hourly); err != nil {
		return err
	}
	if err := writeGroupCSV(filepath.Join(outDir, "consumo_por_dia_semana.csv"), a.Weekday); err != nil {
		return err
	}
	if err := writeGroupCSV(filepath.Join(outDir, "consumo_por_mes.csv"), a.Monthly); err != nil {
		return err
	}
	if err := writeCorrelationCSV(filepath.Join(outDir, "matriz_correlacion.csv"), a.CorrelationVariables, a.Correlation); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "distribucion_high_demand.json"), a.HighDemand); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "histograma_global_active_power.json"), a.Histogram); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "analisis_resumen.md"), []byte(MarkdownSummary(a)), 0644)
}

func MarkdownSummary(a DatasetAnalysis) string {
	var b strings.Builder
	b.WriteString("# Resumen de analisis exploratorio del dataset\n\n")
	b.WriteString("Analisis generado por el proyecto en Go.\n\n")
	b.WriteString(fmt.Sprintf("- Registros limpios analizados: %d\n", a.TotalRecords))
	b.WriteString(fmt.Sprintf("- Umbral p75 de alta demanda: %.6f kW\n", a.HighDemand.ThresholdP75))
	b.WriteString(fmt.Sprintf("- Registros de demanda normal: %d (%.2f%%)\n", a.HighDemand.NormalCount, a.HighDemand.NormalRate*100))
	b.WriteString(fmt.Sprintf("- Registros de alta demanda: %d (%.2f%%)\n\n", a.HighDemand.HighDemandCount, a.HighDemand.HighDemandRate*100))
	b.WriteString("## Principales evidencias\n\n")
	for _, line := range a.Summary {
		b.WriteString("- " + line + "\n")
	}
	return b.String()
}

func rawValues(r preprocessing.PowerRecord) []float64 {
	return []float64{r.GlobalActivePower, r.GlobalReactivePower, r.Voltage, r.GlobalIntensity, r.SubMetering1, r.SubMetering2, r.SubMetering3, features.OtherConsumption(r)}
}

func addAccum(m map[int]*accumulator, k int, gap float64, high int) {
	a := m[k]
	if a == nil {
		a = &accumulator{}
		m[k] = a
	}
	a.count++
	a.sum += gap
	a.high += high
}

func groupStatsFromMap(m map[int]*accumulator, labeler func(int) string) []GroupStat {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]GroupStat, 0, len(keys))
	for _, k := range keys {
		a := m[k]
		out = append(out, GroupStat{Group: labeler(k), Count: a.count, AverageGlobalActivePower: safeDiv(a.sum, float64(a.count)), HighDemandCount: a.high, HighDemandRate: safeDiv(float64(a.high), float64(a.count))})
	}
	return out
}

func hourLabels(v int) string { return fmt.Sprintf("%02d:00", v) }
func weekdayLabels(v int) string {
	names := []string{"Domingo", "Lunes", "Martes", "Miercoles", "Jueves", "Viernes", "Sabado"}
	if v >= 0 && v < len(names) {
		return names[v]
	}
	return strconv.Itoa(v)
}
func monthLabels(v int) string {
	names := []string{"", "Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Setiembre", "Octubre", "Noviembre", "Diciembre"}
	if v >= 1 && v < len(names) {
		return names[v]
	}
	return strconv.Itoa(v)
}

func Stats(name string, values []float64) VariableStats {
	if len(values) == 0 {
		return VariableStats{Name: name}
	}
	copyVals := append([]float64(nil), values...)
	sort.Float64s(copyVals)
	sum := 0.0
	for _, v := range copyVals {
		sum += v
	}
	mean := sum / float64(len(copyVals))
	ss := 0.0
	for _, v := range copyVals {
		d := v - mean
		ss += d * d
	}
	return VariableStats{Name: name, Count: len(copyVals), Mean: mean, StdDev: math.Sqrt(ss / float64(len(copyVals))), Min: copyVals[0], P25: percentileSorted(copyVals, 0.25), Median: percentileSorted(copyVals, 0.5), P75: percentileSorted(copyVals, 0.75), Max: copyVals[len(copyVals)-1]}
}

func Histogram(values []float64, bins int) []DistributionBin {
	if len(values) == 0 || bins <= 0 {
		return nil
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	if max == min {
		return []DistributionBin{{From: min, To: max, Count: len(values)}}
	}
	width := (max - min) / float64(bins)
	out := make([]DistributionBin, bins)
	for i := 0; i < bins; i++ {
		out[i] = DistributionBin{From: min + float64(i)*width, To: min + float64(i+1)*width}
	}
	out[bins-1].To = max
	for _, v := range values {
		idx := int((v - min) / width)
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		out[idx].Count++
	}
	return out
}

func CorrelationMatrix(cols [][]float64) [][]float64 {
	n := len(cols)
	out := make([][]float64, n)
	for i := range out {
		out[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			out[i][j] = pearson(cols[i], cols[j])
		}
	}
	return out
}

func pearson(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	ma, mb := 0.0, 0.0
	for i := 0; i < n; i++ {
		ma += a[i]
		mb += b[i]
	}
	ma /= float64(n)
	mb /= float64(n)
	num, da, db := 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		xa := a[i] - ma
		xb := b[i] - mb
		num += xa * xb
		da += xa * xa
		db += xb * xb
	}
	den := math.Sqrt(da * db)
	if den == 0 {
		return 0
	}
	return num / den
}

func percentile(records []preprocessing.PowerRecord, p float64, pick func(preprocessing.PowerRecord) float64) float64 {
	vals := make([]float64, len(records))
	for i, r := range records {
		vals[i] = pick(r)
	}
	sort.Float64s(vals)
	return percentileSorted(vals, p)
}

func percentileSorted(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(vals)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(vals) {
		idx = len(vals) - 1
	}
	return vals[idx]
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
func writeStatsCSV(path string, stats []VariableStats) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"variable", "count", "mean", "std_dev", "min", "p25", "median", "p75", "max"})
	for _, s := range stats {
		w.Write([]string{s.Name, itoa(s.Count), ftoa(s.Mean), ftoa(s.StdDev), ftoa(s.Min), ftoa(s.P25), ftoa(s.Median), ftoa(s.P75), ftoa(s.Max)})
	}
	return w.Error()
}
func writeGroupCSV(path string, rows []GroupStat) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"grupo", "count", "avg_global_active_power", "high_demand_count", "high_demand_rate"})
	for _, r := range rows {
		w.Write([]string{r.Group, itoa(r.Count), ftoa(r.AverageGlobalActivePower), itoa(r.HighDemandCount), ftoa(r.HighDemandRate)})
	}
	return w.Error()
}
func writeCorrelationCSV(path string, names []string, matrix [][]float64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	header := append([]string{"variable"}, names...)
	w.Write(header)
	for i, row := range matrix {
		rec := []string{names[i]}
		for _, v := range row {
			rec = append(rec, ftoa(v))
		}
		w.Write(rec)
	}
	return w.Error()
}
func itoa(v int) string     { return strconv.Itoa(v) }
func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }
