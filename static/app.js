// API helpers
const API = {
  get: async (url) => {
    const r = await fetch(url);
    if (!r.ok) { const t = await r.text(); throw new Error(`HTTP ${r.status}: ${t}`); }
    return r.json();
  },
  post: async (url, body) => {
    const r = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    if (!r.ok) { const t = await r.text(); throw new Error(`HTTP ${r.status}: ${t}`); }
    return r.json();
  },
  put: async (url, body) => {
    const r = await fetch(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    if (!r.ok) { const t = await r.text(); throw new Error(`HTTP ${r.status}: ${t}`); }
    return r.json();
  },
  del: async (url) => {
    const r = await fetch(url, { method: 'DELETE' });
    if (!r.ok) { const t = await r.text(); throw new Error(`HTTP ${r.status}: ${t}`); }
    return r.json();
  },
};

// State
let currentPage = 'providers';
let editingClaude = null;
let editingCodex = null;

const CLI_PATHS = {
  claudeSettings: '/home/ubuntu/.claude/settings.json',
  claudeJSON: '/home/ubuntu/.claude.json',
  codexConfig: '/home/ubuntu/.codex/config.toml',
  codexAuth: '/home/ubuntu/.codex/auth.json',
};

const CLAUDE_MANAGED_ENV_KEYS = [
  'ANTHROPIC_BASE_URL', 'ANTHROPIC_AUTH_TOKEN', 'ANTHROPIC_MODEL',
  'ANTHROPIC_REASONING_MODEL', 'ANTHROPIC_DEFAULT_HAIKU_MODEL',
  'ANTHROPIC_DEFAULT_SONNET_MODEL', 'ANTHROPIC_DEFAULT_OPUS_MODEL',
  'OPENAI_BASE_URL', 'OPENAI_API_KEY'
];

const CLAUDE_MANAGED_TOP_KEYS = [
  'env', 'hooks', 'permissions', 'language', 'timeout',
  'disableTelemetry', 'hideAiSignatures', 'teammates', 'toolSearch',
  'highThinking', 'disableAutoUpdate', 'includeCoAuthoredBy',
  'skipDangerousModePermissionPrompt'
];

// Utils
function el(tag, attrs = {}, ...children) {
  const e = document.createElement(tag);
  Object.entries(attrs).forEach(([k, v]) => {
    if (v === false || v === undefined || v === null) return;
    if (k === 'className') k = 'class';
    if (k.startsWith('on')) e.addEventListener(k.slice(2).toLowerCase(), v);
    else if (k === 'checked' || k === 'selected' || k === 'value') e[k] = v;
    else e.setAttribute(k, v);
  });
  children.forEach(c => {
    if (typeof c === 'string') e.appendChild(document.createTextNode(c));
    else if (c) e.appendChild(c);
  });
  return e;
}

function fmtJSON(obj) { try { return JSON.stringify(obj, null, 2); } catch { return '{}'; } }
function parseJSON(str) { try { return JSON.parse(str); } catch { return null; } }
function isPlainObject(v) { return v && typeof v === 'object' && !Array.isArray(v); }
function cloneJSON(v) { return JSON.parse(JSON.stringify(v || {})); }
function setOrDelete(obj, key, value) {
  if (value === '' || value === undefined || value === null) delete obj[key];
  else obj[key] = value;
}
function pruneEmptyObjects(obj) {
  if (!isPlainObject(obj)) return obj;
  Object.keys(obj).forEach(key => {
    if (isPlainObject(obj[key])) pruneEmptyObjects(obj[key]);
    if (isPlainObject(obj[key]) && Object.keys(obj[key]).length === 0) delete obj[key];
  });
  return obj;
}
function cleanClaudeSettings(settings) {
  const s = isPlainObject(settings) ? settings : {};
  if (isPlainObject(s.env)) {
    Object.keys(s.env).forEach(key => {
      if (s.env[key] === '' || s.env[key] === undefined || s.env[key] === null) delete s.env[key];
    });
  }
  [
    'disableTelemetry', 'hideAiSignatures', 'teammates', 'toolSearch',
    'highThinking', 'disableAutoUpdate', 'includeCoAuthoredBy',
    'skipDangerousModePermissionPrompt'
  ].forEach(key => {
    if (s[key] === false || s[key] === '' || s[key] === undefined || s[key] === null) delete s[key];
  });
  ['language', 'timeout'].forEach(key => {
    if (s[key] === '' || s[key] === undefined || s[key] === null) delete s[key];
  });
  return pruneEmptyObjects(s);
}
function extractClaudeExtraSettings(settings) {
  const extra = cloneJSON(settings);
  const originalEnv = isPlainObject(extra.env) ? extra.env : null;
  CLAUDE_MANAGED_TOP_KEYS.forEach(key => delete extra[key]);
  if (originalEnv) {
    const envExtra = { ...originalEnv };
    CLAUDE_MANAGED_ENV_KEYS.forEach(key => delete envExtra[key]);
    if (Object.keys(envExtra).length) extra.env = envExtra;
  }
  return pruneEmptyObjects(extra);
}
function extractExtraAuth(auth, envKey = '') {
  const extra = cloneJSON(auth);
  delete extra.env_key;
  delete extra.OPENAI_API_KEY;
  if (envKey) delete extra[envKey];
  return pruneEmptyObjects(extra);
}
function mask(s) { if (!s || s.length < 8) return s; return s.slice(0, 4) + '****' + s.slice(-4); }
function escapeHtml(t) {
  const d = document.createElement('div'); d.textContent = t; return d.innerHTML;
}
function formatDate(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return d.toLocaleString('zh-CN');
}
function formatBackupName(name) {
  // claude_20240102_150405 -> 2024-01-02 15:04:05
  const m = name.match(/_(\d{4})(\d{2})(\d{2})_(\d{2})(\d{2})(\d{2})$/);
  if (!m) return name;
  return `${m[1]}-${m[2]}-${m[3]} ${m[4]}:${m[5]}:${m[6]}`;
}

// Toast notification system
const Toast = {
  container: null,
  ensureContainer() {
    if (!this.container) {
      this.container = el('div', { id: 'toast-container' });
      document.body.appendChild(this.container);
    }
  },
  show(message, type = 'info', duration = 3000) {
    this.ensureContainer();
    const toast = el('div', { className: `toast toast-${type}` }, message);
    this.container.appendChild(toast);
    requestAnimationFrame(() => toast.classList.add('show'));
    setTimeout(() => {
      toast.classList.remove('show');
      setTimeout(() => toast.remove(), 300);
    }, duration);
  },
  success(msg) { this.show(msg, 'success'); },
  error(msg) { this.show(msg, 'error', 5000); },
  info(msg) { this.show(msg, 'info'); },
};

// Navigation
function showPage(page) {
  currentPage = page;
  document.querySelectorAll('.nav-btn').forEach(b => b.classList.toggle('active', b.dataset.page === page));
  editingClaude = null; editingCodex = null;
  if (page === 'providers') renderProviders();
  else if (page === 'current') renderCurrent();
  else if (page === 'backups') renderBackups();
}

document.addEventListener('click', e => {
  if (e.target.matches('.nav-btn')) showPage(e.target.dataset.page);
  if (e.target.closest('.app-title')) showPage('providers');
});

function editToolbar(title, ...actions) {
  return el('div', { className: 'edit-toolbar' },
    el('button', { className: 'back-link', type: 'button', onClick: () => showPage('providers') }, '← 供应商列表'),
    el('h2', {}, title),
    el('div', { className: 'edit-actions' }, ...actions)
  );
}

// Providers Page
async function renderProviders() {
  const main = document.getElementById('main');
  main.innerHTML = '';
  main.classList.add('loading');

  let claudeData, codexData;
  try {
    [claudeData, codexData] = await Promise.all([
      API.get('/ccswitch/api/claude/providers'),
      API.get('/ccswitch/api/codex/providers'),
    ]);
    // 按更新时间倒序排列
    claudeData.providers.sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));
    codexData.providers.sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));
  } catch (err) {
    Toast.error('加载供应商列表失败: ' + err.message);
    main.classList.remove('loading');
    return;
  } finally {
    main.classList.remove('loading');
  }

  // Active provider names for header hint
  const activeClaude = claudeData.providers.find(p => p.id === claudeData.active_id);
  const activeCodex = codexData.providers.find(p => p.id === codexData.active_id);

  const container = el('div', { className: 'providers-page' },
    el('div', { className: 'section' },
      el('div', { className: 'section-header' },
        el('h2', {}, 'Claude Code 供应商'),
        activeClaude ? el('span', { className: 'active-hint' }, `当前: ${activeClaude.name}`) : null
      ),
      el('button', { className: 'btn primary', onClick: () => editClaude() }, '+ 新增 Claude'),
      renderProviderGrid(claudeData.providers, claudeData.active_id, 'claude')
    ),
    el('div', { className: 'section' },
      el('div', { className: 'section-header' },
        el('h2', {}, 'Codex CLI 供应商'),
        activeCodex ? el('span', { className: 'active-hint' }, `当前: ${activeCodex.name}`) : null
      ),
      el('button', { className: 'btn primary', onClick: () => editCodex() }, '+ 新增 Codex'),
      renderProviderGrid(codexData.providers, codexData.active_id, 'codex')
    )
  );
  main.appendChild(container);
}

