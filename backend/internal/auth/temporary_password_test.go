package auth

import (
	"strings"
	"testing"
	"unicode"
)

func TestGenerateTemporaryPassword(t *testing.T) {
	first, err := GenerateTemporaryPassword()
	if err != nil {
		t.Fatalf("GenerateTemporaryPassword() error = %v", err)
	}
	second, err := GenerateTemporaryPassword()
	if err != nil {
		t.Fatalf("GenerateTemporaryPassword() second error = %v", err)
	}
	if len(first) < 8 {
		t.Fatalf("password length = %d, want at least 8", len(first))
	}
	if first == second {
		t.Fatal("two generated passwords unexpectedly match")
	}
	if !strings.ContainsFunc(first, unicode.IsLetter) || !strings.ContainsFunc(first, unicode.IsDigit) {
		t.Fatalf("password %q must contain a letter and a number", first)
	}
}
