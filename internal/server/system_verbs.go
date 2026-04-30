// Package server provides verb builders for the system domain aggregate tool.
//
// This file defines the systemVerbsConfig, buildSystemVerbs, and
// systemToolAnnotations used to register system operations (status, help).
package server

import (
	"time"

	"github.com/torenander/teams-local-mcp/internal/audit"
	"github.com/torenander/teams-local-mcp/internal/auth"
	"github.com/torenander/teams-local-mcp/internal/config"
	"github.com/torenander/teams-local-mcp/internal/observability"
	"github.com/torenander/teams-local-mcp/internal/tools"
	"github.com/torenander/teams-local-mcp/internal/tools/help"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// systemVerbsConfig holds all dependencies needed to construct the system
// domain verb slice.
type systemVerbsConfig struct {
	cfg       config.Config
	registry  *auth.AccountRegistry
	startTime time.Time
	m         *observability.ToolMetrics
	tracer    trace.Tracer
	authMW    func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
	cred      auth.Authenticator
}

// buildSystemVerbs constructs and returns the ordered verb slice and registry
// pointer for the system domain aggregate tool.
func buildSystemVerbs(c systemVerbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
	empty := make(tools.VerbRegistry)
	registryPtr := &empty

	statusHandler := observability.WithObservability(
		"system.status", c.m, c.tracer,
		audit.AuditWrap("system.status", "read", tools.HandleStatus(c.cfg, c.registry, c.startTime)),
	)
	statusVerb := tools.Verb{
		Name:        "status",
		Summary:     "return server health: version, accounts, uptime, config (no Graph call)",
		Description: "Returns the server's current health state: binary version, registered accounts with their connection state, server uptime, and active configuration flags. No Microsoft Graph call is made.",
		Handler:     tools.Handler(statusHandler),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}

	verbs := []tools.Verb{
		help.NewHelpVerb(registryPtr),
		statusVerb,
	}

	return verbs, registryPtr
}

// systemToolAnnotations returns the conservative aggregate MCP annotations for
// the system domain tool.
func systemToolAnnotations() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithTitleAnnotation("System"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	}
}
