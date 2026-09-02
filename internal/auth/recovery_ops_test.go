package auth

import "testing"

// TestIsRecoveryOperation pins the domain classification the middleware uses to
// decide which calls may bypass the pending-auth gate.
func TestIsRecoveryOperation(t *testing.T) {
	tests := []struct {
		tool      string
		operation string
		want      bool
	}{
		{"account", "list", true},
		{"account", "login", true},
		{"account", "refresh", true},
		{"account", "add", true},
		{"chat", "list_chats", false},
		{"teams", "list_teams", false},
		{"system", "status", false},
		{"", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.tool+"."+tc.operation, func(t *testing.T) {
			if got := isRecoveryOperation(toolRequest(tc.tool, tc.operation)); got != tc.want {
				t.Errorf("isRecoveryOperation(%q.%q) = %v, want %v", tc.tool, tc.operation, got, tc.want)
			}
		})
	}
}
