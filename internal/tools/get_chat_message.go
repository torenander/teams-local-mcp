// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides the get_chat_message handler (chat.get_message verb),
// which retrieves a single message from a chat via
// GET /me/chats/{chatId}/messages/{messageId} on the Microsoft Graph API.
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

// NewHandleGetChatMessage creates a tool handler that retrieves a single
// message from a chat by ID.
//
// Side effects: calls GET /me/chats/{chatId}/messages/{messageId}.
func NewHandleGetChatMessage(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return mcp.NewToolResultError("missing required parameter: chat_id"), nil
		}
		if err := validate.ValidateResourceID(chatID, "chat_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		messageID, err := request.RequireString("message_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: message_id"), nil
		}
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var result map[string]any
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			msg, gErr := client.Me().Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).Get(timeoutCtx, nil)
			if gErr != nil {
				return gErr
			}
			result = serializeChatMessage(msg)
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
			from, _ := result["from"].(string)
			date, _ := result["createdDateTime"].(string)
			body, _ := result["body"].(string)
			text := fmt.Sprintf("From: %s\nDate: %s\n\n%s", from, date, body)
			if line := AccountInfoLine(ctx); line != "" {
				text += "\n\n" + line
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
