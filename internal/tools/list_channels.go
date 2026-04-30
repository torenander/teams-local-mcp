// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides the list_channels handler, which retrieves channels for a
// specific team via GET /teams/{id}/channels on the Microsoft Graph API.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/torenander/teams-local-mcp/internal/graph"
	"github.com/torenander/teams-local-mcp/internal/logging"
	"github.com/torenander/teams-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// NewHandleListChannels creates a tool handler that lists channels in a team.
//
// Side effects: calls GET /teams/{id}/channels on the Microsoft Graph API.
func NewHandleListChannels(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()
		logger.Debug("tool called")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		teamID, err := request.RequireString("team_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := validate.ValidateResourceID(teamID, "team_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var resp models.ChannelCollectionResponseable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			resp, gErr = client.Teams().ByTeamId(teamID).Channels().Get(timeoutCtx, nil)
			return gErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "graph API call failed",
				"error", graph.FormatGraphError(err),
				"duration", time.Since(start))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		channels := resp.GetValue()
		results := make([]map[string]any, 0, len(channels))
		for _, ch := range channels {
			results = append(results, serializeChannel(ch))
		}

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(results))
			return mcp.NewToolResultText(FormatChannelsText(results)), nil
		}

		jsonBytes, err := json.Marshal(results)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize channels: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start), "count", len(results))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// serializeChannel extracts key fields from a Graph Channel model.
func serializeChannel(ch models.Channelable) map[string]any {
	m := map[string]any{
		"id":          graph.SafeStr(ch.GetId()),
		"displayName": graph.SafeStr(ch.GetDisplayName()),
		"description": graph.SafeStr(ch.GetDescription()),
	}
	if mt := ch.GetMembershipType(); mt != nil {
		m["membershipType"] = mt.String()
	}
	return m
}
