// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides the list_teams handler, which retrieves the teams the
// authenticated user has joined via GET /me/joinedTeams on the Microsoft
// Graph API.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/torenander/teams-local-mcp/internal/graph"
	"github.com/torenander/teams-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// NewHandleListTeams creates a tool handler that lists the teams the
// authenticated user has joined by calling GET /me/joinedTeams via the
// Graph SDK.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a handler function compatible with the MCP server's verb dispatch.
//
// Side effects: calls GET /me/joinedTeams on the Microsoft Graph API.
func NewHandleListTeams(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request",
			"endpoint", "GET /me/joinedTeams")

		var resp models.TeamCollectionResponseable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var graphErr error
			resp, graphErr = client.Me().JoinedTeams().Get(timeoutCtx, nil)
			return graphErr
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

		teams := resp.GetValue()
		results := make([]map[string]any, 0, len(teams))
		for _, team := range teams {
			results = append(results, serializeTeam(team))
		}

		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"count", len(results))
			return mcp.NewToolResultText(FormatTeamsText(results)), nil
		}

		jsonBytes, err := json.Marshal(results)
		if err != nil {
			logger.ErrorContext(ctx, "json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize teams: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"count", len(results))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// serializeTeam extracts key fields from a Graph Team model into a plain map.
func serializeTeam(team models.Teamable) map[string]any {
	return map[string]any{
		"id":          graph.SafeStr(team.GetId()),
		"displayName": graph.SafeStr(team.GetDisplayName()),
		"description": graph.SafeStr(team.GetDescription()),
	}
}
