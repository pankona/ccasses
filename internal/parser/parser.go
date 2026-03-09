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

const scanBufSize = 1024 * 1024 // 1MB

func newScanner(f *os.File) *bufio.Scanner {
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, scanBufSize), scanBufSize)
	return s
}

func updateTimeRange(start, end *time.Time, ts time.Time) {
	if start.IsZero() || ts.Before(*start) {
		*start = ts
	}
	if end.IsZero() || ts.After(*end) {
		*end = ts
	}
}

// ParseAllProjects は dataDir 以下の全プロジェクトの全セッションをパースして返す
func ParseAllProjects(dataDir string) ([]*model.SessionSummary, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("readdir %s: %w", dataDir, err)
	}
	var all []*model.SessionSummary
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		summaries, err := ParseProject(filepath.Join(dataDir, e.Name()))
		if err != nil {
			continue
		}
		all = append(all, summaries...)
	}
	return all, nil
}

// ParseSubAgents はセッションのサブエージェント情報を返す
func ParseSubAgents(jsonlPath string) ([]model.SubAgentInfo, error) {
	subagentsDir := filepath.Join(strings.TrimSuffix(jsonlPath, ".jsonl"), "subagents")

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
		if !strings.HasSuffix(name, ".jsonl") || strings.Contains(name, "compact") {
			continue
		}
		agentPath := filepath.Join(subagentsDir, name)
		info, err := parseSubAgentFile(agentPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: skip subagent %s: %v\n", name, err)
			continue
		}
		metaPath := strings.TrimSuffix(agentPath, ".jsonl") + ".meta.json"
		if agentType := readAgentType(metaPath); agentType != "" {
			info.AgentType = agentType
		}
		result = append(result, info)
	}
	return result, nil
}

func parseSubAgentFile(path string) (model.SubAgentInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.SubAgentInfo{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var info model.SubAgentInfo
	scanner := newScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		lineBytes := []byte(line)
		var entry model.RawEntry
		if err := json.Unmarshal(lineBytes, &entry); err != nil {
			continue
		}

		if info.ToolUseID == "" && entry.ParentToolUseID != "" {
			info.ToolUseID = entry.ParentToolUseID
		}

		if entry.Timestamp != "" {
			if ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
				updateTimeRange(&info.StartTime, &info.EndTime, ts)

				if entry.Type == "assistant" && entry.Message != nil {
					toolUses := extractTools(entry.Message.Content)
					info.ToolCount += len(toolUses)
					if len(toolUses) > 0 {
						names, details := splitToolUses(toolUses)
						evt := model.SubAgentToolEvent{
							Timestamp:   ts,
							Tools:       names,
							ToolDetails: details,
						}
						info.ToolEvents = append(info.ToolEvents, evt)
					}

					// タイムラインエントリ（トークン情報）の抽出
					te := model.TimelineEntry{
						Timestamp: ts,
						Type:      "assistant",
						Model:     entry.Message.Model,
					}
					if entry.Message.Usage != nil {
						u := entry.Message.Usage.ToTokenUsage()
						te.Tokens = &u
					}
					tools := make([]string, len(toolUses))
					for i, tu := range toolUses {
						tools[i] = tu.Name
					}
					te.Tools = tools
					info.Timeline = append(info.Timeline, te)
				}
			}
		}

		if entry.Type == "user" && entry.Message != nil && info.Prompt == "" {
			_, isToolResult, promptText := extractUserContent(entry.Message.Content)
			if !isToolResult && promptText != "" {
				info.Prompt = promptText
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return model.SubAgentInfo{}, err
	}
	return info, nil
}

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
		Models:      make(map[string]int),
		Tools:       make(map[string]int),
		ToolDetails: make(map[string][]string),
	}
	timeline := &model.SessionTimeline{}
	scanner := newScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry model.RawEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

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

		if entry.Timestamp != "" {
			if ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp); err == nil {
				updateTimeRange(&summary.StartTime, &summary.EndTime, ts)
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
				u := msg.Usage.ToTokenUsage()
				summary.Tokens.Input += u.Input
				summary.Tokens.Output += u.Output
				summary.Tokens.CacheCreation += u.CacheCreation
				summary.Tokens.CacheRead += u.CacheRead
			}
			toolUses := extractTools(msg.Content)
			tools := make([]string, len(toolUses))
			for i, tu := range toolUses {
				tools[i] = tu.Name
				summary.Tools[tu.Name]++
				if tu.Detail != "" && len(summary.ToolDetails[tu.Name]) < maxToolDetailsPerKey {
					summary.ToolDetails[tu.Name] = append(summary.ToolDetails[tu.Name], tu.Detail)
				}
			}
			te := model.TimelineEntry{
				Timestamp: ts,
				Type:      "assistant",
				Model:     msg.Model,
				Tools:     tools,
			}
			if msg.Usage != nil {
				u := msg.Usage.ToTokenUsage()
				te.Tokens = &u
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
	if strings.HasPrefix(projectName, "-") {
		projectName = strings.ReplaceAll(projectName[1:], "-", "/")
	}

	var summaries []*model.SessionSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		summary, _, err := ParseSession(filepath.Join(projectDir, e.Name()))
		if err != nil {
			continue
		}
		summary.Project = projectName
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// toolUse はツール使用の名前と詳細を保持する
type toolUse struct {
	Name   string // "Bash:grep", "Read", "Agent:general-purpose" 等
	Detail string // description or truncated command（Bash のみ、他は空）
}

// splitToolUses は []toolUse を名前スライスと詳細スライスに分離する。
// 詳細が1つもなければ details は nil を返す。
func splitToolUses(tus []toolUse) (names []string, details []string) {
	names = make([]string, len(tus))
	hasDetails := false
	for i, tu := range tus {
		names[i] = tu.Name
		if tu.Detail != "" {
			hasDetails = true
		}
	}
	if hasDetails {
		details = make([]string, len(tus))
		for i, tu := range tus {
			details[i] = tu.Detail
		}
	}
	return
}

func extractTools(content any) []toolUse {
	blocks, ok := content.([]any)
	if !ok {
		return nil
	}
	var tools []toolUse
	for _, b := range blocks {
		block, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if block["type"] != "tool_use" {
			continue
		}
		name, _ := block["name"].(string)
		input, _ := block["input"].(map[string]any)
		var detail string
		switch name {
		case "Agent":
			if input != nil {
				if st, ok := input["subagent_type"].(string); ok && st != "" {
					name = "Agent:" + st
				} else {
					name = "Agent:(not specified)"
				}
				if desc, ok := input["description"].(string); ok && desc != "" {
					detail = truncate(desc, 80)
				}
			}
		case "Bash":
			if input != nil {
				if cmd, ok := input["command"].(string); ok && cmd != "" {
					if fields := strings.Fields(cmd); len(fields) > 0 {
						name = "Bash:" + filepath.Base(fields[0])
					}
					if desc, ok := input["description"].(string); ok && desc != "" {
						detail = truncate(desc, 80)
					} else {
						detail = truncate(cmd, 80)
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
			tools = append(tools, toolUse{Name: name, Detail: detail})
		}
	}
	return tools
}

const maxToolDetailsPerKey = 200
const maxPromptTextLen = 120

func extractUserContent(content any) (textLen int, isToolResult bool, promptText string) {
	switch v := content.(type) {
	case string:
		return len(v), false, truncate(v, maxPromptTextLen)
	case []any:
		var fullText strings.Builder
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
					fullText.WriteString(text)
				}
			}
		}
		promptText = truncate(fullText.String(), maxPromptTextLen)
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