function renderProviderGrid(providers, activeId, type) {
  if (!providers || providers.length === 0) return el('p', { className: 'empty' }, '暂无供应商');
  const grid = el('div', { className: 'provider-grid' });
  providers.forEach(p => {
    const isActive = p.id === activeId;
    const env = type === 'claude' ? (p.settings?.env || {}) : {};
    const model = type === 'claude' ? env.ANTHROPIC_MODEL : p.config_toml?.match(/model\s*=\s*"([^"]+)"/)?.[1];
    let base = '';
    let apiFormat = '';
    if (type === 'claude') {
      base = env.ANTHROPIC_BASE_URL || '';
      apiFormat = base.includes('/proxy/openai') ? 'OpenAI 代理' : 'Anthropic';
    } else if (p.config_toml) {
      const sectionMatch = p.config_toml.match(/\[model_providers\.OpenAI\]([^\[]*)/);
      const mpMatch = sectionMatch ? sectionMatch[1].match(/base_url\s*=\s*"([^"]+)"/) : null;
      if (mpMatch) base = mpMatch[1];
      else {
        const topMatch = p.config_toml.match(/^base_url\s*=\s*"([^"]+)"/m);
        if (topMatch) base = topMatch[1];
      }
    }

    const hasKey = type === 'claude'
      ? !!env.ANTHROPIC_AUTH_TOKEN
      : !!(p.auth_json?.OPENAI_API_KEY);

    const card = el('div', { className: `provider-card ${isActive ? 'active' : ''}` },
      el('div', { className: 'card-header' },
        el('div', {},
          el('strong', {}, p.name),
          p.notes ? el('div', { className: 'card-notes' }, p.notes) : null
        ),
        isActive ? el('span', { className: 'badge' }, '当前激活') : null
      ),
      el('div', { className: 'card-body' },
        el('div', { className: 'card-meta' },
          el('span', { className: 'card-tag' }, model || '无模型'),
          apiFormat ? el('span', { className: 'card-tag tag-format' }, apiFormat) : null,
          hasKey ? el('span', { className: 'card-tag tag-key' }, '已配置 Key') : el('span', { className: 'card-tag tag-nokey' }, '未配置 Key')
        ),
        el('div', { className: 'card-url' }, base || '默认地址'),
        el('div', { className: 'card-time' }, `更新于 ${formatDate(p.updated_at)}`)
      ),
      el('div', { className: 'card-actions' },
        el('button', { className: 'btn', title: '编辑供应商配置', onClick: () => type === 'claude' ? editClaude(p) : editCodex(p) }, '编辑'),
        el('button', { className: 'btn danger', title: '删除此供应商', onClick: () => deleteProvider(type, p.id, p.name) }, '删除'),
        el('button', { className: 'btn primary', title: '应用此供应商到 CLI 配置', onClick: () => applyProvider(type, p.id) }, '应用'),
        el('button', { className: 'btn', title: '测试连接', onClick: () => testProvider(type, p.id) }, '测试')
      )
    );
    grid.appendChild(card);
  });
  return grid;
}

async function deleteProvider(type, id, name) {
  if (!confirm(`确定删除供应商 "${name}" 吗？\n\n此操作不可恢复。`)) return;
  try {
    await API.del(`/ccswitch/api/${type}/providers/${id}`);
    Toast.success(`已删除 "${name}"`);
    renderProviders();
  } catch (err) {
    Toast.error('删除失败: ' + err.message);
  }
}

async function applyProvider(type, id) {
  try {
    await API.post(`/ccswitch/api/${type}/apply`, { id });
    Toast.success('应用成功，已写入 CLI 配置并自动备份');
    renderProviders();
  } catch (err) {
    Toast.error('应用失败: ' + err.message);
  }
}

async function testProvider(type, id) {
  await runTestWithModal(`/ccswitch/api/${type}/test`, { provider_id: id });
}

async function runTestWithModal(url, payload) {
  const streamUrl = url.endsWith('/stream') ? url : `${url}/stream`;
  const startedAt = Date.now();
  const pre = el('pre', { className: 'test-output' }, '');
  const status = el('div', { className: 'test-status' }, '正在连接流式测试接口...');
  showModal(el('div', {},
    el('h3', {}, '测试连接'),
    status,
    pre
  ));

  let lastStatus = '正在连接流式测试接口';
  const timer = setInterval(() => {
    const elapsed = ((Date.now() - startedAt) / 1000).toFixed(1);
    status.textContent = `${lastStatus} ${elapsed}s`;
  }, 400);

  try {
    const response = await fetch(streamUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(parseHTTPError(text));
    }
    if (!response.body) {
      await runLegacyTestResult(url, payload, pre);
      lastStatus = '测试通过';
      clearInterval(timer);
      status.textContent = `测试通过 ${((Date.now() - startedAt) / 1000).toFixed(1)}s`;
      return;
    }

    let done = false;
    await readEventStream(response.body, (event, data) => {
      if (event === 'status') {
        lastStatus = data.message || lastStatus;
      } else if (event === 'delta') {
        pre.textContent += data.text || '';
        pre.scrollTop = pre.scrollHeight;
      } else if (event === 'done') {
        done = true;
        lastStatus = '测试通过';
      } else if (event === 'error') {
        throw new Error(data.message || '测试失败');
      }
    });

    clearInterval(timer);
    const elapsed = ((Date.now() - startedAt) / 1000).toFixed(1);
    status.textContent = `${done ? '测试通过' : '测试结束'} ${elapsed}s`;
    if (!pre.textContent) pre.textContent = '(空回复)';
  } catch (err) {
    clearInterval(timer);
    const elapsed = ((Date.now() - startedAt) / 1000).toFixed(1);
    status.textContent = `测试失败 ${elapsed}s`;
    pre.textContent = pre.textContent ? `${pre.textContent}\n\n[错误] ${err.message}` : err.message;
    Toast.error('测试失败: ' + err.message);
  }
}

async function runLegacyTestResult(url, payload, pre) {
  const res = await API.post(url, payload);
  if (res.error) throw new Error(res.error);
  pre.textContent = res.reply || '(空回复)';
}

async function readEventStream(body, onEvent) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let idx;
    while ((idx = buffer.search(/\r?\n\r?\n/)) >= 0) {
      const block = buffer.slice(0, idx);
      const matched = buffer.slice(idx).match(/^\r?\n\r?\n/)[0];
      buffer = buffer.slice(idx + matched.length);
      handleSSEBlock(block, onEvent);
    }
  }
  buffer += decoder.decode();
  if (buffer.trim()) handleSSEBlock(buffer, onEvent);
}

function handleSSEBlock(block, onEvent) {
  let event = 'message';
  const dataLines = [];
  block.split(/\r?\n/).forEach(line => {
    if (line.startsWith('event:')) event = line.slice(6).trim();
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart());
  });
  if (!dataLines.length) return;
  let data;
  try {
    data = JSON.parse(dataLines.join('\n'));
  } catch {
    data = { text: dataLines.join('\n') };
  }
  onEvent(event, data);
}

function parseHTTPError(text) {
  try {
    const obj = JSON.parse(text);
    return obj.error || text;
  } catch {
    return text || '请求失败';
  }
}

// Claude Edit Page
function editClaude(provider = null) {
  editingClaude = provider || {
    id: '', name: '', website: '', notes: '',
    settings: { env: {} },
    claude_json: {}
  };
  renderClaudeEdit();
}

