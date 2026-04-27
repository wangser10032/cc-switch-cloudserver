package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cc-switch/internal/config"
	"cc-switch/internal/models"
	"cc-switch/internal/proxy"

	"github.com/BurntSushi/toml"
)

type Handler struct {
	Store *config.Store
	Proxy *proxy.Proxy
}

func New(s *config.Store) *Handler {
	return &Handler{
		Store: s,
		Proxy: proxy.New(s),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/ccswitch/api/claude/providers", h.claudeProviders)
	mux.HandleFunc("/ccswitch/api/claude/providers/", h.claudeProviderDetail)
	mux.HandleFunc("/ccswitch/api/claude/apply", h.applyClaude)
	mux.HandleFunc("/ccswitch/api/claude/test", h.testClaude)

	mux.HandleFunc("/ccswitch/api/codex/providers", h.codexProviders)
	mux.HandleFunc("/ccswitch/api/codex/providers/", h.codexProviderDetail)
	mux.HandleFunc("/ccswitch/api/codex/apply", h.applyCodex)
	mux.HandleFunc("/ccswitch/api/codex/test", h.testCodex)

	mux.HandleFunc("/ccswitch/api/current/claude", h.currentClaude)
	mux.HandleFunc("/ccswitch/api/current/codex", h.currentCodex)
	mux.HandleFunc("/ccswitch/api/current/save", h.saveCurrent)

	mux.HandleFunc("/ccswitch/api/backups", h.backups)
	mux.HandleFunc("/ccswitch/api/backups/restore", h.restoreBackup)

	mux.HandleFunc("/ccswitch/proxy/openai/", h.Proxy.Handler)
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) respondError(w http.ResponseWriter, status int, msg string) {
	h.respondJSON(w, status, map[string]string{"error": msg})
}

// Claude Providers
func (h *Handler) claudeProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.respondJSON(w, http.StatusOK, h.Store.ClaudeProviders())
	case http.MethodPost:
		var p models.ClaudeProvider
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if err := h.Store.SaveClaudeProvider(&p); err != nil {
			h.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.respondJSON(w, http.StatusOK, p)
	default:
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) claudeProviderDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/ccswitch/api/claude/providers/")
	if id == "" {
		h.respondError(w, http.StatusBadRequest, "missing id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		p, ok := h.Store.GetClaudeProvider(id)
		if !ok {
			h.respondError(w, http.StatusNotFound, "not found")
			return
		}
		h.respondJSON(w, http.StatusOK, p)
	case http.MethodPut:
		var p models.ClaudeProvider
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid json")
			return
		}
		p.ID = id
		if err := h.Store.SaveClaudeProvider(&p); err != nil {
			h.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.respondJSON(w, http.StatusOK, p)
	case http.MethodDelete:
		if err := h.Store.DeleteClaudeProvider(id); err != nil {
			h.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) applyClaude(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.Store.ApplyClaudeProvider(req.ID); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) testClaude(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ProviderID string `json:"provider_id"`
		BaseURL    string `json:"base_url"`
		APIKey     string `json:"api_key"`
		Model      string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	baseURL := req.BaseURL
	apiKey := req.APIKey
	model := req.Model

	if req.ProviderID != "" {
		p, ok := h.Store.GetClaudeProvider(req.ProviderID)
		if ok {
			if env, ok := p.Settings["env"].(map[string]any); ok {
				if v, ok := env["ANTHROPIC_BASE_URL"].(string); ok && v != "" {
					baseURL = v
				}
				if v, ok := env["ANTHROPIC_AUTH_TOKEN"].(string); ok && v != "" {
					apiKey = v
				}
				if v, ok := env["ANTHROPIC_MODEL"].(string); ok && v != "" {
					model = v
				}
			}
		}
	}

	var missing []string
	if baseURL == "" {
		missing = append(missing, "Base URL")
	}
	if apiKey == "" {
		missing = append(missing, "API Key")
	}
	if model == "" {
		missing = append(missing, "Model")
	}
	if len(missing) > 0 {
		h.respondError(w, http.StatusBadRequest, "缺少: "+strings.Join(missing, ", "))
		return
	}

	// Anthropic Messages API 连通性测试
	body := map[string]any{
		"model":      model,
		"max_tokens": 8,
		"messages":   []map[string]any{{"role": "user", "content": "hi"}},
	}
	b, _ := json.Marshal(body)

	// 构造 URL：确保以 /messages 结尾
	url := strings.TrimSuffix(baseURL, "/")
	if !strings.Contains(url, "/messages") {
		url = url + "/messages"
	}

	client := &http.Client{Timeout: 30 * time.Second}
	testReq, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(b)))
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	testReq.Header.Set("Content-Type", "application/json")
	testReq.Header.Set("Authorization", "Bearer "+apiKey)
	testReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(testReq)
	if err != nil {
		h.respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		s := string(respBody)
		s = strings.ReplaceAll(s, apiKey, "***")
		h.respondError(w, http.StatusBadGateway, fmt.Sprintf("status %d: %s", resp.StatusCode, s))
		return
	}

	// 解析 Anthropic 响应（支持 text 和 thinking 类型）
	var m map[string]any
	json.Unmarshal(respBody, &m)

	var reply string
	if content, ok := m["content"].([]any); ok && len(content) > 0 {
		for _, c := range content {
			if cm, ok := c.(map[string]any); ok {
				// 优先取 text 类型
				if t, ok := cm["type"].(string); ok {
					if t == "text" {
						if text, ok := cm["text"].(string); ok {
							reply = text
							break
						}
					} else if t == "thinking" {
						// 取 thinking 的 thinking 内容
						if text, ok := cm["thinking"].(string); ok {
							reply = "[思考中...] " + text
						}
					}
				}
			}
		}
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"reply": reply})
}

