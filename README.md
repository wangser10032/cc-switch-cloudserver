# cc-switch

**中文** | [English](#english)

cc-switch 是一个本地 Web 工具，用于管理和切换 Claude Code 与 Codex CLI 的供应商配置。它可以保存多套模型、Base URL、API Key、CLI 运行参数和认证配置，并在需要时一键写入当前用户的真实 CLI 配置文件。

> 安全提示：cc-switch 默认只监听 `127.0.0.1:18080`，且不内置登录认证。不要把它直接暴露到公网或不可信局域网。如果确实需要远程访问，请使用反向代理、Basic Auth、VPN 或防火墙限制访问。

## 功能

- 分别管理 Claude Code 与 Codex CLI 供应商配置。
- 新增、编辑、删除、测试和应用供应商。
- 查看并编辑当前真实 Claude/Codex 配置。
- 将当前真实配置保存为供应商模板。
- 应用或编辑真实配置前自动备份。
- 从最近备份中恢复 Claude/Codex 配置。
- 支持 Claude Code 通过本地代理调用 OpenAI Responses API。

## 管理的配置文件

cc-switch 会读写当前系统用户的以下文件：

```text
~/.claude/settings.json
~/.claude.json
~/.codex/config.toml
~/.codex/auth.json
```

cc-switch 自身数据保存在项目目录下：

```text
.ccswitch/
.ccswitch/claude_providers.json
.ccswitch/codex_providers.json
.ccswitch/state.json
.ccswitch/backups/
```

这些运行数据通常包含密钥或登录 token，已经被 `.gitignore` 忽略。不要手动提交它们。

## 安装与运行

### 方式一：直接运行

```bash
go run .
```

访问：

```text
http://localhost:18080/ccswitch/
```

### 方式二：构建二进制

```bash
go build -o cc-switch .
./cc-switch
```

### 方式三：使用启动脚本

```bash
./start.sh start
./start.sh status
./start.sh stop
./start.sh restart
```

脚本会优先运行项目根目录下的 `./cc-switch`，如果不存在则回退到 `go run .`。日志写入 `.ccswitch/server.log`，PID 写入 `.ccswitch/server.pid`。

## 配置监听地址

默认只允许本机访问：

```text
127.0.0.1:18080
```

如需自定义地址：

```bash
CCSWITCH_ADDR=127.0.0.1:18080 ./cc-switch
```

或在项目根目录创建 `.env`，再使用 `start.sh`：

```bash
CCSWITCH_ADDR=127.0.0.1:18080
```

如果设置为 `:18080` 或 `0.0.0.0:18080`，服务会被其他机器访问到。由于本项目不内置认证，请先做好访问控制。

## 导入当前配置

可以把当前真实 CLI 配置导入为一个供应商：

```bash
./cc-switch import-current claude "My Claude Config"
./cc-switch import-current codex "My Codex Config"
./cc-switch import-current all "My Current Config"
```

使用启动脚本时：

```bash
./start.sh import-current claude "My Claude Config"
./start.sh import-current codex "My Codex Config"
./start.sh import-current all "My Current Config"
```

## 使用流程

1. 启动服务并打开 `http://localhost:18080/ccswitch/`。
2. 在 Claude Code 或 Codex CLI 区域新增供应商。
3. 填写 Base URL、模型、API Key 和必要的 CLI 参数。
4. 点击“测试”确认供应商可用。
5. 点击“应用”写入真实 CLI 配置。
6. 如需回滚，到“备份恢复”页面恢复最近备份。

## Claude OpenAI 代理模式

Claude Code 通常使用 Anthropic Messages API。cc-switch 内置一个轻量代理，可以把部分 Claude Code 请求转换为 OpenAI Responses API 请求。

基本用法：

1. 新增或编辑一个 Claude 供应商。
2. 将 Base URL 设置为：

```text
http://127.0.0.1:18080/ccswitch/proxy/openai/<供应商ID>
```

3. 设置代理 token，也就是 Claude 请求侧使用的 `ANTHROPIC_AUTH_TOKEN`。
4. 在供应商配置里设置 OpenAI API Key。
5. 应用该供应商后，通过 Claude Code 使用。

代理只覆盖常见消息字段，不保证兼容所有 Anthropic 或 OpenAI 高级能力。

## 开发

运行测试：

```bash
go test ./...
```

构建检查：

```bash
go build -o /tmp/cc-switch-check .
```

## 发布前检查

- 确认 `.ccswitch/`、`.env`、二进制文件和截图没有被提交。
- 确认没有真实 API Key、JWT、refresh token 或个人账号信息进入 Git。
- 根据你的开源策略补充 `LICENSE`。
- 如接受漏洞反馈，建议补充 `SECURITY.md`。

---

## English

cc-switch is a local web tool for managing and switching provider profiles for Claude Code and Codex CLI. It stores reusable model settings, Base URLs, API keys, CLI options, and authentication data, then writes the selected profile to the current user's real CLI config files.

> Security notice: cc-switch listens on `127.0.0.1:18080` by default and does not include built-in login authentication. Do not expose it directly to the public internet or an untrusted LAN. If remote access is required, protect it with a reverse proxy, Basic Auth, VPN, or firewall rules.

## Features

- Manage Claude Code and Codex CLI provider profiles separately.
- Create, edit, delete, test, and apply providers.
- View and edit the current real Claude/Codex configuration.
- Save the current real configuration as a provider profile.
- Automatically back up real config files before applying or editing them.
- Restore Claude/Codex configuration from recent backups.
- Let Claude Code call the OpenAI Responses API through a local proxy.

## Managed Files

cc-switch reads and writes these files for the current system user:

```text
~/.claude/settings.json
~/.claude.json
~/.codex/config.toml
~/.codex/auth.json
```

cc-switch stores its own runtime data under the project directory:

```text
.ccswitch/
.ccswitch/claude_providers.json
.ccswitch/codex_providers.json
.ccswitch/state.json
.ccswitch/backups/
```

Runtime data may contain API keys or login tokens and is ignored by `.gitignore`. Do not commit it manually.

## Installation And Usage

### Option 1: Run Directly

```bash
go run .
```

Open:

```text
http://localhost:18080/ccswitch/
```

### Option 2: Build A Binary

```bash
go build -o cc-switch .
./cc-switch
```

### Option 3: Use The Start Script

```bash
./start.sh start
./start.sh status
./start.sh stop
./start.sh restart
```

The script runs `./cc-switch` first, and falls back to `go run .` when the binary does not exist. Logs are written to `.ccswitch/server.log`, and the PID is written to `.ccswitch/server.pid`.

## Listening Address

By default, cc-switch only accepts local connections:

```text
127.0.0.1:18080
```

To customize the address:

```bash
CCSWITCH_ADDR=127.0.0.1:18080 ./cc-switch
```

Or create a `.env` file in the project root and use `start.sh`:

```bash
CCSWITCH_ADDR=127.0.0.1:18080
```

If you set it to `:18080` or `0.0.0.0:18080`, other machines may be able to access the service. Since this project has no built-in authentication, configure access control first.

## Import Current Config

You can import the current real CLI configuration as a provider:

```bash
./cc-switch import-current claude "My Claude Config"
./cc-switch import-current codex "My Codex Config"
./cc-switch import-current all "My Current Config"
```

With the start script:

```bash
./start.sh import-current claude "My Claude Config"
./start.sh import-current codex "My Codex Config"
./start.sh import-current all "My Current Config"
```

## Typical Workflow

1. Start the service and open `http://localhost:18080/ccswitch/`.
2. Add a provider under Claude Code or Codex CLI.
3. Fill in the Base URL, model, API key, and required CLI options.
4. Click "Test" to verify the provider.
5. Click "Apply" to write it to the real CLI config files.
6. If you need to roll back, restore a recent backup from the backup page.

## Claude OpenAI Proxy Mode

Claude Code usually speaks the Anthropic Messages API. cc-switch includes a lightweight proxy that can convert part of Claude Code requests into OpenAI Responses API requests.

Basic setup:

1. Create or edit a Claude provider.
2. Set the Base URL to:

```text
http://127.0.0.1:18080/ccswitch/proxy/openai/<provider-id>
```

3. Set the proxy token, which is the `ANTHROPIC_AUTH_TOKEN` used by the Claude request side.
4. Configure the OpenAI API key in the provider.
5. Apply the provider and use it through Claude Code.

The proxy covers common message fields only. It does not guarantee compatibility with every advanced Anthropic or OpenAI feature.

## Development

Run tests:

```bash
go test ./...
```

Build check:

```bash
go build -o /tmp/cc-switch-check .
```

## Before Publishing

- Make sure `.ccswitch/`, `.env`, binaries, and screenshots are not committed.
- Make sure no real API keys, JWTs, refresh tokens, or personal account data are committed.
- Add a `LICENSE` according to your open-source policy.
- Add `SECURITY.md` if you want to accept vulnerability reports.
