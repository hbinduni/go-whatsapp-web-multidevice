package whatsapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePhone(t *testing.T) {
	tests := []struct {
		name        string
		phone       string
		wantNorm    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid phone with plus",
			phone:    "+628131836981",
			wantNorm: "628131836981",
			wantErr:  false,
		},
		{
			name:     "valid phone without plus",
			phone:    "628131836981",
			wantNorm: "628131836981",
			wantErr:  false,
		},
		{
			name:     "valid phone with spaces",
			phone:    "+62 813 1836 981",
			wantNorm: "628131836981",
			wantErr:  false,
		},
		{
			name:     "valid phone with dashes",
			phone:    "+62-813-1836-981",
			wantNorm: "628131836981",
			wantErr:  false,
		},
		{
			name:     "minimum valid length (7 digits)",
			phone:    "1234567",
			wantNorm: "1234567",
			wantErr:  false,
		},
		{
			name:     "maximum valid length (15 digits)",
			phone:    "123456789012345",
			wantNorm: "123456789012345",
			wantErr:  false,
		},
		{
			name:        "empty phone",
			phone:       "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "too short",
			phone:       "123456",
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:        "too long (16 digits)",
			phone:       "1234567890123456",
			wantErr:     true,
			errContains: "too long",
		},
		{
			name:        "arbitrary string b1",
			phone:       "b1",
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:        "arbitrary string client-1",
			phone:       "client-1",
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:        "letters only",
			phone:       "abcdefgh",
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:        "mixed alphanumeric",
			phone:       "phone123",
			wantErr:     true,
			errContains: "too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := ValidatePhone(tt.phone)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantNorm, normalized)
			}
		})
	}
}

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+628131836981", "628131836981"},
		{"628131836981", "628131836981"},
		{"+62 813-1836-981", "628131836981"},
		{"(62) 813 1836 981", "628131836981"},
		{"+1 (555) 123-4567", "15551234567"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePhone(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
