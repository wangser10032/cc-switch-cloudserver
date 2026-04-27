# cc-switch 需求与实现逻辑整理

## 1. 项目定位

cc-switch 是一个运行在本机或服务器上的轻量 Web 工具，用来管理并一键切换当前系统用户的 Claude Code 与 Codex CLI 配置。

它不管理代码项目本身，而是把常用 AI 供应商、模型、认证信息、代理参数和 CLI 运行选项保存为可复用供应商配置。用户点击应用后，cc-switch 会写入对应 CLI 的真实配置文件。

核心目标：

- Claude Code 与 Codex CLI 分开管理，分开展示，分开应用。
- 支持供应商新增、编辑、删除、测试、应用。
- 支持查看和编辑当前真实配置，并保存为供应商。
- 每次写入真实 CLI 配置前自动备份。
- 支持最近备份恢复。
- 支持 Claude Code 通过本地代理使用 OpenAI Responses。
- Codex CLI 直接写入自己的 `config.toml` 和 `auth.json`，不经过 Claude 代理。

## 2. 路径与文件边界

### 2.1 Web 访问路径

Web 固定访问路径：

```text
http://<服务器IP>:18080/ccswitch/
```

`/ccswitch/` 是 HTTP 路由前缀，不是数据目录名。

### 2.2 cc-switch 自身数据

运行数据统一保存在项目目录下的隐藏目录：

```text
./.ccswitch/
./.ccswitch/claude_providers.json
./.ccswitch/codex_providers.json
./.ccswitch/state.json
./.ccswitch/server.pid
./.ccswitch/server.log
./.ccswitch/backups/
```

旧的 `./ccswitch/` 目录不再作为有效数据目录使用，仅可作为历史备份或迁移来源。

### 2.3 真实 CLI 配置

Claude Code 真实配置：

```text
~/.claude/settings.json
~/.claude.json
```

Codex CLI 真实配置：

```text
~/.codex/config.toml
~/.codex/auth.json
```

cc-switch 只有在“应用供应商”或“当前配置页保存”时才写入这些真实配置文件。

## 3. 启动与运行

启动脚本：

```text
./start.sh start
./start.sh stop
./start.sh restart
./start.sh status
./start.sh import-current claude <名称>
./start.sh import-current codex <名称>
./start.sh import-current all <名称>
```

启动脚本规则：

- 读取项目根目录 `.env`。
- 优先运行已有 `./cc-switch` 二进制。
- 没有二进制时回退到 `go run .`。
- PID 写入 `./.ccswitch/server.pid`。
- 日志写入 `./.ccswitch/server.log`。
- 启动时确保必要目录存在。

`~/.claude.json` 不存在时应自动创建，并确保至少包含：

```json
{
  "hasCompletedOnboarding": true
}
```

已有 `~/.claude.json` 字段应尽量保留。

## 4. 页面与用户操作

### 4.1 首页

首页就是可操作的管理界面，不做营销页。

页面主要区域：

- Claude Code 供应商列表。
- Codex CLI 供应商列表。
- 当前配置编辑页。
- 备份恢复页。

供应商卡片展示：

- 名称。
- 模型。
- Base URL 或上游地址。
- 是否当前激活。
- 操作按钮：编辑、删除、应用、测试。

Claude 与 Codex 的激活状态独立维护。

### 4.2 Claude 供应商编辑

编辑 Claude 供应商时只展示 Claude Code 相关内容。

基础字段：

- 名称。
- 官网链接。
- 备注。

常用字段：

- API 格式：Anthropic Messages 或 OpenAI Responses 代理模式。
- Base URL。
- API Key。
- Auth Token。
- 代理 Token。
- 主模型。
- Reasoning Model。
- Haiku / Sonnet / Opus 默认模型。
- Timeout。
- Language。

常用开关：

- 禁用非必要流量。
- 隐藏 AI 签名。
- Teammates 模式。
- Tool Search。
- 高思考模式。
- 禁用自动更新。
- `includeCoAuthoredBy`。
- `skipDangerousModePermissionPrompt`。

高级字段：

- 权限默认模式。
- Hooks JSON。
- Permissions JSON。
- 完整 `settings.json` 编辑器。
- 完整 `.claude.json` 编辑器。

交互规则：

- 左侧表单变化时同步生成右侧 JSON 预览。
- 右侧完整 JSON 被手动编辑后，保存以完整 JSON 为准。
- JSON 无效时禁止保存。
- 密钥类字段默认隐藏，用户可临时显示。
- 模板填入默认值时，不应覆盖用户已经填写的密钥。

### 4.3 Codex 供应商编辑

编辑 Codex 供应商时只展示 Codex CLI 相关内容。

基础字段：

