package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/torenander/teams-local-mcp/internal/auth"
	"github.com/torenander/teams-local-mcp/internal/config"
)

// blockingCred models azidentity's per-client mutex: GetToken blocks for the
// duration of an interactive sign-in and ignores the caller's context
// deadline. account_refresh must never reach it while a flow is in flight.
type blockingCred struct {
	calls   int
	release chan struct{}
}

func (c *blockingCred) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.calls++
	<-c.release
	return azcore.AccessToken{Token: "t", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func refreshRequest(label string) mcp.CallToolRequest {
	var r mcp.CallToolRequest
	r.Params.Name = "account"
	r.Params.Arguments = map[string]any{"label": label}
	return r
}

func resultText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if tc, ok := result.Content[0].(mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

// TestRefreshAccount_DeclinesDuringInteractiveAuth is CR-0067 A4. Before the
// guard, account.refresh hung for the whole sign-in — the sharpest edge of the
// bug, because the recovery guidance names refresh as a recovery step.
func TestRefreshAccount_DeclinesDuringInteractiveAuth(t *testing.T) {
	cred := &blockingCred{release: make(chan struct{})}
	defer close(cred.release)

	registry := auth.NewAccountRegistry()
	if err := registry.Add(&auth.AccountEntry{
		Label:         "work",
		Credential:    cred,
		Authenticated: true,
	}); err != nil {
		t.Fatalf("registry.Add: %v", err)
	}

	handler := HandleRefreshAccount(registry, config.Config{})

	auth.BeginInteractiveAuth()
	defer auth.EndInteractiveAuth()

	type outcome struct {
		text string
		err  error
	}
	got := make(chan outcome, 1)
	go func() {
		res, err := handler(context.Background(), refreshRequest("work"))
		got <- outcome{resultText(res), err}
	}()

	select {
	case o := <-got:
		if o.err != nil {
			t.Fatalf("handler returned a Go error: %v", o.err)
		}
		if !strings.Contains(o.text, "sign-in is already in progress") {
			t.Errorf("result = %q, want an explanation that a sign-in is outstanding", o.text)
		}
		if cred.calls != 0 {
			t.Errorf("GetToken called %d times; refresh must not touch the credential during a sign-in", cred.calls)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("account_refresh HUNG during an interactive sign-in (GetToken calls: %d)", cred.calls)
	}
}

// TestRefreshAccount_RefreshesWhenNoAuthInFlight is the control: with no
// sign-in outstanding the guard must not fire.
func TestRefreshAccount_RefreshesWhenNoAuthInFlight(t *testing.T) {
	cred := &blockingCred{release: make(chan struct{})}
	close(cred.release) // GetToken returns immediately

	registry := auth.NewAccountRegistry()
	if err := registry.Add(&auth.AccountEntry{
		Label:         "work",
		Credential:    cred,
		Authenticated: true,
	}); err != nil {
		t.Fatalf("registry.Add: %v", err)
	}

	handler := HandleRefreshAccount(registry, config.Config{})
	res, err := handler(context.Background(), refreshRequest("work"))
	if err != nil {
		t.Fatalf("handler returned a Go error: %v", err)
	}
	if cred.calls != 1 {
		t.Errorf("GetToken called %d times, want 1", cred.calls)
	}
	if text := resultText(res); !strings.Contains(text, "Account token refreshed") {
		t.Errorf("result = %q, want the refresh confirmation", text)
	}
}
