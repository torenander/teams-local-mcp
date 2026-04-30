// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides the list_channel_messages handler (teams.list_messages verb),
// which retrieves messages from a team channel via
// GET /teams/{teamId}/channels/{channelId}/messages on the Microsoft Graph API.
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
	"github.com/microsoftgraph/msgraph-sdk-go/teams"
)

// NewHandleListChannelMessages creates a tool handler that lists messages in
// a team channel.
//
// Side effects: calls GET /teams/{teamId}/channels/{channelId}/messages.
func NewHandleListChannelMessages(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return mcp.NewToolResultError("missing required parameter: team_id"), nil
		}
		if err := validate.ValidateResourceID(teamID, "team_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		channelID, err := request.RequireString("channel_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: channel_id"), nil
		}
		if err := validate.ValidateResourceID(channelID, "channel_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		maxResults := int32(request.GetFloat("max_results", 25))
		if maxResults < 1 {
			maxResults = 25
		}

		qp := &teams.ItemChannelsItemMessagesRequestBuilderGetQueryParameters{
			Top: &maxResults,
		}
		cfg := &teams.ItemChannelsItemMessagesRequestBuilderGetRequestConfiguration{
			QueryParameters: qp,
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var resp models.ChatMessageCollectionResponseable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			resp, gErr = client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().Get(timeoutCtx, cfg)
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

		messages := resp.GetValue()
		results := make([]map[string]any, 0, len(messages))
		for _, msg := range messages {
			results = append(results, serializeChatMessage(msg))
		}

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(results))
			return mcp.NewToolResultText(FormatChatMessagesText(results)), nil
		}

		jsonBytes, err := json.Marshal(results)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize messages: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start), "count", len(results))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
