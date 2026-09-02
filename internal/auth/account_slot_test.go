package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/mark3labs/mcp-go/mcp"
)

// namedCred is an Authenticator that reports its identity, so a test can tell
// which credential the middleware chose. It deliberately does not implement
// SilentOnly, so TrySilentToken skips it and the interactive path is reached.
type namedCred struct {
	name string
}

func (c *namedCred) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return testRecord(), nil
}

func (c *namedCred) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{}, fmt.Errorf("no cached token for %s", c.name)
}

// TestAccountResolver_ReportsAccountToAuthMiddleware is CR-0067 A6. AuthMiddleware
// wraps AccountResolver (internal/server/verbs_config.go), so the context the
// resolver derives with WithAccountAuth cannot travel back out to the middleware.
// Without a hand-back mechanism, handleAuthError's lookup always misses and
// re-authentication targets the server's default credential no matter which
// account the tool call resolved to.
//
// The assertion is on the credential handed to the authenticate hook, captured
// through a channel because the browser flow authenticates on a background
// goroutine.
func TestAccountResolver_ReportsAccountToAuthMiddleware(t *testing.T) {
	closureCred := &namedCred{name: "closure-default"}
	accountCred := &namedCred{name: "work-account"}

	registry := NewAccountRegistry()
	if err := registry.Add(&AccountEntry{
		Label:          "work",
		AuthMethod:     "browser",
		Authenticator:  accountCred,
		AuthRecordPath: t.TempDir() + "/work_auth_record.json",
		Authenticated:  true,
	}); err != nil {
		t.Fatalf("registry.Add: %v", err)
	}

	chosen := make(chan Authenticator, 1)
	state := newTestState(func(_ context.Context, cred Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		chosen <- cred
		return testRecord(), nil
	}, "browser")
	state.cred = closureCred
	state.preAuthenticated.Store(true)

	calls := 0
	inner := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("InteractiveBrowserCredential: expired token")
		}
		return successResult(), nil
	}

	// Production wrapping order: authMW on the outside, resolver within.
	resolver := AccountResolver(registry)
	wrapped := state.wrap(resolver(inner))

	if _, err := wrapped(context.Background(), toolRequest("chat", "list_chats")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case cred := <-chosen:
		got, ok := cred.(*namedCred)
		if !ok {
			t.Fatalf("authenticate received %T, want *namedCred", cred)
		}
		if got != accountCred {
			t.Errorf("re-authentication targeted %q, want the resolved account's credential %q",
				got.name, accountCred.name)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the authenticate hook was never called")
	}
}

// TestInferAuthMethod_UsesPersistedMethod guards the other half of the A6 port.
// Once the slot hands the resolved account to the middleware, inferAuthMethod's
// answer selects the recovery flow. The pre-A6 rule inspected only the
// credential and collapsed every non-AuthCodeFlow credential to "browser", so a
// device_code account would be re-authenticated through the browser flow --
// which cannot complete with the shipped client ID (AADSTS50011).
//
// The persisted AuthMethod is authoritative when set: it is what built the
// credential, and it survives restarts through accounts.json.
func TestInferAuthMethod_UsesPersistedMethod(t *testing.T) {
	tests := []struct {
		name  string
		entry *AccountEntry
		want  string
	}{
		{"persisted device_code", &AccountEntry{AuthMethod: "device_code"}, "device_code"},
		{"persisted auth_code", &AccountEntry{AuthMethod: "auth_code"}, "auth_code"},
		{"persisted browser", &AccountEntry{AuthMethod: "browser"}, "browser"},
		{"unset falls back to credential inspection", &AccountEntry{Authenticator: &namedCred{}}, "browser"},
		{"unset with device code credential", &AccountEntry{Authenticator: &azidentity.DeviceCodeCredential{}}, "device_code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferAuthMethod(tt.entry); got != tt.want {
				t.Errorf("inferAuthMethod() = %q, want %q", got, tt.want)
			}
		})
	}
}
