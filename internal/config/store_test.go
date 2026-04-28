package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cc-switch/internal/models"
)

func TestApplyClaudeProviderReplacesEnv(t *testing.T) {
	tmp := t.TempDir()
	oldHome := homeDir
	homeDir = tmp
	t.Cleanup(func() {
		homeDir = oldHome
	})
	t.Chdir(tmp)

	claudeDir := filepath.Join(tmp, ClaudeDir)
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		t.Fatal(err)
	}
	oldSettings := []byte(`{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "old-token",
    "ANTHROPIC_BASE_URL": "https://old.example.com",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "old-haiku",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "old-opus",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "old-sonnet",
    "ANTHROPIC_MODEL": "old-model",
    "ANTHROPIC_REASONING_MODEL": "old-reasoning"
  },
  "permissions": {
    "allow": ["Bash(date)"]
  },
  "language": "zh-CN"
}`)
	if err := os.WriteFile(filepath.Join(claudeDir, ClaudeSettingsFile), oldSettings, 0600); err != nil {
		t.Fatal(err)
	}

	s := NewStore()
	s.claudeProviders = []models.ClaudeProvider{
		{
			ID:   "minimax",
			Name: "MiniMax",
			Settings: map[string]any{
				"env": map[string]any{
					"ANTHROPIC_AUTH_TOKEN": "new-token",
					"ANTHROPIC_BASE_URL":   "https://new.example.com",
					"ANTHROPIC_MODEL":      "new-model",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if err := s.ApplyClaudeProvider("minimax"); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(claudeDir, ClaudeSettingsFile))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		t.Fatal(err)
	}
	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatalf("expected env to exist, got %#v", settings["env"])
	}
	want := map[string]any{
		"ANTHROPIC_AUTH_TOKEN": "new-token",
		"ANTHROPIC_BASE_URL":   "https://new.example.com",
		"ANTHROPIC_MODEL":      "new-model",
	}
	if len(env) != len(want) {
		t.Fatalf("expected env to be replaced with %#v, got %#v", want, env)
	}
	for k, v := range want {
		if env[k] != v {
			t.Fatalf("expected env[%s] = %q, got %#v", k, v, env[k])
		}
	}
	if _, ok := env["ANTHROPIC_REASONING_MODEL"]; ok {
		t.Fatalf("expected old reasoning model to be removed, got %#v", env)
	}
	if _, ok := settings["permissions"]; !ok {
		t.Fatalf("expected non-env settings to be preserved, got %#v", settings)
	}
}

func TestApplyCodexProviderOverwritesAuthJSON(t *testing.T) {
	tmp := t.TempDir()
	oldHome := homeDir
	homeDir = tmp
	t.Cleanup(func() {
		homeDir = oldHome
	})
	t.Chdir(tmp)

	codexDir := filepath.Join(tmp, CodexDir)
	if err := os.MkdirAll(codexDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, CodexConfigFile), []byte("model = \"old\"\nmodel_provider = \"OpenAI\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, CodexAuthFile), []byte(`{"OPENAI_API_KEY":"sk-test","auth_mode":"chatgpt"}`), 0600); err != nil {
		t.Fatal(err)
	}

	s := NewStore()
	s.codexProviders = []models.CodexProvider{
		{
			ID:         "official-empty",
			Name:       "Official Empty",
			ConfigTOML: "model = \"gpt-5.5\"\nmodel_provider = \"OpenAI\"\n",
			AuthJSON:   map[string]any{},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	if err := s.ApplyCodexProvider("official-empty"); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(codexDir, CodexAuthFile))
	if err != nil {
		t.Fatal(err)
	}
	var auth map[string]any
	if err := json.Unmarshal(b, &auth); err != nil {
		t.Fatal(err)
	}
	if len(auth) != 0 {
		t.Fatalf("expected auth.json to be overwritten with empty provider auth, got %#v", auth)
	}
}

func TestApplyCodexProviderMapsEnvKeyWithoutKeepingOpenAIAPIKey(t *testing.T) {
	tmp := t.TempDir()
	oldHome := homeDir
	homeDir = tmp
	t.Cleanup(func() {
		homeDir = oldHome
	})
	t.Chdir(tmp)

	codexDir := filepath.Join(tmp, CodexDir)
	if err := os.MkdirAll(codexDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, CodexConfigFile), []byte("model = \"old\"\nmodel_provider = \"OpenAI\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, CodexAuthFile), []byte(`{"OPENAI_API_KEY":"sk-test","old":"value"}`), 0600); err != nil {
		t.Fatal(err)
	}

	s := NewStore()
	s.codexProviders = []models.CodexProvider{
		{
			ID:         "custom-env",
			Name:       "Custom Env",
			ConfigTOML: "model = \"gpt-5.5\"\nmodel_provider = \"OpenAI\"\n",
			AuthJSON: map[string]any{
				"env_key":        "CUSTOM_API_KEY",
				"OPENAI_API_KEY": "sk-new",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if err := s.ApplyCodexProvider("custom-env"); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(codexDir, CodexAuthFile))
	if err != nil {
		t.Fatal(err)
	}
	var auth map[string]any
	if err := json.Unmarshal(b, &auth); err != nil {
		t.Fatal(err)
	}
	if auth["CUSTOM_API_KEY"] != "sk-new" {
		t.Fatalf("expected mapped CUSTOM_API_KEY, got %#v", auth)
	}
	if _, ok := auth["OPENAI_API_KEY"]; ok {
		t.Fatalf("expected OPENAI_API_KEY to be removed when env_key is used, got %#v", auth)
	}
	if _, ok := auth["old"]; ok {
		t.Fatalf("expected old auth field to be removed, got %#v", auth)
	}
}

func TestRestoreBackupRejectsInvalidBackupName(t *testing.T) {
	tmp := t.TempDir()
	oldHome := homeDir
	homeDir = tmp
	t.Cleanup(func() {
		homeDir = oldHome
	})
	t.Chdir(tmp)

	s := NewStore()
	for _, name := range []string{
		"../codex_20260427_140814",
		"codex/20260427_140814",
		`codex\20260427_140814`,
		"claude_20260427_140814",
		"codex_not_a_timestamp",
	} {
		if err := s.RestoreBackup("codex", name); err == nil {
			t.Fatalf("expected invalid backup name %q to be rejected", name)
		}
	}
}
