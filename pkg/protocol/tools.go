package protocol

import "encoding/json"

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	Content []ToolResultContent `json:"content"`
	IsError bool                `json:"isError,omitempty"`
}

type ToolResultContent struct {
	Type string          `json:"type"`
	Text string          `json:"text,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ListToolsRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

type ListToolsResult struct {
	Tools      []ToolDefinition `json:"tools"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func NewToolCallRequest(id json.RawMessage, name string, args json.RawMessage) *Request {
	params, _ := json.Marshal(CallToolRequest{Name: name, Arguments: args}) //nolint:errchkjson
	return NewRequest(id, "tools/call", params)
}

func NewListToolsRequest(id json.RawMessage, cursor string) *Request {
	params, _ := json.Marshal(ListToolsRequest{Cursor: cursor}) //nolint:errchkjson
	return NewRequest(id, "tools/list", params)
}
