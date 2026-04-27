# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

cc-switch is a lightweight web tool for managing and switching Claude Code and Codex CLI configurations. It stores provider configurations (API keys, base URLs, models, settings) and applies them to the actual CLI config files on demand.

**Key capability**: Claude Code can use OpenAI-compatible providers through a local Anthropic-to-OpenAI proxy (`/ccswitch/proxy/openai/{provider_id}`).

## Commands

```bash
# Build
go build -o cc-switch .

# Run server (default port :18080)
./start.sh start
./start.sh stop
./start.sh restart
./start.sh status

# Import current CLI config as a provider
./start.sh import-current claude <name>
./start.sh import-current codex <name>
./start.sh import-current all <name>

# Run tests
go test ./...

# Run single test
go test -run TestApplyCodexProviderOverwritesAuthJSON ./internal/config/
```

## Architecture

```
main.go                 # Entry point, HTTP server setup, SPA routing
internal/
├── config/
│   └── store.go        # Core data layer: provider CRUD, config apply, backup/restore
├── handlers/
│   └── handlers.go     # HTTP handlers for REST API endpoints
├── models/
│   └── models.go       # Data structures: ClaudeProvider, CodexProvider, State
└── proxy/
    └── proxy.go        # Anthropic→OpenAI Responses API proxy
static/                  # Frontend SPA (index.html, app.js, style.css)
```

## Data Storage

**cc-switch data** (in project directory):
- `.ccswitch/claude_providers.json` - Saved Claude provider configs
- `.ccswitch/codex_providers.json` - Saved Codex provider configs
- `.ccswitch/state.json` - Active provider IDs
- `.ccswitch/backups/` - Timestamped backups before each apply

**Real CLI configs** (in user home):
- `~/.claude/settings.json` - Claude Code settings (env vars, model, flags)
- `~/.claude.json` - Claude Code account settings
- `~/.codex/config.toml` - Codex CLI config
- `~/.codex/auth.json` - Codex CLI authentication

## Key Architecture Patterns

### Config Apply Logic (internal/config/store.go)

When applying a provider, the system:
1. Reads current real CLI config
2. Creates timestamped backup
3. Merges provider settings (core fields overwrite, unknown fields preserved)
4. Atomically writes new config (temp file + rename)

**Claude settings merging** (`ApplyClaudeProvider`):
- Core env fields: `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_MODEL`, etc.
- Empty string values → delete the key
- Switching auth method clears old `ANTHROPIC_AUTH_TOKEN`
- Non-core fields preserved unless explicitly set in provider

**Codex auth.json** (`ApplyCodexProvider`):
- **Full file replacement** - no merging with existing auth.json
- If `env_key` is set, maps `OPENAI_API_KEY` value to the custom key
- Empty provider auth writes `{}`

### Proxy Flow (internal/proxy/proxy.go)

```
Claude Code → /ccswitch/proxy/openai/{provider_id}
           → Validate proxy token (from ANTHROPIC_AUTH_TOKEN)
           → Get OpenAI API key from provider settings
           → Convert Anthropic Messages → OpenAI Responses format
           → Forward to upstream
           → Convert response back to Anthropic format
```

## API Endpoints

- `/ccswitch/api/claude/providers` - CRUD for Claude providers
- `/ccswitch/api/codex/providers` - CRUD for Codex providers
- `/ccswitch/api/claude/apply` - Apply Claude provider to real config
- `/ccswitch/api/codex/apply` - Apply Codex provider to real config
- `/ccswitch/api/claude/test`, `/ccswitch/api/codex/test` - Test provider connectivity
- `/ccswitch/api/current/claude`, `/ccswitch/api/current/codex` - Read/write current real configs
- `/ccswitch/api/backups` - List backups
- `/ccswitch/api/backups/restore` - Restore from backup
- `/ccswitch/proxy/openai/{provider_id}` - OpenAI proxy endpoint

## Security Considerations

- No authentication by default - warn on startup
- File permissions: `0600` for files, `0700` for directories
- API keys/tokens redacted in error responses (`***`)
- Proxy requires token validation
- Frontend hides sensitive fields by default
