// Lightweight mock backend used for visual fidelity check during dev.
// Run via: node scripts/mock-backend.mjs (port 8080)
import { createServer } from "node:http";

const config = {
  base_url: "https://sub.example.com",
  sources: {
    subscriptions: [
      { url: "https://sub.example.com/api/v1/sub?token=••••a83f" },
      { url: "https://provider-b.net/link/abc123def456" },
      { url: "https://nodes.example.org/sub/user_42/clash" }
    ],
    snell: [{ url: "https://snell-pool.example.com/list.txt" }],
    vless: [
      { url: "https://reality.example.io/sub/vless-pool" },
      { url: "https://xray.example.net/v/personal" }
    ],
    custom_proxies: [
      { name: "Home Relay", url: "ss://chacha20-ietf-poly1305:secret@home.example.com:8388" },
      { name: "Office SOCKS", url: "socks5://10.0.1.5:1080" }
    ],
    fetch_order: ["subscriptions", "snell", "vless"]
  },
  filters: { exclude: "剩余|流量|官网|Expire|套餐到期" },
  groups: [
    { key: "🇭🇰 香港", value: { match: "香港|HK|🇭🇰", strategy: "url-test" } },
    { key: "🇯🇵 日本", value: { match: "日本|东京|大阪|JP|🇯🇵", strategy: "url-test" } },
    { key: "🇸🇬 新加坡", value: { match: "新加坡|SG|🇸🇬", strategy: "select" } },
    { key: "🇺🇸 美国", value: { match: "美国|US|洛杉矶|🇺🇸", strategy: "url-test" } },
    { key: "🇹🇼 台湾", value: { match: "台湾|台北|TW|🇹🇼", strategy: "select" } },
    { key: "🇪🇺 欧洲", value: { match: "德国|英国|法兰克福|伦敦", strategy: "url-test" } }
  ],
  routing: [
    { key: "🌐 全球代理", value: ["@auto", "@all", "🇭🇰 香港", "🇯🇵 日本", "🇸🇬 新加坡", "🇺🇸 美国"] },
    { key: "🍎 苹果服务", value: ["DIRECT", "🇸🇬 新加坡", "🇺🇸 美国"] },
    { key: "📺 流媒体", value: ["🇭🇰 香港", "🇯🇵 日本", "🇪🇺 欧洲", "🇺🇸 美国"] },
    { key: "🤖 AI 服务", value: ["🇺🇸 美国", "🇪🇺 欧洲", "🌐 全球代理"] },
    { key: "🛑 广告拦截", value: ["REJECT", "DIRECT"] }
  ],
  rulesets: [
    { key: "🌐 全球代理", value: ["https://ruleset.example.com/proxy.list", "https://ruleset.example.com/gfw.list"] },
    { key: "🍎 苹果服务", value: ["https://ruleset.example.com/apple.list", "https://ruleset.example.com/icloud.list"] },
    { key: "📺 流媒体", value: ["https://ruleset.example.com/streaming.list", "https://ruleset.example.com/youtube.list"] },
    { key: "🤖 AI 服务", value: ["https://ruleset.example.com/openai.list", "https://ruleset.example.com/anthropic.list"] }
  ],
  rules: [
    "DOMAIN-SUFFIX,cn,DIRECT",
    "DOMAIN-KEYWORD,github,🌐 全球代理",
    "DOMAIN,localhost,DIRECT",
    "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
    "IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
    "IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
    "PROCESS-NAME,WeChat,DIRECT",
    "PROCESS-NAME,Telegram,🌐 全球代理",
    "DOMAIN-SUFFIX,openai.com,🤖 AI 服务",
    "DOMAIN-SUFFIX,anthropic.com,🤖 AI 服务",
    "GEOIP,CN,DIRECT",
    "MATCH,🌐 全球代理"
  ],
  fallback: "🌐 全球代理",
  templates: { clash: "./templates/clash.yaml", surge: "./templates/surge.conf" }
};

const status = {
  version: "0.9.4",
  commit: "7a3f2d1",
  build_date: "2026-04-22",
  config_source: { location: "/etc/subconverter/config.yaml", type: "local", writable: true },
  config_revision: "rev-1",
  runtime_config_revision: "rev-1",
  config_loaded_at: new Date().toISOString(),
  config_dirty: false,
  capabilities: { config_write: true, reload: true },
  last_reload: { time: new Date(Date.now() - 2 * 60 * 1000).toISOString(), success: true, duration_ms: 142 }
};

const NODES = [
  { name: "🇭🇰 香港 IEPL 01", type: "ss", kind: "subscription", server: "hk01.example.com", port: 443, filtered: false },
  { name: "🇭🇰 香港 IEPL 02", type: "ss", kind: "subscription", server: "hk02.example.com", port: 443, filtered: false },
  { name: "🇭🇰 香港 BGP 03", type: "ss", kind: "subscription", server: "hk03.example.com", port: 8443, filtered: false },
  { name: "🇯🇵 大阪 BGP 01", type: "ss", kind: "subscription", server: "jp01.example.com", port: 443, filtered: false },
  { name: "🇯🇵 东京 IEPL 02", type: "ss", kind: "subscription", server: "jp02.example.com", port: 443, filtered: false },
  { name: "🇸🇬 新加坡 01", type: "ss", kind: "subscription", server: "sg01.example.com", port: 443, filtered: false },
  { name: "🇺🇸 洛杉矶 IEPL 01", type: "ss", kind: "subscription", server: "la01.example.com", port: 443, filtered: false },
  { name: "🏳️ 测试·流量信息", type: "ss", kind: "subscription", server: "info.example.com", port: 80, filtered: true },
  { name: "🇭🇰 Snell HK Premium", type: "snell", kind: "snell", server: "hk-snell.example.com", port: 6160, filtered: false },
  { name: "🇩🇪 法兰克福 Reality", type: "vless", kind: "vless", server: "fra.example.io", port: 443, filtered: false },
  { name: "Home Relay", type: "ss", kind: "custom", server: "home.example.com", port: 8388, filtered: false }
];

