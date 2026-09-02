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
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// This file holds the shared fixtures for the CR-0067 test suite. The
// internal/auth package had no tests before this change set, so every helper
// here is new rather than reused.

// toolRequest builds a call to the named aggregate tool with an operation
// argument, matching the CR-0060 "{domain}.{operation}" dispatch shape that
// isRecoveryOperation classifies on.
func toolRequest(tool, operation string) mcp.CallToolRequest {
	var r mcp.CallToolRequest
	r.Params.Name = tool
	r.Params.Arguments = map[string]any{"operation": operation}
	return r
}

// successResult is the result a healthy inner handler returns.
func successResult() *mcp.CallToolResult {
	return mcp.NewToolResultText("ok")
}

// testRecord is a non-zero AuthenticationRecord for mock authenticators to
// return on success.
func testRecord() azidentity.AuthenticationRecord {
	return azidentity.AuthenticationRecord{
		ClientID: "test-client",
		TenantID: "test-tenant",
		Username: "user@example.com",
	}
}

// elicitUnsupported models a client that does not implement form elicitation.
func elicitUnsupported(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
	return nil, mcpserver.ErrElicitationNotSupported
}

// urlElicitUnsupported models a client that does not implement URL
// elicitation. This is the common case in the field: clients such as Claude
// Code answer elicitation requests with "Method not found".
func urlElicitUnsupported(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
	return nil, mcpserver.ErrElicitationNotSupported
}

// newTestState builds an authMiddlewareState whose authenticate hook is
// stubbed, so tests can drive the real middleware entry point (state.wrap)
// without reaching Entra ID.
//
// Parameters:
//   - authFn: the stub replacing the package-level Authenticate.
//   - authMethod: "auth_code", "browser" or "device_code".
func newTestState(authFn authenticateFunc, authMethod string) *authMiddlewareState {
	return &authMiddlewareState{
		cred:           nil, // most tests never reach the credential
		authRecordPath: "/tmp/teams-mcp-test-auth-record.json",
		authMethod:     authMethod,
		authenticate:   authFn,
		elicit:         elicitUnsupported,
		urlElicit:      urlElicitUnsupported,
		openBrowser:    func(_ string) error { return nil },
		browserTimeout: 2 * time.Second,
		scopes:         []string{"Calendars.ReadWrite"},
	}
}

// authSucceeds is an authenticate stub that reports success and records that
// an interactive flow was started.
func authSucceeds(started *bool) authenticateFunc {
	return func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		*started = true
		return testRecord(), nil
	}
}

// silentCred is a credential whose GetToken always succeeds from cache and
// which declares the SilentOnly guarantee, so TrySilentToken will probe it.
type silentCred struct{}

func (c *silentCred) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return testRecord(), nil
}

func (c *silentCred) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "cached", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func (c *silentCred) SilentOnly() {}

// escalatingCred has a GetToken but makes no silent-only promise. It stands in
// for a credential type added later without DisableAutomaticAuthentication:
// TrySilentToken must never call it, because doing so would open a browser
// window or emit a device code.
type escalatingCred struct {
	getTokenCalls int
}

func (c *escalatingCred) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return testRecord(), nil
}

func (c *escalatingCred) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.getTokenCalls++
	return azcore.AccessToken{Token: "prompted"}, nil
}

// failingSilentCred declares the silent-only guarantee but has an empty cache.
type failingSilentCred struct {
	getTokenCalls int
}

func (c *failingSilentCred) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return testRecord(), nil
}

func (c *failingSilentCred) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.getTokenCalls++
	return azcore.AccessToken{}, fmt.Errorf("authentication required")
}

func (c *failingSilentCred) SilentOnly() {}

// requireWithin fails the test if fn has not returned by d. It exists because
// the defects CR-0067 A4 fixes present as hangs, not as wrong answers, and a
// plain call would hang the test binary instead of failing it.
func requireWithin(t *testing.T, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not return within %v", what, d)
	}
}
