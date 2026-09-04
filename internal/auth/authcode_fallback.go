// Package auth — this file holds the guidance text returned when the auth_code
// flow reaches the browser step but the MCP client cannot be elicited for the
// redirect URL.
//
// The text lives here, rather than inline at the two call sites (the auth
// middleware's handleAuthCodeAuth and tools.HandleAddAccount), so that a single
// test can guard which verbs the guidance names.
//
// Both call sites previously told the caller to use a "complete_auth" tool.
// This server does not register that verb, so the auth_code flow dead-ended:
// the caller was handed an authorization URL and an instruction it could not
// carry out. CR-0067 fixed the same mistake in recoverySteps (errors.go) but
// not here. Because auth_code cannot finish without the redirect URL, the
// guidance now routes to device_code, which is the only flow that completes
// with the shipped client ID, and the dead authorization URL is not offered.
package auth

import "fmt"

// AuthCodeElicitationUnavailableText returns the guidance shown when the auth
// middleware starts the auth_code flow while re-authenticating an existing
// account and the MCP client does not support elicitation.
//
// The recovery routes through remove-then-add, which is the only exit that
// actually works for an account in accounts.json. Two things rule out the
// simpler options: the login verb takes no auth_method, and login resolves the
// method from entry.AuthMethod, falling back to the server default only when
// that is empty (tools/login_account.go). Restoring at startup does the same
// from the persisted value (restore.go). So changing TEAMS_MCP_AUTH_METHOD and
// restarting leaves a stored auth_code account on auth_code, and calling login
// re-enters this dead end. add cannot reuse the label while the account exists,
// so the account has to be removed first -- which is local-only and does not
// revoke anything with Microsoft.
//
// This path became reachable for named accounts with the A6 slot: before it,
// handleAuthError always used the server default, so this text only appeared
// for the memory-only default account, where the env var does work.
//
// Returns the guidance as tool result text.
func AuthCodeElicitationUnavailableText() string {
	return "Authentication could not be completed. The auth_code method needs the " +
		"post-sign-in redirect URL pasted back, and this MCP client does not support " +
		"the prompt this server uses to collect it. Any browser window that opened can " +
		"be closed; signing in there cannot finish the flow.\n\n" +
		"To recover, move the account onto the device_code method:\n" +
		"1. Call account with operation=\"list\" to see which account is disconnected\n" +
		"2. Call account with operation=\"remove\" for that label. This clears local " +
		"tokens only and does not revoke anything with Microsoft\n" +
		"3. Call account with operation=\"add\" using the same label and " +
		"auth_method=\"device_code\"\n" +
		"4. Enter the code it returns at the sign-in page\n" +
		"5. Retry your original request\n\n" +
		"Note: setting TEAMS_MCP_AUTH_METHOD=device_code changes the default for " +
		"accounts added later. It does not move an account already stored as " +
		"auth_code, because login and startup both prefer the account's own " +
		"stored method."
}

// AuthCodeElicitationUnavailableAddError returns the error returned when the
// add-account flow starts the auth_code flow and the MCP client does not
// support elicitation.
//
// Unlike the middleware path this names auth_method directly, because the add
// verb accepts it per call and so needs no server restart.
//
// Parameters:
//   - label: the account label being added.
//
// Returns the guidance as an error.
func AuthCodeElicitationUnavailableAddError(label string) error {
	return fmt.Errorf(
		"elicitation not supported by MCP client: the auth_code method needs the "+
			"post-sign-in redirect URL pasted back, and this client cannot be prompted "+
			"for it. Any browser window that opened can be closed. Retry account with "+
			"operation=\"add\", label %q and auth_method=\"device_code\", then enter the "+
			"code it returns at the sign-in page",
		label)
}