function renderClaudeEdit() {
  const main = document.getElementById('main');
  const p = editingClaude;
  p.settings = p.settings || { env: {} };
  const env = p.settings.env || {};

  main.innerHTML = '';
  const left = el('div', { className: 'edit-left' });
  const right = el('div', { className: 'edit-right' });

  const headerTitle = p.id ? `编辑 Claude 供应商: ${p.name}` : '新增 Claude 供应商';

  // API 格式判断
  const isOpenAIProxy = (env.ANTHROPIC_BASE_URL || '').includes('/proxy/openai');
  const apiFormat = isOpenAIProxy ? 'openai' : 'anthropic';

  // API Format selector
  left.appendChild(el('div', { className: 'field-row' },
    el('label', {}, 'API 格式'),
    el('select', { className: 'field-input', 'data-key': 'api_format', onChange: (e) => {
      const baseUrlInput = left.querySelector('[data-key="base_url"]');
      if (e.target.value === 'openai') {
        if (p.id) {
          baseUrlInput.value = 'http://127.0.0.1:18080/ccswitch/proxy/openai/' + p.id;
        } else {
          baseUrlInput.value = 'http://127.0.0.1:18080/ccswitch/proxy/openai/<供应商ID>';
        }
      } else {
        baseUrlInput.value = '';
      }
      syncToJSON();
    }},
      el('option', { value: 'anthropic', selected: apiFormat === 'anthropic' }, 'Anthropic Messages'),
      el('option', { value: 'openai', selected: apiFormat === 'openai' }, 'OpenAI Responses 代理模式')
    )
  ));

  // Form fields with hints
  const fieldDefs = [
    { label: '名称', key: 'name', value: p.name, type: 'text', hint: '供应商显示名称', group: 'core' },
    { label: 'Base URL', key: 'base_url', value: env.ANTHROPIC_BASE_URL || '', type: 'text', hint: 'Claude 协议入口，如 http://1.12.67.59:8080', group: 'core' },
    { label: 'API Key', key: 'api_key', value: env.ANTHROPIC_AUTH_TOKEN || '', type: 'password', hint: 'Anthropic 模式的认证密钥 (ANTHROPIC_AUTH_TOKEN)', group: 'core' },
    { label: '主模型', key: 'model', value: env.ANTHROPIC_MODEL || '', type: 'text', hint: '默认使用的模型名称，如 glm-5.1', group: 'core' },
    { label: '官网链接', key: 'website', value: p.website, type: 'text', hint: '可选，点击可跳转', group: 'optional' },
    { label: '备注', key: 'notes', value: p.notes, type: 'text', hint: '可选，显示在卡片上', group: 'optional' },
    { label: '代理 Token', key: 'proxy_token', value: env.ANTHROPIC_AUTH_TOKEN || '', type: 'password', hint: 'OpenAI 代理模式的认证密钥 (与 API Key 相同)', group: 'optional' },
    { label: 'OpenAI Base URL', key: 'openai_base_url', value: env.OPENAI_BASE_URL || '', type: 'text', hint: 'OpenAI 代理模式上游地址，如 https://api.openai.com/v1', group: 'optional' },
    { label: 'OpenAI API Key', key: 'openai_api_key', value: env.OPENAI_API_KEY || '', type: 'password', hint: 'OpenAI 代理模式调用上游 Responses API 的密钥', group: 'optional' },
    { label: 'Reasoning Model', key: 'reasoning_model', value: env.ANTHROPIC_REASONING_MODEL || '', type: 'text', hint: '推理模型，可选', group: 'optional' },
    { label: 'Haiku 模型', key: 'haiku', value: env.ANTHROPIC_DEFAULT_HAIKU_MODEL || '', type: 'text', hint: '可选', group: 'optional' },
    { label: 'Sonnet 模型', key: 'sonnet', value: env.ANTHROPIC_DEFAULT_SONNET_MODEL || '', type: 'text', hint: '可选', group: 'optional' },
    { label: 'Opus 模型', key: 'opus', value: env.ANTHROPIC_DEFAULT_OPUS_MODEL || '', type: 'text', hint: '可选', group: 'optional' },
    { label: 'Timeout', key: 'timeout', value: p.settings.timeout || '', type: 'text', hint: '请求超时秒数，如 30', group: 'optional' },
    { label: 'Language', key: 'language', value: p.settings.language || '', type: 'select', options: [
      ['', '不写入'],
      ['chinese', '中文'], ['english', 'English'], ['japanese', '日本語'], ['korean', '한국어'], ['spanish', 'Español'], ['french', 'Français'], ['german', 'Deutsch']
    ], hint: 'Claude Code language 设置，可选', group: 'optional' },
    { label: '权限默认模式', key: 'defaultMode', value: (p.settings.permissions || {}).defaultMode || '', type: 'select', options: [
      ['', '不写入'],
      ['default', 'default - 每类工具首次确认'],
      ['acceptEdits', 'acceptEdits - 自动接受文件编辑'],
      ['plan', 'plan - 只读规划'],
      ['auto', 'auto - 自动模式'],
      ['dontAsk', 'dontAsk - 仅预批准工具'],
      ['bypassPermissions', 'bypassPermissions - 全部允许']
    ], hint: 'permissions.defaultMode，可选' , group: 'optional' },
  ];

  const requiredBox = el('div', { className: 'config-box required-box' },
    el('div', { className: 'config-box-title' }, '基础配置')
  );
  const optionalBox = el('div', { className: 'config-box optional-box' },
    el('div', { className: 'config-box-title' }, '可选配置')
  );
  left.appendChild(requiredBox);
  left.appendChild(optionalBox);

  fieldDefs.forEach(({ label, key, value, type, hint, options }) => {
    const row = el('div', { className: 'field-row' });
    const labelEl = el('label', {}, label);
    if (hint) {
      labelEl.appendChild(el('span', { className: 'field-hint', title: hint }, ' ?'));
    }
    const input = type === 'select'
      ? el('select', { 'data-key': key, className: 'field-input' },
        ...(options || []).map(([val, text]) => el('option', (value || '') === val ? { value: val, selected: 'selected' } : { value: val }, text)))
      : el('input', { type, value: value || '', 'data-key': key, className: 'field-input' });
    row.appendChild(labelEl);
    row.appendChild(input);
    if (type === 'password') {
      const toggle = el('button', { className: 'toggle-mask', type: 'button', onClick: (e) => {
        input.type = input.type === 'password' ? 'text' : 'password';
        e.target.textContent = input.type === 'password' ? '显示' : '隐藏';
      }}, '显示');
      row.appendChild(toggle);
    }
    const def = fieldDefs.find(f => f.key === key);
    (def?.group === 'optional' ? optionalBox : requiredBox).appendChild(row);
  });

  // Switches
  const switchesBox = el('div', { className: 'config-box optional-box' },
    el('div', { className: 'config-box-title' }, '功能开关')
  );
  left.appendChild(switchesBox);
  const switches = [
    ['禁用非必要流量', 'disableTelemetry', !!p.settings.disableTelemetry, 'disableTelemetry'],
    ['隐藏 AI 签名', 'hideAiSignatures', !!p.settings.hideAiSignatures, 'hideAiSignatures'],
    ['Teammates 模式', 'teammates', !!p.settings.teammates, 'teammates'],
    ['Tool Search', 'toolSearch', !!p.settings.toolSearch, 'toolSearch'],
    ['高思考模式', 'highThinking', !!p.settings.highThinking, 'highThinking'],
    ['禁用自动更新', 'disableAutoUpdate', !!p.settings.disableAutoUpdate, 'disableAutoUpdate'],
    ['includeCoAuthoredBy', 'includeCoAuthoredBy', !!p.settings.includeCoAuthoredBy, 'includeCoAuthoredBy'],
    ['跳过危险模式确认', 'skipDangerousModePermissionPrompt', !!p.settings.skipDangerousModePermissionPrompt, 'skipDangerousModePermissionPrompt'],
  ];
  switches.forEach(([label, key, checked, hintKey]) => {
    const row = el('div', { className: 'field-row switch-row' });
    const labelEl = el('label', {}, label);
    const hintMap = {
      disableTelemetry: '停止发送使用统计',
      hideAiSignatures: '隐藏 AI 生成的提交签名',
      teammates: '启用 Teammates 协作模式',
      toolSearch: '允许工具搜索',
      highThinking: '启用高思考模式',
      disableAutoUpdate: '禁止自动更新 Claude Code',
      includeCoAuthoredBy: '在提交中包含 Co-Authored-By',
      skipDangerousModePermissionPrompt: '跳过危险操作确认提示',
    };
    if (hintMap[hintKey]) {
      labelEl.appendChild(el('span', { className: 'field-hint', title: hintMap[hintKey] }, ' ?'));
    }
    row.appendChild(labelEl);
    row.appendChild(el('input', { type: 'checkbox', checked, 'data-switch': key, className: 'field-check' }));
    switchesBox.appendChild(row);
  });

  // Right side: JSON editors
  const settingsTA = el('textarea', { className: 'json-editor', rows: 20 }, fmtJSON(cleanClaudeSettings(p.settings)));

  right.appendChild(el('h3', {}, `${CLI_PATHS.claudeSettings} 预览`));
  right.appendChild(el('p', { className: 'editor-hint' }, '与左侧表单实时同步，可直接编辑高级配置'));
  right.appendChild(settingsTA);

  // .claude.json editor
  const claudeJsonTA = el('textarea', { className: 'json-editor', rows: 8 }, fmtJSON(p.claude_json || {}));
  right.appendChild(el('h3', {}, `${CLI_PATHS.claudeJSON} 预览`));
  right.appendChild(el('p', { className: 'editor-hint' }, 'Claude Code 账户设置，如登录态、订阅信息'));
  right.appendChild(claudeJsonTA);

  // Hooks & Permissions JSON editors
  const hooksTA = el('textarea', { className: 'json-editor', rows: 6 }, fmtJSON(p.settings.hooks || {}));
  const permissionsTA = el('textarea', { className: 'json-editor', rows: 6 }, fmtJSON(p.settings.permissions || {}));
  left.appendChild(el('h4', { className: 'json-section-title' }, 'Hooks JSON'));
  left.appendChild(el('p', { className: 'editor-hint' }, '自定义 hooks 配置'));
  left.appendChild(hooksTA);
  left.appendChild(el('h4', { className: 'json-section-title' }, 'Permissions JSON'));
  left.appendChild(el('p', { className: 'editor-hint' }, '权限配置，如 { "defaultMode": "acceptEdits" }'));
  left.appendChild(permissionsTA);
  const extraSettingsTA = el('textarea', { className: 'json-editor', rows: 8 }, fmtJSON(extractClaudeExtraSettings(p.settings)));
  const extraBox = el('div', { className: 'config-box extra-box' },
    el('div', { className: 'config-box-title' }, '额外配置'),
    el('p', { className: 'editor-hint' }, '右侧 settings.json 中没有固定表项对应的 key 会出现在这里'),
    extraSettingsTA
  );
  left.appendChild(extraBox);

  // Sync form -> JSON
  function syncToJSON() {
    const s = parseJSON(extraSettingsTA.value) || {};
    s.env = s.env || {};
    left.querySelectorAll('.field-input').forEach(inp => {
      const k = inp.dataset.key;
      const v = inp.value;
      if (k === 'name' || k === 'website' || k === 'notes') { p[k] = v; }
      else if (k === 'base_url') setOrDelete(s.env, 'ANTHROPIC_BASE_URL', v);
      else if (k === 'api_key') {
        setOrDelete(s.env, 'ANTHROPIC_AUTH_TOKEN', v);
        const proxyInput = left.querySelector('[data-key="proxy_token"]');
        if (proxyInput && proxyInput.value !== v) proxyInput.value = v;
      }
      else if (k === 'proxy_token') {
        setOrDelete(s.env, 'ANTHROPIC_AUTH_TOKEN', v);
        const apiInput = left.querySelector('[data-key="api_key"]');
        if (apiInput && apiInput.value !== v) apiInput.value = v;
      }
      else if (k === 'model') setOrDelete(s.env, 'ANTHROPIC_MODEL', v);
      else if (k === 'openai_base_url') setOrDelete(s.env, 'OPENAI_BASE_URL', v);
      else if (k === 'openai_api_key') setOrDelete(s.env, 'OPENAI_API_KEY', v);
      else if (k === 'reasoning_model') setOrDelete(s.env, 'ANTHROPIC_REASONING_MODEL', v);
      else if (k === 'haiku') setOrDelete(s.env, 'ANTHROPIC_DEFAULT_HAIKU_MODEL', v);
      else if (k === 'sonnet') setOrDelete(s.env, 'ANTHROPIC_DEFAULT_SONNET_MODEL', v);
      else if (k === 'opus') setOrDelete(s.env, 'ANTHROPIC_DEFAULT_OPUS_MODEL', v);
      else if (k === 'timeout') setOrDelete(s, 'timeout', v !== '' ? Number(v) : '');
      else if (k === 'language') setOrDelete(s, 'language', v);
      else if (k === 'defaultMode') {
        s.permissions = s.permissions || {};
        if (v) s.permissions.defaultMode = v; else delete s.permissions.defaultMode;
      }
    });
    left.querySelectorAll('.field-check').forEach(ch => {
      const k = ch.dataset.switch;
      if (ch.checked) s[k] = true; else delete s[k];
    });
    const hooks = parseJSON(hooksTA.value);
    if (hooks && Object.keys(hooks).length) s.hooks = hooks; else delete s.hooks;
    const perms = parseJSON(permissionsTA.value);
    const defaultMode = left.querySelector('[data-key="defaultMode"]')?.value || '';
    if (perms) {
      if (defaultMode) perms.defaultMode = defaultMode; else delete perms.defaultMode;
      if (Object.keys(perms).length) s.permissions = perms; else delete s.permissions;
    } else if (defaultMode) {
      s.permissions = { defaultMode };
    } else {
      delete s.permissions;
    }
    // sync claude_json from textarea
    const claudeJson = parseJSON(claudeJsonTA.value);
    if (claudeJson) p.claude_json = claudeJson;
    const cleaned = cleanClaudeSettings(s);
    settingsTA.value = fmtJSON(cleaned);
    extraSettingsTA.value = fmtJSON(extractClaudeExtraSettings(cleaned));
  }

  // Sync JSON -> form
  function syncFromJSON() {
    const s = parseJSON(settingsTA.value) || {};
    const env2 = s.env || {};
    const isProxy = (env2.ANTHROPIC_BASE_URL || '').includes('/proxy/openai');
    const fmtSel = left.querySelector('[data-key="api_format"]');
    if (fmtSel) fmtSel.value = isProxy ? 'openai' : 'anthropic';
    const map = {
      base_url: env2.ANTHROPIC_BASE_URL,
      api_key: env2.ANTHROPIC_AUTH_TOKEN,
      proxy_token: env2.ANTHROPIC_AUTH_TOKEN,
      model: env2.ANTHROPIC_MODEL,
      openai_base_url: env2.OPENAI_BASE_URL,
      openai_api_key: env2.OPENAI_API_KEY,
      reasoning_model: env2.ANTHROPIC_REASONING_MODEL,
      haiku: env2.ANTHROPIC_DEFAULT_HAIKU_MODEL,
      sonnet: env2.ANTHROPIC_DEFAULT_SONNET_MODEL,
      opus: env2.ANTHROPIC_DEFAULT_OPUS_MODEL,
      timeout: s.timeout,
      language: s.language,
      defaultMode: (s.permissions || {}).defaultMode,
    };
    left.querySelectorAll('.field-input').forEach(inp => {
      if (map[inp.dataset.key] !== undefined) inp.value = map[inp.dataset.key] || '';
    });
    left.querySelectorAll('.field-check').forEach(ch => {
      ch.checked = !!s[ch.dataset.switch];
    });
    hooksTA.value = fmtJSON(s.hooks || {});
    permissionsTA.value = fmtJSON(s.permissions || {});
    extraSettingsTA.value = fmtJSON(extractClaudeExtraSettings(s));
  }

  left.querySelectorAll('input, select').forEach(i => i.addEventListener('input', syncToJSON));
  hooksTA.addEventListener('input', syncToJSON);
  permissionsTA.addEventListener('input', syncToJSON);
  extraSettingsTA.addEventListener('input', syncToJSON);
  claudeJsonTA.addEventListener('input', syncToJSON);
  settingsTA.addEventListener('input', () => {
    const s = parseJSON(settingsTA.value);
    if (!s) return;
    settingsTA.value = fmtJSON(cleanClaudeSettings(s));
    syncFromJSON();
  });

  const toolbar = editToolbar(headerTitle,
    el('button', { className: 'btn primary', onClick: async () => {
      const s = parseJSON(settingsTA.value);
      if (!s) { Toast.error('settings.json JSON 无效'); return; }
      const cj = parseJSON(claudeJsonTA.value);
      if (cj === null) { Toast.error('.claude.json JSON 无效'); return; }
      const payload = {
        id: p.id, name: p.name, website: p.website, notes: p.notes,
        settings: cleanClaudeSettings(s), claude_json: cj || {}
      };
      try {
        if (p.id) await API.put(`/ccswitch/api/claude/providers/${p.id}`, payload);
        else await API.post('/ccswitch/api/claude/providers', payload);
        Toast.success('保存成功');
        showPage('providers');
      } catch (err) {
        Toast.error('保存失败: ' + err.message);
      }
    }}, '保存'),
    el('button', { className: 'btn', onClick: () => showPage('providers') }, '取消'),
    el('button', { className: 'btn', onClick: async () => {
      const s = parseJSON(settingsTA.value);
      if (!s) { Toast.error('JSON 无效'); return; }
      const baseURL = (s.env && s.env.ANTHROPIC_BASE_URL) || '';
      const apiKey = (s.env && s.env.ANTHROPIC_AUTH_TOKEN) || '';
      const model = (s.env && s.env.ANTHROPIC_MODEL) || '';
      const payload = p.id ? { provider_id: p.id } : { base_url: baseURL, api_key: apiKey, model };
      await runTestWithModal('/ccswitch/api/claude/test', payload);
    }}, '测试连接')
  );

  main.appendChild(toolbar);
  main.appendChild(el('div', { className: 'edit-page' }, left, right));
}

