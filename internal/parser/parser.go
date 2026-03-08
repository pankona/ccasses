package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pankona/ccasses/internal/model"
)

// ParseSubAgents はセッションのサブエージェント情報を返す
// jsonlPath: 親セッションの JSONL ファイルパス
func ParseSubAgents(jsonlPath string) ([]model.SubAgentInfo, error) {
	// サブエージェントディレクトリ: <session-id>/subagents/
	sessionDir := strings.TrimSuffix(jsonlPath, ".jsonl")
	subagentsDir := filepath.Join(sessionDir, "subagents")

	entries, err := os.ReadDir(subagentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("readdir %s: %w", subagentsDir, err)
	}

	var result []model.SubAgentInfo
	for _, e := range entries {
		name := e.Name()
		// agent-<hash>.jsonl のみ処理（compactファイルは除外）
		if !strings.HasSuffix(name, ".jsonl") || strings.Contains(name, "compact") {
			continue
		}
		agentPath := filepath.Join(subagentsDir, name)
		info, err := parseSubAgentFile(agentPath)
		if err != nil {
			continue
		}
		// meta.json からエージェントタイプを取得
		metaPath := strings.TrimSuffix(agentPath, ".jsonl") + ".meta.json"
		if agentType := readAgentType(metaPath); agentType != "" {
			info.AgentType = agentType
		}
		result = append(result, info)
	}
	return result, nil
}

// parseSubAgentFile はサブエージェントの JSONL ファイルをパースして SubAgentInfo を返す
func parseSubAgentFile(path string) (model.SubAgentInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.SubAgentInfo{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var info model.SubAgentInfo
	var toolCount int
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry model.RawEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// parentToolUseID を取得（最初のエントリから）
		if info.ToolUseID == "" {
			// parentToolUseID フィールドを直接読み込む
			var raw map[string]any
			if json.Unmarshal([]byte(line), &raw) == nil {
				if pid, ok := raw["parentToolUseID"].(string); ok && pid != "" {
					info.ToolUseID = pid
				}
			}
		}

		// タイムスタンプの追跡
		if entry.Timestamp != "" {
			ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
			if err == nil {
				if info.StartTime.IsZero() || ts.Before(info.StartTime) {
					info.StartTime = ts
				}
				if info.EndTime.IsZero() || ts.After(info.EndTime) {
					info.EndTime = ts
				}
			}
		}

		if entry.Message == nil {
			continue
		}
		msg := entry.Message

		switch entry.Type {
		case "user":
			// 最初のユーザーメッセージからプロンプトを取得
			if info.Prompt == "" {
				_, isToolResult, promptText := extractUserContent(msg.Content)
				if !isToolResult && promptText != "" {
					info.Prompt = promptText
				}
			}
		case "assistant":
			// ツール使用数をカウント、ToolEventsに記録
			tools := extractTools(msg.Content)
			toolCount += len(tools)
			if len(tools) > 0 {
				ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
				if !ts.IsZero() {
					info.ToolEvents = append(info.ToolEvents, model.SubAgentToolEvent{
						Timestamp: ts,
						Tools:     tools,
					})
				}
			}
		}
	}

	info.ToolCount = toolCount
	if scanner.Err() != nil {
		return model.SubAgentInfo{}, scanner.Err()
	}
	return info, nil
}

