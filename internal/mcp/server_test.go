package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

func TestServeContinuesAfterMalformedRequest(t *testing.T) {
	input := "not-json\n{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}\n"

	var out bytes.Buffer
	srv := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	if err := srv.Serve(t.Context(), bytes.NewBufferString(input), &out); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	line, err := out.ReadBytes('\n')
	if err != nil {
		t.Fatalf("failed to read response line: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(bytes.TrimSpace(line), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result shape: %T", resp.Result)
	}
	if got := result["protocolVersion"]; got != "2024-11-05" {
		t.Fatalf("protocolVersion = %v, want %q", got, "2024-11-05")
	}
}
