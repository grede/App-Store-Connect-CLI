package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestBootstrapAndPromptRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Start(ctx, LaunchSpec{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestACPHelperProcess", "--"},
		Env:     []string{"GO_WANT_ACP_HELPER_PROCESS=1"},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	sessionID, err := client.Bootstrap(ctx, SessionConfig{CWD: "/tmp/project"})
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if sessionID != "session-1" {
		t.Fatalf("sessionID = %q, want session-1", sessionID)
	}

	result, events, err := client.Prompt(ctx, sessionID, "Validate release 2.0.0")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("Status = %q, want completed", result.Status)
	}
	if len(events) == 0 {
		t.Fatalf("events len = 0, want at least one streaming update")
	}
}

func TestACPHelperProcess(*testing.T) {
	if os.Getenv("GO_WANT_ACP_HELPER_PROCESS") != "1" {
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			os.Exit(2)
		}
		id := int64(req["id"].(float64))
		method := req["method"].(string)
		switch method {
		case "initialize":
			respond(id, map[string]any{
				"protocolVersion": "0.1.0",
				"capabilities": map[string]any{
					"sessionUpdates": true,
				},
			})
		case "session/new":
			respond(id, map[string]any{
				"sessionId": "session-1",
			})
		case "session/prompt":
			notify("session/update", map[string]any{
				"sessionId": "session-1",
				"kind":      "message",
				"role":      "assistant",
				"content":   "Validating in progress",
			})
			respond(id, map[string]any{
				"status":  "completed",
				"summary": "Validation completed in bootstrap mode.",
			})
		default:
			respondError(id, -32601, "method not found")
		}
	}
	os.Exit(0)
}

func TestStartDrainsAgentStderr(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Start(ctx, LaunchSpec{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestACPStderrHelperProcess", "--"},
		Env:     []string{"GO_WANT_ACP_STDERR_HELPER_PROCESS=1"},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	sessionID, err := client.Bootstrap(ctx, SessionConfig{CWD: "/tmp/project"})
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if sessionID == "" {
		t.Fatal("sessionID = empty, want non-empty")
	}
}

func TestACPStderrHelperProcess(*testing.T) {
	if os.Getenv("GO_WANT_ACP_STDERR_HELPER_PROCESS") != "1" {
		return
	}

	for i := 0; i < 4096; i++ {
		fmt.Fprintln(os.Stderr, strings.Repeat("stderr-noise-", 32))
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			os.Exit(2)
		}
		id := int64(req["id"].(float64))
		method := req["method"].(string)
		switch method {
		case "initialize":
			respond(id, map[string]any{
				"protocolVersion": "0.1.0",
				"capabilities": map[string]any{
					"sessionUpdates": true,
				},
			})
		case "session/new":
			respond(id, map[string]any{
				"sessionId": "session-1",
			})
		case "session/prompt":
			notify("session/update", map[string]any{
				"sessionId": "session-1",
				"kind":      "message",
				"role":      "assistant",
				"content":   "Validating in progress",
			})
			respond(id, map[string]any{
				"status":  "completed",
				"summary": "Validation completed in bootstrap mode.",
			})
		default:
			respondError(id, -32601, "method not found")
		}
	}
	os.Exit(0)
}

func TestPromptRoundTripHandlesLargeEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Start(ctx, LaunchSpec{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestACPLargeEventHelperProcess", "--"},
		Env:     []string{"GO_WANT_ACP_LARGE_EVENT_HELPER_PROCESS=1"},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	sessionID, err := client.Bootstrap(ctx, SessionConfig{CWD: "/tmp/project"})
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	result, events, err := client.Prompt(ctx, sessionID, "stream a large event")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("Status = %q, want completed", result.Status)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}

	var payload map[string]string
	if err := json.Unmarshal(events[0].Params, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := len(payload["content"]); got <= 64*1024 {
		t.Fatalf("content length = %d, want > 65536", got)
	}
}

func TestACPLargeEventHelperProcess(*testing.T) {
	if os.Getenv("GO_WANT_ACP_LARGE_EVENT_HELPER_PROCESS") != "1" {
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			os.Exit(2)
		}
		id := int64(req["id"].(float64))
		method := req["method"].(string)
		switch method {
		case "initialize":
			respond(id, map[string]any{
				"protocolVersion": "0.1.0",
				"capabilities": map[string]any{
					"sessionUpdates": true,
				},
			})
		case "session/new":
			respond(id, map[string]any{"sessionId": "session-1"})
		case "session/prompt":
			notify("session/update", map[string]any{
				"sessionId": "session-1",
				"content":   strings.Repeat("x", 128*1024),
			})
			respond(id, map[string]any{"status": "completed"})
		default:
			respondError(id, -32601, "method not found")
		}
	}
	os.Exit(0)
}

func respond(id int64, result any) {
	emit(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func respondError(id int64, code int, message string) {
	emit(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func notify(method string, params any) {
	emit(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func emit(payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
}

func TestStartFailsWithoutCommand(t *testing.T) {
	_, err := Start(context.Background(), LaunchSpec{})
	if err == nil {
		t.Fatal("Start() error = nil, want error")
	}
}

func TestCloseKillsProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	client := &Client{cmd: cmd, done: make(chan struct{})}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDrainStderrHandlesNilPipe(t *testing.T) {
	client := &Client{}
	client.drainStderr()
}

func TestDrainStderrConsumesReader(t *testing.T) {
	reader, writer := io.Pipe()
	done := make(chan struct{})
	client := &Client{stderr: reader}

	go func() {
		client.drainStderr()
		close(done)
	}()

	if _, err := writer.Write([]byte("hello stderr")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	_ = writer.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("drainStderr() did not finish")
	}
}
