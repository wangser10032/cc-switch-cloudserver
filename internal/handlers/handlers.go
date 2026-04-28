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

type claudeTestRequest struct {
	ProviderID string `json:"provider_id"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model"`
}

type codexTestRequest struct {
	ProviderID string `json:"provider_id"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model"`
}

type testTarget struct {
	BaseURL  string
	APIKey   string
	Model    string
	WireAPI  string
	AuthMode string
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
	mux.HandleFunc("/ccswitch/api/claude/test/stream", h.testClaudeStream)

	mux.HandleFunc("/ccswitch/api/codex/providers", h.codexProviders)
	mux.HandleFunc("/ccswitch/api/codex/providers/", h.codexProviderDetail)
	mux.HandleFunc("/ccswitch/api/codex/apply", h.applyCodex)
	mux.HandleFunc("/ccswitch/api/codex/test", h.testCodex)
	mux.HandleFunc("/ccswitch/api/codex/test/stream", h.testCodexStream)

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
	var req claudeTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	target, err := h.resolveClaudeTestTarget(req)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	reply, err := testClaudeConnection(target.BaseURL, target.APIKey, target.Model)
	if err != nil {
		s := strings.ReplaceAll(err.Error(), target.APIKey, "***")
		h.respondError(w, http.StatusBadGateway, s)
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"reply": reply})
}

func (h *Handler) testClaudeStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req claudeTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}
	target, err := h.resolveClaudeTestTarget(req)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	stream, ok := newTestEventStream(w, target.APIKey)
	if !ok {
		return
	}
	stream.status("开始测试 Claude Code 供应商")
	if err := streamClaudeConnection(target.BaseURL, target.APIKey, target.Model, stream); err != nil {
		stream.err(err)
		return
	}
	stream.done()
}

func (h *Handler) resolveClaudeTestTarget(req claudeTestRequest) (testTarget, error) {
	target := testTarget{
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
		Model:   req.Model,
	}

	if req.ProviderID != "" {
		p, ok := h.Store.GetClaudeProvider(req.ProviderID)
		if ok {
			if env, ok := p.Settings["env"].(map[string]any); ok {
				if v, ok := env["ANTHROPIC_BASE_URL"].(string); ok && v != "" {
					target.BaseURL = v
				}
				if v, ok := env["ANTHROPIC_AUTH_TOKEN"].(string); ok && v != "" {
					target.APIKey = v
				}
				if v, ok := env["ANTHROPIC_MODEL"].(string); ok && v != "" {
					target.Model = v
				}
			}
		}
	}

	var missing []string
	if target.BaseURL == "" {
		missing = append(missing, "Base URL")
	}
	if target.APIKey == "" {
		missing = append(missing, "API Key")
	}
	if target.Model == "" {
		missing = append(missing, "Model")
	}
	if len(missing) > 0 {
		return target, fmt.Errorf("缺少: %s", strings.Join(missing, ", "))
	}
	return target, nil
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
	var req codexTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	target, err := h.resolveCodexTestTarget(req)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if target.AuthMode == "chatgpt" {
		h.respondJSON(w, http.StatusOK, map[string]string{
			"reply": fmt.Sprintf("官方登录模式配置可用：已找到 Codex 登录 token，模型 %s。此模式不使用普通 OpenAI API Key 发起测试请求。", target.Model),
		})
		return
	}

	// 使用 OpenAI API 进行测试，并尊重 Codex 配置里的 wire_api。
	reply, err := testCodexConnection(target.BaseURL, target.APIKey, target.Model, target.WireAPI)
	if err != nil {
		s := err.Error()
		s = strings.ReplaceAll(s, target.APIKey, "***")
		h.respondError(w, http.StatusBadGateway, s)
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"reply": reply})
}

func (h *Handler) testCodexStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req codexTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}
	target, err := h.resolveCodexTestTarget(req)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	stream, ok := newTestEventStream(w, target.APIKey)
	if !ok {
		return
	}
	stream.status("开始测试 Codex 供应商")
	if target.AuthMode == "chatgpt" {
		stream.status("检测到官方登录模式")
		stream.delta(fmt.Sprintf("官方登录模式配置可用：已找到 Codex 登录 token，模型 %s。此模式不使用普通 OpenAI API Key 发起测试请求。", target.Model))
		stream.done()
		return
	}
	if err := streamCodexConnection(target.BaseURL, target.APIKey, target.Model, target.WireAPI, stream); err != nil {
		stream.err(err)
		return
	}
	stream.done()
}

