// Package server provides verb builders for the chat domain aggregate tool.
//
// This file defines the chatVerbsConfig, buildChatVerbs, and
// chatToolAnnotations used to register chat operations.
package server

import (
	"github.com/torenander/teams-local-mcp/internal/audit"
	"github.com/torenander/teams-local-mcp/internal/config"
	"github.com/torenander/teams-local-mcp/internal/graph"
	"github.com/torenander/teams-local-mcp/internal/observability"
	"github.com/torenander/teams-local-mcp/internal/tools"
	"github.com/torenander/teams-local-mcp/internal/tools/help"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
	"time"
)

// chatVerbsConfig holds dependencies for building chat domain verbs.
type chatVerbsConfig struct {
	retryCfg          graph.RetryConfig
	timeout           time.Duration
	cfg               config.Config
	m                 *observability.ToolMetrics
	tracer            trace.Tracer
	authMW            func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
	accountResolverMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
	readOnly          bool
}

// buildChatVerbs constructs and returns the ordered verb slice and registry
// pointer for the chat domain aggregate tool.
func buildChatVerbs(c chatVerbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
	registryPtr := &tools.VerbRegistry{}

	wrap := func(name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
		return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, audit.AuditWrap(name, auditOp, h)))))
	}
	wrapWrite := func(name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
		return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, ReadOnlyGuard(name, c.readOnly, audit.AuditWrap(name, auditOp, h))))))
	}

	rc := c.retryCfg
	_ = wrapWrite // will be used when send_message is implemented

	verbs := []tools.Verb{
		help.NewHelpVerb(registryPtr),
		buildListChatsVerb(c, rc, wrap),
		buildGetChatVerb(c, rc, wrap),
		buildListChatMessagesVerb(c, rc, wrap),
		buildGetChatMessageVerb(c, rc, wrap),
	}

	if c.cfg.TeamsManageEnabled {
		verbs = append(verbs, buildSendChatMessageVerb(c, rc, wrapWrite))
	}

	return verbs, registryPtr
}

// buildListChatsVerb constructs the list_chats Verb.
func buildListChatsVerb(c chatVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_chats",
		Summary:     "list your 1:1 and group chats",
		Description: "Returns a list of chats the authenticated user is a member of, including 1:1 chats, group chats, and meeting chats. Each entry includes the chat topic, type, and last updated time.",
		Handler:     wrap("chat.list_chats", "read", tools.NewHandleListChats(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
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
func buildGetChatVerb(c chatVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_chat",
		Summary:     "get details for a specific chat by ID",
		Description: "Returns details for a specific chat including topic, type, and members.",
		Handler:     wrap("chat.get_chat", "read", tools.NewHandleGetChat(rc, c.timeout)),
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
				mcp.Description("Account label or UPN to use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildListChatMessagesVerb constructs the list_messages Verb for chat.
func buildListChatMessagesVerb(c chatVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_messages",
		Summary:     "list messages in a chat",
		Description: "Returns messages from a specific chat, ordered by creation time. Each message includes sender, body content, and timestamps.",
		Handler:     wrap("chat.list_messages", "read", tools.NewHandleListChatMessages(rc, c.timeout)),
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
				mcp.Description("Account label or UPN to use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetChatMessageVerb constructs the get_message Verb for chat.
func buildGetChatMessageVerb(c chatVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_message",
		Summary:     "get a specific message from a chat",
		Description: "Returns full details for a single chat message by ID, including body content, sender, and timestamps.",
		Handler:     wrap("chat.get_message", "read", tools.NewHandleGetChatMessage(rc, c.timeout)),
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
				mcp.Description("Account label or UPN to use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildSendChatMessageVerb constructs the send_message Verb for chat (TeamsManageEnabled-gated).
func buildSendChatMessageVerb(c chatVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "send_message",
		Summary:     "send a message to a chat",
		Description: "Sends a new message to a chat. The message appears in the chat for all members. Requires TEAMS_MANAGE_ENABLED=true.",
		Handler:     wrapWrite("chat.send_message", "write", tools.NewHandleSendChatMessage(rc, c.timeout)),
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
				mcp.Description("Account label or UPN to use."),
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
