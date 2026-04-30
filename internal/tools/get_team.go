// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides the get_team handler, which retrieves details for a
// specific team via GET /teams/{id} on the Microsoft Graph API.
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
)

// NewHandleGetTeam creates a tool handler that retrieves details for a
// specific team by ID.
//
// Side effects: calls GET /teams/{id} on the Microsoft Graph API.
func NewHandleGetTeam(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var result map[string]any
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			team, gErr := client.Teams().ByTeamId(teamID).Get(timeoutCtx, nil)
			if gErr != nil {
				return gErr
			}
			result = serializeTeam(team)
			return nil
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "graph API call failed",
				"error", graph.FormatGraphError(err),
				"duration", time.Since(start))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		if outputMode == "text" {
			name, _ := result["displayName"].(string)
			desc, _ := result["description"].(string)
			text := fmt.Sprintf("Team: %s\nID: %s", name, result["id"])
			if desc != "" {
				text += fmt.Sprintf("\nDescription: %s", desc)
			}
			if line := AccountInfoLine(ctx); line != "" {
				text += "\n" + line
			}
			logger.Info("tool completed", "duration", time.Since(start))
			return mcp.NewToolResultText(text), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.ErrorContext(ctx, "json serialization failed", "error", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize result: %s", err.Error())), nil
		}
		logger.Info("tool completed", "duration", time.Since(start))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
