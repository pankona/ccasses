package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pankona/ccasses/internal/parser"
)


// Server は HTTP サーバー
type Server struct {
	port    int
	webFS   fs.FS
	dataDir string // ~/.claude/projects
}

// New はサーバーを作成する
func New(port int, webFS fs.FS) (*Server, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	return &Server{
		port:    port,
		webFS:   webFS,
		dataDir: filepath.Join(homeDir, ".claude", "projects"),
	}, nil
}

// Run はサーバーを起動する
func (s *Server) Run() error {
	mux := http.NewServeMux()

	// API エンドポイント
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionTimeline)

	// 静的ファイル（embed.FS から配信）
	mux.Handle("/", http.FileServer(http.FS(s.webFS)))

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

// handleSessionTimeline は特定セッションのタイムラインを返す
// PATH: /api/sessions/{sessionId}/timeline
func (s *Server) handleSessionTimeline(w http.ResponseWriter, r *http.Request) {
	// /api/sessions/{sessionId}/timeline からsessionIdを抽出
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/")
	if len(parts) < 2 || parts[1] != "timeline" {
		http.NotFound(w, r)
		return
	}
	sessionID := parts[0]

	jsonlPath, err := parser.FindSessionPath(s.dataDir, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	_, timeline, err := parser.ParseSession(jsonlPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	subAgents, err := parser.ParseSubAgents(jsonlPath)
	if err == nil && len(subAgents) > 0 {
		timeline.SubAgents = subAgents
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeline)
}

// handleSessions は全セッションのサマリーを返す
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	summaries, err := parser.ParseAllProjects(s.dataDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}
