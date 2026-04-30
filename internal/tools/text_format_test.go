package tools

import (
	"strings"
	"testing"
)

func TestFormatChatsText_Empty(t *testing.T) {
	got := FormatChatsText(nil)
	if got != "No chats found." {
		t.Errorf("FormatChatsText(nil) = %q, want %q", got, "No chats found.")
	}
}

func TestFormatChatsText(t *testing.T) {
	chats := []map[string]any{
		{"topic": "Project Alpha", "chatType": "group"},
		{"topic": "", "chatType": "oneOnOne"},
	}
	got := FormatChatsText(chats)
	if !strings.Contains(got, "1. Project Alpha [group]") {
		t.Errorf("expected 'Project Alpha [group]' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "2. (No topic) [oneOnOne]") {
		t.Errorf("expected '(No topic) [oneOnOne]' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "2 chat(s) total.") {
		t.Errorf("expected '2 chat(s) total.' in output, got:\n%s", got)
	}
}

func TestFormatTeamsText_Empty(t *testing.T) {
	got := FormatTeamsText(nil)
	if got != "No teams found." {
		t.Errorf("FormatTeamsText(nil) = %q, want %q", got, "No teams found.")
	}
}

func TestFormatTeamsText(t *testing.T) {
	teams := []map[string]any{
		{"displayName": "Engineering", "description": "The eng team"},
		{"displayName": "", "description": ""},
	}
	got := FormatTeamsText(teams)
	if !strings.Contains(got, "1. Engineering") {
		t.Errorf("expected 'Engineering' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "The eng team") {
		t.Errorf("expected description in output, got:\n%s", got)
	}
	if !strings.Contains(got, "2. (Unnamed)") {
		t.Errorf("expected '(Unnamed)' for empty displayName, got:\n%s", got)
	}
}

func TestFormatChannelsText_Empty(t *testing.T) {
	got := FormatChannelsText(nil)
	if got != "No channels found." {
		t.Errorf("FormatChannelsText(nil) = %q, want %q", got, "No channels found.")
	}
}

func TestFormatChannelsText(t *testing.T) {
	channels := []map[string]any{
		{"displayName": "General", "membershipType": "standard"},
		{"displayName": "Private", "membershipType": "private"},
	}
	got := FormatChannelsText(channels)
	if !strings.Contains(got, "General [standard]") {
		t.Errorf("expected 'General [standard]' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "2 channel(s) total.") {
		t.Errorf("expected '2 channel(s) total.' in output, got:\n%s", got)
	}
}

func TestFormatChatMessagesText_Empty(t *testing.T) {
	got := FormatChatMessagesText(nil)
	if got != "No messages found." {
		t.Errorf("FormatChatMessagesText(nil) = %q, want %q", got, "No messages found.")
	}
}

func TestFormatChatMessagesText(t *testing.T) {
	messages := []map[string]any{
		{"from": "Alice", "createdDateTime": "2026-04-30T12:00:00Z", "body": "Hello!"},
		{"from": "", "createdDateTime": "2026-04-30T12:01:00Z", "body": ""},
	}
	got := FormatChatMessagesText(messages)
	if !strings.Contains(got, "1. Alice | 2026-04-30T12:00:00Z") {
		t.Errorf("expected Alice message line, got:\n%s", got)
	}
	if !strings.Contains(got, "Hello!") {
		t.Errorf("expected body content, got:\n%s", got)
	}
	if !strings.Contains(got, "2. (Unknown)") {
		t.Errorf("expected '(Unknown)' for empty from, got:\n%s", got)
	}
}

func TestFormatChatMessagesText_TruncatesLongBody(t *testing.T) {
	longBody := strings.Repeat("x", 300)
	messages := []map[string]any{
		{"from": "Bob", "createdDateTime": "2026-04-30T12:00:00Z", "body": longBody},
	}
	got := FormatChatMessagesText(messages)
	if !strings.Contains(got, "...") {
		t.Errorf("expected truncation ellipsis in output, got:\n%s", got)
	}
}

func TestFormatAccountsText_Empty(t *testing.T) {
	got := FormatAccountsText(nil)
	if got != "No accounts registered." {
		t.Errorf("FormatAccountsText(nil) = %q, want %q", got, "No accounts registered.")
	}
}

func TestFormatAccountsText(t *testing.T) {
	accounts := []map[string]any{
		{"label": "default", "authenticated": true, "email": "alice@contoso.com", "auth_method": "device_code"},
		{"label": "work", "authenticated": false, "email": "", "auth_method": ""},
	}
	got := FormatAccountsText(accounts)
	if !strings.Contains(got, "default — alice@contoso.com (authenticated, device_code)") {
		t.Errorf("expected formatted default account, got:\n%s", got)
	}
	if !strings.Contains(got, "work (disconnected)") {
		t.Errorf("expected formatted work account, got:\n%s", got)
	}
	if !strings.Contains(got, "2 account(s) total.") {
		t.Errorf("expected '2 account(s) total.' in output, got:\n%s", got)
	}
}

func TestFormatAccountLine(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		email    string
		advisory []string
		want     string
	}{
		{"with email", "default", "alice@contoso.com", nil, "Account: default (alice@contoso.com)"},
		{"without email", "default", "", nil, "Account: default"},
		{"empty label", "", "", nil, ""},
		{"with advisory", "default", "alice@contoso.com", []string{"Warning: token expiring"}, "Account: default (alice@contoso.com)\nWarning: token expiring"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAccountLine(tt.label, tt.email, tt.advisory...)
			if got != tt.want {
				t.Errorf("FormatAccountLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatStatusText(t *testing.T) {
	status := statusResponse{
		Version:             "1.0.0",
		ServerUptimeSeconds: 3661,
		Accounts: []statusAccount{
			{Label: "default", Authenticated: true, UPN: "alice@contoso.com", AuthMethod: "device_code"},
		},
		Config: statusConfig{
			Features: statusConfigFeatures{
				ReadOnly:           false,
				TeamsEnabled:       true,
				TeamsManageEnabled: true,
			},
		},
	}
	got := FormatStatusText(status)
	if !strings.Contains(got, "v1.0.0") {
		t.Errorf("expected version in output, got:\n%s", got)
	}
	if !strings.Contains(got, "1h 1m") {
		t.Errorf("expected formatted uptime, got:\n%s", got)
	}
	if !strings.Contains(got, "teams=on") {
		t.Errorf("expected teams=on, got:\n%s", got)
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		seconds int64
		want    string
	}{
		{30, "30s"},
		{90, "1m"},
		{3661, "1h 1m"},
		{7200, "2h 0m"},
	}
	for _, tt := range tests {
		got := formatUptime(tt.seconds)
		if got != tt.want {
			t.Errorf("formatUptime(%d) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}
