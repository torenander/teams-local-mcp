package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

func TestParseSince(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"2026-09-22", time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), true},
		{"2026-09-22T08:30:00Z", time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC), true},
		{"2026-09-22T10:30:00+02:00", time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC), true},
		{"yesterday", time.Time{}, false},
	}
	for _, c := range cases {
		got, err := parseSince(c.in)
		if c.ok && (err != nil || !got.Equal(c.want)) {
			t.Errorf("parseSince(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("parseSince(%q) expected error", c.in)
		}
	}
}

func newChatWithPreview(at time.Time, from string) models.Chatable {
	chat := models.NewChat()
	id := "19:abc@unq.gbl.spaces"
	chat.SetId(&id)
	ct := models.ONEONONE_CHATTYPE
	chat.SetChatType(&ct)

	preview := models.NewChatMessageInfo()
	preview.SetCreatedDateTime(&at)
	mt := models.MESSAGE_CHATMESSAGETYPE
	preview.SetMessageType(&mt)
	if from != "" {
		user := models.NewIdentity()
		user.SetDisplayName(&from)
		fromSet := models.NewChatMessageFromIdentitySet()
		fromSet.SetUser(user)
		preview.SetFrom(fromSet)
	}
	chat.SetLastMessagePreview(preview)
	return chat
}

func TestSerializeChatIncludesLastMessage(t *testing.T) {
	at := time.Date(2026, 9, 22, 13, 0, 22, 0, time.UTC)
	m := serializeChat(newChatWithPreview(at, "Vaishali Tanwar"))

	if got := m["lastMessageDateTime"]; got != "2026-09-22T13:00:22Z" {
		t.Errorf("lastMessageDateTime = %v", got)
	}
	if got := m["lastMessageFrom"]; got != "Vaishali Tanwar" {
		t.Errorf("lastMessageFrom = %v", got)
	}
	if got := m["lastMessageType"]; got != "message" {
		t.Errorf("lastMessageType = %v", got)
	}
	if got := m["chatType"]; got != "oneOnOne" {
		t.Errorf("chatType = %v", got)
	}
}

func TestSerializeChatWithoutPreview(t *testing.T) {
	chat := models.NewChat()
	m := serializeChat(chat)
	if _, ok := m["lastMessageDateTime"]; ok {
		t.Error("lastMessageDateTime should be absent without a preview")
	}
	if lastMessageTime(chat) != nil {
		t.Error("lastMessageTime should be nil without a preview")
	}
}

func TestFormatChatsTextShowsLastMessage(t *testing.T) {
	at := time.Date(2026, 9, 22, 13, 0, 22, 0, time.UTC)
	out := FormatChatsText([]map[string]any{serializeChat(newChatWithPreview(at, "Vaishali Tanwar"))})
	if !strings.Contains(out, "Last message: 2026-09-22T13:00:22Z by Vaishali Tanwar") {
		t.Errorf("text output missing last message line:\n%s", out)
	}
}
