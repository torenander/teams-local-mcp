// Package server provides shared configuration and middleware helpers for
// domain verb builders.
package server

import (
	"github.com/torenander/teams-local-mcp/internal/audit"
	"github.com/torenander/teams-local-mcp/internal/config"
	"github.com/torenander/teams-local-mcp/internal/graph"
	"github.com/torenander/teams-local-mcp/internal/observability"
	"github.com/torenander/teams-local-mcp/internal/tools"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
	"time"
)

// verbsConfig holds shared dependencies for building any domain's verbs.
type verbsConfig struct {
	retryCfg          graph.RetryConfig
	timeout           time.Duration
	cfg               config.Config
	m                 *observability.ToolMetrics
	tracer            trace.Tracer
	authMW            func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
	accountResolverMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
	readOnly          bool
}

// wrapRead builds a read-path middleware chain for a verb handler.
func wrapRead(c verbsConfig, name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
	return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, audit.AuditWrap(name, auditOp, h)))))
}

// wrapWrite builds a write-path middleware chain (includes ReadOnlyGuard).
func wrapWrite(c verbsConfig, name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
	return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, ReadOnlyGuard(name, c.readOnly, audit.AuditWrap(name, auditOp, h))))))
}