// Codex Edit Page
function editCodex(provider = null) {
  editingCodex = provider || {
    id: '', name: '', website: '', notes: '',
    config_toml: [
      'model_provider = "OpenAI"',
      'model = "gpt-5.5"',
      'review_model = "gpt-5.5"',
      'model_reasoning_effort = "xhigh"',
      'disable_response_storage = true',
      'network_access = "enabled"',
      'windows_wsl_setup_acknowledged = true',
      'model_context_window = 1000000',
      'model_auto_compact_token_limit = 900000',
      '',
      '[model_providers.OpenAI]',
      'name = "OpenAI"',
      'base_url = ""',
      'wire_api = "responses"',
      'requires_openai_auth = true',
      ''
    ].join('\n'),
    auth_json: { OPENAI_API_KEY: '' }
  };
  renderCodexEdit();
}

function renderCodexEdit() {
  const main = document.getElementById('main');
  const p = editingCodex;
  main.innerHTML = '';

  const left = el('div', { className: 'edit-left' });
  const right = el('div', { className: 'edit-right' });

  const headerTitle = p.id ? `编辑 Codex 供应商: ${p.name}` : '新增 Codex 供应商';

  function parseCodexTomlBlocks(toml) {
    const blocks = [];
    let current = { header: '', lines: [] };
    String(toml || '').split('\n').forEach(line => {
      if (/^\s*\[[^\]]+\]\s*$/.test(line)) {
        blocks.push(current);
        current = { header: line.trim(), lines: [] };
      } else {
        current.lines.push(line);
      }
    });
    blocks.push(current);
    return blocks;
  }

  function tomlKeyOfLine(line) {
    const m = line.match(/^\s*([A-Za-z0-9_.-]+)\s*=/);
    return m ? m[1] : '';
  }

  function tomlValueFromLines(lines, key) {
    const line = lines.find(l => tomlKeyOfLine(l) === key);
    if (!line) return '';
    const raw = line.slice(line.indexOf('=') + 1).trim();
    const quoted = raw.match(/^"([^"]*)"$/);
    return quoted ? quoted[1] : raw;
  }

  function providerSectionHeader(provider) {
    return `[model_providers.${provider || 'OpenAI'}]`;
  }

  function codexTopLines(toml) {
    const block = parseCodexTomlBlocks(toml).find(b => !b.header);
    return block ? block.lines : [];
  }

  function codexProviderLines(toml, provider) {
    const header = providerSectionHeader(provider);
    const block = parseCodexTomlBlocks(toml).find(b => b.header === header);
    return block ? block.lines : [];
  }

  // 从现有 toml 提取值
  let tomlModel = '', tomlProvider = '', tomlBaseUrl = '', tomlWire = '';
  let tomlApproval = '', tomlSandbox = '', tomlReasoning = '', tomlPersonality = '';
  let tomlServiceTier = '', tomlReviewModel = '', tomlNetworkAccess = '';
  let tomlContextWindow = '', tomlCompactLimit = '';
  let tomlDisableStorage = '', tomlRequiresAuth = '', tomlWslAck = '';
  if (p.config_toml) {
    const top = codexTopLines(p.config_toml);
    tomlProvider = tomlValueFromLines(top, 'model_provider');
    const providerLines = codexProviderLines(p.config_toml, tomlProvider || 'OpenAI');
    tomlModel = tomlValueFromLines(top, 'model');
    tomlReviewModel = tomlValueFromLines(top, 'review_model');
    tomlBaseUrl = tomlValueFromLines(providerLines, 'base_url') || tomlValueFromLines(top, 'base_url');
    tomlWire = tomlValueFromLines(providerLines, 'wire_api') || tomlValueFromLines(top, 'wire_api');
    tomlApproval = tomlValueFromLines(top, 'approval_policy');
    tomlSandbox = tomlValueFromLines(top, 'sandbox_mode');
    tomlReasoning = tomlValueFromLines(top, 'model_reasoning_effort');
    tomlPersonality = tomlValueFromLines(top, 'personality');
    tomlServiceTier = tomlValueFromLines(top, 'service_tier');
    tomlNetworkAccess = tomlValueFromLines(top, 'network_access');
    tomlContextWindow = tomlValueFromLines(top, 'model_context_window');
    tomlCompactLimit = tomlValueFromLines(top, 'model_auto_compact_token_limit');
    tomlDisableStorage = tomlValueFromLines(top, 'disable_response_storage');
    tomlRequiresAuth = tomlValueFromLines(providerLines, 'requires_openai_auth');
    tomlWslAck = tomlValueFromLines(top, 'windows_wsl_setup_acknowledged');
  }

  const fieldDefs = [
    { label: '名称', key: 'name', value: p.name, type: 'text', hint: '供应商显示名称' },
    { label: '官网链接', key: 'website', value: p.website, type: 'text', hint: '可选' },
    { label: '备注', key: 'notes', value: p.notes, type: 'text', hint: '可选，显示在卡片上' },
    { label: 'Model', key: 'model', value: tomlModel, type: 'text', hint: '如 gpt-5.5' },
    { label: 'Review Model', key: 'review_model', value: tomlReviewModel, type: 'text', hint: '评审模型，可与 Model 相同' },
    { label: 'Model Provider', key: 'model_provider', value: tomlProvider, type: 'text', hint: '如 OpenAI' },
    { label: 'Base URL', key: 'base_url', value: tomlBaseUrl, type: 'text', hint: '自定义 API 地址，留空使用官方' },
    { label: 'Wire API', key: 'wire_api', value: tomlWire, type: 'text', hint: '如 responses' },
    { label: 'Approval Policy', key: 'approval_policy', value: tomlApproval, type: 'text', hint: '如 suggest / auto-edit / full-auto' },
    { label: 'Sandbox Mode', key: 'sandbox_mode', value: tomlSandbox, type: 'text', hint: '如 container' },
    { label: 'Model Reasoning Effort', key: 'model_reasoning_effort', value: tomlReasoning, type: 'text', hint: '如 low / medium / high / xhigh' },
    { label: 'Network Access', key: 'network_access', value: tomlNetworkAccess, type: 'text', hint: '如 enabled' },
    { label: 'Context Window', key: 'model_context_window', value: tomlContextWindow, type: 'text', hint: '如 1000000' },
    { label: 'Compact Limit', key: 'model_auto_compact_token_limit', value: tomlCompactLimit, type: 'text', hint: '如 900000' },
    { label: 'Personality', key: 'personality', value: tomlPersonality, type: 'text', hint: '如 friendly / concise' },
    { label: 'Service Tier', key: 'service_tier', value: tomlServiceTier, type: 'text', hint: '如 default / flex / plus' },
    { label: 'Env Key', key: 'env_key', value: p.auth_json?.env_key || '', type: 'text', hint: '自定义环境变量键名，通常无需填写' },
    { label: 'API Key', key: 'api_key', value: p.auth_json?.[p.auth_json?.env_key || 'OPENAI_API_KEY'] || p.auth_json?.OPENAI_API_KEY || '', type: 'password', hint: 'OpenAI API Key' },
  ];
  fieldDefs.forEach(({ label, key, value, type, hint }) => {
    const row = el('div', { className: 'field-row' });
    const labelEl = el('label', {}, label);
    if (hint) labelEl.appendChild(el('span', { className: 'field-hint', title: hint }, ' ?'));
    const input = el('input', { type, value: value || '', 'data-key': key, className: 'field-input' });
    row.appendChild(labelEl);
    row.appendChild(input);
    if (type === 'password') {
      const toggle = el('button', { className: 'toggle-mask', type: 'button', onClick: (e) => {
        input.type = input.type === 'password' ? 'text' : 'password';
        e.target.textContent = input.type === 'password' ? '显示' : '隐藏';
      }}, '显示');
      row.appendChild(toggle);
    }
    left.appendChild(row);
  });

  // Checkboxes
  left.appendChild(el('h4', { className: 'switches-title' }, '功能开关'));
  const codexChecks = [
    ['禁用响应存储', 'disable_response_storage', tomlDisableStorage === 'true', '不在服务端存储对话历史'],
    ['WSL 已确认', 'windows_wsl_setup_acknowledged', tomlWslAck === 'true', '对应 windows_wsl_setup_acknowledged'],
    ['需要 OpenAI 认证', 'requires_openai_auth', tomlRequiresAuth === 'true', '非官方 OpenAI 代理时需要'],
  ];
  codexChecks.forEach(([label, key, checked, hint]) => {
    const row = el('div', { className: 'field-row switch-row' });
    const labelEl = el('label', {}, label);
    labelEl.appendChild(el('span', { className: 'field-hint', title: hint }, ' ?'));
    row.appendChild(labelEl);
    row.appendChild(el('input', { type: 'checkbox', checked, 'data-check': key, className: 'field-check' }));
    left.appendChild(row);
  });

  const initialExtraConfig = stripManagedCodexToml(p.config_toml || '');
  const initialConfig = `${buildManagedCodexToml(initialExtraConfig)}\n`;
  const configTA = el('textarea', { className: 'json-editor', rows: 20 }, initialConfig);
  const authTA = el('textarea', { className: 'json-editor', rows: 10 }, fmtJSON(p.auth_json));
  const extraConfigTA = el('textarea', { className: 'json-editor', rows: 8 }, initialExtraConfig);
  const extraAuthTA = el('textarea', { className: 'json-editor', rows: 6 }, fmtJSON(extractExtraAuth(p.auth_json, p.auth_json?.env_key || '')));

  left.appendChild(el('div', { className: 'config-box extra-box' },
    el('div', { className: 'config-box-title' }, '额外 config.toml 配置'),
    el('p', { className: 'editor-hint' }, '右侧 config.toml 中没有固定表项对应的条目会出现在这里'),
    extraConfigTA
  ));
  left.appendChild(el('div', { className: 'config-box extra-box' },
    el('div', { className: 'config-box-title' }, '额外 auth.json 配置'),
    el('p', { className: 'editor-hint' }, '右侧 auth.json 中没有固定表项对应的 key 会出现在这里'),
    extraAuthTA
  ));

  right.appendChild(el('h3', {}, `${CLI_PATHS.codexConfig} 预览`));
  right.appendChild(el('p', { className: 'editor-hint' }, '与左侧表单实时同步，可直接编辑完整配置'));
  right.appendChild(configTA);
  right.appendChild(el('h3', {}, `${CLI_PATHS.codexAuth} 预览`));
  right.appendChild(el('p', { className: 'editor-hint' }, '认证信息，通常只需填写 API Key'));
  right.appendChild(authTA);

  function tomlQuote(value) {
    return JSON.stringify(String(value || ''));
  }

  function stripManagedCodexToml(toml) {
    const managedTopKeys = [
      'model', 'review_model', 'model_provider', 'base_url', 'wire_api',
      'approval_policy', 'sandbox_mode', 'model_reasoning_effort',
      'personality', 'service_tier', 'disable_response_storage',
      'network_access', 'windows_wsl_setup_acknowledged',
      'model_context_window', 'model_auto_compact_token_limit'
    ];
    const managedProviderKeys = ['name', 'model', 'base_url', 'wire_api', 'requires_openai_auth'];
    return parseCodexTomlBlocks(toml).map(block => {
      const isModelProvider = /^\s*\[model_providers\.[^\]]+\]\s*$/.test(block.header);
      const managedKeys = isModelProvider ? managedProviderKeys : managedTopKeys;
      const lines = block.lines.filter(line => {
        const key = tomlKeyOfLine(line);
        return !key || !managedKeys.includes(key);
      });
      const hasContent = lines.some(line => line.trim() !== '');
      if (!block.header) return lines.join('\n');
      return hasContent ? [block.header, ...lines].join('\n') : '';
    }).join('\n\n').replace(/\n{3,}/g, '\n\n').trim();
  }

  function splitCodexExtraToml(toml, provider) {
    const providerHeader = providerSectionHeader(provider);
    const topLines = [];
    const providerExtraLines = [];
    const otherBlocks = [];
    parseCodexTomlBlocks(stripManagedCodexToml(toml)).forEach(block => {
      const lines = block.lines.filter(line => line.trim() !== '');
      if (!block.header) {
        topLines.push(...lines);
      } else if (block.header === providerHeader) {
        providerExtraLines.push(...lines);
      } else if (lines.length) {
        otherBlocks.push([block.header, ...lines].join('\n'));
      }
    });
    return { topLines, providerExtraLines, otherBlocks };
  }

  function buildManagedCodexToml(extraToml = '') {
    const value = key => left.querySelector(`[data-key="${key}"]`)?.value.trim() || '';
    const checked = key => !!left.querySelector(`[data-check="${key}"]`)?.checked;
    const provider = value('model_provider') || 'OpenAI';
    const extra = splitCodexExtraToml(extraToml, provider);
    const baseURL = value('base_url');
    const wireAPI = value('wire_api') || 'responses';
    const lines = [];

    [
      ['model_provider', provider],
      ['model', value('model')],
      ['review_model', value('review_model')],
      ['model_reasoning_effort', value('model_reasoning_effort')],
      ['approval_policy', value('approval_policy')],
      ['sandbox_mode', value('sandbox_mode')],
      ['personality', value('personality')],
      ['service_tier', value('service_tier')],
    ].forEach(([key, val]) => {
      if (val) lines.push(`${key} = ${tomlQuote(val)}`);
    });
    if (checked('disable_response_storage')) lines.push('disable_response_storage = true');
    if (value('network_access')) lines.push(`network_access = ${tomlQuote(value('network_access'))}`);
    if (checked('windows_wsl_setup_acknowledged')) lines.push('windows_wsl_setup_acknowledged = true');
    if (value('model_context_window')) lines.push(`model_context_window = ${value('model_context_window')}`);
    if (value('model_auto_compact_token_limit')) lines.push(`model_auto_compact_token_limit = ${value('model_auto_compact_token_limit')}`);
    if (extra.topLines.length) lines.push(...extra.topLines);

    lines.push('');
    lines.push(`[model_providers.${provider}]`);
    lines.push(`name = ${tomlQuote(provider)}`);
    if (baseURL) lines.push(`base_url = ${tomlQuote(baseURL)}`);
    if (wireAPI) lines.push(`wire_api = ${tomlQuote(wireAPI)}`);
    if (checked('requires_openai_auth')) lines.push('requires_openai_auth = true');
    if (extra.providerExtraLines.length) lines.push(...extra.providerExtraLines);
    if (extra.otherBlocks.length) lines.push('', ...extra.otherBlocks);

    return lines.join('\n').trim();
  }

  function syncToTOML() {
    p.name = left.querySelector('[data-key="name"]').value;
    p.website = left.querySelector('[data-key="website"]').value;
    p.notes = left.querySelector('[data-key="notes"]').value;
    const custom = stripManagedCodexToml(extraConfigTA.value).trim();
    if (extraConfigTA.value.trim() !== custom) extraConfigTA.value = custom;
    const managed = buildManagedCodexToml(custom);
    configTA.value = `${managed}\n`;

    const apiKey = left.querySelector('[data-key="api_key"]').value;
    const envKey = left.querySelector('[data-key="env_key"]').value;
    p.auth_json = parseJSON(extraAuthTA.value) || {};
    if (envKey) {
      p.auth_json.env_key = envKey;
      if (apiKey) p.auth_json[envKey] = apiKey; else delete p.auth_json[envKey];
      delete p.auth_json.OPENAI_API_KEY;
    } else if (apiKey) {
      p.auth_json.OPENAI_API_KEY = apiKey;
    } else {
      delete p.auth_json.OPENAI_API_KEY;
    }
    authTA.value = fmtJSON(p.auth_json);
    extraAuthTA.value = fmtJSON(extractExtraAuth(p.auth_json, envKey));
  }

  function syncFromTOML() {
    const toml = configTA.value;
    const top = codexTopLines(toml);
    const provider = tomlValueFromLines(top, 'model_provider') || 'OpenAI';
    const providerLines = codexProviderLines(toml, provider);
    const map = {
      model: tomlValueFromLines(top, 'model'),
      review_model: tomlValueFromLines(top, 'review_model'),
      model_provider: provider,
      base_url: tomlValueFromLines(providerLines, 'base_url') || tomlValueFromLines(top, 'base_url'),
      wire_api: tomlValueFromLines(providerLines, 'wire_api') || tomlValueFromLines(top, 'wire_api'),
      approval_policy: tomlValueFromLines(top, 'approval_policy'),
      sandbox_mode: tomlValueFromLines(top, 'sandbox_mode'),
      model_reasoning_effort: tomlValueFromLines(top, 'model_reasoning_effort'),
      network_access: tomlValueFromLines(top, 'network_access'),
      model_context_window: tomlValueFromLines(top, 'model_context_window'),
      model_auto_compact_token_limit: tomlValueFromLines(top, 'model_auto_compact_token_limit'),
      personality: tomlValueFromLines(top, 'personality'),
      service_tier: tomlValueFromLines(top, 'service_tier')
    };
    for (const [key, val] of Object.entries(map)) {
      const inp = left.querySelector(`[data-key="${key}"]`);
      if (inp) inp.value = val || '';
    }
    left.querySelectorAll('.field-check').forEach(ch => {
      if (ch.dataset.check === 'requires_openai_auth') {
        ch.checked = tomlValueFromLines(providerLines, 'requires_openai_auth') === 'true';
      } else {
        ch.checked = tomlValueFromLines(top, ch.dataset.check) === 'true';
      }
    });
    extraConfigTA.value = stripManagedCodexToml(toml);
  }

  function syncFromAuth() {
    const auth = parseJSON(authTA.value) || {};
    const envKey = auth.env_key || '';
    const apiKey = envKey ? auth[envKey] : auth.OPENAI_API_KEY;
    const envKeyInput = left.querySelector('[data-key="env_key"]');
    const apiKeyInput = left.querySelector('[data-key="api_key"]');
    if (envKeyInput) envKeyInput.value = envKey;
    if (apiKeyInput) apiKeyInput.value = apiKey || '';
    extraAuthTA.value = fmtJSON(extractExtraAuth(auth, envKey));
  }

  left.querySelectorAll('input, select').forEach(i => i.addEventListener('input', syncToTOML));
  left.querySelectorAll('.field-check').forEach(ch => ch.addEventListener('change', syncToTOML));
  extraConfigTA.addEventListener('input', syncToTOML);
  extraAuthTA.addEventListener('input', syncToTOML);
  configTA.addEventListener('input', syncFromTOML);
  authTA.addEventListener('input', syncFromAuth);

  const toolbar = editToolbar(headerTitle,
    el('button', { className: 'btn primary', onClick: async () => {
      const auth = parseJSON(authTA.value);
      if (!auth) { Toast.error('auth.json JSON 无效'); return; }
      const payload = {
        id: p.id, name: p.name, website: p.website, notes: p.notes,
        config_toml: configTA.value, auth_json: auth
      };
      try {
        if (p.id) await API.put(`/ccswitch/api/codex/providers/${p.id}`, payload);
        else await API.post('/ccswitch/api/codex/providers', payload);
        Toast.success('保存成功');
        showPage('providers');
      } catch (err) {
        Toast.error('保存失败: ' + err.message);
      }
    }}, '保存'),
    el('button', { className: 'btn', onClick: () => showPage('providers') }, '取消'),
    el('button', { className: 'btn', onClick: async () => {
      const auth = parseJSON(authTA.value) || {};
      const envKey = auth.env_key || 'OPENAI_API_KEY';
      const apiKey = auth[envKey] || auth.OPENAI_API_KEY || '';
      const payload = p.id ? { provider_id: p.id } : { base_url: '', api_key: apiKey, model: '' };
      await runTestWithModal('/ccswitch/api/codex/test', payload);
    }}, '测试连接')
  );

  main.appendChild(toolbar);
  main.appendChild(el('div', { className: 'edit-page' }, left, right));
}