// Codex Providers
func (h *Handler) codexProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.respondJSON(w, http.StatusOK, h.Store.CodexProviders())
	case http.MethodPost:
		var p models.CodexProvider
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid json")
			return
		}
		// 验证 toml
		var tm map[string]any
		if err := toml.Unmarshal([]byte(p.ConfigTOML), &tm); err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid toml: "+err.Error())
			return
		}
		if err := h.Store.SaveCodexProvider(&p); err != nil {
			h.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.respondJSON(w, http.StatusOK, p)
	default:
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) codexProviderDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/ccswitch/api/codex/providers/")
	if id == "" {
		h.respondError(w, http.StatusBadRequest, "missing id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		p, ok := h.Store.GetCodexProvider(id)
		if !ok {
			h.respondError(w, http.StatusNotFound, "not found")
			return
		}
		h.respondJSON(w, http.StatusOK, p)
	case http.MethodPut:
		var p models.CodexProvider
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid json")
			return
		}
		p.ID = id
		if err := h.Store.SaveCodexProvider(&p); err != nil {
			h.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.respondJSON(w, http.StatusOK, p)
	case http.MethodDelete:
		if err := h.Store.DeleteCodexProvider(id); err != nil {
			h.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) applyCodex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.Store.ApplyCodexProvider(req.ID); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) testCodex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ProviderID string `json:"provider_id"`
		BaseURL    string `json:"base_url"`
		APIKey     string `json:"api_key"`
		Model      string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	baseURL := req.BaseURL
	apiKey := req.APIKey
	model := req.Model
	authMode := ""
	accessToken := ""

	if req.ProviderID != "" {
		p, ok := h.Store.GetCodexProvider(req.ProviderID)
		if ok {
			// 检查是否为官方登录模式
			if v, ok := p.AuthJSON["auth_mode"].(string); ok {
				authMode = v
			}
			if tokens, ok := p.AuthJSON["tokens"].(map[string]any); ok {
				if v, ok := tokens["access_token"].(string); ok && v != "" {
					accessToken = v
				}
			}

			// 从 config toml 中解析 base_url 和 model
			var tm map[string]any
			toml.Unmarshal([]byte(p.ConfigTOML), &tm)
			if m, ok := tm["model"].(string); ok && m != "" {
				model = m
			}
			modelProvider := "OpenAI"
			if v, ok := tm["model_provider"].(string); ok && v != "" {
				modelProvider = v
			}
			if v, ok := tm["base_url"].(string); ok && v != "" {
				baseURL = v
			}
			if mp, ok := tm["model_providers"].(map[string]any); ok {
				if provider, ok := mp[modelProvider].(map[string]any); ok {
					if v, ok := provider["base_url"].(string); ok && v != "" {
						baseURL = v
					}
					if v, ok := provider["model"].(string); ok && v != "" && model == "" {
						model = v
					}
				} else {
					// 尝试从任意 provider 获取 base_url
					for _, raw := range mp {
						if provider, ok := raw.(map[string]any); ok {
							if v, ok := provider["base_url"].(string); ok && v != "" {
								baseURL = v
								break
							}
						}
					}
				}
			}
			// 从 auth.json 获取 API Key
			envKey := "OPENAI_API_KEY"
			if v, ok := p.AuthJSON["env_key"].(string); ok && v != "" {
				envKey = v
			}
			if v, ok := p.AuthJSON[envKey].(string); ok && v != "" {
				apiKey = v
			} else if v, ok := p.AuthJSON["OPENAI_API_KEY"].(string); ok && v != "" {
				apiKey = v
			}
		}
	}

	// 优先使用 access_token（官方登录模式）
	if accessToken != "" {
		apiKey = accessToken
	}

	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// 官方登录模式：如果没有 API Key 但有 auth_mode=chatgpt，提示用户
	if apiKey == "" && authMode == "chatgpt" {
		h.respondError(w, http.StatusBadRequest, "官方登录模式：请通过 codex auth 登录获取 token")
		return
	}

	// 官方登录模式：如果没有 model，使用默认值
	if model == "" && accessToken != "" {
		model = "gpt-4o"
	}

	var missing []string
	if apiKey == "" {
		missing = append(missing, "API Key")
	}
	if model == "" {
		missing = append(missing, "Model")
	}
	if len(missing) > 0 {
		h.respondError(w, http.StatusBadRequest, "缺少: "+strings.Join(missing, ", "))
		return
	}

	// 使用 OpenAI API 进行测试（优先 Responses API，回退到 Chat Completions）
	reply, err := testCodexConnection(baseURL, apiKey, model)
	if err != nil {
		s := err.Error()
		s = strings.ReplaceAll(s, apiKey, "***")
		h.respondError(w, http.StatusBadGateway, s)
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"reply": reply})
}

