package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cc-switch/internal/models"
)

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
