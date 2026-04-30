// Package server provides verb builders for the teams domain aggregate tool.
//
// This file defines the teamsVerbsConfig, buildTeamsVerbs, and
// teamsToolAnnotations used to register teams/channel operations.
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

// teamsVerbsConfig holds dependencies for building teams domain verbs.
type teamsVerbsConfig struct {
	retryCfg          graph.RetryConfig
	timeout           time.Duration
	cfg               config.Config
	m                 *observability.ToolMetrics
	tracer            trace.Tracer
	authMW            func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
	accountResolverMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
	readOnly          bool
}

// buildTeamsVerbs constructs and returns the ordered verb slice and registry
// pointer for the teams domain aggregate tool.
func buildTeamsVerbs(c teamsVerbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
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
		buildListTeamsVerb(c, rc, wrap),
	}

	return verbs, registryPtr
}

// buildListTeamsVerb constructs the list_teams Verb.
func buildListTeamsVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_teams",
		Summary:     "list teams you are a member of",
		Description: "Returns a list of Microsoft Teams that the authenticated user has joined. Each entry includes the team display name, description, and ID.",
		Handler:     wrap("teams.list_teams", "read", tools.NewHandleListTeams(rc, c.timeout)),
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
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// teamsToolAnnotations returns the conservative aggregate MCP annotations for
// the teams domain tool.
func teamsToolAnnotations() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithTitleAnnotation("Teams"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	}
}