// Current Config Page
async function renderCurrent() {
  const main = document.getElementById('main');
  main.innerHTML = '';
  main.classList.add('loading');

  let claude, codex, claudeProviders, codexProviders;
  try {
    [claude, codex, claudeProviders, codexProviders] = await Promise.all([
      API.get('/ccswitch/api/current/claude'),
      API.get('/ccswitch/api/current/codex'),
      API.get('/ccswitch/api/claude/providers'),
      API.get('/ccswitch/api/codex/providers'),
    ]);
  } catch (err) {
    Toast.error('加载当前配置失败: ' + err.message);
    main.classList.remove('loading');
    return;
  } finally {
    main.classList.remove('loading');
  }

  const activeClaude = claudeProviders.providers.find(p => p.id === claudeProviders.active_id);
  const activeCodex = codexProviders.providers.find(p => p.id === codexProviders.active_id);

  const section = el('div', { className: 'current-page' });

  // Claude section
  section.appendChild(el('h2', {}, 'Claude Code 当前配置'));
  if (activeClaude) {
    section.appendChild(el('div', { className: 'active-banner' },
      el('span', { className: 'active-banner-label' }, '当前激活供应商:'),
      el('strong', {}, activeClaude.name),
      el('button', { className: 'btn btn-sm', onClick: () => editClaude(activeClaude) }, '去编辑')
    ));
  }
  const claudeSettingsTA = el('textarea', { className: 'config-editor', rows: 15 }, fmtJSON(claude.settings));
  const claudeJsonTA = el('textarea', { className: 'config-editor', rows: 10 }, fmtJSON(claude.claude_json));
  section.appendChild(el('h3', {}, CLI_PATHS.claudeSettings));
  section.appendChild(claudeSettingsTA);
  section.appendChild(el('h3', {}, CLI_PATHS.claudeJSON));
  section.appendChild(claudeJsonTA);
  section.appendChild(el('div', { className: 'actions' },
    el('button', { className: 'btn', onClick: () => {
      const s = parseJSON(claudeSettingsTA.value);
      if (s) claudeSettingsTA.value = fmtJSON(s);
      const c = parseJSON(claudeJsonTA.value);
      if (c) claudeJsonTA.value = fmtJSON(c);
    }}, '格式化'),
    el('button', { className: 'btn primary', onClick: async () => {
      const s = parseJSON(claudeSettingsTA.value);
      const c = parseJSON(claudeJsonTA.value);
      if (!s || !c) { Toast.error('JSON 无效'); return; }
      try {
        await API.put('/ccswitch/api/current/claude', { settings: s, claude_json: c });
        Toast.success('保存成功（已自动备份）');
      } catch (err) {
        Toast.error('保存失败: ' + err.message);
      }
    }}, '保存 Claude 配置'),
    el('button', { className: 'btn', onClick: async () => {
      const s = parseJSON(claudeSettingsTA.value);
      if (!s) { Toast.error('JSON 无效'); return; }
      const baseURL = (s.env && s.env.ANTHROPIC_BASE_URL) || '';
      const apiKey = (s.env && s.env.ANTHROPIC_AUTH_TOKEN) || '';
      const model = (s.env && s.env.ANTHROPIC_MODEL) || '';
      await runTestWithModal('/ccswitch/api/claude/test', { base_url: baseURL, api_key: apiKey, model });
    }}, '测试当前配置'),
    el('button', { className: 'btn danger', onClick: async () => {
      if (!confirm('确定将当前 Claude 配置写入 ~/.claude/settings.json 并备份原配置？')) return;
      const s = parseJSON(claudeSettingsTA.value);
      const c = parseJSON(claudeJsonTA.value);
      if (!s || !c) { Toast.error('JSON 无效'); return; }
      try {
        await API.put('/ccswitch/api/current/claude', { settings: s, claude_json: c });
        Toast.success('已应用当前配置（已自动备份）');
      } catch (err) {
        Toast.error('应用失败: ' + err.message);
      }
    }}, '应用当前配置'),
    el('button', { className: 'btn', onClick: () => {
      const name = prompt('保存为供应商名称:');
      if (!name) return;
      API.post('/ccswitch/api/current/save', { tool: 'claude', name })
        .then(() => Toast.success('已保存为供应商'))
        .catch(err => Toast.error('保存失败: ' + err.message));
    }}, '保存为 Claude 供应商')
  ));

  // Codex section
  section.appendChild(el('h2', {}, 'Codex CLI 当前配置'));
  if (activeCodex) {
    section.appendChild(el('div', { className: 'active-banner' },
      el('span', { className: 'active-banner-label' }, '当前激活供应商:'),
      el('strong', {}, activeCodex.name),
      el('button', { className: 'btn btn-sm', onClick: () => editCodex(activeCodex) }, '去编辑')
    ));
  }
  const codexConfigTA = el('textarea', { className: 'config-editor', rows: 15 }, codex.config || '');
  const codexAuthTA = el('textarea', { className: 'config-editor', rows: 10 }, fmtJSON(codex.auth));
  section.appendChild(el('h3', {}, CLI_PATHS.codexConfig));
  section.appendChild(codexConfigTA);
  section.appendChild(el('h3', {}, CLI_PATHS.codexAuth));
  section.appendChild(codexAuthTA);
  section.appendChild(el('div', { className: 'actions' },
    el('button', { className: 'btn', onClick: () => {
      codexConfigTA.value = codexConfigTA.value.trim();
      const a = parseJSON(codexAuthTA.value);
      if (a) codexAuthTA.value = fmtJSON(a);
    }}, '格式化'),
    el('button', { className: 'btn primary', onClick: async () => {
      const auth = parseJSON(codexAuthTA.value);
      if (!auth) { Toast.error('auth.json 无效'); return; }
      try {
        await API.put('/ccswitch/api/current/codex', { config: codexConfigTA.value, auth });
        Toast.success('保存成功（已自动备份）');
      } catch (err) {
        Toast.error('保存失败: ' + err.message);
      }
    }}, '保存 Codex 配置'),
    el('button', { className: 'btn', onClick: async () => {
      const auth = parseJSON(codexAuthTA.value) || {};
      const envKey = auth.env_key || 'OPENAI_API_KEY';
      const apiKey = auth[envKey] || auth.OPENAI_API_KEY || '';
      const toml = codexConfigTA.value;
      let baseUrl = '';
      let model = '';
      const m = toml.match(/model\s*=\s*"([^"]+)"/); if (m) model = m[1];
      let bu = toml.match(/^base_url\s*=\s*"([^"]+)"/m);
      if (bu) baseUrl = bu[1];
      const providerMatch = toml.match(/model_provider\s*=\s*"([^"]+)"/);
      const providerName = providerMatch ? providerMatch[1].replace(/[.*+?^${}()|[\]\\]/g, '\\$&') : 'OpenAI';
      const sectionMatch = toml.match(new RegExp(`\\[model_providers\\.${providerName}\\]([^\\[]*)`)) || toml.match(/\[model_providers\.[^\]]+\]([^\[]*)/);
      const mp = sectionMatch ? sectionMatch[1].match(/base_url\s*=\s*"([^"]+)"/) : null;
      if (mp) baseUrl = mp[1];
      await runTestWithModal('/ccswitch/api/codex/test', { base_url: baseUrl, api_key: apiKey, model });
    }}, '测试当前配置'),
    el('button', { className: 'btn danger', onClick: async () => {
      if (!confirm('确定将当前 Codex 配置写入 ~/.codex/ 并备份原配置？')) return;
      const auth = parseJSON(codexAuthTA.value);
      if (!auth) { Toast.error('auth.json 无效'); return; }
      try {
        await API.put('/ccswitch/api/current/codex', { config: codexConfigTA.value, auth });
        Toast.success('已应用当前配置（已自动备份）');
      } catch (err) {
        Toast.error('应用失败: ' + err.message);
      }
    }}, '应用当前配置'),
    el('button', { className: 'btn', onClick: () => {
      const name = prompt('保存为供应商名称:');
      if (!name) return;
      API.post('/ccswitch/api/current/save', { tool: 'codex', name })
        .then(() => Toast.success('已保存为供应商'))
        .catch(err => Toast.error('保存失败: ' + err.message));
    }}, '保存为 Codex 供应商')
  ));

  main.appendChild(section);
}

