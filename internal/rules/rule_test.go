package rules

import (
	"testing"

	"github.com/lookinalx/fiona/internal/scanner"
)

func TestRulesGetDestination(t *testing.T) {
	tests := []struct {
		name          string
		inputRule     Rule
		inputFileInfo *scanner.FileInfo
		wantResult    string
	}{
		{
			name:          "ByExtension GetDestination",
			inputRule:     &ByExtensionRule{true},
			inputFileInfo: &scanner.FileInfo{Extension: ".jpg"},
			wantResult:    "jpg",
		},
		{
			name:          "ByDate GetDestination",
			inputRule:     &ByDateRule{true},
			inputFileInfo: &scanner.FileInfo{Date: "December 2008"},
			wantResult:    "December 2008",
		},
		{
			name:          "ByMonth GetDestination",
			inputRule:     &ByMonthRule{true},
			inputFileInfo: &scanner.FileInfo{Date: "December 2008"},
			wantResult:    "December",
		},
		{
			name:          "ByYear GetDestination",
			inputRule:     &ByYearRule{true},
			inputFileInfo: &scanner.FileInfo{Date: "December 2008"},
			wantResult:    "2008",
		},
		{
			name:          "BySize GetDestination",
			inputRule:     &BySizeRule{true},
			inputFileInfo: &scanner.FileInfo{Size: 1_000_000},
			wantResult:    "medium",
		},
		{
			name:          "ByType GetDestination",
			inputRule:     &ByTypeRule{true},
			inputFileInfo: &scanner.FileInfo{Extension: ".png", Type: "images"},
			wantResult:    "images",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := tt.inputRule.GetDestination(tt.inputFileInfo); gotResult != tt.wantResult {
				t.Errorf("GetDestination() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
