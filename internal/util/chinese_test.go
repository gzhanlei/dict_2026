package util

import (
	"testing"
)

func TestIsChinese(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"你好", true},        // 中文
		{"世界", true},        // 中文
		{"中国", true},        // 中文
		{"hello", false},      // 英文
		{"こんにちは", false},  // 日文
		{"", false},           // 空字符串
		{"hello你好", true},   // 混合，包含中文
		{"こんにちはworld", false}, // 日文+英文，无中文
	}

	for _, test := range tests {
		result := IsChinese(test.input)
		if result != test.expected {
			t.Errorf("IsChinese(%q) = %v; expected %v", test.input, result, test.expected)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"你好", "zh"},
		{"世界", "zh"},
		{"こんにちは", "ja"},
		{"テスト", "ja"},
		{"hello", "en"},
		{"world", "en"},
		{"", "en"}, // 默认英文
		{"hello你好", "zh"}, // 包含中文，优先中文
		{"こんにちはworld", "ja"}, // 日文+英文，优先日文
	}

	for _, test := range tests {
		result := DetectLanguage(test.input)
		if result != test.expected {
			t.Errorf("DetectLanguage(%q) = %v; expected %v", test.input, result, test.expected)
		}
	}
}

func TestProcessQueryByLanguage(t *testing.T) {
	tests := []struct {
		input    string
		language string
		expected string
	}{
		{"こんにちは", "ja", "konnichiha"}, // 日语转罗马字
		{"テスト", "ja", "tesuto"},
		{"你好", "zh", "你好"},              // 中文保持不变
		{"世界", "zh", "世界"},
		{"hello", "en", "hello"},           // 英文保持不变
		{"world", "en", "world"},
	}

	for _, test := range tests {
		result := ProcessQueryByLanguage(test.input, test.language)
		if result != test.expected {
			t.Errorf("ProcessQueryByLanguage(%q, %v) = %v; expected %v", test.input, test.language, result, test.expected)
		}
	}
}