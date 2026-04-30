package validate

import (
	"strings"
	"testing"
)

func TestValidateStringLength(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		param   string
		max     int
		wantErr bool
	}{
		{"within limit", "hello", "body", 10, false},
		{"at limit", "hello", "body", 5, false},
		{"exceeds limit", "hello world", "body", 5, true},
		{"empty string", "", "body", 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStringLength(tt.value, tt.param, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStringLength() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), tt.param) {
				t.Errorf("error should contain param name %q, got %q", tt.param, err.Error())
			}
		})
	}
}

func TestValidateResourceID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		param   string
		wantErr bool
	}{
		{"valid ID", "abc-123", "chat_id", false},
		{"empty", "", "chat_id", true},
		{"too long", strings.Repeat("a", MaxResourceIDLen+1), "chat_id", true},
		{"at max length", strings.Repeat("a", MaxResourceIDLen), "chat_id", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResourceID(tt.value, tt.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResourceID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContentType(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{"text", false},
		{"html", false},
		{"TEXT", false},
		{"HTML", false},
		{"json", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := ValidateContentType(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContentType(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDatetime(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"RFC3339", "2026-04-15T09:00:00Z", false},
		{"RFC3339 with offset", "2026-04-15T09:00:00+02:00", false},
		{"with milliseconds", "2026-04-15T09:00:00.000Z", false},
		{"no timezone", "2026-04-15T09:00:00", false},
		{"invalid", "not-a-date", true},
		{"date only", "2026-04-15", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDatetime(tt.value, "start")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDatetime(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email   string
		wantErr bool
	}{
		{"alice@contoso.com", false},
		{"not-an-email", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestValidateImportance(t *testing.T) {
	for _, v := range []string{"low", "normal", "high", "LOW", "Normal"} {
		if err := ValidateImportance(v); err != nil {
			t.Errorf("ValidateImportance(%q) unexpected error: %v", v, err)
		}
	}
	if err := ValidateImportance("critical"); err == nil {
		t.Error("ValidateImportance(\"critical\") should return error")
	}
}

func TestValidateRecipients(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantN   int
		wantErr bool
	}{
		{"single", "alice@contoso.com", 1, false},
		{"multiple", "alice@contoso.com, bob@contoso.com", 2, false},
		{"empty", "", 0, false},
		{"whitespace only", "  ", 0, false},
		{"invalid email", "alice@contoso.com, not-email", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateRecipients(tt.value, "to")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRecipients() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != tt.wantN {
				t.Errorf("ValidateRecipients() got %d recipients, want %d", len(got), tt.wantN)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := Truncate(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}
