package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"cc-switch/internal/models"

	"github.com/BurntSushi/toml"
)

const (
	DataDir             = ".ccswitch"
	ClaudeProvidersFile = "claude_providers.json"
	CodexProvidersFile  = "codex_providers.json"
	StateFile           = "state.json"
	BackupsDir          = "backups"
	ClaudeDir           = ".claude"
	ClaudeSettingsFile  = "settings.json"
	ClaudeJSONFile      = ".claude.json"
	CodexDir            = ".codex"
	CodexConfigFile     = "config.toml"
	CodexAuthFile       = "auth.json"
)

var homeDir string

func init() {
	var err error
	homeDir, err = os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
}

func ProjectDir() string {
	// 优先使用可执行文件所在目录；go run 临时文件时回退到当前工作目录
	ex, err := os.Executable()
	if err == nil {
		if strings.Contains(ex, os.TempDir()) {
			wd, _ := os.Getwd()
			return filepath.Join(wd, DataDir)
		}
		return filepath.Join(filepath.Dir(ex), DataDir)
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, DataDir)
}

func EnsureDirs() error {
	base := ProjectDir()
	dirs := []string{
		base,
		filepath.Join(base, BackupsDir),
		filepath.Join(homeDir, ClaudeDir),
		filepath.Join(homeDir, CodexDir),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return err
		}
	}
	return nil
}

func EnsureClaudeJSON() error {
	path := filepath.Join(homeDir, ClaudeJSONFile)
	data := map[string]any{"hasCompletedOnboarding": true}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		b, _ := json.MarshalIndent(data, "", "  ")
		return os.WriteFile(path, b, 0600)
	}
	// 已存在则确保 hasCompletedOnboarding
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		// JSON 损坏时保留原文件并追加 hasCompletedOnboarding
		m = data
	} else {
		m["hasCompletedOnboarding"] = true
	}
	b, _ = json.MarshalIndent(m, "", "  ")
	return os.WriteFile(path, b, 0600)
}

// Store 管理所有配置存储
type Store struct {
	mu              sync.RWMutex
	claudeProviders []models.ClaudeProvider
	codexProviders  []models.CodexProvider
	state           models.State
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	base := ProjectDir()
	if err := s.loadJSON(filepath.Join(base, ClaudeProvidersFile), &s.claudeProviders); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load claude providers: %w", err)
	}
	if err := s.loadJSON(filepath.Join(base, CodexProvidersFile), &s.codexProviders); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load codex providers: %w", err)
	}
	if err := s.loadJSON(filepath.Join(base, StateFile), &s.state); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load state: %w", err)
	}

	// 首次启动无供应商时初始化模板
	if len(s.claudeProviders) == 0 && len(s.codexProviders) == 0 {
		s.initTemplates()
	}
	return nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	base := ProjectDir()
	if err := s.saveJSON(filepath.Join(base, ClaudeProvidersFile), s.claudeProviders); err != nil {
		return err
	}
	if err := s.saveJSON(filepath.Join(base, CodexProvidersFile), s.codexProviders); err != nil {
		return err
	}
	return s.saveJSON(filepath.Join(base, StateFile), s.state)
}

func (s *Store) loadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (s *Store) saveJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b, 0600)
}

func cleanClaudeSettings(settings map[string]any) map[string]any {
	if settings == nil {
		return map[string]any{}
	}
	if env, ok := settings["env"].(map[string]any); ok {
		cleanEmptyValues(env)
		if len(env) == 0 {
			delete(settings, "env")
		}
	}
	for _, key := range []string{
		"disableTelemetry", "hideAiSignatures", "teammates", "toolSearch",
		"highThinking", "disableAutoUpdate", "includeCoAuthoredBy",
		"skipDangerousModePermissionPrompt",
	} {
		if v, ok := settings[key]; ok {
			if b, isBool := v.(bool); isBool && !b {
				delete(settings, key)
			}
		}
	}
	cleanEmptyValues(settings)
	cleanEmptyMaps(settings)
	return settings
}

func cleanEmptyValues(m map[string]any) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			if val == "" {
				delete(m, k)
			}
		case nil:
			delete(m, k)
		case map[string]any:
			cleanEmptyValues(val)
			cleanEmptyMaps(val)
		}
	}
}

