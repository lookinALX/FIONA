package suggest

import "testing"

func TestClosest(t *testing.T) {
	presets := []string{"downloads", "photos", "by-date", "by-type"}

	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"photso", "photos", true},  // swapped letters
		{"Photos", "photos", true},  // case
		{"photo", "photos", true},   // missing letter
		{"bydate", "by-date", true}, // missing dash
		{"music", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		got, ok := Closest(tt.input, presets)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Closest(%q) = %q, %v; want %q, %v", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestClosest_Placeholders(t *testing.T) {
	got, ok := Closest("yaer", []string{"category", "ext", "year", "month", "day", "date", "size", "ml"})
	if !ok || got != "year" {
		t.Errorf("Closest(yaer) = %q, %v", got, ok)
	}
}
