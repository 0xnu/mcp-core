package protocol

import "encoding/json"

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type ListResourcesResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

type ListResourceTemplatesResult struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	NextCursor        string             `json:"nextCursor,omitempty"`
}

type ReadResourceRequest struct {
	URI string `json:"uri"`
}

type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type SubscribeRequest struct {
	URI string `json:"uri"`
}

type UnsubscribeRequest struct {
	URI string `json:"uri"`
}

func NewListResourcesRequest(id json.RawMessage, _ string) *Request {
	return NewRequest(id, "resources/list", nil)
}

func NewReadResourceRequest(id json.RawMessage, uri string) *Request {
	params, _ := json.Marshal(ReadResourceRequest{URI: uri}) //nolint:errchkjson
	return NewRequest(id, "resources/read", params)
}

func NewListResourceTemplatesRequest(id json.RawMessage) *Request {
	return NewRequest(id, "resources/templates/list", nil)
}

func NewSubscribeRequest(id json.RawMessage, uri string) *Request {
	params, _ := json.Marshal(SubscribeRequest{URI: uri}) //nolint:errchkjson
	return NewRequest(id, "resources/subscribe", params)
}