func cleanEmptyMaps(m map[string]any) {
	for k, v := range m {
		if nested, ok := v.(map[string]any); ok && len(nested) == 0 {
			delete(m, k)
		}
	}
}

func (s *Store) initTemplates() {
	now := time.Now()
	s.claudeProviders = []models.ClaudeProvider{
		newClaudeTemplate("volc-ark", "火山方舟", "https://console.volcengine.com/ark", map[string]any{
			"ANTHROPIC_BASE_URL": "https://ark.cn-beijing.volces.com/api/v3",
			"ANTHROPIC_MODEL":    "doubao-1-5-pro-32k-250115",
		}),
		newClaudeTemplate("zhipu-glm", "GLM / 智谱", "https://open.bigmodel.cn/", map[string]any{
			"ANTHROPIC_BASE_URL": "https://open.bigmodel.cn/api/paas/v4",
			"ANTHROPIC_MODEL":    "glm-5.1",
		}),
		newClaudeTemplate("minimax", "MiniMax", "https://platform.minimaxi.com/", map[string]any{
			"ANTHROPIC_BASE_URL": "https://api.minimaxi.com/v1",
			"ANTHROPIC_MODEL":    "MiniMax-Text-01",
		}),
		newClaudeTemplate("xiaomi-mimo", "小米 MiMo", "https://platform.xiaomimimo.com", map[string]any{
			"ANTHROPIC_BASE_URL": "https://api.xiaomimimo.com/v1",
			"ANTHROPIC_MODEL":    "mimo-v2-pro",
		}),
		newClaudeTemplate("openai-proxy", "OpenAI 代理", "https://platform.openai.com/", map[string]any{
			"ANTHROPIC_BASE_URL": "http://127.0.0.1:18080/ccswitch/proxy/openai/<供应商ID>",
			"ANTHROPIC_MODEL":    "gpt-5.5",
		}),
	}

	s.codexProviders = []models.CodexProvider{
		{
			ID:         "openai-official",
			Name:       "OpenAI 官网登录空配置",
			Website:    "https://platform.openai.com/",
			Notes:      "官方登录模式，请通过 codex auth 登录",
			ConfigTOML: "model = \"gpt-5.5\"\nmodel_provider = \"OpenAI\"\n",
			AuthJSON:   map[string]any{},
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
}

func newClaudeTemplate(id, name, website string, env map[string]any) models.ClaudeProvider {
	now := time.Now()
	settings := map[string]any{
		"env": env,
	}
	return models.ClaudeProvider{
		ID:         id,
		Name:       name,
		Website:    website,
		Notes:      "",
		Settings:   settings,
		ClaudeJSON: map[string]any{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// ClaudeProviders
type ClaudeProviderListResp struct {
	Providers []models.ClaudeProvider `json:"providers"`
	ActiveID  string                  `json:"active_id"`
}

func (s *Store) ClaudeProviders() ClaudeProviderListResp {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ClaudeProviderListResp{
		Providers: append([]models.ClaudeProvider(nil), s.claudeProviders...),
		ActiveID:  s.state.ActiveClaudeProviderID,
	}
}

func (s *Store) GetClaudeProvider(id string) (*models.ClaudeProvider, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.claudeProviders {
		if p.ID == id {
			cp := p
			return &cp, true
		}
	}
	return nil, false
}

func (s *Store) SaveClaudeProvider(p *models.ClaudeProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.Settings = cleanClaudeSettings(p.Settings)
	p.UpdatedAt = time.Now()
	if p.ID == "" {
		p.ID = generateID("claude")
		p.CreatedAt = p.UpdatedAt
		// 自动替换 OpenAI 代理 Base URL 中的占位符
		if env, ok := p.Settings["env"].(map[string]any); ok {
			if baseURL, ok := env["ANTHROPIC_BASE_URL"].(string); ok {
				if strings.Contains(baseURL, "<供应商ID>") {
					env["ANTHROPIC_BASE_URL"] = strings.ReplaceAll(baseURL, "<供应商ID>", p.ID)
				}
			}
		}
		s.claudeProviders = append(s.claudeProviders, *p)
	} else {
		found := false
		for i := range s.claudeProviders {
			if s.claudeProviders[i].ID == p.ID {
				s.claudeProviders[i] = *p
				found = true
				break
			}
		}
		if !found {
			s.claudeProviders = append(s.claudeProviders, *p)
		}
	}
	return s.saveLocked()
}

func (s *Store) DeleteClaudeProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := make([]models.ClaudeProvider, 0, len(s.claudeProviders))
	for _, p := range s.claudeProviders {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}
	s.claudeProviders = filtered
	if s.state.ActiveClaudeProviderID == id {
		s.state.ActiveClaudeProviderID = ""
	}
	return s.saveLocked()
}

// CodexProviders
type CodexProviderListResp struct {
	Providers []models.CodexProvider `json:"providers"`
	ActiveID  string                 `json:"active_id"`
}

func (s *Store) CodexProviders() CodexProviderListResp {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return CodexProviderListResp{
		Providers: append([]models.CodexProvider(nil), s.codexProviders...),
		ActiveID:  s.state.ActiveCodexProviderID,
	}
}

func (s *Store) GetCodexProvider(id string) (*models.CodexProvider, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.codexProviders {
		if p.ID == id {
			cp := p
			return &cp, true
		}
	}
	return nil, false
}

func (s *Store) SaveCodexProvider(p *models.CodexProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.UpdatedAt = time.Now()
	if p.ID == "" {
		p.ID = generateID("codex")
		p.CreatedAt = p.UpdatedAt
		s.codexProviders = append(s.codexProviders, *p)
	} else {
		found := false
		for i := range s.codexProviders {
			if s.codexProviders[i].ID == p.ID {
				s.codexProviders[i] = *p
				found = true
				break
			}
		}
		if !found {
			s.codexProviders = append(s.codexProviders, *p)
		}
	}
	return s.saveLocked()
}

func (s *Store) DeleteCodexProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := make([]models.CodexProvider, 0, len(s.codexProviders))
	for _, p := range s.codexProviders {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}
	s.codexProviders = filtered
	if s.state.ActiveCodexProviderID == id {
		s.state.ActiveCodexProviderID = ""
	}
	return s.saveLocked()
}

// Apply 应用供应商到真实配置
func (s *Store) ApplyClaudeProvider(id string) error {
	p, ok := s.GetClaudeProvider(id)
	if !ok {
		return fmt.Errorf("provider not found")
	}
	p.Settings = cleanClaudeSettings(p.Settings)

	settingsPath := filepath.Join(homeDir, ClaudeDir, ClaudeSettingsFile)

	// 1. 先读取当前真实配置
	existingSettings := map[string]any{}
	if b, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(b, &existingSettings)
	}

	// 只覆盖核心 env 字段和 model 字段，保留其他
	providerEnv := map[string]any{}
	if p.Settings["env"] != nil {
		if e, ok := p.Settings["env"].(map[string]any); ok {
			providerEnv = e
		}
	}

	existingEnv := map[string]any{}
	if existingSettings["env"] != nil {
		if e, ok := existingSettings["env"].(map[string]any); ok {
			existingEnv = e
		}
	}

	// 切换认证方式时，先清理旧的 ANTHROPIC_AUTH_TOKEN
	delete(existingEnv, "ANTHROPIC_AUTH_TOKEN")

	// 核心字段列表：provider 中存在的字段覆盖，空字符串则删除
	coreEnvKeys := []string{
		"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_MODEL",
		"ANTHROPIC_REASONING_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL",
		"ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL",
	}
	for _, k := range coreEnvKeys {
		if v, ok := providerEnv[k]; ok {
			if vs, sok := v.(string); sok && vs == "" {
				delete(existingEnv, k)
			} else {
				existingEnv[k] = v
			}
		}
	}
	existingSettings["env"] = existingEnv

	if v, ok := p.Settings["model"]; ok {
		if vs, sok := v.(string); sok && vs == "" {
			delete(existingSettings, "model")
		} else {
			existingSettings["model"] = v
		}
	}

	// 覆盖 provider.Settings 中的其他非 env 顶层字段（language、timeout、开关等）
	for k, v := range p.Settings {
		if k == "env" || k == "model" {
			continue
		}
		switch val := v.(type) {
		case string:
			if val == "" {
				delete(existingSettings, k)
			} else {
				existingSettings[k] = v
			}
		case bool:
			if !val {
				delete(existingSettings, k)
			} else {
				existingSettings[k] = v
			}
		case float64:
			if val == 0 {
				delete(existingSettings, k)
			} else {
				existingSettings[k] = v
			}
		case int:
			if val == 0 {
				delete(existingSettings, k)
			} else {
				existingSettings[k] = v
			}
		case map[string]any:
			if len(val) == 0 {
				delete(existingSettings, k)
			} else {
				existingSettings[k] = v
			}
		default:
			existingSettings[k] = v
		}
	}

	// 2. 创建备份
	if err := backupFiles("claude", settingsPath); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	settingsBytes, err := json.MarshalIndent(cleanClaudeSettings(existingSettings), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := atomicWriteFile(settingsPath, settingsBytes, 0600); err != nil {
		return fmt.Errorf("write settings.json: %w", err)
	}

	s.mu.Lock()
	s.state.ActiveClaudeProviderID = id
	s.state.UpdatedAt = time.Now()
	err = s.saveLocked()
	s.mu.Unlock()
	return err
}

func (s *Store) ApplyCodexProvider(id string) error {
	p, ok := s.GetCodexProvider(id)
	if !ok {
		return fmt.Errorf("provider not found")
	}

	configPath := filepath.Join(homeDir, CodexDir, CodexConfigFile)
	authPath := filepath.Join(homeDir, CodexDir, CodexAuthFile)

	// 1. 先读取当前真实配置
	existingConfig := map[string]any{}
	if b, err := os.ReadFile(configPath); err == nil {
		_ = toml.Unmarshal(b, &existingConfig)
	}
	providerConfig := map[string]any{}
	if err := toml.Unmarshal([]byte(p.ConfigTOML), &providerConfig); err != nil {
		return fmt.Errorf("parse provider toml: %w", err)
	}

	// 以供应商 TOML 为最终基础，保持原始格式
	result := strings.TrimSpace(p.ConfigTOML)

	// 从供应商 TOML 中删除旧的 [model_providers.OpenAI] 节（如果有）
	result = deleteTomlSection(result, "[model_providers.OpenAI]")
	// 删除会同步到 model_providers.OpenAI 的顶层字段，避免重复
	for _, k := range []string{"base_url", "wire_api", "requires_openai_auth"} {
		result = deleteTomlTopKey(result, k)
	}
	result = strings.TrimSpace(result)

	// 处理 model_providers.OpenAI 下的核心字段
	provOpenAI := map[string]any{}
	if provMP, ok := providerConfig["model_providers"].(map[string]any); ok {
		if po, ok := provMP["OpenAI"].(map[string]any); ok {
			provOpenAI = po
		}
	}
	// 顶层字段也同步到 model_providers.OpenAI
	for _, k := range []string{"base_url", "wire_api", "requires_openai_auth"} {
		if v, ok := providerConfig[k]; ok {
			provOpenAI[k] = v
		}
	}

	existingMP := map[string]any{}
	if existingConfig["model_providers"] != nil {
		if em, ok := existingConfig["model_providers"].(map[string]any); ok {
			existingMP = em
		}
	}
	existingOpenAI := map[string]any{}
	if existingMP["OpenAI"] != nil {
		if eo, ok := existingMP["OpenAI"].(map[string]any); ok {
			existingOpenAI = eo
		}
	}
	for _, k := range []string{"name", "base_url", "wire_api", "requires_openai_auth", "model"} {
		if v, ok := provOpenAI[k]; ok {
			if v == "" || v == nil || v == false {
				delete(existingOpenAI, k)
			} else {
				existingOpenAI[k] = v
			}
		} else {
			delete(existingOpenAI, k)
		}
	}

	// 构建 [model_providers.OpenAI] 节
	var openAISection strings.Builder
	if len(existingOpenAI) > 0 {
		openAISection.WriteString("\n\n[model_providers.OpenAI]\n")
		for _, k := range []string{"name", "model", "base_url", "wire_api", "requires_openai_auth"} {
			if v, ok := existingOpenAI[k]; ok {
				switch val := v.(type) {
				case string:
					openAISection.WriteString(fmt.Sprintf("%s = \"%s\"\n", k, val))
				case bool:
					openAISection.WriteString(fmt.Sprintf("%s = %v\n", k, val))
				case int:
					openAISection.WriteString(fmt.Sprintf("%s = %d\n", k, val))
				case int64:
					openAISection.WriteString(fmt.Sprintf("%s = %d\n", k, val))
				case float64:
					openAISection.WriteString(fmt.Sprintf("%s = %v\n", k, val))
				}
			}
		}
	}

	// 2. 创建备份
	if err := backupFiles("codex", configPath, authPath); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// 拼接最终 TOML：供应商基础 + OpenAI 节
	var finalBuf strings.Builder
	finalBuf.WriteString(result)
	if openAISection.Len() > 0 {
		finalBuf.WriteString(openAISection.String())
	}

	// auth.json 以供应商配置为准覆盖写入，避免旧凭证残留。
	newAuth := map[string]any{}
	envKey := "OPENAI_API_KEY"
	if ek, ok := p.AuthJSON["env_key"].(string); ok && ek != "" {
		envKey = ek
	}

	for k, v := range p.AuthJSON {
		if v == nil || v == "" {
			continue
		}
		// env_key 映射：将 OPENAI_API_KEY 的值复制到 env_key 指定的键
		if k == "OPENAI_API_KEY" && envKey != "OPENAI_API_KEY" {
			newAuth[envKey] = v
		} else {
			newAuth[k] = v
		}
	}
	authBytes, err := json.MarshalIndent(newAuth, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auth.json: %w", err)
	}

	if err := atomicWriteFile(configPath, []byte(finalBuf.String()), 0600); err != nil {
		return fmt.Errorf("write config.toml: %w", err)
	}
	if err := atomicWriteFile(authPath, authBytes, 0600); err != nil {
		return fmt.Errorf("write auth.json: %w", err)
	}

	s.mu.Lock()
	s.state.ActiveCodexProviderID = id
	s.state.UpdatedAt = time.Now()
	err = s.saveLocked()
	s.mu.Unlock()
	return err
}

// deleteTomlSection 从 TOML 字符串中删除指定节及其内容
func deleteTomlSection(tomlStr, sectionHeader string) string {
	idx := strings.Index(tomlStr, sectionHeader)
	if idx == -1 {
		return tomlStr
	}
	start := idx
	if start > 0 && tomlStr[start-1] == '\n' {
		start--
	}
	end := len(tomlStr)
	for i := idx + len(sectionHeader); i < len(tomlStr); i++ {
		if tomlStr[i] == '[' && (i == 0 || tomlStr[i-1] == '\n') {
			end = i
			break
		}
	}
	return tomlStr[:start] + tomlStr[end:]
}

// deleteTomlTopKey 从 TOML 字符串中删除指定顶层键及其值
func deleteTomlTopKey(tomlStr, key string) string {
	lines := strings.Split(tomlStr, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" =") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// CurrentConfig 读写当前真实配置
func (s *Store) ReadCurrentClaudeSettings() (map[string]any, error) {
	path := filepath.Join(homeDir, ClaudeDir, ClaudeSettingsFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) ReadCurrentClaudeJSON() (map[string]any, error) {
	path := filepath.Join(homeDir, ClaudeJSONFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) ReadCurrentCodexConfig() (string, error) {
	path := filepath.Join(homeDir, CodexDir, CodexConfigFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func (s *Store) ReadCurrentCodexAuth() (map[string]any, error) {
	path := filepath.Join(homeDir, CodexDir, CodexAuthFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) WriteCurrentClaudeSettings(v map[string]any) error {
	path := filepath.Join(homeDir, ClaudeDir, ClaudeSettingsFile)
	if err := backupFiles("claude", path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cleanClaudeSettings(v), "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b, 0600)
}

func (s *Store) WriteCurrentClaudeJSON(v map[string]any) error {
	path := filepath.Join(homeDir, ClaudeJSONFile)
	if err := backupFiles("claude", path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b, 0600)
}

func (s *Store) WriteCurrentCodexConfig(tomlStr string) error {
	path := filepath.Join(homeDir, CodexDir, CodexConfigFile)
	if err := backupFiles("codex", path); err != nil {
		return err
	}
	// 验证 TOML
	var m map[string]any
	if err := toml.Unmarshal([]byte(tomlStr), &m); err != nil {
		return fmt.Errorf("invalid toml: %w", err)
	}
	return atomicWriteFile(path, []byte(tomlStr), 0600)
}

func (s *Store) WriteCurrentCodexAuth(v map[string]any) error {
	path := filepath.Join(homeDir, CodexDir, CodexAuthFile)
	if err := backupFiles("codex", path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b, 0600)
}

// Backup / Restore
func backupFiles(tool string, paths ...string) error {
	timestamp := time.Now().Format("20060102_150405")
	backupDir := filepath.Join(ProjectDir(), BackupsDir, tool+"_"+timestamp)
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return err
	}
	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		name := filepath.Base(p)
		if err := os.WriteFile(filepath.Join(backupDir, name), b, 0600); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListBackups(tool string) ([]string, error) {
	base := filepath.Join(ProjectDir(), BackupsDir)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []string
	prefix := tool + "_"
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			result = append(result, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(result)))
	return result, nil
}

func (s *Store) RestoreBackup(tool, backupName string) error {
	var targets []string
	if tool == "claude" {
		targets = []string{
			filepath.Join(homeDir, ClaudeDir, ClaudeSettingsFile),
			filepath.Join(homeDir, ClaudeJSONFile),
		}
	} else if tool == "codex" {
		targets = []string{
			filepath.Join(homeDir, CodexDir, CodexConfigFile),
			filepath.Join(homeDir, CodexDir, CodexAuthFile),
		}
	} else {
		return fmt.Errorf("unknown tool")
	}

	if !validBackupName(tool, backupName) {
		return fmt.Errorf("invalid backup name")
	}
	backupDir := filepath.Join(ProjectDir(), BackupsDir, backupName)
	info, err := os.Stat(backupDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("backup not found")
	}

	for _, target := range targets {
		name := filepath.Base(target)
		src := filepath.Join(backupDir, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := atomicWriteFile(target, b, 0600); err != nil {
			return err
		}
	}
	return nil
}

func validBackupName(tool, backupName string) bool {
	if backupName == "" || filepath.Base(backupName) != backupName || strings.Contains(backupName, `\`) {
		return false
	}
	prefix := tool + "_"
	if !strings.HasPrefix(backupName, prefix) {
		return false
	}
	_, err := time.Parse("20060102_150405", strings.TrimPrefix(backupName, prefix))
	return err == nil
}

// ImportCurrent 导入当前配置为供应商
func (s *Store) ImportCurrent(tool, name string) error {
	if tool == "claude" || tool == "all" {
		settings, _ := s.ReadCurrentClaudeSettings()
		claudeJSON, _ := s.ReadCurrentClaudeJSON()
		p := models.ClaudeProvider{
			ID:         generateID("claude"),
			Name:       name,
			Settings:   settings,
			ClaudeJSON: claudeJSON,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := s.SaveClaudeProvider(&p); err != nil {
			return err
		}
	}
	if tool == "codex" || tool == "all" {
		configStr, _ := s.ReadCurrentCodexConfig()
		auth, _ := s.ReadCurrentCodexAuth()
		p := models.CodexProvider{
			ID:         generateID("codex"),
			Name:       name,
			ConfigTOML: configStr,
			AuthJSON:   auth,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := s.SaveCodexProvider(&p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) saveLocked() error {
	base := ProjectDir()
	if err := s.saveJSON(filepath.Join(base, ClaudeProvidersFile), s.claudeProviders); err != nil {
		return err
	}
	if err := s.saveJSON(filepath.Join(base, CodexProvidersFile), s.codexProviders); err != nil {
		return err
	}
	return s.saveJSON(filepath.Join(base, StateFile), s.state)
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
