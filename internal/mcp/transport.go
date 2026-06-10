package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// ndjsonTransport implements MCP stdio transport using newline-delimited JSON.
// Each message is a single line of JSON followed by '\n'.
type ndjsonTransport struct {
	scanner *bufio.Scanner
	writer  io.Writer
	mu      sync.Mutex
}

func newNDJSONTransport(r io.Reader, w io.Writer) *ndjsonTransport {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	return &ndjsonTransport{
		scanner: scanner,
		writer:  w,
	}
}

func (t *ndjsonTransport) readRequest() (Request, error) {
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return Request{}, fmt.Errorf("read line: %w", err)
		}
		return Request{}, io.EOF
	}
	var req Request
	if err := json.Unmarshal(t.scanner.Bytes(), &req); err != nil {
		return Request{}, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

func (t *ndjsonTransport) writeResponse(resp Response) error {
	return t.writeLine(resp)
}

func (t *ndjsonTransport) writeLine(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	data = append(data, '\n')
	_, err = t.writer.Write(data)
	return err
}
