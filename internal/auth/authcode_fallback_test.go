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

	// The middleware path re-authenticates an account that already exists. The
	// login verb takes no auth_method, so the only way out is to change the
	// server's configured method.
	if !strings.Contains(messages["middleware"], "TEAMS_MCP_AUTH_METHOD") {
		t.Errorf("middleware guidance does not name the env var that selects the method:\n%s", messages["middleware"])
	}

	// The add path can pass auth_method directly, and must keep naming the label.
	if !strings.Contains(messages["add"], "auth_method") {
		t.Errorf("add guidance does not name the auth_method parameter:\n%s", messages["add"])
	}
	if !strings.Contains(messages["add"], "work") {
		t.Errorf("add guidance dropped the account label:\n%s", messages["add"])
	}
}
