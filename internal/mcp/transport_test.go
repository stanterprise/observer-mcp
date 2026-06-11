package mcp

import (
	"bytes"
	"errors"
	"testing"
)

func TestReadRequestLineTooLargeIsRecoverable(t *testing.T) {
	tr := newNDJSONTransport(bytes.NewBufferString("{\"jsonrpc\":\"2.0\"}\n"), &bytes.Buffer{})
	tr.maxLineBytes = 4

	_, err := tr.readRequest()
	if err == nil {
		t.Fatal("expected error for oversized request line")
	}

	var protocolErr *recoverableProtocolError
	if !errors.As(err, &protocolErr) {
		t.Fatalf("expected recoverableProtocolError, got %T (%v)", err, err)
	}
}

func TestReadRequestParsesLastLineWithoutTrailingNewline(t *testing.T) {
	tr := newNDJSONTransport(bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}"), &bytes.Buffer{})

	req, err := tr.readRequest()
	if err != nil {
		t.Fatalf("readRequest() error = %v", err)
	}
	if req.Method != "initialize" {
		t.Fatalf("method = %q, want %q", req.Method, "initialize")
	}
}
