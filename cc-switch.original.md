# cc-switch 需求与实现逻辑整理（重写前备份）

说明：这是根据本次会话中重写前读取到的 `cc-switch.md` 内容恢复的旧版备份。由于原文件已经被重写，且项目中没有发现自动生成的旧版副本，此文件用于保留重写前的需求结构和主要内容。敏感示例在用户要求下保留方向不做脱敏处理；但会话中未完整保留的超长 token/JWT 内容无法逐字恢复。

## 1. 项目定位

cc-switch 是一个运行在本机或云服务器上的轻量 Web 工具，用来快速切换当前系统用户的 Claude Code 与 Codex CLI 配置。

它解决的问题不是“管理代码项目”，而是“把常用 AI 供应商、模型、认证、代理和 CLI 运行参数保存成可复用配置，并在需要时一键切换对应 CLI 的真实配置文件”。

核心目标：

- 快速切换 Claude Code 使用的供应商、模型、Base URL、Token、常用运行选项。
- 快速切换 Codex CLI 使用的模型供应商、模型、认证、approval policy、sandbox mode 等配置。
- Claude Code 与 Codex CLI 配置必须分开管理，不能混在同一个配置详情里。
- 用户可以通过表单编辑常用字段，也可以直接编辑完整配置内容。
- 每次写入真实 CLI 配置前必须备份，出现问题可以恢复最近一次配置。
- 内置常见供应商模板，用户通常只需要补充自己的凭证。

## 2. 用户视角的核心操作逻辑

### 2.1 启动与访问

用户运行启动脚本或二进制后，通过浏览器访问：

```text
http://<服务器IP>:18080/ccswitch/
```

启动时系统应自动补齐必要目录和空配置文件，避免用户先手动创建：

todo: .ccswitch目录放在本目录下

- `~/.ccswitch/`
- `~/.ccswitch/backups/`

tip：以下为修改claudecode和codex配置的主要文件，无论是配置、保存还是切换都是直接对这些文件进行操作

- `~/.claude/settings.json`
- `~/.claude.json`
- `~/.codex/config.toml`
- `~/.codex/auth.json`

`~/.claude.json` 中应确保存在：

```json
{
  "hasCompletedOnboarding": true
}
```

已有字段必须保留。

### 2.2 查看供应商

首页应直接是可操作的供应商列表，不做营销页。tip：claudecode和codex cli都是壳，可以进行替换为其他模型的url和key等，或者中转

列表分两列：

- Claude Code 供应商：对应 `~/.claude/settings.json` 和 `~/.claude.json`
- Codex CLI 供应商：对应 `~/.codex/config.toml` 和 `~/.codex/auth.json`

每张供应商卡片应让用户一眼看到：

- 名称
- 模型
- Base URL 或上游地址
- 是否当前激活
- 操作：编辑、删除、应用、测试

Claude 与 Codex 的当前激活状态独立显示，允许二者使用不同供应商。

### 2.3 新增或编辑 Claude Code 供应商

左侧为表单，右侧为当前表单填写之后形成的文件预览，以右侧实际文件预览json为准，文件预览也可以进行编辑，编辑也同步到左侧表单

用户点击“新增 Claude”或编辑 Claude 卡片后，只看到 Claude Code 相关字段。

tip：以下字段可以根据实际情况进行修改，现在只是参考！

基础字段：

- 名称
- 官网链接
- 备注

常用配置字段：

- API 格式：Anthropic Messages 或 OpenAI Responses 代理模式
- Base URL
- API Key
- Auth Token
- 代理 Token
- 主模型
- Reasoning Model
- Haiku / Sonnet / Opus 默认模型
- Timeout
- Language

常用开关：

- 禁用非必要流量
- 隐藏 AI 签名
- Teammates 模式
- Tool Search
- 高思考模式
- 禁用自动更新
- includeCoAuthoredBy
- skipDangerousModePermissionPrompt

高级字段：

- 权限默认模式
- Hooks JSON
- Permissions JSON
- 完整 `settings.json` 编辑器

交互规则：

- 表单字段变化时，应生成或同步最终 `settings.json`，右侧json预览同理
- 用户手动编辑完整 JSON 后，保存时以完整 JSON 为准。
- JSON 无效时禁止保存，并展示明确错误。
- 选择模板时填入默认值，但不得覆盖用户已经填写的 API Key、Auth Token、代理 Token。
- 密钥类字段默认隐藏，允许用户临时显示。

### 2.4 新增或编辑 Codex CLI 供应商

左侧为表单，右侧为当前表单填写之后形成的文件预览，以右侧实际文件预览为准，文件预览也可以进行编辑，编辑也同步到左侧表单，同上。

tip：注意这里右侧显示的文件为 `~/.codex/config.toml`和 `~/.codex/auth.json`

