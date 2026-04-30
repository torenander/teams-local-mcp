// Package server provides verb builders for the chat domain aggregate tool.
//
// This file defines buildChatVerbs and chatToolAnnotations used to register
// chat operations.
package server

import (
	"github.com/torenander/teams-local-mcp/internal/graph"
	"github.com/torenander/teams-local-mcp/internal/tools"
	"github.com/torenander/teams-local-mcp/internal/tools/help"
	"github.com/mark3labs/mcp-go/mcp"
)

// buildChatVerbs constructs and returns the ordered verb slice and registry
// pointer for the chat domain aggregate tool.
func buildChatVerbs(c verbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
	registryPtr := &tools.VerbRegistry{}
	rc := c.retryCfg

	verbs := []tools.Verb{
		help.NewHelpVerb(registryPtr),
		buildListChatsVerb(c, rc),
		buildGetChatVerb(c, rc),
		buildListChatMessagesVerb(c, rc),
		buildGetChatMessageVerb(c, rc),
	}

	if c.cfg.TeamsManageEnabled {
		verbs = append(verbs, buildSendChatMessageVerb(c, rc))
	}

	return verbs, registryPtr
}

// buildListChatsVerb constructs the list_chats Verb.
func buildListChatsVerb(c verbsConfig, rc graph.RetryConfig) tools.Verb {
	return tools.Verb{
		Name:        "list_chats",
		Summary:     "list your 1:1 and group chats",
		Description: "Returns a list of chats the authenticated user is a member of, including 1:1 chats, group chats, and meeting chats. Each entry includes the chat topic, type, and last updated time.",
		Handler:     wrapRead(c, "chat.list_chats", "read", tools.NewHandleListChats(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("account",
				mcp.Description(tools.AccountParamDescription),
			),
			mcp.WithNumber("max_results",
				mcp.Description("Maximum number of chats to return (default 25)."),
				mcp.Min(1),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetChatVerb constructs the get_chat Verb.
func buildGetChatVerb(c verbsConfig, rc graph.RetryConfig) tools.Verb {
	return tools.Verb{
		Name:        "get_chat",
		Summary:     "get details for a specific chat by ID",
		Description: "Returns details for a specific chat including topic, type, and members.",
		Handler:     wrapRead(c, "chat.get_chat", "read", tools.NewHandleGetChat(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("chat_id", mcp.Required(),
				mcp.Description("The unique identifier of the chat."),
			),
			mcp.WithString("account",
				mcp.Description(tools.AccountParamDescription),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildListChatMessagesVerb constructs the list_messages Verb for chat.
func buildListChatMessagesVerb(c verbsConfig, rc graph.RetryConfig) tools.Verb {
	return tools.Verb{
		Name:        "list_messages",
		Summary:     "list messages in a chat",
		Description: "Returns messages from a specific chat, ordered by creation time. Each message includes sender, body content, and timestamps.",
		Handler:     wrapRead(c, "chat.list_messages", "read", tools.NewHandleListChatMessages(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("chat_id", mcp.Required(),
				mcp.Description("The unique identifier of the chat."),
			),
			mcp.WithNumber("max_results",
				mcp.Description("Maximum number of messages to return (default 25)."),
				mcp.Min(1),
			),
			mcp.WithString("account",
				mcp.Description(tools.AccountParamDescription),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetChatMessageVerb constructs the get_message Verb for chat.
func buildGetChatMessageVerb(c verbsConfig, rc graph.RetryConfig) tools.Verb {
	return tools.Verb{
		Name:        "get_message",
		Summary:     "get a specific message from a chat",
		Description: "Returns full details for a single chat message by ID, including body content, sender, and timestamps.",
		Handler:     wrapRead(c, "chat.get_message", "read", tools.NewHandleGetChatMessage(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("chat_id", mcp.Required(),
				mcp.Description("The unique identifier of the chat."),
			),
			mcp.WithString("message_id", mcp.Required(),
				mcp.Description("The unique identifier of the message."),
			),
			mcp.WithString("account",
				mcp.Description(tools.AccountParamDescription),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildSendChatMessageVerb constructs the send_message Verb for chat (TeamsManageEnabled-gated).
func buildSendChatMessageVerb(c verbsConfig, rc graph.RetryConfig) tools.Verb {
	return tools.Verb{
		Name:        "send_message",
		Summary:     "send a message to a chat",
		Description: "Sends a new message to a chat. The message appears in the chat for all members. Requires TEAMS_MANAGE_ENABLED=true.",
		Handler:     wrapWrite(c, "chat.send_message", "write", tools.NewHandleSendChatMessage(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("chat_id", mcp.Required(),
				mcp.Description("The unique identifier of the chat to send the message to."),
			),
			mcp.WithString("body", mcp.Required(),
				mcp.Description("Message body content."),
			),
			mcp.WithString("content_type",
				mcp.Description("Body content type: 'text' (default) or 'html'."),
				mcp.Enum("text", "html"),
			),
			mcp.WithString("account",
				mcp.Description(tools.AccountParamDescription),
			),
		},
	}
}

// chatToolAnnotations returns the conservative aggregate MCP annotations for
// the chat domain tool.
func chatToolAnnotations() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithTitleAnnotation("Chat"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	}
}
