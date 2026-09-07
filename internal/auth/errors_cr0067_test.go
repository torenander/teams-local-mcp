package auth

import (
	"fmt"
	"strings"
	"testing"
)

// TestRecoverySteps_MethodSpecific is CR-0067 A5. The pre-CR guidance told the
// LLM to "call account_add", which registers a NEW account: following it
// produced duplicate registry entries instead of a working session. It also
// used the pre-CR-0060 flat tool names, which this server no longer exposes.
func TestRecoverySteps_MethodSpecific(t *testing.T) {
	for _, method := range []string{"", "browser", "device_code", "auth_code"} {
		name := method
		if name == "" {
			name = "unknown"
		}
		t.Run(name, func(t *testing.T) {
			steps := recoverySteps(method)

			if !strings.Contains(steps, `operation="login"`) {
				t.Errorf("guidance for %q does not name operation=\"login\":\n%s", method, steps)
			}
			if !strings.Contains(steps, `operation="list"`) {
				t.Errorf("guidance for %q does not start from operation=\"list\":\n%s", method, steps)
			}
			// account_add / account_list are the pre-CR-0060 flat tool names.
			for _, stale := range []string{"account_add", "account_list"} {
				if strings.Contains(steps, stale) {
					t.Errorf("guidance for %q names the removed tool %q:\n%s", method, stale, steps)
				}
			}
			// Adding an account must never be the primary recovery step.
			addIdx := strings.Index(steps, `operation="add"`)
			loginIdx := strings.Index(steps, `operation="login"`)
			if addIdx >= 0 && addIdx < loginIdx {
				t.Errorf("guidance for %q offers add before login:\n%s", method, steps)
			}
			if !strings.Contains(steps, "Retry your original request") {
				t.Errorf("guidance for %q does not end by retrying:\n%s", method, steps)
			}
		})
	}

	// This server does not register a complete_auth verb (see the CR's
	// "Not ported" section), so no guidance may point the LLM at one.
	for _, method := range []string{"", "browser", "device_code", "auth_code"} {
		if strings.Contains(recoverySteps(method), "complete_auth") {
			t.Errorf("guidance for %q names complete_auth, which this server does not register", method)
		}
	}

	if !strings.Contains(recoverySteps("device_code"), "code") {
		t.Error("device_code guidance should mention entering the code")
	}
	if !strings.Contains(recoverySteps("auth_code"), "address bar") {
		t.Error("auth_code guidance should explain the redirect URL step")
	}
}

// TestClassifyAuthError_PreservesDetail is the second half of A5: before
// CR-0067 the bare "authentication required" marker was tested before the
// generic branch, so flow-specific detail composed by the middleware was
// swallowed.
func TestClassifyAuthError_PreservesDetail(t *testing.T) {
	err := fmt.Errorf("authentication required: device code prompt was not received from Entra ID")
	got := classifyAuthError(err)
	if !strings.Contains(got, "device code prompt was not received") {
		t.Errorf("classifyAuthError = %q, want the explanatory detail preserved", got)
	}
}

func TestClassifyAuthError_BareAuthRequired(t *testing.T) {
	got := classifyAuthError(fmt.Errorf("authentication required"))
	if got != "Authentication is required for this account." {
		t.Errorf("classifyAuthError = %q, want the bare message", got)
	}
}

// TestClassifyAuthError_StripsCredentialPrefix covers the shape azidentity and
// AuthCodeCredential actually produce: "<CredentialName>: authentication
// required[: detail]".
func TestClassifyAuthError_StripsCredentialPrefix(t *testing.T) {
	got := classifyAuthError(fmt.Errorf("DeviceCodeCredential: authentication required"))
	if got != "Authentication is required for this account." {
		t.Errorf("classifyAuthError = %q, want the SDK class name stripped", got)
	}
	if strings.Contains(got, "DeviceCodeCredential") {
		t.Errorf("classifyAuthError leaked an SDK class name: %q", got)
	}
}

func TestAuthRequiredDetail(t *testing.T) {
	tests := []struct {
		in         string
		wantDetail string
		wantOK     bool
	}{
		{"authentication required", "", true},
		{"authentication required: device code prompt was not received.", "device code prompt was not received", true},
		{"AuthCodeCredential: authentication required", "", true},
		{"Authentication Required - token expired", "token expired", true},
		{"something else entirely", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			detail, ok := authRequiredDetail(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("authRequiredDetail(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
			}
			if detail != tc.wantDetail {
				t.Errorf("authRequiredDetail(%q) detail = %q, want %q", tc.in, detail, tc.wantDetail)
			}
		})
	}
}

// TestFormatAuthError_DelegatesToMethodAware keeps the method-agnostic wrapper
// in step with the method-aware variant.
func TestFormatAuthError_DelegatesToMethodAware(t *testing.T) {
	err := fmt.Errorf("authentication required")
	if FormatAuthError(err) != FormatAuthErrorFor(err, "") {
		t.Error("FormatAuthError diverged from FormatAuthErrorFor with an unknown method")
	}
}
