package protocol

import "encoding/json"

const JSONRPCVersion = "2.0"

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
}

type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type ErrorObject struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func NewRequest(id json.RawMessage, method string, params json.RawMessage) *Request {
	return &Request{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

func NewResponse(id json.RawMessage, result json.RawMessage) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
}

func NewErrorResponse(id json.RawMessage, code int, message string) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &ErrorObject{
			Code:    code,
			Message: message,
		},
	}
}

func NewNotification(method string, params json.RawMessage) *Notification {
	return &Notification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	}
}

func (r *Request) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Response) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (n *Notification) Marshal() ([]byte, error) {
	return json.Marshal(n)
}

func ParseRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func ParseResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func ParseNotification(data []byte) (*Notification, error) {
	var notif Notification
	if err := json.Unmarshal(data, &notif); err != nil {
		return nil, err
	}
	return &notif, nil
}

func IsRequest(data []byte) bool {
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	return msg.ID != nil && msg.Method != ""
}

func IsNotification(data []byte) bool {
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	return msg.ID == nil && msg.Method != ""
}

func IsResponse(data []byte) bool {
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	return msg.ID != nil && msg.Method == ""
}
