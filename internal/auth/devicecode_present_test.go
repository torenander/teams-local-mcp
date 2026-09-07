package auth

import (
	"strings"
	"testing"
)

// TestDeviceCodeElicitMessage_QuotesTheCode guards the defect that live testing
// exposed in outlook-local-mcp CR-0067 A7. URL-mode elicitation shows the user
// a link and this message and nothing else, and the sign-in page does NOT
// pre-fill the code field — so a message that omits the code strands the user
// on the right page with nothing to type.
func TestDeviceCodeElicitMessage_QuotesTheCode(t *testing.T) {
	msg := deviceCodeElicitMessage(DeviceCodePrompt{UserCode: "ABCD1234"})
	if !strings.Contains(msg, "ABCD1234") {
		t.Errorf("elicitation message = %q, must quote the user code", msg)
	}

	// No string may claim the code is pre-filled: it is not.
	lower := strings.ToLower(msg)
	for _, forbidden := range []string{"already filled", "pre-filled", "prefilled", "filled in for you"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("elicitation message = %q, must not claim the code is pre-filled (%q)", msg, forbidden)
		}
	}

	// With no code there is nothing to quote, but the message must still work.
	if got := deviceCodeElicitMessage(DeviceCodePrompt{}); got == "" {
		t.Error("elicitation message is empty when no user code is present")
	}
}
