package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ndjsonTransport implements MCP stdio transport using newline-delimited JSON.
// Each message is a single line of JSON followed by '\n'.
type ndjsonTransport struct {
	reader       *bufio.Reader
	writer       io.Writer
	mu           sync.Mutex
	maxLineBytes int
}

const defaultMaxLineBytes = 8 * 1024 * 1024

type recoverableProtocolError struct {
	message string
}

func (e *recoverableProtocolError) Error() string {
	return e.message
}

func newNDJSONTransport(r io.Reader, w io.Writer) *ndjsonTransport {
	return &ndjsonTransport{
		reader:       bufio.NewReaderSize(r, 64*1024),
		writer:       w,
		maxLineBytes: defaultMaxLineBytes,
	}
}

func (t *ndjsonTransport) readRequest() (Request, error) {
	line, err := t.readLine()
	if err != nil {
		return Request{}, err
	}

	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return Request{}, &recoverableProtocolError{message: "empty request line"}
	}

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return Request{}, &recoverableProtocolError{message: fmt.Sprintf("decode request: %v", err)}
	}
	return req, nil
}

func (t *ndjsonTransport) readLine() ([]byte, error) {
	line := make([]byte, 0, 1024)

	for {
		frag, err := t.reader.ReadSlice('\n')
		line = append(line, frag...)

		if len(line) > t.maxLineBytes {
			if !bytes.HasSuffix(line, []byte{'\n'}) {
				t.discardUntilNewline()
			}
			return nil, &recoverableProtocolError{message: fmt.Sprintf("request line exceeds %d bytes", t.maxLineBytes)}
		}

		if err == nil {
			return line, nil
		}

		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}

		if errors.Is(err, io.EOF) {
			if len(line) == 0 {
				return nil, io.EOF
			}
			return line, nil
		}

		return nil, fmt.Errorf("read line: %w", err)
	}
}

func (t *ndjsonTransport) discardUntilNewline() {
	for {
		_, err := t.reader.ReadSlice('\n')
		if err == nil || errors.Is(err, io.EOF) {
			return
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return
		}
	}
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
