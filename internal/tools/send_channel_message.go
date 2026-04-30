// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides the send_channel_message handler (teams.send_message verb),
// which sends a message to a team channel via
// POST /teams/{teamId}/channels/{channelId}/messages on the Microsoft Graph API.
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/torenander/teams-local-mcp/internal/graph"
	"github.com/torenander/teams-local-mcp/internal/logging"
	"github.com/torenander/teams-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// NewHandleSendChannelMessage creates a tool handler that sends a message to
// a team channel.
//
// Side effects: calls POST /teams/{teamId}/channels/{channelId}/messages.
func NewHandleSendChannelMessage(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()
		logger.Debug("tool called")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
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

		body, err := request.RequireString("body")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: body"), nil
		}

		contentType := request.GetString("content_type", "text")

		msg := models.NewChatMessage()
		msgBody := models.NewItemBody()
		msgBody.SetContent(&body)
		if contentType == "html" {
			ct := models.HTML_BODYTYPE
			msgBody.SetContentType(&ct)
		} else {
			ct := models.TEXT_BODYTYPE
			msgBody.SetContentType(&ct)
		}
		msg.SetBody(msgBody)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var newMessageID string
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			sent, gErr := client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().Post(timeoutCtx, msg, nil)
			if gErr != nil {
				return gErr
			}
			newMessageID = graph.SafeStr(sent.GetId())
			return nil
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "send channel message failed", "error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.InfoContext(ctx, "channel message sent",
			"team_id", teamID,
			"channel_id", channelID,
			"message_id", newMessageID,
			"duration", time.Since(start))

		response := fmt.Sprintf("Message sent to channel.\nMessage ID: %s\nTeam ID: %s\nChannel ID: %s", newMessageID, teamID, channelID)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}
