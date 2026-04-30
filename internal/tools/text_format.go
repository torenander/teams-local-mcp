// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides plain-text formatters for the "text" output mode on read
// tools. Each formatter takes serialized data (maps from the graph serialization
// layer) and produces a human-readable plain-text string.
package tools

import (
	"fmt"
	"strings"
)

// FormatAccountsText formats a slice of account maps into a numbered
// plain-text listing showing each account's label, User Principal Name (UPN)
// when available, authentication state, and auth_method.
//
// Parameters:
//   - accounts: slice of account maps, each expected to contain "label"
//     (string), "authenticated" (bool), and optionally "email" (string) and
//     "auth_method" (string) keys.
//
// Returns a formatted plain-text string. Returns "No accounts registered."
// when the slice is nil or empty.
//
// Side effects: none.
func FormatAccountsText(accounts []map[string]any) string {
	if len(accounts) == 0 {
		return "No accounts registered."
	}

	var b strings.Builder
	for i, a := range accounts {
		label, _ := a["label"].(string)
		if label == "" {
			label = "(unnamed)"
		}
		authed, _ := a["authenticated"].(bool)
		state := "disconnected"
		if authed {
			state = "authenticated"
		}
		email, _ := a["email"].(string)
		method, _ := a["auth_method"].(string)
		parenthetical := state
		if method != "" {
			parenthetical = state + ", " + method
		}
		if email != "" {
			fmt.Fprintf(&b, "%d. %s — %s (%s)\n", i+1, label, email, parenthetical)
		} else {
			fmt.Fprintf(&b, "%d. %s (%s)\n", i+1, label, parenthetical)
		}
	}

	fmt.Fprintf(&b, "\n%d account(s) total.", len(accounts))

	return b.String()
}

// FormatAccountLine returns a formatted "Account: label (upn)" line suitable
// for appending to write-tool confirmation responses.
//
// Parameters:
//   - label: the account label (e.g., "default", "work").
//   - email: the account email/UPN address; may be empty.
//   - advisory: optional advisory text produced by the AccountResolver.
//
// Returns a single- or two-line string, or the empty string when label is
// empty.
//
// Side effects: none.
func FormatAccountLine(label, email string, advisory ...string) string {
	if label == "" {
		return ""
	}
	var base string
	if email != "" {
		base = fmt.Sprintf("Account: %s (%s)", label, email)
	} else {
		base = "Account: " + label
	}
	for _, a := range advisory {
		if a != "" {
			return base + "\n" + a
		}
	}
	return base
}

// FormatStatusText formats a statusResponse struct into a human-readable
// plain-text summary showing server version, uptime, account list with
// authentication state, and feature flags.
//
// Parameters:
//   - status: the statusResponse struct from the status tool handler.
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatStatusText(status statusResponse) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Server: teams-local-mcp v%s\n", status.Version)
	fmt.Fprintf(&b, "Uptime: %s\n", formatUptime(status.ServerUptimeSeconds))

	if len(status.Accounts) > 0 {
		b.WriteString("\nAccounts:\n")
		for _, acct := range status.Accounts {
			state := "disconnected"
			if acct.Authenticated {
				state = "authenticated"
			}
			switch {
			case acct.UPN != "" && acct.AuthMethod != "":
				fmt.Fprintf(&b, "  %s: %s — %s (%s)\n", acct.Label, state, acct.UPN, acct.AuthMethod)
			case acct.UPN != "":
				fmt.Fprintf(&b, "  %s: %s — %s\n", acct.Label, state, acct.UPN)
			case acct.AuthMethod != "":
				fmt.Fprintf(&b, "  %s: %s (%s)\n", acct.Label, state, acct.AuthMethod)
			default:
				fmt.Fprintf(&b, "  %s: %s\n", acct.Label, state)
			}
		}
	}

	readOnly := "off"
	if status.Config.Features.ReadOnly {
		readOnly = "on"
	}
	teams := "off"
	if status.Config.Features.TeamsEnabled {
		teams = "on"
	}
	teamsManage := "off"
	if status.Config.Features.TeamsManageEnabled {
		teamsManage = "on"
	}
	fmt.Fprintf(&b, "\nFeatures: read-only=%s, teams=%s, teams-manage=%s", readOnly, teams, teamsManage)

	return b.String()
}