- 名称。
- 官网链接。
- 备注。

常用字段：

- `model`
- `model_provider`
- `base_url`
- `wire_api`
- `requires_openai_auth`
- `env_key`
- API Key
- `approval_policy`
- `sandbox_mode`
- `model_reasoning_effort`
- `personality`
- `service_tier`
- `disable_response_storage`

高级字段：

- `query_params`
- `http_headers`
- `env_http_headers`
- 完整 `config.toml` 编辑器。
- 完整 `auth.json` 编辑器。

交互规则：

- 左侧表单变化时同步生成右侧 `config.toml` 和 `auth.json` 预览。
- 右侧完整编辑器被手动编辑后，保存以完整编辑器内容为准。
- TOML 或 JSON 无效时禁止保存。
- `auth.json` 预览就是应用时写入的认证对象。

## 5. 应用配置逻辑

用户点击供应商卡片上的“应用”后：

1. 读取当前真实 CLI 配置。
2. 创建备份。
3. 根据供应商生成下一份配置。
4. 校验 TOML / JSON 可序列化。
5. 写入临时文件。
6. 原子替换目标文件。
7. 更新对应工具的激活供应商 ID。

Claude 供应商只写 Claude Code 文件：

```text
~/.claude/settings.json
~/.claude.json
```

Codex 供应商只写 Codex CLI 文件：

```text
~/.codex/config.toml
~/.codex/auth.json
```

## 6. Claude Code 写入规则

Claude 供应商主要管理 `~/.claude/settings.json` 中与供应商和模型相关的字段。

核心 env 字段：

```text
ANTHROPIC_BASE_URL
ANTHROPIC_AUTH_TOKEN
ANTHROPIC_MODEL
ANTHROPIC_REASONING_MODEL
ANTHROPIC_DEFAULT_HAIKU_MODEL
ANTHROPIC_DEFAULT_SONNET_MODEL
ANTHROPIC_DEFAULT_OPUS_MODEL
OPENAI_API_KEY
```

写入规则：

- 保留不属于 cc-switch 管理的未知字段。
- 切换认证方式时，先清理旧的 `ANTHROPIC_AUTH_TOKEN`。
- 供应商字段为空时删除真实配置中的对应旧字段。
- `settings.env` 中只覆盖核心 env 字段，其他 env 字段默认保留。
- `model` 字段由供应商覆盖，空值则删除。
- `language`、`timeout`、权限、hooks、开关等顶层字段可由供应商覆盖；没有出现在供应商里的字段默认保留。
- `.claude.json` 同样按供应商中提供的字段覆盖，未知字段默认保留。
- 保存当前配置页时，用户提交的完整 JSON 会直接写入真实文件。

OpenAI 代理模式下：

- Claude Code 的 `ANTHROPIC_BASE_URL` 指向本地代理。
- Claude Code 请求携带的 `ANTHROPIC_AUTH_TOKEN` 是代理 Token。
- 真实 OpenAI API Key 保存在 Claude 供应商配置中，由本地代理调用上游时使用。

## 7. Codex CLI 写入规则

Codex 供应商管理两个文件：

```text
~/.codex/config.toml
~/.codex/auth.json
```

### 7.1 config.toml

`config.toml` 必须保持合法 TOML。

应用供应商时：

- 以供应商 `config_toml` 为最终基础。
- 删除供应商 TOML 中旧的 `[model_providers.OpenAI]` 节后重新生成。
- 顶层 `base_url`、`wire_api`、`requires_openai_auth` 会同步进 `[model_providers.OpenAI]`。
- `[model_providers.OpenAI]` 中管理这些核心字段：
  - `name`
  - `model`
  - `base_url`
  - `wire_api`
  - `requires_openai_auth`
- 现有真实配置里的 OpenAI provider 字段只作为补充来源；供应商显式提供的字段优先生效。
- 供应商字段为空或 false 时，会删除对应核心字段，避免旧供应商残留。

### 7.2 auth.json

`auth.json` 必须保持合法 JSON。

当前规则是整文件替换：

- 应用 Codex 供应商时，不再合并旧 `~/.codex/auth.json`。
- 写入内容完全由供应商的 `auth_json` 生成。
- 供应商 `auth_json` 没有 `OPENAI_API_KEY` 时，旧文件里的 `OPENAI_API_KEY` 必须被删除。
- 空 `auth_json` 会写入 `{}`，用于 OpenAI 官网登录空配置。
- 字段值为 `null` 或空字符串时不写入。
- 如果配置了 `env_key`，则把 `OPENAI_API_KEY` 的值写入 `env_key` 指定的键，并不额外保留 `OPENAI_API_KEY`。

示例：非官方 OpenAI 兼容供应商通常只需要：