用户点击“新增 Codex”或编辑 Codex 卡片后，只看到 Codex CLI 相关字段。

基础字段：

- 名称
- 官网链接
- 备注

常用配置字段：

- `model`
- `model_provider`
- `base_url`
- `wire_api`
- `env_key`
- API Key
- `approval_policy`
- `sandbox_mode`
- `model_reasoning_effort`
- `personality`
- `service_tier`
- `disable_response_storage`
- `requires_openai_auth`

高级字段：

- `query_params`
- `http_headers`
- `env_http_headers`
- 完整 `config.toml` 编辑器
- 完整 `auth.json` 编辑器

交互规则：

- 表单字段变化时，应生成最终 Codex 配置，以右侧文件预览为准
- 用户手动编辑完整配置后，保存时以完整编辑器内容为准。
- TOML / JSON 解析失败时禁止保存。
- 注意 外部代理时一般只有key 但是官方时会存在很多其他项

### 2.5 应用配置

用户点击供应商卡片上的“应用”后：

- Claude 供应商只写入 Claude Code 配置。
- Codex 供应商只写入 Codex CLI 配置。
- 写入前自动备份当前真实配置，写入日志持久化
- 写入成功后更新对应工具的激活供应商。

应用 Claude 供应商时写入：

```text
~/.claude/settings.json
~/.claude.json
```

应用 Codex 供应商时写入：

```text
~/.codex/config.toml
~/.codex/auth.json
```

### 2.6 测试连接

通过url、key、model等发送"hi"给上游看是否能接收到回复，通过小窗口展示回复

### 2.7 当前配置页

用户应能直接查看和编辑本地环境中的真实生效配置：（可能不同于当前cc-switch应用的，由于用户的登录操作等等行为，这是正常的）

- `~/.claude.json`
- `~/.claude/settings.json`
- `~/.codex/config.toml`
- `~/.codex/auth.json`

每个配置块应支持：

- 查看
- 编辑
- 格式化
- 保存
- 应用
- 测试当前配置
- 保存当前配置到对应的供应商（codex和claudecode）各自的右上角，保存时自动弹窗提示用户命名等基础操作

保存当前配置时也必须先备份。

### 2.8 备份恢复

用户应能分别恢复：

- 最近一次 Claude Code 配置备份
- 最近一次 Codex CLI 配置备份

恢复操作应明确提示恢复目标，不应混淆 Claude 与 Codex。

### 2.9 导入当前配置

用户应能把当前真实 CLI 配置保存为固定供应商：

```text
./start.sh import-current claude <名称>
./start.sh import-current codex <名称>
./start.sh import-current all <名称>
```

导入时只读取现有配置并保存成供应商，不修改真实配置。

## 3. 功能需求

### 3.1 供应商管理

TODO：暂时不进行内置操作

供应商 ID 应自动生成，并保证唯一。

### 3.2 Claude Code 配置能力

- `~/.claude.json`
- `~/.claude/settings.json`

Claude 供应商应该主要针对以下进行覆盖写入其他不涉及 提供商/中转商的无需更改。

涉及环境和对应模型的必须要更改

settings.json文件

```text
model
env.ANTHROPIC_BASE_URL
env.ANTHROPIC_AUTH_TOKEN
env.ANTHROPIC_MODEL
env.ANTHROPIC_REASONING_MODEL
env.ANTHROPIC_DEFAULT_HAIKU_MODEL
env.ANTHROPIC_DEFAULT_SONNET_MODEL
env.ANTHROPIC_DEFAULT_OPUS_MODEL
```

其他如

```text
language
skipDangerousModePermissionPrompt
permissions
hooks
```

无需覆盖修改，切换时保留

参考：

```json
{
  "autoUpdatesChannel": "latest",
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "ark-f70f6e1f-b128-4682-81d5-e8033f7279bd-b71cb",
    "ANTHROPIC_BASE_URL": "https://ark.cn-beijing.volces.com/api/coding",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.1",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "glm-5.1",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.1",
    "ANTHROPIC_MODEL": "glm-5.1",
    "ANTHROPIC_REASONING_MODEL": "glm-5.1"
  },
  "hooks": {},
  "includeCoAuthoredBy": false,
  "language": "中文",
  "permissions": {
    "allow": [
      "Bash(pip show:*)",
      "Bash(python snipe.py test)"
    ],
    "defaultMode": "acceptEdits"
  },
  "skipDangerousModePermissionPrompt": true
}
```

.claude.json 一般只有一个key，一般不用做特殊修改，但是也提供其他同settings.json文件的待遇

写入规则：

