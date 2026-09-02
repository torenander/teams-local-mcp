package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/torenander/teams-local-mcp/internal/config"
)

func TestTrySilentToken_NilCredential(t *testing.T) {
	if TrySilentToken(context.Background(), nil, []string{"Calendars.ReadWrite"}) {
		t.Error("TrySilentToken(nil) = true, want false")
	}
}

func TestTrySilentToken_Success(t *testing.T) {
	if !TrySilentToken(context.Background(), &silentCred{}, []string{"Calendars.ReadWrite"}) {
		t.Error("TrySilentToken with a warm cache = false, want true")
	}
}

func TestTrySilentToken_Failure(t *testing.T) {
	cred := &failingSilentCred{}
	if TrySilentToken(context.Background(), cred, []string{"Calendars.ReadWrite"}) {
		t.Error("TrySilentToken with an empty cache = true, want false")
	}
	if cred.getTokenCalls != 1 {
		t.Errorf("GetToken called %d times, want 1", cred.getTokenCalls)
	}
}

// TestTrySilentToken_SkipsEscalatingCredential is the safety property of A1:
// a credential that has not promised to be silent-only is never probed, because
// azidentity's GetToken falls through to an interactive flow on a cache miss.
func TestTrySilentToken_SkipsEscalatingCredential(t *testing.T) {
	cred := &escalatingCred{}
	if TrySilentToken(context.Background(), cred, []string{"Calendars.ReadWrite"}) {
		t.Error("TrySilentToken probed an ineligible credential")
	}
	if cred.getTokenCalls != 0 {
		t.Errorf("GetToken called %d times on an ineligible credential, want 0", cred.getTokenCalls)
	}
}

// TestSilentOnlyAcquirer_Eligibility pins the allowlist. Anything not on it
// must fail safe.
func TestSilentOnlyAcquirer_Eligibility(t *testing.T) {
	cfg := config.Config{
		ClientID:  "d3590ed6-52b3-4102-aeff-aad2292ab01c",
		TenantID:  "common",
		CacheName: "teams-local-mcp-test",
	}

	browserCred, err := azidentity.NewInteractiveBrowserCredential(&azidentity.InteractiveBrowserCredentialOptions{
		ClientID: cfg.ClientID, TenantID: cfg.TenantID, DisableAutomaticAuthentication: true,
	})
	if err != nil {
		t.Fatalf("NewInteractiveBrowserCredential: %v", err)
	}
	deviceCred, err := azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
		ClientID: cfg.ClientID, TenantID: cfg.TenantID, DisableAutomaticAuthentication: true,
	})
	if err != nil {
		t.Fatalf("NewDeviceCodeCredential: %v", err)
	}

	tests := []struct {
		name     string
		cred     Authenticator
		eligible bool
	}{
		{"nil", nil, false},
		{"declares SilentOnly", &silentCred{}, true},
		{"azidentity browser", browserCred, true},
		{"azidentity device code", deviceCred, true},
		{"unrecognised with GetToken", &escalatingCred{}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := silentOnlyAcquirer(tc.cred) != nil
			if got != tc.eligible {
				t.Errorf("silentOnlyAcquirer eligible = %v, want %v", got, tc.eligible)
			}
		})
	}
}

// TestAuthCodeCredential_ImplementsSilentTokenCredential pins the marker that
// makes the auth_code credential probe-eligible.
func TestAuthCodeCredential_ImplementsSilentTokenCredential(t *testing.T) {
	var _ SilentTokenCredential = (*AuthCodeCredential)(nil)
}

// TestSetupCredential_GetTokenIsSilentOnly is the behavioural pin for
// DisableAutomaticAuthentication. azidentity produces AuthenticationRequiredError
// only on that branch, so removing the option from either credential fails
// this test rather than silently reintroducing mid-tool-call prompts.
//
// This test performs no network I/O: with no cached token and no authentication
// record, AcquireTokenSilent fails locally and the flag short-circuits before
// any request is made.
func TestSetupCredential_GetTokenIsSilentOnly(t *testing.T) {
	for _, method := range []string{"browser", "device_code"} {
		t.Run(method, func(t *testing.T) {
			cfg := config.Config{
				ClientID:       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
				TenantID:       "common",
				AuthMethod:     method,
				CacheName:      "teams-local-mcp-test-" + method,
				AuthRecordPath: t.TempDir() + "/auth_record.json",
				TokenStorage:   "file",
			}

			cred, _, err := SetupCredential(cfg)
			if err != nil {
				t.Fatalf("SetupCredential(%s): %v", method, err)
			}

			_, err = cred.GetToken(context.Background(), policy.TokenRequestOptions{
				Scopes: []string{"Calendars.ReadWrite"},
			})
			if err == nil {
				t.Fatalf("GetToken on a cold %s credential succeeded; expected AuthenticationRequiredError", method)
			}

			var required *azidentity.AuthenticationRequiredError
			if !errors.As(err, &required) {
				t.Fatalf("GetToken error = %v (%T), want *azidentity.AuthenticationRequiredError. "+
					"DisableAutomaticAuthentication is probably not set on the %s credential", err, err, method)
			}

			if !IsAuthError(err) {
				t.Error("IsAuthError did not classify AuthenticationRequiredError as an auth error")
			}
			if got := classifyAuthError(err); got != "Authentication is required for this account." {
				t.Errorf("classifyAuthError = %q; SDK-level advice must not reach the LLM", got)
			}
		})
	}
}