// formatUptime converts seconds to a human-readable duration string.
func formatUptime(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// FormatChatsText formats a slice of serialized chat maps into a numbered
// plain-text listing.
//
// Parameters:
//   - chats: slice of chat maps, each expected to contain "topic", "chatType",
//     and "lastUpdatedDateTime" keys.
//
// Returns a formatted plain-text string. Returns "No chats found." when the
// slice is nil or empty.
//
// Side effects: none.
func FormatChatsText(chats []map[string]any) string {
	if len(chats) == 0 {
		return "No chats found."
	}

	var b strings.Builder
	for i, c := range chats {
		topic, _ := c["topic"].(string)
		chatType, _ := c["chatType"].(string)
		if topic == "" {
			topic = "(No topic)"
		}
		fmt.Fprintf(&b, "%d. %s [%s]\n", i+1, topic, chatType)

		if members, ok := c["members"].(string); ok && members != "" {
			fmt.Fprintf(&b, "   Members: %s\n", members)
		}

		if i < len(chats)-1 {
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "\n%d chat(s) total.", len(chats))
	return b.String()
}

// FormatChatMessagesText formats a slice of serialized chat message maps into
// a numbered plain-text listing.
//
// Parameters:
//   - messages: slice of message maps, each expected to contain "from",
//     "createdDateTime", and "body" keys.
//
// Returns a formatted plain-text string. Returns "No messages found." when the
// slice is nil or empty.
//
// Side effects: none.
func FormatChatMessagesText(messages []map[string]any) string {
	if len(messages) == 0 {
		return "No messages found."
	}

	var b strings.Builder
	for i, m := range messages {
		from, _ := m["from"].(string)
		if from == "" {
			from = "(Unknown)"
		}
		date, _ := m["createdDateTime"].(string)
		body, _ := m["bodyPreview"].(string)
		if body == "" {
			body, _ = m["body"].(string)
		}

		fmt.Fprintf(&b, "%d. %s | %s\n", i+1, from, date)
		if body != "" {
			// Truncate long messages for text output.
			if len(body) > 200 {
				body = body[:200] + "..."
			}
			fmt.Fprintf(&b, "   %s\n", body)
		}

		if i < len(messages)-1 {
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "\n%d message(s) total.", len(messages))
	return b.String()
}

// FormatTeamsText formats a slice of serialized team maps into a numbered
// plain-text listing.
//
// Parameters:
//   - teams: slice of team maps, each expected to contain "displayName" and
//     "description" keys.
//
// Returns a formatted plain-text string. Returns "No teams found." when the
// slice is nil or empty.
//
// Side effects: none.
func FormatTeamsText(teams []map[string]any) string {
	if len(teams) == 0 {
		return "No teams found."
	}

	var b strings.Builder
	for i, t := range teams {
		name, _ := t["displayName"].(string)
		if name == "" {
			name = "(Unnamed)"
		}
		desc, _ := t["description"].(string)
		fmt.Fprintf(&b, "%d. %s\n", i+1, name)
		if desc != "" {
			fmt.Fprintf(&b, "   %s\n", desc)
		}

		if i < len(teams)-1 {
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "\n%d team(s) total.", len(teams))
	return b.String()
}

// FormatChannelsText formats a slice of serialized channel maps into a
// numbered plain-text listing.
//
// Parameters:
//   - channels: slice of channel maps, each expected to contain "displayName"
//     and "description" keys.
//
// Returns a formatted plain-text string. Returns "No channels found." when the
// slice is nil or empty.
//
// Side effects: none.
func FormatChannelsText(channels []map[string]any) string {
	if len(channels) == 0 {
		return "No channels found."
	}

	var b strings.Builder
	for i, ch := range channels {
		name, _ := ch["displayName"].(string)
		if name == "" {
			name = "(Unnamed)"
		}
		membershipType, _ := ch["membershipType"].(string)
		fmt.Fprintf(&b, "%d. %s", i+1, name)
		if membershipType != "" {
			fmt.Fprintf(&b, " [%s]", membershipType)
		}
		b.WriteString("\n")

		if i < len(channels)-1 {
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "\n%d channel(s) total.", len(channels))
	return b.String()
}
