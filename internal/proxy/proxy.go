package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cc-switch/internal/config"
)

type Proxy struct {
	Store *config.Store
}

func New(s *config.Store) *Proxy {
	return &Proxy{Store: s}
}

func (p *Proxy) Handler(w http.ResponseWriter, r *http.Request) {
	providerID := strings.TrimPrefix(r.URL.Path, "/ccswitch/proxy/openai/")
	if idx := strings.Index(providerID, "/"); idx >= 0 {
		providerID = providerID[:idx]
	}
	if providerID == "" {
		http.Error(w, "missing provider id", http.StatusBadRequest)
		return
	}

	prov, ok := p.Store.GetClaudeProvider(providerID)
	if !ok {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	// 提取代理 token：从请求头 x-api-key 或 authorization 中获取
	authHeader := r.Header.Get("Authorization")
	proxyToken := ""
	if strings.HasPrefix(authHeader, "Bearer ") {
		proxyToken = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		proxyToken = r.Header.Get("x-api-key")
	}

	// 从 provider settings 中取得代理 token（env.ANTHROPIC_AUTH_TOKEN）
	expectedToken := ""
	if env, ok := prov.Settings["env"].(map[string]any); ok {
		if v, ok := env["ANTHROPIC_AUTH_TOKEN"].(string); ok {
			expectedToken = v
		}
	}

	if expectedToken == "" {
		http.Error(w, "proxy token not configured", http.StatusUnauthorized)
		return
	}
	if proxyToken != expectedToken {
		http.Error(w, "invalid proxy token", http.StatusUnauthorized)
		return
	}

	// 读取 Anthropic 请求体
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var anthropicReq map[string]any
	if err := json.Unmarshal(bodyBytes, &anthropicReq); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// 转换为 OpenAI Responses 请求
	openAIReq := convertAnthropicToOpenAI(anthropicReq)

	// 获取 OpenAI API Key
	apiKey := ""
	if env, ok := prov.Settings["env"].(map[string]any); ok {
		if v, ok := env["OPENAI_API_KEY"].(string); ok {
			apiKey = v
		}
	}
	if apiKey == "" {
		// 尝试从 claude.json 的 api_key 获取
		if v, ok := prov.ClaudeJSON["openai_api_key"].(string); ok {
			apiKey = v
		}
	}
	if apiKey == "" {
		http.Error(w, "no openai api key configured", http.StatusBadRequest)
		return
	}

	// 获取 base_url，默认为 OpenAI 官方
	baseURL := "https://api.openai.com/v1"
	if env, ok := prov.Settings["env"].(map[string]any); ok {
		if v, ok := env["OPENAI_BASE_URL"].(string); ok && v != "" {
			baseURL = strings.TrimSuffix(v, "/")
		}
	}

	// 构造 OpenAI 请求
	respURL := baseURL + "/responses"
	openAIBody, _ := json.Marshal(openAIReq)
	req, err := http.NewRequest("POST", respURL, bytes.NewReader(openAIBody))
	if err != nil {
		http.Error(w, "create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "read upstream", http.StatusBadGateway)
		return
	}

	// 转换响应
	if resp.StatusCode == http.StatusOK {
		var openAIResp map[string]any
		if err := json.Unmarshal(respBody, &openAIResp); err == nil {
			anthropicResp := convertOpenAIToAnthropic(openAIResp)
			b, _ := json.Marshal(anthropicResp)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(b)
			return
		}
	}

	// 透传非 200 或解析失败（脱敏）
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	safeBody := strings.ReplaceAll(string(respBody), apiKey, "***")
	w.Write([]byte(safeBody))
}

func convertAnthropicToOpenAI(anthropic map[string]any) map[string]any {
	openAI := map[string]any{}

	// model
	if model, ok := anthropic["model"].(string); ok {
		openAI["model"] = model
	}

	// messages -> input
	if msgs, ok := anthropic["messages"].([]any); ok {
		openAI["input"] = msgs
	}

	// max_tokens -> max_output_tokens
	if v, ok := anthropic["max_tokens"]; ok {
		openAI["max_output_tokens"] = v
	}

	// temperature
	if v, ok := anthropic["temperature"]; ok {
		openAI["temperature"] = v
	}

	// system -> instructions
	if sys, ok := anthropic["system"].(string); ok {
		openAI["instructions"] = sys
	} else if sysArr, ok := anthropic["system"].([]any); ok && len(sysArr) > 0 {
		if s, ok := sysArr[0].(map[string]any); ok {
			openAI["instructions"] = s["text"]
		}
	}

	// tools -> tools ( Anthropic tools format is close enough for simple cases )
	if tools, ok := anthropic["tools"].([]any); ok {
		openAI["tools"] = tools
	}

	// stream
	if v, ok := anthropic["stream"]; ok {
		openAI["stream"] = v
	}

	return openAI
}

func convertOpenAIToAnthropic(openAI map[string]any) map[string]any {
	anthropic := map[string]any{}
	anthropic["type"] = "message"
	anthropic["role"] = "assistant"

	if id, ok := openAI["id"].(string); ok {
		anthropic["id"] = id
	}
	if model, ok := openAI["model"].(string); ok {
		anthropic["model"] = model
	}

	// usage
	if usage, ok := openAI["usage"].(map[string]any); ok {
		anthropic["usage"] = map[string]any{
			"input_tokens":  usage["input_tokens"],
			"output_tokens": usage["output_tokens"],
		}
	}

	// output -> content
	if output, ok := openAI["output"].([]any); ok && len(output) > 0 {
		if first, ok := output[0].(map[string]any); ok {
			if content, ok := first["content"].([]any); ok {
				var contents []map[string]any
				for _, c := range content {
					if cm, ok := c.(map[string]any); ok {
						contents = append(contents, cm)
					}
				}
				anthropic["content"] = contents
			} else {
				// 直接取 text
				if text, ok := first["text"].(string); ok {
					anthropic["content"] = []map[string]any{{"type": "text", "text": text}}
				}
			}
		} else {
			anthropic["content"] = []map[string]any{}
		}
	} else {
		anthropic["content"] = []map[string]any{}
	}

	anthropic["stop_reason"] = "end_turn"
	return anthropic
}

func (p *Proxy) TestConnection(providerID, baseURL, apiKey, model string) (string, error) {
	// OpenAI Responses 最小连通性测试。
	body := map[string]any{
		"model":             model,
		"input":             "hi",
		"max_output_tokens": 8,
	}
	b, _ := json.Marshal(body)
	url := baseURL
	if !strings.Contains(url, "/responses") {
		url = strings.TrimSuffix(url, "/") + "/responses"
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
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
		// 脱敏
		s := string(respBody)
		s = strings.ReplaceAll(s, apiKey, "***")
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, s)
	}

	var m map[string]any
	if err := json.Unmarshal(respBody, &m); err != nil {
		return "", err
	}

	if text, ok := m["output_text"].(string); ok {
		return text, nil
	}
	if output, ok := m["output"].([]any); ok {
		for _, item := range output {
			msg, ok := item.(map[string]any)
			if !ok {
				continue
			}
			content, ok := msg["content"].([]any)
			if !ok {
				continue
			}
			for _, c := range content {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				if text, ok := cm["text"].(string); ok {
					return text, nil
				}
			}
		}
	}
	return string(respBody), nil
}
