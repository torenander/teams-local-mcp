// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides the list_chat_messages handler (chat.list_messages verb),
// which retrieves messages from a specific chat via
// GET /me/chats/{id}/messages on the Microsoft Graph API.
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
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// NewHandleListChatMessages creates a tool handler that lists messages in a
// specific chat.
//
// Side effects: calls GET /me/chats/{id}/messages on the Microsoft Graph API.
func NewHandleListChatMessages(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		chatID, err := request.RequireString("chat_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := validate.ValidateResourceID(chatID, "chat_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		maxResults := int32(request.GetFloat("max_results", 25))
		if maxResults < 1 {
			maxResults = 25
		}

		qp := &users.ItemChatsItemMessagesRequestBuilderGetQueryParameters{
			Top: &maxResults,
		}
		cfg := &users.ItemChatsItemMessagesRequestBuilderGetRequestConfiguration{
			QueryParameters: qp,
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var resp models.ChatMessageCollectionResponseable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			resp, gErr = client.Me().Chats().ByChatId(chatID).Messages().Get(timeoutCtx, cfg)
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

// serializeChatMessage extracts key fields from a Graph ChatMessage model.
func serializeChatMessage(msg models.ChatMessageable) map[string]any {
	m := map[string]any{
		"id": graph.SafeStr(msg.GetId()),
	}

	if t := msg.GetCreatedDateTime(); t != nil {
		m["createdDateTime"] = t.Format(time.RFC3339)
	}

	// Extract sender display name.
	if from := msg.GetFrom(); from != nil {
		if user := from.GetUser(); user != nil {
			m["from"] = graph.SafeStr(user.GetDisplayName())
		}
	}

	// Extract body content.
	if body := msg.GetBody(); body != nil {
		m["body"] = graph.SafeStr(body.GetContent())
		if ct := body.GetContentType(); ct != nil {
			m["bodyContentType"] = ct.String()
		}
	}

	if mt := msg.GetMessageType(); mt != nil {
		m["messageType"] = mt.String()
	}

	return m
}