// Backups Page
async function renderBackups() {
  const main = document.getElementById('main');
  main.innerHTML = '';
  main.classList.add('loading');

  let claudeBackups, codexBackups;
  try {
    [claudeBackups, codexBackups] = await Promise.all([
      API.get('/ccswitch/api/backups?tool=claude'),
      API.get('/ccswitch/api/backups?tool=codex'),
    ]);
  } catch (err) {
    Toast.error('加载备份列表失败: ' + err.message);
    main.classList.remove('loading');
    return;
  } finally {
    main.classList.remove('loading');
  }

  const section = el('div', { className: 'backups-page' });

  section.appendChild(el('h2', {}, 'Claude Code 备份'));
  if (!claudeBackups.backups || claudeBackups.backups.length === 0) {
    section.appendChild(el('p', { className: 'empty' }, '无备份'));
  } else {
    section.appendChild(renderBackupList('claude', claudeBackups.backups));
  }

  section.appendChild(el('h2', {}, 'Codex CLI 备份'));
  if (!codexBackups.backups || codexBackups.backups.length === 0) {
    section.appendChild(el('p', { className: 'empty' }, '无备份'));
  } else {
    section.appendChild(renderBackupList('codex', codexBackups.backups));
  }

  main.appendChild(section);
}

function renderBackupList(tool, backups) {
  const container = el('div', {});
  if (backups.length > 0) {
    const latest = backups[0];
    container.appendChild(el('button', { className: 'btn primary', onClick: async () => {
      if (!confirm(`确定恢复 ${tool} 的最近一次备份？\n\n${formatBackupName(latest)}\n\n当前配置将被覆盖。`)) return;
      try {
        await API.post('/ccswitch/api/backups/restore', { tool, backup_name: latest });
        Toast.success('恢复成功');
      } catch (err) {
        Toast.error('恢复失败: ' + err.message);
      }
    }}, `恢复最近一次 ${tool} 备份`));
  }
  const ul = el('ul', { className: 'backup-list' });
  backups.forEach(b => {
    ul.appendChild(el('li', {},
      el('div', {},
        el('div', { className: 'backup-name' }, formatBackupName(b)),
        el('div', { className: 'backup-raw' }, b)
      ),
      el('button', { className: 'btn', onClick: async () => {
        if (!confirm(`恢复 ${tool} 备份？\n\n${formatBackupName(b)}\n\n当前配置将被覆盖。`)) return;
        try {
          await API.post('/ccswitch/api/backups/restore', { tool, backup_name: b });
          Toast.success('恢复成功');
        } catch (err) {
          Toast.error('恢复失败: ' + err.message);
        }
      }}, '恢复')
    ));
  });
  container.appendChild(ul);
  return container;
}

// Modal
function showModal(content) {
  const modal = document.getElementById('modal');
  const body = document.getElementById('modal-body');
  body.innerHTML = '';
  body.appendChild(content);
  modal.classList.remove('hidden');
}

document.addEventListener('click', e => {
  if (e.target.closest && e.target.closest('.close-btn')) document.getElementById('modal').classList.add('hidden');
  if (e.target.matches && e.target.matches('#modal')) document.getElementById('modal').classList.add('hidden');
});

// Global error handling for async operations
window.addEventListener('unhandledrejection', e => {
  console.error('Unhandled error:', e.reason);
  Toast.error('操作失败: ' + (e.reason?.message || e.reason || '未知错误'));
});

// Init
showPage('providers');
