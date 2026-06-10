package mcp

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

// SSEHandler implements the MCP HTTP+SSE transport.
//
// Protocol flow:
//  1. Client opens GET /sse → server sends "endpoint" event with the POST URL.
//  2. Client POSTs JSON-RPC messages to /message?sessionId=<id>.
//  3. Server sends responses as "message" events on the open SSE stream.
type SSEHandler struct {
	server    *Server
	logger    *slog.Logger
	authToken string
	sessions  sync.Map // sessionID -> *sseSession
}

type sseSession struct {
	ch chan []byte
}

// NewSSEHandler creates an SSEHandler. authToken may be empty to disable auth.
func NewSSEHandler(server *Server, logger *slog.Logger, authToken string) *SSEHandler {
	return &SSEHandler{
		server:    server,
		logger:    logger,
		authToken: authToken,
	}
}

// Register mounts the SSE and message endpoints on mux.
func (h *SSEHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/sse", h.handleSSE)
	mux.HandleFunc("/message", h.handleMessage)
}

func (h *SSEHandler) authenticate(r *http.Request) bool {
	if h.authToken == "" {
		return true
	}
	return r.Header.Get("Authorization") == "Bearer "+h.authToken
}

func (h *SSEHandler) handleSSE(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sessionID := newSessionID()
	sess := &sseSession{ch: make(chan []byte, 64)}
	h.sessions.Store(sessionID, sess)
	defer h.sessions.Delete(sessionID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Announce the POST endpoint to the client.
	fmt.Fprintf(w, "event: endpoint\ndata: /message?sessionId=%s\n\n", sessionID)
	flusher.Flush()

	h.logger.Info("SSE session opened", "session", sessionID)

	for {
		select {
		case <-r.Context().Done():
			h.logger.Info("SSE session closed", "session", sessionID)
			return
		case data := <-sess.ch:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (h *SSEHandler) handleMessage(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authenticate(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	val, ok := h.sessions.Load(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	sess := val.(*sseSession)

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Notifications (no ID) require no response.
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	resp := h.server.handle(req)
	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "marshal error", http.StatusInternalServerError)
		return
	}

	select {
	case sess.ch <- data:
	default:
		h.logger.Warn("SSE session channel full, dropping response", "session", sessionID)
	}
	w.WriteHeader(http.StatusAccepted)
}

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
