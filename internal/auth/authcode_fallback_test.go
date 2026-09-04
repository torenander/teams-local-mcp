package auth

import (
	"strings"
	"testing"
)

// TestAuthCodeElicitationUnavailable_GuidanceIsActionable is the elicitation-
// fallback half of the complete_auth problem. CR-0067 removed the unregistered
// complete_auth verb from recoverySteps (see errors_cr0067_test.go) but left it
// in the two elicitation-failure messages, so a client without elicitation
// support was told to call a verb this server does not expose and the auth_code
// flow dead-ended there.
//
// The guidance must instead route the caller to device_code, which is the only
// flow that completes with the shipped client ID.
func TestAuthCodeElicitationUnavailable_GuidanceIsActionable(t *testing.T) {
	messages := map[string]string{
		"middleware": AuthCodeElicitationUnavailableText(),
		"add":        AuthCodeElicitationUnavailableAddError("work").Error(),
	}

	for name, msg := range messages {
		t.Run(name, func(t *testing.T) {
			// This server registers help, add, remove, list, login, logout and
			// refresh on the account tool. complete_auth is not among them.
			if strings.Contains(msg, "complete_auth") {
				t.Errorf("guidance names complete_auth, which this server does not register:\n%s", msg)
			}
			if !strings.Contains(msg, "device_code") {
				t.Errorf("guidance does not route the caller to device_code:\n%s", msg)
			}
		})
	}

	// The middleware path re-authenticates an account that may already be in
	// accounts.json. Only remove-then-add actually moves such an account onto
	// device_code: login takes no auth_method and resolves the method from
	// entry.AuthMethod (tools/login_account.go), and startup rebuilds from the
	// persisted value (restore.go), so changing TEAMS_MCP_AUTH_METHOD and
	// restarting leaves a stored auth_code account exactly where it was.
	mw := messages["middleware"]
	for _, want := range []string{`operation="remove"`, `operation="add"`, "auth_method"} {
		if !strings.Contains(mw, want) {
			t.Errorf("middleware guidance does not name %s, so it cannot be carried out "+
				"for an account in accounts.json:\n%s", want, mw)
		}
	}
	// Guard the regression directly: login is not an exit from this state, so
	// offering it would put the caller back in the same dead end.
	if strings.Contains(mw, `operation="login"`) {
		t.Errorf("middleware guidance offers login, which re-enters the auth_code dead end "+
			"for a stored account:\n%s", mw)
	}

	// The add path can pass auth_method directly, and must keep naming the label.
	if !strings.Contains(messages["add"], "auth_method") {
		t.Errorf("add guidance does not name the auth_method parameter:\n%s", messages["add"])
	}
	if !strings.Contains(messages["add"], "work") {
		t.Errorf("add guidance dropped the account label:\n%s", messages["add"])
	}
}
