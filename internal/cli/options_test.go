package cli

import (
	"errors"
	"testing"
)

func TestValidatePrimaryCriteriaFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "valid mimetype flag",
			input:   CritMIMEType,
			wantErr: nil,
		},
		{
			name:    "valid extension flag",
			input:   CritExtension,
			wantErr: nil,
		},
		{
			name:    "valid year flag",
			input:   CritYear,
			wantErr: nil,
		},
		{
			name:    "valid month flag",
			input:   CritMonth,
			wantErr: nil,
		},
		{
			name:    "valid creation date flag",
			input:   CritCrDate,
			wantErr: nil,
		},
		{
			name:    "valid modification date flag",
			input:   CritModDate,
			wantErr: nil,
		},
		{
			name:    "valid size flag",
			input:   CritSize,
			wantErr: nil,
		},

		{
			name:    "invalid criteria",
			input:   "invalid",
			wantErr: ErrInvalidPrimaryCriteria,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: ErrEmptyPrimaryCriteria,
		},
		{
			name:    "random string",
			input:   "foobar",
			wantErr: ErrInvalidPrimaryCriteria,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrimaryCriteriaFlag(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validatePrimaryCriteriaFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
