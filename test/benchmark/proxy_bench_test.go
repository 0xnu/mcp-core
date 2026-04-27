package benchmark

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/0xnu/mcp-core/internal/cache"
	"github.com/0xnu/mcp-core/pkg/protocol"
)

func BenchmarkJSONRPCParse(b *testing.B) {
	data := []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}`)

	b.ResetTimer()
	for range b.N {
		_, err := protocol.ParseRequest(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONRPCMarshal(b *testing.B) {
	req := protocol.NewRequest(json.RawMessage(`"1"`), "tools/list", nil)

	b.ResetTimer()
	for range b.N {
		_, err := req.Marshal()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageDetection(b *testing.B) {
	requestData := []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}`)
	responseData := []byte(`{"jsonrpc":"2.0","id":"1","result":{"tools":[]}}`)
	notifData := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	b.ResetTimer()
	for range b.N {
		protocol.IsRequest(requestData)
		protocol.IsResponse(responseData)
		protocol.IsNotification(notifData)
	}
}

func BenchmarkCacheGetSet(b *testing.B) {
	schemaCache := cache.NewSchemaCache(3600*time.Second, 1000, 1024*1024)

	tools := []protocol.ToolDefinition{
		{Name: "tool1", Description: "First tool", InputSchema: json.RawMessage(`{}`)},
		{Name: "tool2", Description: "Second tool", InputSchema: json.RawMessage(`{}`)},
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			schemaCache.Set("backend", tools)
			schemaCache.Get("backend")
			schemaCache.Invalidate("backend")
		}
	})
}

func BenchmarkResponseCache(b *testing.B) {
	respCache := cache.NewResponseCache(cache.ResponseCacheConfig{
		TTL:      3600 * time.Second,
		MaxSize:  1000,
		MaxEntry: 1024 * 1024,
	})

	key := "tools/list:{}"
	data := json.RawMessage(`{"tools":[]}`)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			respCache.Set(key, data)
			respCache.Get(key)
		}
	})
}

func BenchmarkToolCallRequest(b *testing.B) {
	for range b.N {
		protocol.NewToolCallRequest(
			json.RawMessage(`"bench-id"`),
			"echo",
			json.RawMessage(`{"text":"hello"}`),
		)
	}
}

func BenchmarkToolResultUnmarshal(b *testing.B) {
	data := []byte(`{"content":[{"type":"text","text":"Echo: hello"}],"isError":false}`)

	b.ResetTimer()
	for range b.N {
		var result protocol.ToolResult
		if err := json.Unmarshal(data, &result); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConcurrentProtocolOps(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		req := protocol.NewRequest(json.RawMessage(`"1"`), "tools/list", nil)
		for pb.Next() {
			data, _ := req.Marshal()
			parsed, _ := protocol.ParseRequest(data)
			_ = parsed
		}
	})
}

func BenchmarkCacheConcurrent(b *testing.B) {
	schemaCache := cache.NewSchemaCache(3600*time.Second, 1000, 1024*1024)
	tools := []protocol.ToolDefinition{
		{Name: "bench-tool", InputSchema: json.RawMessage(`{}`)},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			schemaCache.Set("backend", tools)
			schemaCache.Get("backend")
		}
	})
}
