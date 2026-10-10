package rules

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeComponent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain name", "images", "images"},
		{"spaces inside are kept", "January 2025", "January 2025"},
		{"path traversal", "../../x", ".._.._x"},
		{"backslashes", `a\b`, "a_b"},
		{"forbidden characters", `a<b>c:d"e|f?g*h`, "a_b_c_d_e_f_g_h"},
		{"control characters", "a\x00b\nc\td", "a_b_c_d"},
		{"spaces around", "  docs  ", "docs"},
		{"trailing dots and spaces", "docs. . .", "docs"},
		{"empty", "", "_other"},
		{"only spaces", "   ", "_other"},
		{"dot", ".", "_other"},
		{"dot dot", "..", "_other"},
		{"only dots", "....", "_other"},
		{"leading dot is kept", ".config", ".config"},
		{"windows reserved", "CON", "_CON"},
		{"windows reserved lower case", "nul", "_nul"},
		{"windows reserved with number", "COM1", "_COM1"},
		{"windows reserved with number", "LPT9", "_LPT9"},
		{"not reserved", "COM10", "COM10"},
		{"unicode", "фото 2025", "фото 2025"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeComponent(tt.input); got != tt.want {
				t.Errorf("SanitizeComponent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeComponent_Truncates(t *testing.T) {
	// 'я' takes 2 bytes, so the cut at 120 bytes falls between runes for an odd prefix
	got := SanitizeComponent("a" + strings.Repeat("я", 100))

	if len(got) > maxComponentBytes {
		t.Errorf("length = %d bytes, want at most %d", len(got), maxComponentBytes)
	}
	if !utf8.ValidString(got) {
		t.Errorf("result is not valid UTF-8: %q", got)
	}
	if want := "a" + strings.Repeat("я", 59); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitizeComponent_TruncationDoesNotLeaveTrailingDot(t *testing.T) {
	got := SanitizeComponent(strings.Repeat("a", 119) + ". tail")

	if want := strings.Repeat("a", 119); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitizeComponent_ResultIsLocalComponent(t *testing.T) {
	inputs := []string{"../../x", "/etc/passwd", `C:\Windows`, "..", "a/../../b", "", "NUL", "x\x00y"}

	for _, input := range inputs {
		got := SanitizeComponent(input)

		if strings.ContainsAny(got, `/\`) {
			t.Errorf("SanitizeComponent(%q) = %q contains a path separator", input, got)
		}
		if !filepath.IsLocal(got) {
			t.Errorf("SanitizeComponent(%q) = %q is not a local path", input, got)
		}
	}
}
