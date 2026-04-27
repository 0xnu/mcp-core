package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/0xnu/mcp-core/internal/config"
)

const hubAddr = "http://127.0.0.1:9020"

type CLI struct {
	stdout *tabwriter.Writer
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cli := &CLI{
		stdout: tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0),
	}

	cmd := os.Args[1]
	os.Args = os.Args[1:]

	commands := map[string]func(){
		"init":     cmdInit,
		"start":    cmdStart,
		"stop":     cmdStop,
		"validate": cmdValidate,
		"reload":   cmdReload,
		"drain":    cmdDrain,
		"version":  cmdVersion,
		"status":   cli.cmdStatus,
		"trace":    cli.cmdTrace,
	}

	fn, ok := commands[cmd]
	if !ok {
		fmt.Printf("unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
	fn()
}

func printUsage() {
	fmt.Println(`corectl - mcp-core command-line interface

Usage:
  corectl init       Auto-discover MCP servers, scaffold config
  corectl start      Launch the mcp-core daemon
  corectl stop       Graceful shutdown
  corectl status     Connected backends, cache stats, latency
  corectl trace      Live request/response inspection per tool
  corectl validate   Check config syntax and backend reachability
  corectl reload     Hot-reload config without dropping connections
  corectl drain      Gracefully remove a backend from rotation
  corectl version    Server and CLI version, MCP spec version`)
}

func cmdInit() {
	fmt.Println("Scanning for existing MCP configurations...")

	detected, err := config.DetectExistingConfigs()
	if err != nil {
		log.Fatalf("detection error: %v", err)
	}

	if len(detected) > 0 {
		fmt.Printf("Found %d existing config(s):\n", len(detected))
		for _, d := range detected {
			fmt.Printf("  - %s\n", d)
		}
	}

	cfg := config.ScaffoldConfig()
	path := config.DefaultConfigPath()

	if err := cfg.Save(path); err != nil {
		log.Fatalf("save config: %v", err)
	}

	fmt.Printf("Scaffolded config written to %s\n", path)
	fmt.Println("Run 'corectl start' to launch mcp-core")
}

func cmdStart() {
	fmt.Println("Starting mcp-core daemon...")
	fmt.Println("Run the mcp-core binary directly:")
	fmt.Println("  mcp-core")
}

func cmdStop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hubAddr+"/shutdown", nil)
	if err != nil {
		cancel()
		log.Fatalf("create stop request: %v", err) //nolint:gocritic
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		log.Fatalf("stop error: %v", err)
	}
	defer resp.Body.Close()
	fmt.Printf("mcp-core stopped (%d)\n", resp.StatusCode)
}

func (cli *CLI) cmdStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hubAddr+"/health", nil)
	if err != nil {
		cancel()
		log.Fatalf("create status request: %v", err) //nolint:gocritic
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		log.Fatalf("status error (is mcp-core running?): %v", err)
	}
	defer resp.Body.Close()

	var status map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		log.Fatalf("decode response: %v", err)
	}

	fmt.Println("mcp-core status:")
	fmt.Fprintf(cli.stdout, "Status:\t%s\n", status["status"])
	fmt.Fprintf(cli.stdout, "Version:\t%s\n", status["version"])
	fmt.Fprintf(cli.stdout, "Backends:\t%.0f\n", status["backends"])
	fmt.Fprintf(cli.stdout, "Streams:\t%.0f\n", status["streams"])
	fmt.Fprintf(cli.stdout, "Requests:\t%.0f\n", status["requestsTotal"])
	fmt.Fprintf(cli.stdout, "Failed:\t%.0f\n", status["requestsFailed"])
	fmt.Fprintf(cli.stdout, "Circuit Breaks:\t%.0f\n", status["circuitBreaks"])
	cli.stdout.Flush()
}

func (cli *CLI) cmdTrace() {
	fmt.Println("Connecting to mcp-core trace stream...")
	fmt.Println()

	scanner, endpoint := connectSSE()
	fmt.Printf("Session established: %s\n", endpoint)
	fmt.Println("Listening for tool calls...")
	fmt.Println()

	go traceEvents(scanner)

	fmt.Println("Press Ctrl+C to stop tracing")
	waitForInterrupt()
}

func connectSSE() (*bufio.Scanner, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hubAddr+"/sse", nil)
	if err != nil {
		cancel()
		log.Fatalf("create SSE request: %v", err) //nolint:gocritic
	}

	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // body stays open for scanner used in traceEvents goroutine
	if err != nil {
		cancel()
		log.Fatalf("SSE connect error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 65536), 65536)

	var endpoint string
	for scanner.Scan() {
		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			endpoint = after
			if strings.Contains(endpoint, "/sse?session=") {
				break
			}
		}
	}
	return scanner, endpoint
}

func traceEvents(scanner *bufio.Scanner) {
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var respData map[string]any
		if err := json.Unmarshal([]byte(line), &respData); err == nil {
			if method, ok := respData["method"]; ok {
				fmt.Printf("[REQUEST] method=%s id=%v\n", method, respData["id"])
			}
			if result, ok := respData["result"]; ok {
				fmt.Printf("[RESPONSE] id=%v result=%s\n", respData["id"], truncateJSON(result, 100))
			}
			if errData, ok := respData["error"]; ok {
				fmt.Printf("[ERROR] id=%v error=%s\n", respData["id"], truncateJSON(errData, 100))
			}
		}
	}
}

func truncateJSON(v any, maxLen int) string {
	data, _ := json.Marshal(v) //nolint:errchkjson
	s := string(data)
	if len(s) > maxLen {
		s = s[:maxLen] + "..."
	}
	return s
}

func cmdValidate() {
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	fmt.Println("Config is valid")

	for _, be := range cfg.Backends {
		fmt.Printf("  - %s (%s)", be.Name, be.Type)
		switch be.Type {
		case "stdio":
			fmt.Printf(": %s %v", be.Command, be.Args)
		case "sse":
			fmt.Printf(": %s", be.URL)
		}
		fmt.Println()
	}
}

func cmdReload() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hubAddr+"/reload", nil)
	if err != nil {
		cancel()
		log.Fatalf("create reload request: %v", err) //nolint:gocritic
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		log.Fatalf("reload error: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatalf("decode reload response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("reload failed: %v", result["error"])
	}

	fmt.Printf("Config reloaded successfully\n")
}

func cmdDrain() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: corectl drain <backend-name>")
		os.Exit(1)
	}

	backendName := os.Args[1]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body := fmt.Sprintf(`{"backend":"%s"}`, backendName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hubAddr+"/drain", strings.NewReader(body))
	if err != nil {
		cancel()
		log.Fatalf("create drain request: %v", err) //nolint:gocritic
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // user-provided backend name in body, intentional
	if err != nil {
		cancel()
		log.Fatalf("drain request: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatalf("decode drain response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("drain failed: %v", result["error"])
	}

	fmt.Printf("Backend '%s' drained successfully\n", backendName)
}

func cmdVersion() {
	fmt.Printf("corectl version 0.1.0-dev\n")
	fmt.Printf("MCP spec: 2025-03-26\n")
}

func waitForInterrupt() {
	c := make(chan os.Signal, 1)
	<-c
}
