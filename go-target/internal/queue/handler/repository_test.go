package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShortSHA(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc123def456789", "abc123de"},
		{"abc12345", "abc12345"},
		{"abc1234", "abc1234"},
		{"abc", "abc"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, shortSHA(tt.input))
		})
	}
}
