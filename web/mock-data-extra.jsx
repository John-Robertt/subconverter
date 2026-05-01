// More mock data for the new screens.

const VALIDATION_ERRORS = [
  {
    id: 'e1', severity: 'error', field: 'sources.ss[2].url',
    message: '订阅地址无法连接：DNS 解析失败',
    detail: 'GET https://nodes.example.org/sub/user_42/clash → ENOTFOUND',
    page: 'sources',
  },
  {
    id: 'e2', severity: 'error', field: 'groups[2].regex',
    message: '正则表达式语法错误',
    detail: 'Unterminated character class at position 12',
    page: 'groups',
  },
  {
    id: 'e3', severity: 'error', field: 'routing[0].members',
    message: '@all 与 @auto 不能同时出现在一个服务组中',
    detail: '🌐 全球代理 同时引用了 @all 和 @auto',
    page: 'routing',
  },
  {
    id: 'w1', severity: 'warning', field: 'rulesets.r3',
    message: '规则集 URL 响应缓慢',
    detail: 'streaming.list 加载耗时 4.2s（阈值 2s）',
    page: 'rulesets',
  },
  {
    id: 'w2', severity: 'warning', field: 'groups[4]',
    message: '分组未匹配到任何节点',
    detail: '🇹🇼 台湾 当前匹配 0 个节点',
    page: 'groups',
  },
  {
    id: 'i1', severity: 'info', field: 'fallback',
    message: 'fallback 未设置',
    detail: '建议指定一个服务组作为最终兜底',
    page: 'other',
  },
];

const RELOAD_HISTORY = [
  { time: '14:21:47', status: 'success', source: 'webhook', changes: '+2 节点 · groups[2].regex 修改', dur: '142ms' },
  { time: '13:55:12', status: 'success', source: 'manual', changes: '新增 SS 订阅 ×1', dur: '189ms' },
  { time: '12:30:08', status: 'failed', source: 'auto', changes: '上游订阅超时', dur: '5.0s' },
  { time: '11:18:43', status: 'success', source: 'auto', changes: '定时刷新', dur: '108ms' },
  { time: '09:02:11', status: 'success', source: 'webhook', changes: 'rulesets.r1 URL 变更', dur: '203ms' },
  { time: '昨天 22:14', status: 'success', source: 'manual', changes: '路由策略重排序', dur: '94ms' },
  { time: '昨天 18:30', status: 'success', source: 'auto', changes: '定时刷新', dur: '112ms' },
];

const INLINE_RULES = [
  { type: 'DOMAIN-SUFFIX', match: 'cn', target: 'DIRECT' },
  { type: 'DOMAIN-KEYWORD', match: 'github', target: '🌐 全球代理' },
  { type: 'DOMAIN', match: 'localhost', target: 'DIRECT' },
  { type: 'IP-CIDR', match: '10.0.0.0/8', target: 'DIRECT', noResolve: true },
  { type: 'IP-CIDR', match: '172.16.0.0/12', target: 'DIRECT', noResolve: true },
  { type: 'IP-CIDR', match: '192.168.0.0/16', target: 'DIRECT', noResolve: true },
  { type: 'PROCESS-NAME', match: 'WeChat', target: 'DIRECT' },
  { type: 'PROCESS-NAME', match: 'Telegram', target: '🌐 全球代理' },
  { type: 'DOMAIN-SUFFIX', match: 'openai.com', target: '🤖 AI 服务' },
  { type: 'DOMAIN-SUFFIX', match: 'anthropic.com', target: '🤖 AI 服务' },
  { type: 'GEOIP', match: 'CN', target: 'DIRECT' },
  { type: 'MATCH', match: '', target: '🌐 全球代理' },
];

Object.assign(window, { VALIDATION_ERRORS, RELOAD_HISTORY, INLINE_RULES });