func (h *Handler) resolveCodexTestTarget(req codexTestRequest) (testTarget, error) {
	target := testTarget{
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
		Model:   req.Model,
	}
	authMode := ""
	accessToken := ""

	if req.ProviderID != "" {
		p, ok := h.Store.GetCodexProvider(req.ProviderID)
		if ok {
			// 检查是否为官方登录模式
			if v, ok := p.AuthJSON["auth_mode"].(string); ok {
				authMode = v
				target.AuthMode = v
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
				target.Model = m
			}
			modelProvider := "OpenAI"
			if v, ok := tm["model_provider"].(string); ok && v != "" {
				modelProvider = v
			}
			if v, ok := tm["base_url"].(string); ok && v != "" {
				target.BaseURL = v
			}
			if mp, ok := tm["model_providers"].(map[string]any); ok {
				if provider, ok := mp[modelProvider].(map[string]any); ok {
					if v, ok := provider["base_url"].(string); ok && v != "" {
						target.BaseURL = v
					}
					if v, ok := provider["model"].(string); ok && v != "" && target.Model == "" {
						target.Model = v
					}
					if v, ok := provider["wire_api"].(string); ok && v != "" {
						target.WireAPI = v
					}
				} else {
					// 尝试从任意 provider 获取 base_url
					for _, raw := range mp {
						if provider, ok := raw.(map[string]any); ok {
							if v, ok := provider["base_url"].(string); ok && v != "" {
								target.BaseURL = v
							}
							if v, ok := provider["wire_api"].(string); ok && v != "" {
								target.WireAPI = v
							}
							if target.BaseURL != "" || target.WireAPI != "" {
								break
							}
						}
					}
				}
			}
			if v, ok := tm["wire_api"].(string); ok && v != "" {
				target.WireAPI = v
			}
			// 从 auth.json 获取 API Key
			envKey := "OPENAI_API_KEY"
			if v, ok := p.AuthJSON["env_key"].(string); ok && v != "" {
				envKey = v
			}
			if v, ok := p.AuthJSON[envKey].(string); ok && v != "" {
				target.APIKey = v
			} else if v, ok := p.AuthJSON["OPENAI_API_KEY"].(string); ok && v != "" {
				target.APIKey = v
			}
		}
	}

	// 优先使用 access_token（官方登录模式）
	if accessToken != "" {
		target.APIKey = accessToken
	}

	if target.BaseURL == "" {
		target.BaseURL = "https://api.openai.com/v1"
	}

	// 官方登录模式：如果没有 API Key 但有 auth_mode=chatgpt，提示用户
	if target.APIKey == "" && authMode == "chatgpt" {
		return target, fmt.Errorf("官方登录模式：请通过 codex auth 登录获取 token")
	}

	// 官方登录模式：如果没有 model，使用默认值
	if target.Model == "" && accessToken != "" {
		target.Model = "gpt-4o"
	}

	var missing []string
	if target.APIKey == "" {
		missing = append(missing, "API Key")
	}
	if target.Model == "" {
		missing = append(missing, "Model")
	}
	if len(missing) > 0 {
		return target, fmt.Errorf("缺少: %s", strings.Join(missing, ", "))
	}
	return target, nil
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

type testEventStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
	apiKey  string
}

func newTestEventStream(w http.ResponseWriter, apiKey string) (*testEventStream, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	stream := &testEventStream{w: w, flusher: flusher, apiKey: apiKey}
	stream.send("open", map[string]string{"status": "ok"})
	return stream, true
}

func (s *testEventStream) status(message string) {
	s.send("status", map[string]string{"message": s.sanitize(message)})
}

func (s *testEventStream) delta(text string) {
	if text == "" {
		return
	}
	s.send("delta", map[string]string{"text": s.sanitize(text)})
}

func (s *testEventStream) done() {
	s.send("done", map[string]string{"status": "ok"})
}

func (s *testEventStream) err(err error) {
	s.send("error", map[string]string{"message": s.sanitize(err.Error())})
}

func (s *testEventStream) send(event string, payload any) {
	b, _ := json.Marshal(payload)
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, b)
	s.flusher.Flush()
}

func (s *testEventStream) sanitize(text string) string {
	if s.apiKey != "" {
		text = strings.ReplaceAll(text, s.apiKey, "***")
	}
	return text
}

func streamClaudeConnection(baseURL, apiKey, model string, stream *testEventStream) error {
	var errs []string
	for _, url := range anthropicTestURLs(baseURL) {
		stream.status("尝试 Anthropic Messages: " + url)
		if err := streamAnthropicMessages(url, apiKey, model, stream); err == nil {
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", url, err))
		}
	}

	stream.status("尝试 OpenAI Chat Completions 兼容接口")
	if err := streamChatCompletion(baseURL, apiKey, model, stream); err == nil {
		return nil
	} else {
		errs = append(errs, fmt.Sprintf("chat/completions: %v", err))
	}

	stream.status("尝试 OpenAI Responses 兼容接口")
	if err := streamResponsesAPI(baseURL, apiKey, model, stream); err == nil {
		return nil
	} else {
		errs = append(errs, fmt.Sprintf("responses: %v", err))
	}

	return fmt.Errorf("%s", strings.Join(errs, " | "))
}