- 保留不属于 cc-switch 管理的未知字段。
- 切换认证方式时，先清理旧的 `ANTHROPIC_AUTH_TOKEN`，再写入当前认证字段。
- 字段为空时删除对应旧字段，避免旧供应商残留。
- 启用 OpenAI 本地代理并配置代理 Token 时，写入 Claude Code 的 `ANTHROPIC_AUTH_TOKEN` 应是代理 Token。
- 普通 Anthropic 兼容供应商可使用 `ANTHROPIC_AUTH_TOKEN`。
- 完整 JSON 编辑器被用户手动编辑后，以完整 JSON 为最终写入源，并且左侧表单也应该同步更新

### 3.3 Codex CLI 配置能力

- `~/.codex/config.toml`
- `~/.codex/auth.json`

Codex 供应商应针对这些点进行修改：

config.toml中的

```text
[model_providers.OpenAI]
name
base_url
wire_api
requires_openai_auth
model
```

auth.json 中的

```text
OPENAI_API_KEY
```

`~/.codex/config.toml`参考 官方

```toml
model = "gpt-5.5"
model_reasoning_effort = "high"
personality = "pragmatic"
sandbox_mode = "danger-full-access"
approval_policy = "never"

[projects.'\\?\D:\BaiduSyncdisk\code\hjl\rs-system-905']
trust_level = "trusted"

[projects.'D:\BaiduSyncdisk\code\opencode']
trust_level = "trusted"

[projects.'D:\BaiduSyncdisk\code\opencode\go-blog']
trust_level = "trusted"

[projects.'\\?\D:\BaiduSyncdisk\code\opencode\go-blog']
trust_level = "trusted"

[projects.'\\?\D:\opensource\fmt']
trust_level = "trusted"

[projects.'D:\opensource']
trust_level = "trusted"

[projects.'C:\wsl2']
trust_level = "trusted"

[projects.'C:\Users\zisui']
trust_level = "trusted"

[projects.'D:\BaiduSyncdisk\code\opencode\AItemp']
trust_level = "trusted"

[projects.'D:\BaiduSyncdisk\code\opencode\AItemp\screenshot-ai-server']
trust_level = "trusted"

[projects.'D:\BaiduSyncdisk\code\opencode\llm']
trust_level = "trusted"

[projects.'D:\BaiduSyncdisk\code\opencode\GLMjiaoben']
trust_level = "trusted"

[windows]
sandbox = "unelevated"

[notice]
hide_full_access_warning = true

[notice.model_migrations]
"gpt-5.3-codex" = "gpt-5.4"

[mcp_servers.playwright]
type = "stdio"
command = "npx"
args = ["@playwright/mcp@latest"]

[mcp_servers.figma]
url = "https://mcp.figma.com/mcp"

[plugins."figma@openai-curated"]
enabled = true

[tui.model_availability_nux]
"gpt-5.5" = 3
```

`~/.codex/config.toml`参考，非官方

```toml
model_provider = "OpenAI"
model = "gpt-5.4"
review_model = "gpt-5.4"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
windows_wsl_setup_acknowledged = true
model_context_window = 1000000
model_auto_compact_token_limit = 900000

[model_providers.OpenAI]
name = "OpenAI"
base_url = "http://150.158.16.207:8080"
wire_api = "responses"
requires_openai_auth = true
```

auth.json参考 以下为官方的，一般 供应商 仅有OPENAI_API_KEY一个key，当切换到 非openai官方供应商时去掉其他key ，涉及官方openai时应该如实保留其他key

```json
{
  "auth_mode": "chatgpt",
  "OPENAI_API_KEY": null,
  "tokens": {
    "id_token": "<原文为超长 JWT，本次会话未完整保留，无法逐字恢复>",
    "access_token": "<原文为超长 JWT，本次会话未完整保留，无法逐字恢复>",
    "refresh_token": "<原文为 refresh token，本次会话未完整保留，无法逐字恢复>",
    "account_id": "68d59a6c-df8e-4007-8dcf-10d287577b95"
  },
  "last_refresh": "2026-04-25T05:06:44.293059400Z"
}
```

以下为非官方的：仅有OPENAI_API_KEY

```json
{
  "OPENAI_API_KEY": "sk-4667bb73a4777296accbea08f26e5adfdea364bd7c4d4f085f1a96d59aa55ed6"
}
```

写入规则：

- `config.toml` 使用 TOML 格式保存，不应伪装成 JSON 文件。
- 前端可用 txt 结构编辑 TOML 的中间表示，但底层写盘必须是合法 TOML。
- `auth.json` 保存凭证，必须是合法 JSON。
- 保留 Codex 现有未知字段和其他凭证,仅仅针对核心的少数几个字段进行修改，注意非官方的auth.json只能有一个OPENAI_API_KEY，而官方不一样
- API Key 优先写入 `auth.json` 的 `env_key` 字段。
- Codex 不走 cc-switch 的 Claude OpenAI 代理，不写入 `proxy_token`。
- 完整 TOML / JSON 编辑器被用户手动编辑后，以完整编辑器内容为最终写入源。

