package keychain

import (
	"testing"
)

func Test_isHex(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"", true},
		{"a1b2c3", true},
		{"A1B2C3", true},
		{"1234567890abcdef", true},
		{"12345", false},   // Odd length
		{"g1h2", false},    // Non-hex characters
		{"!@#$", false},    // Special characters
		{"abcdefg", false}, // Odd length with valid hex characters
		{"ABCDEF", true},
		{"abcdef", true},
		{"1234abcd", true},
		{"1234abcg", false}, // Contains 'g'
		{"12 34", false},    // Contains space
		{"", true},          // Empty string
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isHex(tt.input)
			if result != tt.expected {
				t.Errorf("isHex(%q) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}
