package util

import (
	"testing"
)

func TestIsJapaneseBasic(t *testing.T) {
	if !IsJapanese("こんにちは") {
		t.Error("Expected true for Japanese hiragana")
	}
	if IsJapanese("hello") {
		t.Error("Expected false for English")
	}
}

func TestToRomajiBasic(t *testing.T) {
	if result := ToRomaji("あ"); result != "a" {
		t.Errorf("Expected 'a' for 'あ', got '%s'", result)
	}
	if result := ToRomaji("か"); result != "ka" {
		t.Errorf("Expected 'ka' for 'か', got '%s'", result)
	}
}