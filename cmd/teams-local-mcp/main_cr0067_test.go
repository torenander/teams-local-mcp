package main

import (
	"testing"
	"time"

	"github.com/torenander/teams-local-mcp/internal/auth"
	"github.com/torenander/teams-local-mcp/internal/config"
)

// TestStartupTokenProbe_RunsForEveryMethod is CR-0067 item 2. Before this
// change the probe skipped device_code entirely and guessed at readiness from
// an os.Stat of the auth record, because GetToken would otherwise have emitted
// a device code nobody asked for. With DisableAutomaticAuthentication set on
// both azidentity credentials, GetToken is silent-only, so the probe can ask
// the credential directly for every method.
//
// The assertions are: the probe returns quickly, it never marks a cold
// credential as pre-authenticated, and — critically — it emits no device code.
// A device code would surface as a call to the UserPrompt callback, which
// cannot happen on the DisableAutomaticAuthentication branch.
func TestStartupTokenProbe_RunsForEveryMethod(t *testing.T) {
	for _, method := range []string{"browser", "device_code"} {
		t.Run(method, func(t *testing.T) {
			cfg := config.Config{
				ClientID:       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
				TenantID:       "common",
				AuthMethod:     method,
				CacheName:      "teams-local-mcp-probe-" + method,
				AuthRecordPath: t.TempDir() + "/auth_record.json",
				TokenStorage:   "file",
			}

			cred, _, err := auth.SetupCredential(cfg)
			if err != nil {
				t.Fatalf("SetupCredential(%s): %v", method, err)
			}

			marked := false
			done := make(chan struct{})
			start := time.Now()
			go func() {
				defer close(done)
				probeStartupToken(cred, method, func() { marked = true }, []string{"Calendars.ReadWrite"})
			}()

			select {
			case <-done:
			case <-time.After(startupProbeTimeout + 5*time.Second):
				t.Fatalf("probeStartupToken(%s) did not return; it must never block on interactive auth", method)
			}

			if elapsed := time.Since(start); elapsed > startupProbeTimeout+2*time.Second {
				t.Errorf("probe took %v, want at most %v", elapsed, startupProbeTimeout)
			}
			if marked {
				t.Errorf("probe marked a cold %s credential as pre-authenticated", method)
			}
		})
	}
}
