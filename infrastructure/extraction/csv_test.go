package extraction_test

import (
	"strings"
	"testing"

	"github.com/helixml/kodit/infrastructure/extraction"
)

func TestCSVText_EmptyContent(t *testing.T) {
	csv := extraction.NewCSVText()
	result, err := csv.Text([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestCSVText_WhitespaceOnly(t *testing.T) {
	csv := extraction.NewCSVText()
	result, err := csv.Text([]byte("   \n  "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestCSVText_HeaderOnly(t *testing.T) {
	csv := extraction.NewCSVText()
	result, err := csv.Text([]byte("name,age,city\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Headers: name age city") {
		t.Errorf("expected header line, got: %q", result)
	}
}

func TestCSVText_StringColumnsIndexed(t *testing.T) {
	csv := extraction.NewCSVText()
	result, err := csv.Text([]byte("name,city\nalice,london\nbob,paris\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "alice") {
		t.Errorf("expected 'alice' in result, got: %q", result)
	}
	if !strings.Contains(result, "london") {
		t.Errorf("expected 'london' in result, got: %q", result)
	}
}

func TestCSVText_NumericColumnsSkipped(t *testing.T) {
	csv := extraction.NewCSVText()
	result, err := csv.Text([]byte("name,age,score\nalice,30,9.5\nbob,25,8.1\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "alice") {
		t.Errorf("expected 'alice' in result, got: %q", result)
	}
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Values:") {
			if strings.Contains(line, "30") || strings.Contains(line, "9.5") {
				t.Errorf("numeric values should not appear in Values line: %q", line)
			}
		}
	}
}

func TestCSVText_Deduplication(t *testing.T) {
	csv := extraction.NewCSVText()
	result, err := csv.Text([]byte("status\nactive\nactive\nactive\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Values:") {
			fields := strings.Fields(strings.TrimPrefix(line, "Values:"))
			count := 0
			for _, f := range fields {
				if f == "active" {
					count++
				}
			}
			if count != 1 {
				t.Errorf("expected 'active' exactly once in Values line, got %d: %q", count, line)
			}
		}
	}
}

func TestCSVText_TopFiveRows(t *testing.T) {
	csv := extraction.NewCSVText()
	result, err := csv.Text([]byte("name\nrow1\nrow2\nrow3\nrow4\nrow5\nrow6\nrow7\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Top rows:") {
		t.Fatalf("expected 'Top rows:' section, got: %q", result)
	}

	topIdx := strings.Index(result, "Top rows:")
	topSection := result[topIdx:]
	if strings.Contains(topSection, "row6") || strings.Contains(topSection, "row7") {
		t.Errorf("Top rows section should not contain row6 or row7: %q", topSection)
	}
	for i := 1; i <= 5; i++ {
		needle := "row" + string(rune('0'+i))
		if !strings.Contains(topSection, needle) {
			t.Errorf("expected %s in Top rows section: %q", needle, topSection)
		}
	}
}

func TestCSVText_MixedColumnsOnlyStringsInValues(t *testing.T) {
	csv := extraction.NewCSVText()
	result, err := csv.Text([]byte("product,price,category\nwidget,9.99,gadget\ngizmo,14.50,gadget\ndongle,3.00,accessory\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Values:") {
			if strings.Contains(line, "9.99") || strings.Contains(line, "14.50") {
				t.Errorf("numeric price values in Values line: %q", line)
			}
			if !strings.Contains(line, "widget") || !strings.Contains(line, "gadget") {
				t.Errorf("expected string values in Values line: %q", line)
			}
		}
	}
}

func TestCSVText_HeaderIncluded(t *testing.T) {
	csv := extraction.NewCSVText()
	result, err := csv.Text([]byte("first_name,last_name,age\nalice,smith,30\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "first_name") || !strings.Contains(result, "last_name") {
		t.Errorf("expected column headers in result: %q", result)
	}
}
