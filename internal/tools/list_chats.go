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
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
	"github.com/torenander/teams-local-mcp/internal/graph"
	"github.com/torenander/teams-local-mcp/internal/logging"
)

const (
	// listChatsPageSize is the Graph API maximum $top for GET /me/chats.
	listChatsPageSize int32 = 50
	// listChatsDefaultMax is the default result cap when neither max_results
	// nor since is given.
	listChatsDefaultMax = 25
	// listChatsSinceMax is the default result cap when since is given and
	// max_results is not; since is then the effective bound.
	listChatsSinceMax = 1000
)

// NewHandleListChats creates a tool handler that lists the authenticated
// user's chats by calling GET /me/chats via the Graph SDK.
//
// Chats are ordered by the time of their most recent message (newest first),
// with lastMessagePreview expanded. Results are paged via @odata.nextLink
// until max_results is reached, the pages run out, or — when since is given —
// a chat whose last message is older than since is reached.
//
// Note: Graph's lastUpdatedDateTime reflects renames and membership changes,
// not new messages. Use lastMessageDateTime to detect activity.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for each Graph API call.
//
// Returns a handler function compatible with the MCP server's verb dispatch.
//
// Side effects: calls GET /me/chats (one or more pages) on the Microsoft Graph API.
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

		var since *time.Time
		if raw := strings.TrimSpace(request.GetString("since", "")); raw != "" {
			t, perr := parseSince(raw)
			if perr != nil {
				return mcp.NewToolResultError(perr.Error()), nil
			}
			since = &t
		}

		defaultMax := listChatsDefaultMax
		if since != nil {
			defaultMax = listChatsSinceMax
		}
		maxResults := int(request.GetFloat("max_results", float64(defaultMax)))
		if maxResults < 1 {
			maxResults = defaultMax
		}

		pageSize := listChatsPageSize
		if int32(maxResults) < pageSize {
			pageSize = int32(maxResults)
		}

		logger.Debug("graph API request",
			"endpoint", "GET /me/chats",
			"top", pageSize,
			"max_results", maxResults,
			"since", since)

		reqCfg := &users.ItemChatsRequestBuilderGetRequestConfiguration{
			QueryParameters: &users.ItemChatsRequestBuilderGetQueryParameters{
				Top:     &pageSize,
				Expand:  []string{"lastMessagePreview"},
				Orderby: []string{"lastMessagePreview/createdDateTime desc"},
			},
		}

		results := make([]map[string]any, 0, maxResults)
		nextLink := ""
		pages := 0
		reachedSince := false

		for {
			var resp models.ChatCollectionResponseable
			err = graph.RetryGraphCall(ctx, retryCfg, func() error {
				timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
				defer cancel()
				var graphErr error
				if nextLink == "" {
					resp, graphErr = client.Me().Chats().Get(timeoutCtx, reqCfg)
				} else {
					resp, graphErr = client.Me().Chats().WithUrl(nextLink).Get(timeoutCtx, nil)
				}
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
			pages++

			for _, chat := range resp.GetValue() {
				if since != nil {
					last := lastMessageTime(chat)
					// Sorted newest-first: the first chat older than since
					// (or with no message at all) ends the listing.
					if last == nil || last.Before(*since) {
						reachedSince = true
						break
					}
				}
				results = append(results, serializeChat(chat))
				if len(results) >= maxResults {
					break
				}
			}

			if reachedSince || len(results) >= maxResults {
				break
			}
			link := resp.GetOdataNextLink()
			if link == nil || *link == "" {
				break
			}
			nextLink = *link
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"count", len(results),
			"pages", pages)

		if outputMode == "text" {
			return mcp.NewToolResultText(FormatChatsText(results)), nil
		}

		jsonBytes, err := json.Marshal(results)
		if err != nil {
			logger.ErrorContext(ctx, "json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize chats: %s", err.Error())), nil
		}
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// parseSince accepts an RFC 3339 timestamp or a YYYY-MM-DD date (interpreted
// as midnight UTC).
func parseSince(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid since %q: use RFC 3339 (2026-09-22T00:00:00Z) or YYYY-MM-DD", raw)
}

// lastMessageTime returns the creation time of the chat's most recent message,
// or nil when the preview is absent.
func lastMessageTime(chat models.Chatable) *time.Time {
	p := chat.GetLastMessagePreview()
	if p == nil {
		return nil
	}
	return p.GetCreatedDateTime()
}

// serializeChat extracts key fields from a Graph Chat model into a plain map.
// When lastMessagePreview is present (expanded), lastMessageDateTime,
// lastMessageFrom and lastMessageType are included.
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
	if p := chat.GetLastMessagePreview(); p != nil {
		if t := p.GetCreatedDateTime(); t != nil {
			m["lastMessageDateTime"] = t.Format(time.RFC3339)
		}
		if mt := p.GetMessageType(); mt != nil {
			m["lastMessageType"] = mt.String()
		}
		if from := p.GetFrom(); from != nil {
			if u := from.GetUser(); u != nil {
				m["lastMessageFrom"] = graph.SafeStr(u.GetDisplayName())
			}
		}
	}
	return m
}
