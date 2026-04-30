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
