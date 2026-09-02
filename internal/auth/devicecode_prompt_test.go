package auth

import (
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

func TestNewDeviceCodePrompt(t *testing.T) {
	got := NewDeviceCodePrompt(azidentity.DeviceCodeMessage{
		Message:         "To sign in, open https://microsoft.com/devicelogin and enter ABCD1234",
		UserCode:        "ABCD1234",
		VerificationURL: "https://microsoft.com/devicelogin",
	})
	if got.UserCode != "ABCD1234" {
		t.Errorf("UserCode = %q, want ABCD1234", got.UserCode)
	}
	if got.VerificationURL != "https://microsoft.com/devicelogin" {
		t.Errorf("VerificationURL = %q", got.VerificationURL)
	}
	if !strings.Contains(got.Message, "ABCD1234") {
		t.Errorf("Message = %q, want the Entra sentence verbatim", got.Message)
	}
}

func TestDeviceCodePrompt_SignInURL(t *testing.T) {
	tests := []struct {
		name   string
		prompt DeviceCodePrompt
		want   string
	}{
		{
			name:   "verification URL and code",
			prompt: DeviceCodePrompt{UserCode: "ABCD1234", VerificationURL: "https://microsoft.com/devicelogin"},
			want:   "https://microsoft.com/devicelogin?otc=ABCD1234",
		},
		{
			name:   "no verification URL falls back to the known page",
			prompt: DeviceCodePrompt{UserCode: "ABCD1234"},
			want:   "https://microsoft.com/devicelogin?otc=ABCD1234",
		},
		{
			name:   "no user code leaves the base URL alone",
			prompt: DeviceCodePrompt{VerificationURL: "https://microsoft.com/devicelogin"},
			want:   "https://microsoft.com/devicelogin",
		},
		{
			name:   "pre-existing query parameters are preserved",
			prompt: DeviceCodePrompt{UserCode: "ABCD1234", VerificationURL: "https://example.test/device?tenant=contoso"},
			want:   "https://example.test/device?otc=ABCD1234&tenant=contoso",
		},
		{
			name:   "empty prompt",
			prompt: DeviceCodePrompt{},
			want:   "https://microsoft.com/devicelogin",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.prompt.SignInURL(); got != tc.want {
				t.Errorf("SignInURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
