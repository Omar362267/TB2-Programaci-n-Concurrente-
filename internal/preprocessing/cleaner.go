package preprocessing

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type DiscardReason string

const (
	ReasonEmptyLine      DiscardReason = "empty_line"
	ReasonMissingValue   DiscardReason = "missing_value"
	ReasonInvalidColumns DiscardReason = "invalid_column_count"
	ReasonInvalidDate    DiscardReason = "invalid_datetime"
	ReasonInvalidNumber  DiscardReason = "invalid_number"
	ReasonInvalidRange   DiscardReason = "invalid_range"
)

type PowerRecord struct {
	LineNumber          int       `json:"line_number"`
	Timestamp           time.Time `json:"timestamp"`
	GlobalActivePower   float64   `json:"global_active_power"`
	GlobalReactivePower float64   `json:"global_reactive_power"`
	Voltage             float64   `json:"voltage"`
	GlobalIntensity     float64   `json:"global_intensity"`
	SubMetering1        float64   `json:"sub_metering_1"`
	SubMetering2        float64   `json:"sub_metering_2"`
	SubMetering3        float64   `json:"sub_metering_3"`
}

type ParseResult struct {
	Record PowerRecord
	Valid  bool
	Reason DiscardReason
	Err    error
}

func ParseRawRecord(lineNumber int, raw string) ParseResult {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return invalid(lineNumber, ReasonEmptyLine, "linea vacia")
	}
	if strings.Contains(raw, "?") {
		return invalid(lineNumber, ReasonMissingValue, "contiene valores faltantes representados por ?")
	}

	parts := strings.Split(raw, ";")
	if len(parts) != 9 {
		return invalid(lineNumber, ReasonInvalidColumns, fmt.Sprintf("se esperaban 9 columnas y se recibieron %d", len(parts)))
	}

	timestamp, err := parseTimestamp(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	if err != nil {
		return invalid(lineNumber, ReasonInvalidDate, err.Error())
	}

	values := make([]float64, 7)
	for i := 2; i < len(parts); i++ {
		v, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if err != nil {
			return invalid(lineNumber, ReasonInvalidNumber, fmt.Sprintf("columna %d: %v", i+1, err))
		}
		values[i-2] = v
	}

	record := PowerRecord{
		LineNumber:          lineNumber,
		Timestamp:           timestamp,
		GlobalActivePower:   values[0],
		GlobalReactivePower: values[1],
		Voltage:             values[2],
		GlobalIntensity:     values[3],
		SubMetering1:        values[4],
		SubMetering2:        values[5],
		SubMetering3:        values[6],
	}

	if err := validateRanges(record); err != nil {
		return invalid(lineNumber, ReasonInvalidRange, err.Error())
	}

	return ParseResult{Record: record, Valid: true}
}

func parseTimestamp(dateValue, timeValue string) (time.Time, error) {
	value := dateValue + " " + timeValue
	layouts := []string{
		"02/01/2006 15:04:05",
		"2/1/2006 15:04:05",
		"2/01/2006 15:04:05",
		"02/1/2006 15:04:05",
	}
	var lastErr error
	for _, layout := range layouts {
		timestamp, err := time.Parse(layout, value)
		if err == nil {
			return timestamp, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

func invalid(lineNumber int, reason DiscardReason, msg string) ParseResult {
	return ParseResult{Valid: false, Reason: reason, Err: fmt.Errorf("linea %d: %s", lineNumber, msg)}
}

func validateRanges(r PowerRecord) error {
	if r.GlobalActivePower < 0 {
		return fmt.Errorf("Global_active_power negativo")
	}
	if r.GlobalReactivePower < 0 {
		return fmt.Errorf("Global_reactive_power negativo")
	}
	if r.Voltage <= 0 {
		return fmt.Errorf("Voltage debe ser mayor que cero")
	}
	if r.GlobalIntensity < 0 {
		return fmt.Errorf("Global_intensity negativo")
	}
	if r.SubMetering1 < 0 || r.SubMetering2 < 0 || r.SubMetering3 < 0 {
		return fmt.Errorf("sub_metering negativo")
	}
	return nil
}
