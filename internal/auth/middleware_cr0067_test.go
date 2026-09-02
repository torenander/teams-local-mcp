package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestMiddleware_PendingAuth_BlocksOrdinaryVerb keeps the pre-existing
// behaviour for non-recovery verbs: while a background flow runs they are told
// to wait.
func TestMiddleware_PendingAuth_BlocksOrdinaryVerb(t *testing.T) {
	var started bool
	state := newTestState(authSucceeds(&started), "device_code")
	state.preAuthenticated.Store(true)
	state.begin() // publish an attempt that never finishes

	called := false
	wrapped := state.wrap(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return successResult(), nil
	})

	result, err := wrapped(context.Background(), toolRequest("chat", "list_chats"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("ordinary verb should not run while authentication is pending")
	}
	if !strings.Contains(extractResultText(result), "still in progress") {
		t.Errorf("result = %q, want the pending-auth message", extractResultText(result))
	}
}

// TestMiddleware_PendingAuth_AllowsAccountVerb is CR-0067 A4: the verbs that
// repair authentication must stay reachable while a flow is outstanding.
func TestMiddleware_PendingAuth_AllowsAccountVerb(t *testing.T) {
	var started bool
	state := newTestState(authSucceeds(&started), "device_code")
	state.preAuthenticated.Store(true)
	state.begin() // publish an attempt that never finishes

	called := false
	wrapped := state.wrap(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return successResult(), nil
	})

	result, err := wrapped(context.Background(), toolRequest("account", "list"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("account verb must run while authentication is pending")
	}
	if result == nil || result.IsError {
		t.Errorf("account verb result = %+v, want the handler's own result", result)
	}
}

// TestMiddleware_FreshCredential_AllowsAccountVerb is the other half of A4: a
// cold credential must not divert account verbs into an auth prompt, or the
// user cannot even list which accounts need attention.
func TestMiddleware_FreshCredential_AllowsAccountVerb(t *testing.T) {
	var authStarted bool
	state := newTestState(authSucceeds(&authStarted), "device_code")

	called := false
	wrapped := state.wrap(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return successResult(), nil
	})

	if _, err := wrapped(context.Background(), toolRequest("account", "list")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("account verb must run against a fresh credential")
	}
	if authStarted {
		t.Error("account verb must not trigger an authentication flow")
	}
}

// TestMiddleware_FreshCredential_SilentRefreshSkipsPrompt is CR-0067 A1: when
// the token cache can still satisfy the request, no interactive flow starts.
func TestMiddleware_FreshCredential_SilentRefreshSkipsPrompt(t *testing.T) {
	var authStarted bool
	state := newTestState(authSucceeds(&authStarted), "device_code")
	state.cred = &silentCred{}

	called := false
	wrapped := state.wrap(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return successResult(), nil
	})

	result, err := wrapped(context.Background(), toolRequest("chat", "list_chats"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler should run after a successful silent refresh")
	}
	if authStarted {
		t.Error("no interactive authentication should be started after a silent refresh")
	}
	if result == nil || result.IsError {
		t.Errorf("result = %+v, want the handler's own result", result)
	}
	if !state.authenticated.Load() {
		t.Error("a successful silent refresh should mark the middleware authenticated")
	}
}

// TestHandleAuthError_SilentRefreshRetries verifies that a silent refresh in
// the auth-error path retries the original call instead of prompting. This is
// the headline behaviour of A1: on the default device_code method, an expired
// access token backed by a live refresh token no longer emits a device code.
func TestHandleAuthError_SilentRefreshRetries(t *testing.T) {
	var authStarted bool
	state := newTestState(authSucceeds(&authStarted), "device_code")
	state.preAuthenticated.Store(true)
	state.cred = &silentCred{}

	callCount := 0
	wrapped := state.wrap(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("DeviceCodeCredential: authentication required")
		}
		return successResult(), nil
	})

	result, err := wrapped(context.Background(), toolRequest("chat", "list_chats"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authStarted {
		t.Error("interactive authentication should not run when the cache still has a token")
	}
	if callCount != 2 {
		t.Errorf("handler call count = %d, want 2 (original + retry)", callCount)
	}
	if result == nil || result.IsError {
		t.Errorf("result = %+v, want the retried handler result", result)
	}
}

// TestHandleAuthError_SilentFailureFallsThroughToInteractive is the negative
// half of A1: when the cache cannot satisfy the request, the interactive flow
// must still run.
func TestHandleAuthError_SilentFailureFallsThroughToInteractive(t *testing.T) {
	var authStarted bool
	state := newTestState(authSucceeds(&authStarted), "browser")
	state.preAuthenticated.Store(true)
	state.cred = &failingSilentCred{}

	callCount := 0
	wrapped := state.wrap(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("InteractiveBrowserCredential: authentication required")
		}
		return successResult(), nil
	})

	if _, err := wrapped(context.Background(), toolRequest("chat", "list_chats")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !authStarted {
		t.Error("interactive authentication must run when the silent attempt fails")
	}
}

// TestPendingOutcome covers the race-free publication of the background
// attempt's result (CR-0067 A4). Before this change the completion channel and
// error were plain struct fields written by the goroutine and read unguarded
// at middleware entry.
func TestPendingOutcome(t *testing.T) {
	state := newTestState(nil, "device_code")

	if running, err := state.pendingOutcome(); running || err != nil {
		t.Errorf("pendingOutcome with no attempt = (%v, %v), want (false, nil)", running, err)
	}

	attempt := state.begin()
	if running, err := state.pendingOutcome(); !running || err != nil {
		t.Errorf("pendingOutcome while running = (%v, %v), want (true, nil)", running, err)
	}

	want := fmt.Errorf("sign-in abandoned")
	attempt.finish(want)
	running, err := state.pendingOutcome()
	if running {
		t.Error("pendingOutcome reports running after finish")
	}
	if err == nil || err.Error() != want.Error() {
		t.Errorf("pendingOutcome err = %v, want %v", err, want)
	}

	state.settle()
	if state.pendingAuth.Load() {
		t.Error("settle did not clear the pending flag")
	}
}

// TestBackgroundAuthContextIsBounded pins CR-0067 item 4: neither background
// flow may run on an unbounded context.Background(), because an abandoned
// sign-in would otherwise hold pendingAuth for the process lifetime.
func TestBackgroundAuthContextIsBounded(t *testing.T) {
	for _, method := range []string{"browser", "device_code"} {
		t.Run(method, func(t *testing.T) {
			deadlines := make(chan time.Time, 1)
			state := newTestState(func(ctx context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
				dl, ok := ctx.Deadline()
				if !ok {
					deadlines <- time.Time{}
				} else {
					deadlines <- dl
				}
				return testRecord(), nil
			}, method)
			state.preAuthenticated.Store(true)

			calls := 0
			wrapped := state.wrap(func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				calls++
				if calls == 1 {
					return nil, fmt.Errorf("authentication required")
				}
				return successResult(), nil
			})

			requireWithin(t, 10*time.Second, "the "+method+" flow", func() {
				_, _ = wrapped(context.Background(), toolRequest("chat", "list_chats"))
			})

			select {
			case dl := <-deadlines:
				if dl.IsZero() {
					t.Fatalf("%s background auth context has no deadline", method)
				}
				if remaining := time.Until(dl); remaining > backgroundAuthTimeout+time.Minute {
					t.Errorf("%s deadline is %v away, want at most %v", method, remaining, backgroundAuthTimeout)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("%s authenticate hook was never invoked", method)
			}
		})
	}
}