// readAgentType は meta.json からエージェントタイプを読み込む
func readAgentType(metaPath string) string {
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return ""
	}
	var meta struct {
		AgentType string `json:"agentType"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return ""
	}
	return meta.AgentType
}

// ParseSession は1つの JSONL ファイルを読み込んでサマリーとタイムラインを返す
func ParseSession(jsonlPath string) (*model.SessionSummary, *model.SessionTimeline, error) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", jsonlPath, err)
	}
	defer f.Close()

	summary := &model.SessionSummary{
		Models: make(map[string]int),
		Tools:  make(map[string]int),
	}
	timeline := &model.SessionTimeline{}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry model.RawEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // パースできない行はスキップ
		}

		// セッションメタ情報（最初のエントリから取得）
		if summary.SessionID == "" && entry.SessionID != "" {
			summary.SessionID = entry.SessionID
			timeline.SessionID = entry.SessionID
		}
		if summary.Branch == "" && entry.GitBranch != "" {
			summary.Branch = entry.GitBranch
		}
		if summary.Slug == "" && entry.Slug != "" {
			summary.Slug = entry.Slug
		}
		if summary.Version == "" && entry.Version != "" {
			summary.Version = entry.Version
		}

		// タイムスタンプの追跡
		if entry.Timestamp != "" {
			ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
			if err == nil {
				if summary.StartTime.IsZero() || ts.Before(summary.StartTime) {
					summary.StartTime = ts
				}
				if summary.EndTime.IsZero() || ts.After(summary.EndTime) {
					summary.EndTime = ts
				}
			}
		}

		if entry.Message == nil {
			continue
		}

		msg := entry.Message
		ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

		switch entry.Type {
		case "assistant":
			summary.AssistantTurnCount++

			if msg.Model != "" {
				summary.Models[msg.Model]++
			}

			if msg.Usage != nil {
				summary.Tokens.Input += msg.Usage.InputTokens
				summary.Tokens.Output += msg.Usage.OutputTokens
				summary.Tokens.CacheCreation += msg.Usage.CacheCreationInputTokens
				summary.Tokens.CacheRead += msg.Usage.CacheReadInputTokens
			}

			// content からツール使用を抽出
			tools := extractTools(msg.Content)
			for _, t := range tools {
				summary.Tools[t]++
			}

			te := model.TimelineEntry{
				Timestamp: ts,
				Type:      "assistant",
				Model:     msg.Model,
				Tools:     tools,
			}
			if msg.Usage != nil {
				te.Tokens = &model.TokenUsage{
					Input:         msg.Usage.InputTokens,
					Output:        msg.Usage.OutputTokens,
					CacheCreation: msg.Usage.CacheCreationInputTokens,
					CacheRead:     msg.Usage.CacheReadInputTokens,
				}
			}
			timeline.Timeline = append(timeline.Timeline, te)

		case "user":
			textLen, isToolResult, promptText := extractUserContent(msg.Content)
			entryType := "user"
			if isToolResult {
				entryType = "tool_result"
			} else {
				summary.UserPromptCount++
			}
			timeline.Timeline = append(timeline.Timeline, model.TimelineEntry{
				Timestamp: ts,
				Type:      entryType,
				TextLen:   textLen,
				Text:      promptText,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan %s: %w", jsonlPath, err)
	}

	return summary, timeline, nil
}

// FindSessionPath は dataDir 以下から sessionId に対応する JSONL ファイルのパスを返す
func FindSessionPath(dataDir, sessionID string) (string, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return "", fmt.Errorf("readdir %s: %w", dataDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(dataDir, e.Name(), sessionID+".jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("session %s not found", sessionID)
}

// ParseProject は1つのプロジェクトディレクトリ以下の全セッションをパースする
func ParseProject(projectDir string) ([]*model.SessionSummary, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("readdir %s: %w", projectDir, err)
	}

	projectName := filepath.Base(projectDir)
	// ディレクトリ名のプレフィックス "-home-pankona-..." を人間が読みやすい形式に変換
	if strings.HasPrefix(projectName, "-") {
		projectName = strings.ReplaceAll(projectName[1:], "-", "/")
	}

	var summaries []*model.SessionSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(projectDir, e.Name())
		summary, _, err := ParseSession(path)
		if err != nil {
			continue
		}
		summary.Project = projectName
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// extractTools は content から tool_use ブロックのツール名一覧を返す
func extractTools(content any) []string {
	if content == nil {
		return nil
	}
	blocks, ok := content.([]any)
	if !ok {
		return nil
	}

	var tools []string
	for _, b := range blocks {
		block, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if block["type"] == "tool_use" {
			name, _ := block["name"].(string)
			input, _ := block["input"].(map[string]any)
			switch name {
			case "Agent":
				if input != nil {
					if st, ok := input["subagent_type"].(string); ok && st != "" {
						name = "Agent:" + st
					}
				}
			case "Bash":
				if input != nil {
					if cmd, ok := input["command"].(string); ok && cmd != "" {
						fields := strings.Fields(cmd)
						if len(fields) > 0 {
							name = "Bash:" + filepath.Base(fields[0])
						}
					}
				}
			case "Skill":
				if input != nil {
					if skill, ok := input["skill"].(string); ok && skill != "" {
						name = "Skill:" + skill
					}
				}
			}
			if name != "" {
				tools = append(tools, name)
			}
		}
	}
	return tools
}

const maxPromptTextLen = 120

// extractUserContent はユーザーメッセージのテキスト長、tool_resultかどうか、冒頭テキストを返す
func extractUserContent(content any) (textLen int, isToolResult bool, promptText string) {
	switch v := content.(type) {
	case string:
		return len(v), false, truncate(v, maxPromptTextLen)
	case []any:
		var fullText string
		for _, b := range v {
			block, ok := b.(map[string]any)
			if !ok {
				continue
			}
			switch block["type"] {
			case "tool_result":
				isToolResult = true
			case "text":
				if text, ok := block["text"].(string); ok {
					textLen += len(text)
					fullText += text
				}
			}
		}
		promptText = truncate(fullText, maxPromptTextLen)
	}
	return
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
