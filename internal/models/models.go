package models

import "time"

// ClaudeProvider 表示一个 Claude Code 供应商配置
type ClaudeProvider struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Website   string          `json:"website"`
	Notes     string          `json:"notes"`
	Settings  map[string]any  `json:"settings"`
	ClaudeJSON map[string]any `json:"claude_json"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// CodexProvider 表示一个 Codex CLI 供应商配置
type CodexProvider struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Website    string                 `json:"website"`
	Notes      string                 `json:"notes"`
	ConfigTOML string                 `json:"config_toml"`
	AuthJSON   map[string]any         `json:"auth_json"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// State 保存激活状态
type State struct {
	ActiveClaudeProviderID string    `json:"active_claude_provider_id"`
	ActiveCodexProviderID  string    `json:"active_codex_provider_id"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// BackupMeta 备份元数据
type BackupMeta struct {
	Tool      string    `json:"tool"`
	Timestamp time.Time `json:"timestamp"`
	Files     []string  `json:"files"`
}
