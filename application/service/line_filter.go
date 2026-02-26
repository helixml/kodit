package service

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// LineFilter extracts specific line ranges from file content.
// Supports formats like "L17-L26,L45,L55-L90".
type LineFilter struct {
	ranges []lineRange
}

type lineRange struct {
	start int
	end   int
}

// NewLineFilter parses a line filter parameter.
// An empty param returns a pass-through filter.
func NewLineFilter(param string) (LineFilter, error) {
	param = strings.TrimSpace(param)
	if param == "" {
		return LineFilter{}, nil
	}

	parts := strings.Split(param, ",")
	ranges := make([]lineRange, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		r, err := parseRange(part)
		if err != nil {
			return LineFilter{}, fmt.Errorf("invalid line range %q: %w", part, err)
		}
		ranges = append(ranges, r)
	}

	if len(ranges) == 0 {
		return LineFilter{}, fmt.Errorf("empty line filter")
	}

	return LineFilter{ranges: ranges}, nil
}

// Apply extracts matching lines from content.
// If no ranges are set (pass-through), returns the original content.
func (f LineFilter) Apply(content []byte) []byte {
	if len(f.ranges) == 0 {
		return content
	}

	lines := bytes.Split(content, []byte("\n"))
	var result [][]byte

	for _, r := range f.ranges {
		start := r.start - 1
		end := r.end

		if start >= len(lines) {
			continue
		}
		if end > len(lines) {
			end = len(lines)
		}
		if start < 0 {
			start = 0
		}

		result = append(result, lines[start:end]...)
	}

	return bytes.Join(result, []byte("\n"))
}

// ApplyWithLineNumbers extracts matching lines and prefixes each with its
// original 1-based line number and a tab character.
// If no ranges are set (pass-through), all lines are numbered.
func (f LineFilter) ApplyWithLineNumbers(content []byte) []byte {
	lines := bytes.Split(content, []byte("\n"))
	var result [][]byte

	if len(f.ranges) == 0 {
		for i, line := range lines {
			result = append(result, []byte(fmt.Sprintf("%d\t%s", i+1, line)))
		}
		return bytes.Join(result, []byte("\n"))
	}

	for _, r := range f.ranges {
		start := r.start - 1
		end := r.end

		if start >= len(lines) {
			continue
		}
		if end > len(lines) {
			end = len(lines)
		}
		if start < 0 {
			start = 0
		}

		for i := start; i < end; i++ {
			result = append(result, []byte(fmt.Sprintf("%d\t%s", i+1, lines[i])))
		}
	}

	return bytes.Join(result, []byte("\n"))
}

// Empty returns true if this is a pass-through filter.
func (f LineFilter) Empty() bool {
	return len(f.ranges) == 0
}

func parseRange(s string) (lineRange, error) {
	if idx := strings.Index(s, "-L"); idx > 0 {
		startStr := strings.TrimPrefix(s[:idx], "L")
		endStr := strings.TrimPrefix(s[idx+1:], "L")

		start, err := strconv.Atoi(startStr)
		if err != nil {
			return lineRange{}, fmt.Errorf("invalid start line: %w", err)
		}
		end, err := strconv.Atoi(endStr)
		if err != nil {
			return lineRange{}, fmt.Errorf("invalid end line: %w", err)
		}
		if start < 1 || end < 1 {
			return lineRange{}, fmt.Errorf("line numbers must be positive")
		}
		if start > end {
			return lineRange{}, fmt.Errorf("start line %d exceeds end line %d", start, end)
		}
		return lineRange{start: start, end: end}, nil
	}

	lineStr := strings.TrimPrefix(s, "L")
	line, err := strconv.Atoi(lineStr)
	if err != nil {
		return lineRange{}, fmt.Errorf("invalid line number: %w", err)
	}
	if line < 1 {
		return lineRange{}, fmt.Errorf("line numbers must be positive")
	}
	return lineRange{start: line, end: line}, nil
}
