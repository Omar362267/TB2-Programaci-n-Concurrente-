package visualization

import (
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/analysis"
)

const svgHeader = `<svg xmlns="http://www.w3.org/2000/svg" width="900" height="520" viewBox="0 0 900 520">`

func EnsureDir(dir string) error { return os.MkdirAll(dir, 0755) }

func BarChart(path, title, xLabel, yLabel string, rows []analysis.GroupStat) error {
	var labels []string
	var values []float64
	for _, r := range rows {
		labels = append(labels, r.Group)
		values = append(values, r.AverageGlobalActivePower)
	}
	return SimpleBars(path, title, xLabel, yLabel, labels, values)
}

func SimpleBars(path, title, xLabel, yLabel string, labels []string, values []float64) error {
	if len(values) == 0 {
		return os.WriteFile(path, []byte(svgHeader+`<text x="30" y="40">Sin datos</text></svg>`), 0644)
	}
	max := maxFloat(values)
	if max <= 0 {
		max = 1
	}
	left, top, width, height := 90.0, 70.0, 760.0, 340.0
	barGap := 4.0
	barW := (width / float64(len(values))) - barGap
	if barW < 2 {
		barW = 2
	}
	var b strings.Builder
	b.WriteString(svgHeader)
	b.WriteString(baseStyle())
	b.WriteString(fmt.Sprintf(`<text class="title" x="450" y="35">%s</text>`, esc(title)))
	drawAxes(&b, left, top, width, height, xLabel, yLabel)
	for i, v := range values {
		h := (v / max) * height
		x := left + float64(i)*(width/float64(len(values))) + barGap/2
		y := top + height - h
		b.WriteString(fmt.Sprintf(`<rect class="bar" x="%.2f" y="%.2f" width="%.2f" height="%.2f"><title>%s: %.4f</title></rect>`, x, y, barW, h, esc(labels[i]), v))
		if len(values) <= 31 {
			rot := ""
			if len(labels[i]) > 4 {
				rot = fmt.Sprintf(` transform="rotate(-45 %.2f 445)"`, x+barW/2)
			}
			b.WriteString(fmt.Sprintf(`<text class="tick" x="%.2f" y="445" text-anchor="end"%s>%s</text>`, x+barW/2, rot, esc(labels[i])))
		}
	}
	for i := 0; i <= 5; i++ {
		v := max * float64(i) / 5
		y := top + height - (float64(i)/5)*height
		b.WriteString(fmt.Sprintf(`<line class="grid" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, left, y, left+width, y))
		b.WriteString(fmt.Sprintf(`<text class="tick" x="82" y="%.1f" text-anchor="end">%.2f</text>`, y+4, v))
	}
	b.WriteString(`</svg>`)
	return write(path, b.String())
}

func Histogram(path, title string, bins []analysis.DistributionBin) error {
	labels := make([]string, len(bins))
	values := make([]float64, len(bins))
	for i, bin := range bins {
		labels[i] = fmt.Sprintf("%.1f-%.1f", bin.From, bin.To)
		values[i] = float64(bin.Count)
	}
	return SimpleBars(path, title, "Global_active_power (kW)", "Frecuencia", labels, values)
}

func LineChart(path, title, xLabel, yLabel string, xs []string, values []float64) error {
	if len(values) == 0 {
		return os.WriteFile(path, []byte(svgHeader+`<text x="30" y="40">Sin datos</text></svg>`), 0644)
	}
	max := maxFloat(values)
	min := minFloat(values)
	if max == min {
		max = min + 1
	}
	left, top, width, height := 90.0, 70.0, 760.0, 340.0
	var b strings.Builder
	b.WriteString(svgHeader)
	b.WriteString(baseStyle())
	b.WriteString(fmt.Sprintf(`<text class="title" x="450" y="35">%s</text>`, esc(title)))
	drawAxes(&b, left, top, width, height, xLabel, yLabel)
	points := make([]string, 0, len(values))
	for i, v := range values {
		x := left
		if len(values) > 1 {
			x += float64(i) * width / float64(len(values)-1)
		}
		y := top + height - ((v-min)/(max-min))*height
		points = append(points, fmt.Sprintf("%.2f,%.2f", x, y))
		b.WriteString(fmt.Sprintf(`<circle class="point" cx="%.2f" cy="%.2f" r="3"><title>%s: %.6f</title></circle>`, x, y, labelAt(xs, i), v))
	}
	b.WriteString(fmt.Sprintf(`<polyline class="line" points="%s"/>`, strings.Join(points, " ")))
	for i := 0; i <= 5; i++ {
		v := min + (max-min)*float64(i)/5
		y := top + height - (float64(i)/5)*height
		b.WriteString(fmt.Sprintf(`<line class="grid" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`, left, y, left+width, y))
		b.WriteString(fmt.Sprintf(`<text class="tick" x="82" y="%.1f" text-anchor="end">%.3f</text>`, y+4, v))
	}
	b.WriteString(`</svg>`)
	return write(path, b.String())
}

func Heatmap(path, title string, names []string, matrix [][]float64) error {
	cell := 68.0
	left, top := 210.0, 80.0
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="900" height="760" viewBox="0 0 900 760">`)
	b.WriteString(baseStyle())
	b.WriteString(fmt.Sprintf(`<text class="title" x="450" y="35">%s</text>`, esc(title)))
	for i, name := range names {
		x := left + float64(i)*cell + cell/2
		b.WriteString(fmt.Sprintf(`<text class="tick" x="%.1f" y="72" text-anchor="end" transform="rotate(-45 %.1f 72)">%s</text>`, x, x, esc(short(name))))
		b.WriteString(fmt.Sprintf(`<text class="tick" x="200" y="%.1f" text-anchor="end">%s</text>`, top+float64(i)*cell+cell/2+4, esc(short(name))))
	}
	for i := range matrix {
		for j := range matrix[i] {
			v := matrix[i][j]
			opacity := math.Abs(v)
			cls := "heat-pos"
			if v < 0 {
				cls = "heat-neg"
			}
			x := left + float64(j)*cell
			y := top + float64(i)*cell
			b.WriteString(fmt.Sprintf(`<rect class="%s" opacity="%.3f" x="%.1f" y="%.1f" width="%.1f" height="%.1f"><title>%s vs %s: %.4f</title></rect>`, cls, 0.15+0.85*opacity, x, y, cell-2, cell-2, esc(names[i]), esc(names[j]), v))
			b.WriteString(fmt.Sprintf(`<text class="cell" x="%.1f" y="%.1f" text-anchor="middle">%.2f</text>`, x+cell/2, y+cell/2+4, v))
		}
	}
	b.WriteString(`</svg>`)
	return write(path, b.String())
}

func ConfusionMatrix(path string, tp, tn, fp, fn int) error {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="680" height="520" viewBox="0 0 680 520">`)
	b.WriteString(baseStyle())
	b.WriteString(`<text class="title" x="340" y="35">Matriz de confusion</text>`)
	left, top, cell := 210.0, 120.0, 150.0
	labels := [][]string{{"TN", fmt.Sprintf("%d", tn)}, {"FP", fmt.Sprintf("%d", fp)}, {"FN", fmt.Sprintf("%d", fn)}, {"TP", fmt.Sprintf("%d", tp)}}
	idx := 0
	for r := 0; r < 2; r++ {
		for c := 0; c < 2; c++ {
			x := left + float64(c)*cell
			y := top + float64(r)*cell
			b.WriteString(fmt.Sprintf(`<rect class="matrix" x="%.1f" y="%.1f" width="%.1f" height="%.1f"/>`, x, y, cell-2, cell-2))
			b.WriteString(fmt.Sprintf(`<text class="cellbig" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, x+cell/2, y+58, labels[idx][0]))
			b.WriteString(fmt.Sprintf(`<text class="cellvalue" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, x+cell/2, y+100, labels[idx][1]))
			idx++
		}
	}
	b.WriteString(`<text class="axis" x="360" y="455" text-anchor="middle">Prediccion</text>`)
	b.WriteString(`<text class="axis" x="70" y="270" transform="rotate(-90 70 270)" text-anchor="middle">Valor real</text>`)
	b.WriteString(`<text class="tick" x="285" y="105" text-anchor="middle">Pred. 0</text><text class="tick" x="435" y="105" text-anchor="middle">Pred. 1</text>`)
	b.WriteString(`<text class="tick" x="190" y="200" text-anchor="end">Real 0</text><text class="tick" x="190" y="350" text-anchor="end">Real 1</text>`)
	b.WriteString(`</svg>`)
	return write(path, b.String())
}

func baseStyle() string {
	return `<style>
		.title{font:700 22px Arial,sans-serif; text-anchor:middle; fill:#222}
		.axis{font:600 14px Arial,sans-serif; fill:#333}.tick{font:11px Arial,sans-serif; fill:#333}.cell{font:12px Arial,sans-serif; fill:#111}.cellbig{font:700 28px Arial,sans-serif; fill:#222}.cellvalue{font:700 24px Arial,sans-serif; fill:#222}
		.grid{stroke:#ddd; stroke-width:1}.bar{fill:#4d7ea8}.line{fill:none; stroke:#4d7ea8; stroke-width:3}.point{fill:#222}.heat-pos{fill:#4d7ea8}.heat-neg{fill:#d45d5d}.matrix{fill:#e7eef6; stroke:#fff; stroke-width:2}
	</style>`
}

func drawAxes(b *strings.Builder, left, top, width, height float64, xLabel, yLabel string) {
	b.WriteString(fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#222"/>`, left, top+height, left+width, top+height))
	b.WriteString(fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#222"/>`, left, top, left, top+height))
	b.WriteString(fmt.Sprintf(`<text class="axis" x="%.1f" y="500" text-anchor="middle">%s</text>`, left+width/2, esc(xLabel)))
	b.WriteString(fmt.Sprintf(`<text class="axis" x="22" y="%.1f" transform="rotate(-90 22 %.1f)" text-anchor="middle">%s</text>`, top+height/2, top+height/2, esc(yLabel)))
}

func write(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
func esc(s string) string { return html.EscapeString(s) }
func short(s string) string {
	if len(s) > 18 {
		return s[:18]
	}
	return s
}
func maxFloat(v []float64) float64 {
	m := v[0]
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}
func minFloat(v []float64) float64 {
	m := v[0]
	for _, x := range v {
		if x < m {
			m = x
		}
	}
	return m
}
func labelAt(labels []string, i int) string {
	if i >= 0 && i < len(labels) {
		return labels[i]
	}
	return fmt.Sprintf("%d", i)
}
