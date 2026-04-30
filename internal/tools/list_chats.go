// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides the list_chats handler, which retrieves the authenticated
// user's chats via GET /me/chats on the Microsoft Graph API.
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

// NewHandleListChats creates a tool handler that lists the authenticated
// user's chats by calling GET /me/chats via the Graph SDK.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a handler function compatible with the MCP server's verb dispatch.
//
// Side effects: calls GET /me/chats on the Microsoft Graph API.
func NewHandleListChats(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		maxResultsFloat := request.GetFloat("max_results", 25)
		maxResults := int32(maxResultsFloat)
		if maxResults < 1 {
			maxResults = 25
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request",
			"endpoint", "GET /me/chats",
			"top", maxResults)

		var resp models.ChatCollectionResponseable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var graphErr error
			resp, graphErr = client.Me().Chats().Get(timeoutCtx, nil)
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

		chats := resp.GetValue()
		results := make([]map[string]any, 0, len(chats))
		for _, chat := range chats {
			results = append(results, serializeChat(chat))
		}

		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"count", len(results))
			return mcp.NewToolResultText(FormatChatsText(results)), nil
		}

		jsonBytes, err := json.Marshal(results)
		if err != nil {
			logger.ErrorContext(ctx, "json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize chats: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"count", len(results))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// serializeChat extracts key fields from a Graph Chat model into a plain map.
func serializeChat(chat models.Chatable) map[string]any {
	m := map[string]any{
		"id":       graph.SafeStr(chat.GetId()),
		"chatType": "",
		"topic":    graph.SafeStr(chat.GetTopic()),
	}
	if ct := chat.GetChatType(); ct != nil {
		m["chatType"] = ct.String()
	}
	if t := chat.GetLastUpdatedDateTime(); t != nil {
		m["lastUpdatedDateTime"] = t.Format(time.RFC3339)
	}
	return m
}