```json
{
  "OPENAI_API_KEY": "sk-..."
}
```

示例：使用自定义环境变量键：

```json
{
  "env_key": "CUSTOM_API_KEY",
  "CUSTOM_API_KEY": "sk-..."
}
```

示例：官网登录空配置：

```json
{}
```

注意：不要在文档、模板或日志中保存真实 token、JWT、refresh token 或完整 API Key。

## 8. 测试连接

测试连接通过用户提供或供应商保存的 URL、Key、Model 向上游发送简单请求，验证是否能收到响应。

要求：

- 测试连接不应自动修改真实配置。
- 测试结果在弹窗或面板中展示。
- 错误信息需要可读，但不能泄露完整密钥。

## 9. 当前配置页

当前配置页展示真实生效文件：

```text
~/.claude/settings.json
~/.claude.json
~/.codex/config.toml
~/.codex/auth.json
```

支持操作：

- 查看。
- 编辑。
- 格式化。
- 保存。
- 测试当前配置。
- 保存当前配置为 Claude 或 Codex 供应商。

当前真实配置可能与 cc-switch 记录的激活供应商不同。例如用户手动执行 `codex auth`、手动改文件、CLI 自动刷新 token，都会导致真实配置变化。这是允许的。

保存当前配置时必须先备份真实文件。

## 10. 备份与恢复

备份目录：

```text
./.ccswitch/backups/
```

备份规则：

- 应用 Claude 供应商前，备份 `~/.claude/settings.json` 和 `~/.claude.json`。
- 应用 Codex 供应商前，备份 `~/.codex/config.toml` 和 `~/.codex/auth.json`。
- 当前配置页直接保存真实文件前，也要备份对应文件。
- 备份按工具和时间戳分目录保存。

恢复规则：

- Claude 与 Codex 分开恢复。
- 恢复操作必须明确目标工具和备份名。
- 恢复只写回对应工具的真实配置文件。

## 11. OpenAI 代理

Claude Code 不能直接把 OpenAI 官方地址当作 Anthropic 接口使用。Claude 使用 OpenAI 时通过本地 Anthropic 兼容代理：

```text
Claude Code
-> http://127.0.0.1:18080/ccswitch/proxy/openai/{provider_id}
-> OpenAI Responses API
```

代理要求：

- 校验 Claude Code 请求携带的代理 Token。
- 从 Claude 供应商配置中读取 OpenAI API Key。
- 将 Anthropic Messages 请求转换为 OpenAI Responses 请求。
- 将 OpenAI 响应转换为 Anthropic Messages 兼容响应。
- 至少支持非流式请求。
- 日志和错误必须脱敏。

Codex CLI 使用 OpenAI 时不走该代理，而是直接写入 Codex 的 `model_providers` 与 `auth.json`。

## 12. 内置模板

首次启动且供应商文件不存在时生成模板。

Claude 模板：

- 火山方舟。
- GLM / 智谱。
- MiniMax。
- 小米 MiMo。
- OpenAI 代理。

Codex 模板：

- OpenAI 官网登录空配置。

模板不得包含真实或看起来像真实的默认密钥。

## 13. 安全约束

当前项目默认不做登录鉴权，因此必须明确风险：

- 公网可访问时，任何访问者都可能读取或修改配置。
- API Key 和 Token 按用户需求明文保存在本地文件。
- 生产环境建议只监听本机、内网访问，或使用反向代理 Basic Auth。

安全底线：

- 前端默认不展示完整密钥，除非用户主动点击显示。
- 日志不输出完整密钥。
- API 错误不回显完整密钥。
- 测试连接结果脱敏。
- OpenAI 代理必须校验代理 Token。
- 文件权限尽量使用 `0600`，目录使用 `0700`。
- 文档和模板不得包含真实 token、JWT、refresh token。

## 14. 验收标准

项目达到可用状态时应满足：

- 全新用户启动后能直接打开 Web 页面。
- 运行数据只写入 `./.ccswitch/`。
- Claude 和 Codex 供应商分列展示。
- 用户能新增、编辑、删除、测试、应用 Claude 供应商。
- 用户能新增、编辑、删除、测试、应用 Codex 供应商。
- Claude 应用只影响 Claude Code 真实配置。
- Codex 应用只影响 Codex CLI 真实配置。
- Codex `auth.json` 应用时是整文件替换，不残留旧 `OPENAI_API_KEY`。
- 当前配置页能展示真实配置，并能保存为供应商。
- 每次应用或直接保存真实配置前都会产生备份。
- 备份可以按 Claude / Codex 分开恢复。
- Claude 使用 OpenAI 时通过本地代理工作。
- Codex 使用 OpenAI 时直接写入 `config.toml` 与 `auth.json`。
