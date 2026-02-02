package util

import (
	"testing"
)

func TestToRomajiFinal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"こんにちは", "konnichiha"},
		{"ありがとう", "arigatou"},
		{"テスト", "tesuto"},
		{"hello", "hello"},
	}

	for _, test := range tests {
		result := ToRomaji(test.input)
		if result != test.expected {
			t.Errorf("ToRomaji(%q) = %q; expected %q", test.input, result, test.expected)
		}
	}
}

func TestIsJapaneseFinal(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"こんにちは", true},
		{"テスト", true},
		{"hello", false},
	}

	for _, test := range tests {
		result := IsJapanese(test.input)
		if result != test.expected {
			t.Errorf("IsJapanese(%q) = %v; expected %v", test.input, result, test.expected)
		}
	}
}