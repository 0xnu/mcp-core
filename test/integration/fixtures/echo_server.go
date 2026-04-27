package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

var tools = []map[string]any{
	{
		"name":        "echo",
		"description": "Echo back the input text",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string", "description": "Text to echo"},
			},
			"required": []string{"text"},
		},
	},
	{
		"name":        "add",
		"description": "Add two numbers",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number", "description": "First number"},
				"b": map[string]any{"type": "number", "description": "Second number"},
			},
			"required": []string{"a", "b"},
		},
	},
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type params struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *errorObj       `json:"error,omitempty"`
}

type errorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolResult struct {
	Content []contentItem `json:"content"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func handleRequest(req request) response {
	if req.Method == "tools/list" {
		return response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": tools},
		}
	}

	if req.Method == "tools/call" {
		var p params
		_ = json.Unmarshal(req.Params, &p)

		switch p.Name {
		case "echo":
			var echoArgs struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(p.Arguments, &echoArgs)
			return response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: toolResult{
					Content: []contentItem{{Type: "text", Text: "Echo: " + echoArgs.Text}},
				},
			}
		case "add":
			var addArgs struct {
				A float64 `json:"a"`
				B float64 `json:"b"`
			}
			_ = json.Unmarshal(p.Arguments, &addArgs)
			sum := addArgs.A + addArgs.B
			text := strconv.FormatFloat(sum, 'f', -1, 64)
			if sum == math.Trunc(sum) && !math.IsInf(sum, 0) {
				text = strconv.FormatInt(int64(sum), 10)
			}
			return response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: toolResult{
					Content: []contentItem{{Type: "text", Text: text}},
				},
			}
		default:
			return response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &errorObj{Code: -32601, Message: "Tool not found: " + p.Name},
			}
		}
	}

	return response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error:   &errorObj{Code: -32601, Message: "Method not found: " + req.Method},
	}
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(os.Stderr, "JSON parse error: %v\n", err)
			continue
		}
		resp := handleRequest(req)
		data, _ := json.Marshal(resp) //nolint:errchkjson
		fmt.Println(string(data))
	}
}