// Current Config
func (h *Handler) currentClaude(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, _ := h.Store.ReadCurrentClaudeSettings()
		claudeJSON, _ := h.Store.ReadCurrentClaudeJSON()
		h.respondJSON(w, http.StatusOK, map[string]any{
			"settings":    settings,
			"claude_json": claudeJSON,
		})
	case http.MethodPut:
		var req struct {
			Settings   map[string]any `json:"settings"`
			ClaudeJSON map[string]any `json:"claude_json"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if req.Settings != nil {
			if err := h.Store.WriteCurrentClaudeSettings(req.Settings); err != nil {
				h.respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if req.ClaudeJSON != nil {
			if err := h.Store.WriteCurrentClaudeJSON(req.ClaudeJSON); err != nil {
				h.respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) currentCodex(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		configStr, _ := h.Store.ReadCurrentCodexConfig()
		auth, _ := h.Store.ReadCurrentCodexAuth()
		h.respondJSON(w, http.StatusOK, map[string]any{
			"config": configStr,
			"auth":   auth,
		})
	case http.MethodPut:
		var req struct {
			Config *string        `json:"config"`
			Auth   map[string]any `json:"auth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if req.Config != nil {
			if err := h.Store.WriteCurrentCodexConfig(*req.Config); err != nil {
				h.respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if req.Auth != nil {
			if err := h.Store.WriteCurrentCodexAuth(req.Auth); err != nil {
				h.respondError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) saveCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Tool string `json:"tool"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		h.respondError(w, http.StatusBadRequest, "name required")
		return
	}
	if req.Tool != "claude" && req.Tool != "codex" && req.Tool != "all" {
		h.respondError(w, http.StatusBadRequest, "tool must be claude, codex or all")
		return
	}
	if err := h.Store.ImportCurrent(req.Tool, req.Name); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Backups
func (h *Handler) backups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	tool := r.URL.Query().Get("tool")
	if tool == "" {
		h.respondError(w, http.StatusBadRequest, "tool required")
		return
	}
	list, err := h.Store.ListBackups(tool)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"backups": list})
}

func (h *Handler) restoreBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Tool       string `json:"tool"`
		BackupName string `json:"backup_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.Store.RestoreBackup(req.Tool, req.BackupName); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CLI import handler
func (h *Handler) HandleCLIImport(tool, name string) error {
	return h.Store.ImportCurrent(tool, name)
}

// testCodexConnection 测试 Codex/OpenAI 兼容 API 连接
// 支持 Responses API (SSE) 和 Chat Completions API
func testCodexConnection(baseURL, apiKey, model string) (string, error) {
	// 先尝试 Responses API (sub2api 等代理使用这个)
	if reply, err := testResponsesAPI(baseURL, apiKey, model); err == nil {
		return reply, nil
	}
	// 回退到 Chat Completions API
	return testChatCompletion(baseURL, apiKey, model)
}

// testResponsesAPI 测试 OpenAI Responses API (SSE 流式响应)
func testResponsesAPI(baseURL, apiKey, model string) (string, error) {
	body := map[string]any{
		"model":       model,
		"input":       []map[string]any{{"role": "user", "content": "hi"}},
		"instructions": "You are a helpful assistant.",
	}
	b, _ := json.Marshal(body)

	url := strings.TrimSuffix(baseURL, "/")
	if !strings.Contains(url, "/responses") {
		url = url + "/responses"
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()


	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析 SSE 流式响应，提取 response.output_text.done 事件的 text
	scanner := bufio.NewScanner(resp.Body)
	const maxScanTokenSize = 1024 * 1024 // 1MB
	scanner.Buffer(make([]byte, 256), maxScanTokenSize)
	var fullText strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var event map[string]any
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				// 查找 output_text.done 或 output_text.delta
				if eventType, ok := event["type"].(string); ok {
					// response.output_text.done 直接包含 text 字段
					if eventType == "response.output_text.done" {
						if text, ok := event["text"].(string); ok {
							return text, nil
						}
					}
					// response.output_item.done 包含完整消息
					if eventType == "response.output_item.done" {
						if item, ok := event["item"].(map[string]any); ok {
							if content, ok := item["content"].([]any); ok {
								for _, c := range content {
									if cm, ok := c.(map[string]any); ok {
										if cm["type"] == "output_text" {
											if text, ok := cm["text"].(string); ok {
												return text, nil
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if fullText.Len() > 0 {
		return fullText.String(), nil
	}
	return "", fmt.Errorf("no response content")
}

// testChatCompletion 使用 OpenAI Chat Completions API 测试连接
func testChatCompletion(baseURL, apiKey, model string) (string, error) {
	body := map[string]any{
		"model":       model,
		"max_tokens":  8,
		"messages":    []map[string]any{{"role": "user", "content": "hi"}},
	}
	b, _ := json.Marshal(body)

	// 构造 URL：确保以 /chat/completions 结尾
	url := strings.TrimSuffix(baseURL, "/")
	if !strings.Contains(url, "/chat/completions") {
		url = url + "/chat/completions"
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var m map[string]any
	if err := json.Unmarshal(respBody, &m); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	// 提取 assistant 回复
	if choices, ok := m["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if msg, ok := choice["message"].(map[string]any); ok {
				if text, ok := msg["content"].(string); ok {
					return text, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no response content")
}