### 3.4 OpenAI 接入能力

Claude Code 使用 OpenAI 时不能直接把 `ANTHROPIC_BASE_URL` 设置为 `https://api.openai.com/v1`。

Claude OpenAI 供应商应通过本地 Anthropic 兼容代理：

```text
Claude Code
-> http://127.0.0.1:18080/ccswitch/proxy/openai/{provider_id}
-> https://api.openai.com/v1/responses
```

代理要求：

- 校验 Claude Code 请求携带的代理 Token。
- 使用供应商中保存的 OpenAI API Key 调用上游。
- 将 Anthropic Messages 请求转换为 OpenAI Responses 请求。
- 将 OpenAI 响应转换回 Anthropic Messages 兼容响应。
- 至少支持非流式请求；后续可扩展流式。
- 错误与日志必须脱敏。

Codex CLI 使用 OpenAI 时直接写入 Codex 的 `model_providers` 与 `auth.json`，不经过上述 Claude 代理。

### 3.5 内置模板

首次启动且供应商文件不存在时应生成模板。

Claude 模板：

- 火山方舟
- GLM / 智谱
- MiniMax
- 小米 MiMo
- OpenAI 代理

Codex 模板：

- OpenAI 官网登录空配置

模板不得包含真实或看起来像真实的默认密钥。

## 4. 底层实现逻辑

### 4.1 配置文件边界

cc-switch 自身文件：

```text
ccswitch/claude_providers.json
ccswitch/codex_providers.json
ccswitch/state.json
ccswitch/backups/
```

真实 CLI 文件：

```text
claude/settings.json
claude.json
codex/config.toml
codex/auth.json
```

建议把供应商数据和激活状态分开保存，避免未来迁移困难：

- 供应商列表保存稳定配置。
- 激活状态保存当前选择。
- 应用供应商才写入真实 CLI 文件。

核心原则：

- ClaudeProvider 只处理 Claude Code。
- CodexProvider 只处理 Codex CLI。
- 不再使用一个 Provider 同时承载 Claude 与 Codex。
- 结构化字段用于方便编辑，完整配置用于兜底和最终表达。
- 保存时应记录时间，便于用户判断配置新旧。

### 4.4 写入与备份逻辑

写入流程：

1. 读取当前真实配置。
2. 创建备份。
3. 根据供应商生成下一份配置。
4. 校验生成结果可序列化。
5. 写入临时文件。
6. 原子替换目标文件。
7. 更新激活状态。
8. 返回成功。

### 4.5 前端状态逻辑

页面结构：

- 供应商列表 该网页可以新建，编辑，测试等
- 当前文件配置 可以编辑 保存为供应商
- 备份恢复

状态刷新规则：

- 新增、编辑、删除、应用、恢复、保存当前配置
- 编辑页保存成功后返回列表。
- 操作失败不清空当前表单。
- 测试连接不刷新配置状态，除非测试目标是当前配置且用户同时保存了配置。

### 4.6 启动脚本逻辑

启动脚本应支持：

```text
stop之前的存在的
持久化
start
status
读取 .env
后台运行并写入 PID
日志输出到文件
```

启动时优先运行已有二进制；没有二进制时回退到 `go run .`。

## 5. 安全与约束

当前项目默认不做登录鉴权，因此必须在文档和启动输出中提示风险：

- 公网可访问时，任何访问者都可能读取或修改配置。
- API Key 和 Token 当前按用户需求明文保存在本地文件。
- 生产环境建议只监听本机、内网访问，或使用反向代理 Basic Auth。

必须实现的安全底线：

- 前端不展示完整密钥，除非用户点击显示。
- 日志不输出完整密钥。
- API 错误不回显完整密钥。
- 测试连接结果脱敏。
- OpenAI 代理必须校验代理 Token。
- 文件权限尽量使用 `0600`，目录使用 `0700`。

## 6. 验收标准

项目达到可用状态时，应满足：

- 全新用户启动后能直接打开 Web 页面。
- Claude 和 Codex 供应商分列展示。
- 用户能新增一个 Claude 供应商，保存、测试、应用后 Claude Code 使用新配置。
- 用户能新增一个 Codex 供应商，保存、应用后 Codex CLI 使用新配置。
- 当前配置页能展示真实配置，并能保存为供应商。
- 每次应用或直接保存真实配置前都会产生备份。
- 最近备份可以恢复。
- OpenAI 作为 Claude 供应商时通过本地代理工作，不把 OpenAI 官方地址直接写给 Claude Code。
- Codex 使用 OpenAI 时直接写入 `config.toml` 与 `auth.json`。
