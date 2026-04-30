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

	verbs := []tools.Verb{
		help.NewHelpVerb(registryPtr),
		buildListTeamsVerb(c, rc, wrap),
		buildGetTeamVerb(c, rc, wrap),
		buildListChannelsVerb(c, rc, wrap),
		buildListChannelMessagesVerb(c, rc, wrap),
	}

	if c.cfg.TeamsManageEnabled {
		verbs = append(verbs, buildSendChannelMessageVerb(c, rc, wrapWrite))
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
				mcp.Description(tools.AccountParamDescription),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetTeamVerb constructs the get_team Verb.
func buildGetTeamVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_team",
		Summary:     "get details for a specific team by ID",
		Description: "Returns details for a specific team including display name, description, and ID.",
		Handler:     wrap("teams.get_team", "read", tools.NewHandleGetTeam(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("team_id", mcp.Required(),
				mcp.Description("The unique identifier of the team."),
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

// buildListChannelsVerb constructs the list_channels Verb.
func buildListChannelsVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_channels",
		Summary:     "list channels in a team",
		Description: "Returns all channels in a specified team, including display name, description, and membership type.",
		Handler:     wrap("teams.list_channels", "read", tools.NewHandleListChannels(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("team_id", mcp.Required(),
				mcp.Description("The unique identifier of the team."),
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

// buildListChannelMessagesVerb constructs the list_messages Verb for teams channels.
func buildListChannelMessagesVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_messages",
		Summary:     "list messages in a team channel",
		Description: "Returns messages from a specific team channel, ordered by creation time.",
		Handler:     wrap("teams.list_messages", "read", tools.NewHandleListChannelMessages(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("team_id", mcp.Required(),
				mcp.Description("The unique identifier of the team."),
			),
			mcp.WithString("channel_id", mcp.Required(),
				mcp.Description("The unique identifier of the channel."),
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

// buildSendChannelMessageVerb constructs the send_message Verb for teams channels (TeamsManageEnabled-gated).
func buildSendChannelMessageVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "send_message",
		Summary:     "send a message to a team channel",
		Description: "Sends a new message to a team channel. The message is visible to all channel members. Requires TEAMS_MANAGE_ENABLED=true.",
		Handler:     wrapWrite("teams.send_message", "write", tools.NewHandleSendChannelMessage(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("team_id", mcp.Required(),
				mcp.Description("The unique identifier of the team."),
			),
			mcp.WithString("channel_id", mcp.Required(),
				mcp.Description("The unique identifier of the channel."),
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
