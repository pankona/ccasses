package model

import "time"

// SessionSummary はセッション全体の集計結果
type SessionSummary struct {
	SessionID          string         `json:"sessionId"`
	Project            string         `json:"project"`
	Branch             string         `json:"branch"`
	Slug               string         `json:"slug"`
	Version            string         `json:"version"`
	StartTime          time.Time      `json:"startTime"`
	EndTime            time.Time      `json:"endTime"`
	Models             map[string]int `json:"models"`
	Tokens             TokenUsage     `json:"tokens"`
	Tools              map[string]int `json:"tools"`
	UserPromptCount    int            `json:"userPromptCount"`
	AssistantTurnCount int            `json:"assistantTurnCount"`
}

// TokenUsage はトークン使用量の集計
type TokenUsage struct {
	Input         int `json:"input"`
	Output        int `json:"output"`
	CacheCreation int `json:"cacheCreation"`
	CacheRead     int `json:"cacheRead"`
}

// SessionTimeline はセッション内の時系列データ
type SessionTimeline struct {
	SessionID string          `json:"sessionId"`
	Timeline  []TimelineEntry `json:"timeline"`
	SubAgents []SubAgentInfo  `json:"subAgents,omitempty"`
}

// SubAgentInfo はサブエージェント（Agent ツール使用）の情報
type SubAgentInfo struct {
	ToolUseID  string              `json:"toolUseId"` // 親セッションの tool_use ID
	AgentType  string              `json:"agentType"` // general-purpose, Explore, Plan 等
	StartTime  time.Time           `json:"startTime"`
	EndTime    time.Time           `json:"endTime"`
	ToolCount  int                 `json:"toolCount"`
	Prompt     string              `json:"prompt"` // 最初のユーザープロンプト（冒頭120文字）
	ToolEvents []SubAgentToolEvent `json:"toolEvents,omitempty"`
}

// SubAgentToolEvent はサブエージェント内の1アシスタントターンのツール使用
type SubAgentToolEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Tools     []string  `json:"tools"`
}

// TimelineEntry は1つの時系列エントリ
type TimelineEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"` // "assistant", "user", "tool_result"
	Model     string      `json:"model,omitempty"`
	Tokens    *TokenUsage `json:"tokens,omitempty"`
	Tools     []string    `json:"tools,omitempty"`
	TextLen   int         `json:"textLen,omitempty"`
	Text      string      `json:"text,omitempty"` // ユーザープロンプトの冒頭テキスト（最大120文字）
}

// RawEntry は JSONL の1エントリ（パース用）
type RawEntry struct {
	ParentUUID      string      `json:"parentUuid"`
	IsSidechain     bool        `json:"isSidechain"`
	UserType        string      `json:"userType"`
	Cwd             string      `json:"cwd"`
	SessionID       string      `json:"sessionId"`
	Version         string      `json:"version"`
	GitBranch       string      `json:"gitBranch"`
	Slug            string      `json:"slug"`
	Type            string      `json:"type"`
	Message         *RawMessage `json:"message"`
	Timestamp       string      `json:"timestamp"`
	UUID            string      `json:"uuid"`
	PermissionMode  string      `json:"permissionMode"`
	DurationMs      *int        `json:"durationMs"`
	ParentToolUseID string      `json:"parentToolUseID"`
}

// RawMessage は message フィールドの内容
type RawMessage struct {
	Model      string       `json:"model"`
	ID         string       `json:"id"`
	Type       string       `json:"type"`
	Role       string       `json:"role"`
	Content    any          `json:"content"`
	StopReason *string      `json:"stop_reason"`
	Usage      *RawUsage    `json:"usage"`
}

// RawUsage はトークン使用量の生データ
type RawUsage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

// ToTokenUsage は RawUsage を TokenUsage に変換する
func (u *RawUsage) ToTokenUsage() TokenUsage {
	return TokenUsage{
		Input:         u.InputTokens,
		Output:        u.OutputTokens,
		CacheCreation: u.CacheCreationInputTokens,
		CacheRead:     u.CacheReadInputTokens,
	}
}

// RawContentBlock は content 配列の1要素
type RawContentBlock struct {
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Input     any         `json:"input"`
	Text      string      `json:"text"`
	ToolUseID string      `json:"tool_use_id"`
}