func streamCodexConnection(baseURL, apiKey, model, wireAPI string, stream *testEventStream) error {
	preferChat := wireAPI == "chat" || wireAPI == "chat_completions" || wireAPI == "chat-completions"
	if preferChat {
		stream.status("尝试 OpenAI Chat Completions")
		if err := streamChatCompletion(baseURL, apiKey, model, stream); err == nil {
			return nil
		}
		stream.status("Chat Completions 未通过，回退到 Responses")
		return streamResponsesAPI(baseURL, apiKey, model, stream)
	}

	stream.status("尝试 OpenAI Responses")
	if err := streamResponsesAPI(baseURL, apiKey, model, stream); err == nil {
		return nil
	}
	stream.status("Responses 未通过，回退到 Chat Completions")
	return streamChatCompletion(baseURL, apiKey, model, stream)
}

func streamAnthropicMessages(url, apiKey, model string, stream *testEventStream) error {
	body := map[string]any{
		"model":      model,
		"max_tokens": 64,
		"messages":   []map[string]any{{"role": "user", "content": "hi"}},
		"stream":     true,
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		respBody, _ := io.ReadAll(resp.Body)
		reply, err := extractAnthropicReply(respBody)
		if err != nil {
			return err
		}
		stream.delta(reply)
		return nil
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256), 1024*1024)
	wrote := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event["type"] == "error" {
			return fmt.Errorf("%v", event["error"])
		}
		if block, ok := event["content_block"].(map[string]any); ok && block["type"] == "text" {
			if text, ok := block["text"].(string); ok && text != "" {
				stream.delta(text)
				wrote = true
			}
		}
		if delta, ok := event["delta"].(map[string]any); ok {
			if text, ok := delta["text"].(string); ok && text != "" {
				stream.delta(text)
				wrote = true
			}
			if text, ok := delta["thinking"].(string); ok && text != "" {
				stream.delta(text)
				wrote = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !wrote {
		return fmt.Errorf("no response content")
	}
	return nil
}

func streamResponsesAPI(baseURL, apiKey, model string, stream *testEventStream) error {
	body := map[string]any{
		"model":        model,
		"input":        []map[string]any{{"role": "user", "content": "hi"}},
		"instructions": "You are a helpful assistant.",
		"stream":       true,
	}
	b, _ := json.Marshal(body)

	url := responsesURL(baseURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		respBody, _ := io.ReadAll(resp.Body)
		reply, err := extractResponsesReply(respBody)
		if err != nil {
			return err
		}
		stream.delta(reply)
		return nil
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256), 1024*1024)
	wrote := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if errObj, ok := event["error"]; ok && errObj != nil {
			return fmt.Errorf("%v", errObj)
		}
		eventType, _ := event["type"].(string)
		if eventType == "response.output_text.delta" {
			if text, ok := event["delta"].(string); ok && text != "" {
				stream.delta(text)
				wrote = true
			}
		}
		if eventType == "response.output_text.done" && !wrote {
			if text, ok := event["text"].(string); ok && text != "" {
				stream.delta(text)
				wrote = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !wrote {
		return fmt.Errorf("no response content")
	}
	return nil
}

func streamChatCompletion(baseURL, apiKey, model string, stream *testEventStream) error {
	body := map[string]any{
		"model":      model,
		"max_tokens": 64,
		"messages":   []map[string]any{{"role": "user", "content": "hi"}},
		"stream":     true,
	}
	b, _ := json.Marshal(body)

	url := chatCompletionURL(baseURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		respBody, _ := io.ReadAll(resp.Body)
		reply, err := extractChatCompletionReply(respBody)
		if err != nil {
			return err
		}
		stream.delta(reply)
		return nil
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256), 1024*1024)
	wrote := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if choices, ok := event["choices"].([]any); ok && len(choices) > 0 {
			choice, _ := choices[0].(map[string]any)
			delta, _ := choice["delta"].(map[string]any)
			if text, ok := delta["content"].(string); ok && text != "" {
				stream.delta(text)
				wrote = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !wrote {
		return fmt.Errorf("no response content")
	}
	return nil
}

func testClaudeConnection(baseURL, apiKey, model string) (string, error) {
	var errs []string
	for _, url := range anthropicTestURLs(baseURL) {
		reply, err := testAnthropicMessages(url, apiKey, model)
		if err == nil {
			return reply, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", url, err))
	}

	// Some Claude Code-compatible providers are actually OpenAI-compatible
	// endpoints behind ANTHROPIC_* environment variables.
	if reply, err := testChatCompletion(baseURL, apiKey, model); err == nil {
		return reply, nil
	} else {
		errs = append(errs, fmt.Sprintf("chat/completions: %v", err))
	}
	if reply, err := testResponsesAPI(baseURL, apiKey, model); err == nil {
		return reply, nil
	} else {
		errs = append(errs, fmt.Sprintf("responses: %v", err))
	}

	return "", fmt.Errorf("%s", strings.Join(errs, " | "))
}

func anthropicTestURLs(baseURL string) []string {
	base := strings.TrimSuffix(baseURL, "/")
	if strings.Contains(base, "/messages") {
		return []string{base}
	}
	return []string{
		base + "/v1/messages",
		base + "/messages",
	}
}

func extractAnthropicReply(respBody []byte) (string, error) {
	var m map[string]any
	if err := json.Unmarshal(respBody, &m); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if content, ok := m["content"].([]any); ok && len(content) > 0 {
		for _, c := range content {
			if cm, ok := c.(map[string]any); ok {
				if cm["type"] == "text" {
					if text, ok := cm["text"].(string); ok {
						return text, nil
					}
				}
				if cm["type"] == "thinking" {
					if text, ok := cm["thinking"].(string); ok {
						return "[思考中...] " + text, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("no response content")
}

func testAnthropicMessages(url, apiKey, model string) (string, error) {
	body := map[string]any{
		"model":      model,
		"max_tokens": 8,
		"messages":   []map[string]any{{"role": "user", "content": "hi"}},
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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

	return extractAnthropicReply(respBody)
}

// testCodexConnection 测试 Codex/OpenAI 兼容 API 连接
// 支持 Responses API (SSE) 和 Chat Completions API
func testCodexConnection(baseURL, apiKey, model, wireAPI string) (string, error) {
	preferChat := wireAPI == "chat" || wireAPI == "chat_completions" || wireAPI == "chat-completions"
	if preferChat {
		if reply, err := testChatCompletion(baseURL, apiKey, model); err == nil {
			return reply, nil
		}
		return testResponsesAPI(baseURL, apiKey, model)
	}

	// 默认优先 Responses API，兼容 sub2api 和官方 OpenAI Responses 配置。
	if reply, err := testResponsesAPI(baseURL, apiKey, model); err == nil {
		return reply, nil
	}
	return testChatCompletion(baseURL, apiKey, model)
}

func responsesURL(baseURL string) string {
	url := strings.TrimSuffix(baseURL, "/")
	if !strings.Contains(url, "/responses") {
		url = url + "/responses"
	}
	return url
}

func chatCompletionURL(baseURL string) string {
	url := strings.TrimSuffix(baseURL, "/")
	if !strings.Contains(url, "/chat/completions") {
		url = url + "/chat/completions"
	}
	return url
}

func extractResponsesReply(respBody []byte) (string, error) {
	var m map[string]any
	if err := json.Unmarshal(respBody, &m); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if text, ok := m["output_text"].(string); ok && text != "" {
		return text, nil
	}
	if output, ok := m["output"].([]any); ok {
		var out strings.Builder
		for _, item := range output {
			im, _ := item.(map[string]any)
			if content, ok := im["content"].([]any); ok {
				for _, c := range content {
					cm, _ := c.(map[string]any)
					if text, ok := cm["text"].(string); ok {
						out.WriteString(text)
					}
				}
			}
		}
		if out.Len() > 0 {
			return out.String(), nil
		}
	}
	return "", fmt.Errorf("no response content")
}

func extractChatCompletionReply(respBody []byte) (string, error) {
	var m map[string]any
	if err := json.Unmarshal(respBody, &m); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

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

// testResponsesAPI 测试 OpenAI Responses API (SSE 流式响应)
func testResponsesAPI(baseURL, apiKey, model string) (string, error) {
	body := map[string]any{
		"model":        model,
		"input":        []map[string]any{{"role": "user", "content": "hi"}},
		"instructions": "You are a helpful assistant.",
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, responsesURL(baseURL), bytes.NewReader(b))
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
					if eventType == "response.output_text.delta" {
						if text, ok := event["delta"].(string); ok {
							fullText.WriteString(text)
						}
					}
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
		"model":      model,
		"max_tokens": 8,
		"messages":   []map[string]any{{"role": "user", "content": "hi"}},
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, chatCompletionURL(baseURL), bytes.NewReader(b))
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

	return extractChatCompletionReply(respBody)
}
