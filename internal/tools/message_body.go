// Package tools provides MCP tool definitions and handler constructors for the
// Teams MCP Server.
//
// This file provides shared helpers for constructing Graph API request bodies.
package tools

import "github.com/microsoftgraph/msgraph-sdk-go/models"

// BuildChatMessageBody constructs a ChatMessage model with the given body and
// content type ("text" or "html").
func BuildChatMessageBody(body, contentType string) *models.ChatMessage {
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
	return msg
}
