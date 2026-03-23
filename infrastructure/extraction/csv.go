package extraction

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

// CSVText converts CSV content into an indexable text representation.
//
// The output contains three sections joined by newlines:
//  1. All column header names (if a header row is present).
//  2. Deduplicated string values from every non-numeric column.
//  3. The first few data rows written back as CSV.
//
// A column is considered numeric when every non-empty value in that column
// can be parsed as a float64. Columns with at least one non-numeric value are
// treated as string columns.
type CSVText struct {
	previewRows int
}

// NewCSVText creates a CSVText with default settings.
func NewCSVText() *CSVText {
	return &CSVText{previewRows: 5}
}

// Text converts CSV bytes into a searchable string.
func (c *CSVText) Text(content []byte) (string, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return "", nil
	}

	r := csv.NewReader(bytes.NewReader(content))
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return "", fmt.Errorf("parse csv: %w", err)
	}
	if len(records) == 0 {
		return "", nil
	}

	headers := records[0]
	dataRows := records[1:]

	numCols := len(headers)
	numericCol := make([]bool, numCols)
	for i := range numericCol {
		numericCol[i] = true
	}
	for _, row := range dataRows {
		for i := 0; i < numCols && i < len(row); i++ {
			v := strings.TrimSpace(row[i])
			if v == "" {
				continue
			}
			if _, parseErr := strconv.ParseFloat(v, 64); parseErr != nil {
				numericCol[i] = false
			}
		}
	}

	seen := make([]map[string]struct{}, numCols)
	for i := range seen {
		seen[i] = make(map[string]struct{})
	}
	for _, row := range dataRows {
		for i := 0; i < numCols && i < len(row); i++ {
			if numericCol[i] {
				continue
			}
			v := strings.TrimSpace(row[i])
			if v != "" {
				seen[i][v] = struct{}{}
			}
		}
	}

	var sb strings.Builder

	sb.WriteString("Headers: ")
	sb.WriteString(strings.Join(headers, " "))
	sb.WriteByte('\n')

	var vals []string
	for i := range headers {
		if numericCol[i] {
			continue
		}
		for v := range seen[i] {
			vals = append(vals, v)
		}
	}
	if len(vals) > 0 {
		sb.WriteString("Values: ")
		sb.WriteString(strings.Join(vals, " "))
		sb.WriteByte('\n')
	}

	preview := dataRows
	if len(preview) > c.previewRows {
		preview = preview[:c.previewRows]
	}
	if len(preview) > 0 {
		sb.WriteString("Top rows:\n")
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		for _, row := range preview {
			if writeErr := w.Write(row); writeErr != nil {
				return "", fmt.Errorf("write csv preview: %w", writeErr)
			}
		}
		w.Flush()
		sb.WriteString(buf.String())
	}

	return strings.TrimSpace(sb.String()), nil
}
