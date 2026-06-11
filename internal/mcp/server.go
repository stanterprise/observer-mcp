package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
)

type Server struct {
	logger *slog.Logger
	tools  map[string]Tool
}

func NewServer(logger *slog.Logger, tools []Tool) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	toolMap := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}
	return &Server{logger: logger, tools: toolMap}
}

func (s *Server) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	tr := newNDJSONTransport(stdin, stdout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := tr.readRequest()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			var protocolErr *recoverableProtocolError
			if errors.As(err, &protocolErr) {
				s.logger.Warn("ignoring malformed MCP request", "error", protocolErr.Error())
				continue
			}

			return fmt.Errorf("read request: %w", err)
		}

		if req.Method == "notifications/initialized" {
			continue
		}

		resp := s.handle(req)
		if req.ID == nil {
			continue
		}
		if err := tr.writeResponse(resp); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
	}
}

func (s *Server) handle(req Request) Response {
	switch req.Method {
	case "initialize":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]any{
				"name":    "observer-mcp",
				"version": "0.1.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		}}
	case "tools/list":
		tools := make([]Tool, 0, len(s.tools))
		for _, t := range s.tools {
			tools = append(tools, Tool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": tools}}
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return rpcErr(req.ID, -32602, "invalid tools/call params")
		}

		tool, ok := s.tools[params.Name]
		if !ok {
			return rpcErr(req.ID, -32601, "unknown tool")
		}

		result, err := tool.Handler(CallContext{}, params.Arguments)
		if err != nil {
			s.logger.Error("tool execution failed", "tool", params.Name, "error", err)
			return Response{JSONRPC: "2.0", ID: req.ID, Result: ToolCallResult{
				IsError: true,
				Content: []TextContent{{Type: "text", Text: err.Error()}},
			}}
		}

		payload, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return rpcErr(req.ID, -32603, "failed to marshal tool result")
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: ToolCallResult{
			Content: []TextContent{{Type: "text", Text: string(payload)}},
		}}
	default:
		return rpcErr(req.ID, -32601, "method not found")
	}
}

func rpcErr(id any, code int, message string) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
}
