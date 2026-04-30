// Package server registers all MCP tool handlers on the server instance.
//
// This file wires the four domain aggregate tools (chat, teams, account,
// system) with their middleware chains and verb registries.
package server

import (
	"log/slog"
	"time"

	"github.com/torenander/teams-local-mcp/internal/auth"
	"github.com/torenander/teams-local-mcp/internal/config"
	"github.com/torenander/teams-local-mcp/internal/graph"
	"github.com/torenander/teams-local-mcp/internal/observability"
	"github.com/torenander/teams-local-mcp/internal/tools"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// RegisterTools registers all MCP tool handlers on the given server.
//
// Parameters:
//   - s: the MCP server instance.
//   - retryCfg: retry configuration for Graph API calls.
//   - timeout: max duration for a single Graph API request.
//   - m: observability metrics.
//   - t: OTEL tracer.
//   - readOnly: when true, write handlers are blocked.
//   - authMW: authentication middleware factory.
//   - registry: account registry for multi-account resolution.
//   - cfg: server configuration.
//   - cred: default account authenticator (for complete_auth).
//
// Side effects: registers tool handlers on the server.
func RegisterTools(s *mcpserver.MCPServer, retryCfg graph.RetryConfig, timeout time.Duration, m *observability.ToolMetrics, t trace.Tracer, readOnly bool, authMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc, registry *auth.AccountRegistry, cfg config.Config, cred auth.Authenticator) {
	accountResolverMW := auth.AccountResolver(registry)

	// Account domain.
	accVerbs, accRegistry := buildAccountVerbs(accountVerbsConfig{
		registry: registry,
		cfg:      cfg,
		m:        m,
		tracer:   t,
		authMW:   authMW,
	})
	populatedAcc := tools.RegisterDomainTool(s, tools.DomainToolConfig{
		Domain:          "account",
		Intro:           "Account management for Microsoft accounts connected to the Teams MCP server.",
		Verbs:           accVerbs,
		ToolAnnotations: accountToolAnnotations(),
	})
	*accRegistry = populatedAcc

	// System domain.
	sysVerbs, sysRegistry := buildSystemVerbs(systemVerbsConfig{
		cfg:       cfg,
		registry:  registry,
		startTime: time.Now(),
		m:         m,
		tracer:    t,
		authMW:    authMW,
		cred:      cred,
	})
	populated := tools.RegisterDomainTool(s, tools.DomainToolConfig{
		Domain:          "system",
		Intro:           "System diagnostics and authentication utilities for the Teams MCP server.",
		Verbs:           sysVerbs,
		ToolAnnotations: systemToolAnnotations(),
	})
	*sysRegistry = populated

	toolCount := 2

	// Chat domain (when Teams is enabled).
	if cfg.TeamsEnabled {
		chatVerbs, chatRegistry := buildChatVerbs(verbsConfig{
			retryCfg:          retryCfg,
			timeout:           timeout,
			cfg:               cfg,
			m:                 m,
			tracer:            t,
			authMW:            authMW,
			accountResolverMW: accountResolverMW,
			readOnly:          readOnly,
		})
		populatedChat := tools.RegisterDomainTool(s, tools.DomainToolConfig{
			Domain:          "chat",
			Intro:           "Chat operations for Microsoft Teams via Microsoft Graph.",
			Verbs:           chatVerbs,
			ToolAnnotations: chatToolAnnotations(),
		})
		*chatRegistry = populatedChat
		toolCount++
	}

	// Teams domain (when Teams is enabled).
	if cfg.TeamsEnabled {
		teamsVerbs, teamsRegistry := buildTeamsVerbs(verbsConfig{
			retryCfg:          retryCfg,
			timeout:           timeout,
			cfg:               cfg,
			m:                 m,
			tracer:            t,
			authMW:            authMW,
			accountResolverMW: accountResolverMW,
			readOnly:          readOnly,
		})
		populatedTeams := tools.RegisterDomainTool(s, tools.DomainToolConfig{
			Domain:          "teams",
			Intro:           "Teams and channel operations for Microsoft Teams via Microsoft Graph.",
			Verbs:           teamsVerbs,
			ToolAnnotations: teamsToolAnnotations(),
		})
		*teamsRegistry = populatedTeams
		toolCount++
	}

	slog.Info("tool registration complete", "tools", toolCount)
}
