package protocol

import "encoding/json"

type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type ListPromptsResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

type GetPromptRequest struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

type GetPromptResult struct {
	Messages []PromptMessage `json:"messages"`
}

type PromptMessage struct {
	Role    string        `json:"role"`
	Content PromptContent `json:"content"`
}

type PromptContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func NewListPromptsRequest(id json.RawMessage) *Request {
	return NewRequest(id, "prompts/list", nil)
}

func NewGetPromptRequest(id json.RawMessage, name string, args map[string]string) *Request {
	params, _ := json.Marshal(GetPromptRequest{Name: name, Arguments: args}) //nolint:errchkjson
	return NewRequest(id, "prompts/get", params)
}
