package config

import "testing"

// TestInferAuthMethod_DefaultsUnchanged pins the inference behaviour that
// CR-0067 deliberately did NOT change. outlook-local-mcp tried inferring
// auth_code for well-known client IDs and reverted it after live testing (a
// Microsoft anti-phishing interstitial blocks the nativeclient redirect
// pattern), and re-confirmed that browser fails with AADSTS50011 against the
// first-party application. This test exists so that a future reader who has
// the same idea has to change a test with the reasons written down, rather
// than a one-line default.
func TestInferAuthMethod_DefaultsUnchanged(t *testing.T) {
	const officeClientID = "d3590ed6-52b3-4102-aeff-aad2292ab01c"

	tests := []struct {
		name       string
		clientID   string
		explicit   string
		wantMethod string
		wantSource string
	}{
		{"shipped default client ID", officeClientID, "", "device_code", "inferred"},
		{"well-known teams-local-mcp", "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3", "", "device_code", "inferred"},
		{"well-known teams-desktop", "1fec8e78-bce4-4aaf-ab1b-5451cc387264", "", "device_code", "inferred"},
		{"custom client ID", "11111111-2222-3333-4444-555555555555", "", "browser", "default"},
		{"explicit wins over inference", officeClientID, "auth_code", "auth_code", "explicit"},
		{"explicit browser wins", officeClientID, "browser", "browser", "explicit"},
		{"explicit device_code on a custom ID", "11111111-2222-3333-4444-555555555555", "device_code", "device_code", "explicit"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			method, source := InferAuthMethod(tc.clientID, tc.explicit)
			if method != tc.wantMethod || source != tc.wantSource {
				t.Errorf("InferAuthMethod(%q, %q) = (%q, %q), want (%q, %q)",
					tc.clientID, tc.explicit, method, source, tc.wantMethod, tc.wantSource)
			}
		})
	}
}

// TestDefaultClientIDIsTheFirstPartyApp records the shipped default explicitly.
// It matters because the CR-0067 rejection of browser and auth_code as defaults
// is evidence gathered against THIS application, not against the
// teams-local-mcp registration that also appears in WellKnownClientIDs.
func TestDefaultClientIDIsTheFirstPartyApp(t *testing.T) {
	t.Setenv("TEAMS_MCP_CLIENT_ID", "")
	got := ResolveClientID(GetEnv("TEAMS_MCP_CLIENT_ID", "outlook-desktop"))
	if got != "d3590ed6-52b3-4102-aeff-aad2292ab01c" {
		t.Errorf("default client ID = %q, want the outlook-desktop first-party app. "+
			"If this changed deliberately, the CR-0067 default-inference evidence needs revisiting.", got)
	}
}