function send(res, status, body, headers = {}) {
  res.writeHead(status, { "Content-Type": "application/json", "Access-Control-Allow-Origin": "*", ...headers });
  res.end(JSON.stringify(body));
}

function sendText(res, status, text, headers = {}) {
  res.writeHead(status, { "Content-Type": "text/plain", "Access-Control-Allow-Origin": "*", ...headers });
  res.end(text);
}

const CLASH_PREVIEW = `# Generated by subconverter\nmixed-port: 7890\nallow-lan: true\nmode: rule\nlog-level: info\n\nproxies:\n  - name: "🇭🇰 香港 IEPL 01"\n    type: ss\n    server: hk01.example.com\n    port: 443\n\nproxy-groups:\n  - name: "🌐 全球代理"\n    type: select\n    proxies: ["🇭🇰 香港", "🇯🇵 日本"]\n\nrules:\n  - GEOIP,CN,DIRECT\n  - MATCH,🌐 全球代理\n`;
const SURGE_PREVIEW = `# Generated by subconverter\n[General]\nloglevel = notify\n\n[Proxy]\n🇭🇰 香港 IEPL 01 = ss, hk01.example.com, 443\n\n[Proxy Group]\n🌐 全球代理 = select, 🇭🇰 香港, 🇯🇵 日本\n\n[Rule]\nGEOIP,CN,DIRECT\nFINAL,🌐 全球代理\n`;

const server = createServer(async (req, res) => {
  const { method, url } = req;
  if (method === "OPTIONS") {
    res.writeHead(204, { "Access-Control-Allow-Origin": "*", "Access-Control-Allow-Methods": "*", "Access-Control-Allow-Headers": "*" });
    res.end();
    return;
  }
  const u = new URL(url, "http://localhost");
  const path = u.pathname;
  if (path === "/healthz") return sendText(res, 200, "ok");
  if (path === "/api/auth/status") {
    const envMode = process.env.MOCK_AUTH_MODE;
    if (envMode === "setup") return send(res, 200, { authed: false, setup_required: true, setup_token_required: true, locked_until: "" });
    if (envMode === "locked") return send(res, 200, { authed: false, setup_required: false, setup_token_required: false, locked_until: "14:32" });
    if (envMode === "logout") return send(res, 200, { authed: false, setup_required: false, setup_token_required: false, locked_until: "" });
    return send(res, 200, { authed: true, setup_required: false, setup_token_required: false, locked_until: "" });
  }
  if (path === "/api/auth/login") return send(res, 200, { redirect: "/sources" });
  if (path === "/api/auth/logout") return send(res, 200, {});
  if (path === "/api/status") return send(res, 200, status);
  if (path === "/api/config" && method === "GET") return send(res, 200, { config_revision: status.config_revision, config });
  if (path === "/api/config" && method === "PUT") return send(res, 200, { config_revision: status.config_revision });
  if (path === "/api/reload") return send(res, 200, { success: true, duration_ms: 142 });
  if (path === "/api/config/validate") {
    if (process.env.MOCK_VALIDATE === "errors") {
      return send(res, 200, {
        valid: false,
        errors: [
          { severity: "error", code: "config.regex.invalid", message: "正则表达式语法错误", display_path: "groups[2].regex", locator: { json_pointer: "/config/groups/2/value/match", section: "groups", index: 2 } }
        ],
        warnings: [
          { severity: "warning", code: "config.group.no_match", message: "分组未匹配到任何节点", display_path: "groups[4]", locator: { json_pointer: "/config/groups/4", section: "groups", index: 4 } }
        ],
        infos: [
          { severity: "info", code: "config.fallback.unset", message: "fallback 未设置", display_path: "fallback", locator: { json_pointer: "/config/fallback", section: "fallback" } }
        ]
      });
    }
    return send(res, 200, { valid: true, errors: [], warnings: [], infos: [] });
  }
  if (path === "/api/preview/nodes") {
    return send(res, 200, {
      nodes: NODES,
      total: NODES.length,
      active_count: NODES.filter(n => !n.filtered).length,
      filtered_count: NODES.filter(n => n.filtered).length
    });
  }
  if (path === "/api/preview/groups") {
    const node_groups = config.groups.map(g => ({
      name: g.key,
      strategy: g.value.strategy,
      match: g.value.match,
      members: NODES.filter(n => { try { return new RegExp(g.value.match).test(n.name); } catch { return false; } }).map(n => n.name)
    }));
    const service_groups = config.routing.map(r => ({ name: r.key, strategy: "select", members: r.value }));
    return send(res, 200, { node_groups, chained_groups: [], service_groups, all_proxies: NODES.map(n => n.name) });
  }
  if (path === "/api/generate/preview") {
    const fmt = u.searchParams.get("format");
    return send(res, 200, { content: fmt === "surge" ? SURGE_PREVIEW : CLASH_PREVIEW, format: fmt });
  }
  if (path === "/generate" || path === "/api/generate/preview") {
    const fmt = u.searchParams.get("format");
    return sendText(res, 200, fmt === "surge" ? SURGE_PREVIEW : CLASH_PREVIEW);
  }
  if (path === "/api/generate/link") {
    const fmt = u.searchParams.get("format");
    return send(res, 200, { url: `${config.base_url}/generate?format=${fmt}&token=••••`, token_included: true });
  }
  return send(res, 404, { code: "not_found", message: path });
});

server.listen(8080, () => console.log("mock backend on :8080"));
