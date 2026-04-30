package tools

import (
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

func TestBuildChatMessageBody_Text(t *testing.T) {
	body := "Hello, world!"
	msg := BuildChatMessageBody(body, "text")

	if msg == nil {
		t.Fatal("BuildChatMessageBody returned nil")
	}
	msgBody := msg.GetBody()
	if msgBody == nil {
		t.Fatal("message body is nil")
	}
	if got := *msgBody.GetContent(); got != body {
		t.Errorf("body content = %q, want %q", got, body)
	}
	if got := *msgBody.GetContentType(); got != models.TEXT_BODYTYPE {
		t.Errorf("content type = %v, want TEXT_BODYTYPE", got)
	}
}

func TestBuildChatMessageBody_HTML(t *testing.T) {
	body := "<p>Hello</p>"
	msg := BuildChatMessageBody(body, "html")

	msgBody := msg.GetBody()
	if msgBody == nil {
		t.Fatal("message body is nil")
	}
	if got := *msgBody.GetContentType(); got != models.HTML_BODYTYPE {
		t.Errorf("content type = %v, want HTML_BODYTYPE", got)
	}
}

func TestBuildChatMessageBody_DefaultsToText(t *testing.T) {
	msg := BuildChatMessageBody("test", "unknown")

	msgBody := msg.GetBody()
	if got := *msgBody.GetContentType(); got != models.TEXT_BODYTYPE {
		t.Errorf("content type = %v, want TEXT_BODYTYPE for unknown content type", got)
	}
}
