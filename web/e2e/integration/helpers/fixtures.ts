// Test fixtures: mirror configs/base_config.yaml semantics in JSON form, with
// curated values that exercise A1-A8 + B1-B3 page behavior.
// Values intentionally mix ASCII (GRP_*) and emoji (🌐 全球代理) keys to
// surface YAML-encoding edge cases similar to production configs.

import type {
  AuthStatus,
  Config,
  ConfigSnapshot,
  GroupPreviewResponse,
  NodePreviewResponse,
  StatusResponse,
  ValidateResult
} from "../../../src/api/types";

export const REVISION_BASE = "sha256:base000000000000000000000000000000000000000000000000000000000000";
export const REVISION_AFTER_SAVE = "sha256:next1111111111111111111111111111111111111111111111111111111111";
export const REVISION_DRIFT = "sha256:drift22222222222222222222222222222222222222222222222222222222222";

export const fixtureConfig: Config = {
  base_url: "https://sub.example.com",
  templates: {
    clash: "configs/base_clash.yaml",
    surge: "configs/base_surge.conf"
  },
  sources: {
    subscriptions: [
      { url: "https://sub.example.com/api/v1/sub?token=upstream-secret-1" },
      { url: "https://provider-b.net/link/abc123def456" }
    ],
    snell: [{ url: "https://snell-pool.example.com/list.txt" }],
    vless: [{ url: "https://reality.example.io/sub/vless-pool" }],
    custom_proxies: [
      { name: "Home Relay", url: "ss://chacha20-ietf-poly1305:secret@home.example.com:8388" }
    ],
    fetch_order: ["subscriptions", "snell", "vless"]
  },
  filters: { exclude: "剩余|流量|官网|Expire" },
  groups: [
    { key: "GRP_HK", value: { match: "(港|HK)", strategy: "url-test" } },
    { key: "GRP_JP", value: { match: "(日本|JP|东京)", strategy: "url-test" } },
    { key: "GRP_SG", value: { match: "(新加坡|SG)", strategy: "select" } }
  ],
  routing: [
    { key: "SVC_PROXY", value: ["@auto", "@all", "GRP_HK", "GRP_JP"] },
    { key: "SVC_DIRECT", value: ["DIRECT"] }
  ],
  rulesets: [
    { key: "SVC_PROXY", value: ["https://ruleset.example.com/proxy.list"] }
  ],
  rules: [
    "DOMAIN-SUFFIX,cn,DIRECT",
    "DOMAIN-KEYWORD,github,SVC_PROXY",
    "GEOIP,CN,DIRECT",
    "MATCH,SVC_PROXY"
  ],
  fallback: "SVC_PROXY"
};

export function configSnapshot(config: Config = fixtureConfig, revision = REVISION_BASE): ConfigSnapshot {
  return { config_revision: revision, config };
}

export const fixtureStatus: StatusResponse = {
  version: "2.0.0-test",
  commit: "test-commit",
  build_date: "2026-05-04",
  config_source: { location: "/etc/subconverter/config.yaml", type: "local", writable: true },
  config_revision: REVISION_BASE,
  runtime_config_revision: REVISION_BASE,
  config_loaded_at: "2026-05-04T08:00:00Z",
  config_dirty: false,
  capabilities: { config_write: true, reload: true },
  last_reload: { time: "2026-05-04T07:58:00Z", success: true, duration_ms: 142 },
  runtime_environment: {
    listen_addr: "0.0.0.0:8080",
    working_dir: "/var/lib/subconverter",
    go_runtime: "go1.26.2 darwin/arm64",
    memory_alloc_mb: "38.4",
    request_count_24h: 12847,
    uptime_seconds: 3600 * 6 + 14 * 60
  }
};

export const readonlyStatus: StatusResponse = {
  ...fixtureStatus,
  config_source: { location: "https://config.example.com/config.yaml", type: "remote", writable: false },
  capabilities: { config_write: false, reload: true }
};

export const dirtyStatus: StatusResponse = {
  ...fixtureStatus,
  config_revision: REVISION_AFTER_SAVE,
  runtime_config_revision: REVISION_BASE,
  config_dirty: true
};

export const fixtureAuthAuthed: AuthStatus = {
  authed: true,
  setup_required: false,
  setup_token_required: false,
  locked_until: ""
};

export const fixtureAuthLogout: AuthStatus = {
  authed: false,
  setup_required: false,
  setup_token_required: false,
  locked_until: ""
};

export const fixtureAuthSetup: AuthStatus = {
  authed: false,
  setup_required: true,
  setup_token_required: true,
  locked_until: ""
};

export const fixtureAuthLocked: AuthStatus = {
  authed: false,
  setup_required: false,
  setup_token_required: false,
  locked_until: "2099-01-01T00:00:00Z"
};

export const fixtureNodes: NodePreviewResponse = {
  nodes: [
    { name: "🇭🇰 香港 IEPL 01", type: "ss", kind: "subscription", server: "hk01.example.com", port: 443, filtered: false },
    { name: "🇭🇰 香港 IEPL 02", type: "ss", kind: "subscription", server: "hk02.example.com", port: 443, filtered: false },
    { name: "🇯🇵 大阪 BGP 01", type: "ss", kind: "subscription", server: "jp01.example.com", port: 443, filtered: false },
    { name: "🇯🇵 东京 IEPL 02", type: "ss", kind: "subscription", server: "jp02.example.com", port: 443, filtered: false },
    { name: "🇸🇬 新加坡 01", type: "ss", kind: "subscription", server: "sg01.example.com", port: 443, filtered: false },
    { name: "🏳️ 测试·流量信息", type: "ss", kind: "subscription", server: "info.example.com", port: 80, filtered: true },
    { name: "🇭🇰 Snell HK Premium", type: "snell", kind: "snell", server: "hk-snell.example.com", port: 6160, filtered: false },
    { name: "🇩🇪 法兰克福 Reality", type: "vless", kind: "vless", server: "fra.example.io", port: 443, filtered: false },
    { name: "Home Relay", type: "ss", kind: "custom", server: "home.example.com", port: 8388, filtered: false }
  ],
  total: 9,
  active_count: 8,
  filtered_count: 1
};

export const fixtureGroups: GroupPreviewResponse = {
  node_groups: [
    {
      name: "GRP_HK",
      strategy: "url-test",
      match: "(港|HK)",
      members: ["🇭🇰 香港 IEPL 01", "🇭🇰 香港 IEPL 02", "🇭🇰 Snell HK Premium"]
    },
    {
      name: "GRP_JP",
      strategy: "url-test",
      match: "(日本|JP|东京)",
      members: ["🇯🇵 大阪 BGP 01", "🇯🇵 东京 IEPL 02"]
    },
    {
      name: "GRP_SG",
      strategy: "select",
      match: "(新加坡|SG)",
      members: ["🇸🇬 新加坡 01"]
    }
  ],
  chained_groups: [],
  service_groups: [
    {
      name: "SVC_PROXY",
      strategy: "select",
      members: ["@auto", "@all", "GRP_HK", "GRP_JP"],
      expanded_members: [
        { value: "GRP_HK", origin: "auto_expanded" },
        { value: "GRP_JP", origin: "auto_expanded" },
        { value: "🇭🇰 香港 IEPL 01", origin: "all_expanded" },
        { value: "🇯🇵 东京 IEPL 02", origin: "all_expanded" }
      ]
    },
    {
      name: "SVC_DIRECT",
      strategy: "select",
      members: ["DIRECT"],
      expanded_members: [{ value: "DIRECT", origin: "literal" }]
    }
  ],
  all_proxies: [
    "🇭🇰 香港 IEPL 01",
    "🇭🇰 香港 IEPL 02",
    "🇯🇵 大阪 BGP 01",
    "🇯🇵 东京 IEPL 02",
    "🇸🇬 新加坡 01",
    "🇭🇰 Snell HK Premium",
    "🇩🇪 法兰克福 Reality",
    "Home Relay"
  ]
};

export const validateOk: ValidateResult = {
  valid: true,
  errors: [],
  warnings: [],
  infos: []
};

export const validateWithErrors: ValidateResult = {
  valid: false,
  errors: [
    {
      severity: "error",
      code: "config.regex.invalid",
      message: "正则表达式语法错误：unterminated character class",
      display_path: "groups.GRP_JP.match",
      locator: { section: "groups", index: 1, value_path: "match", json_pointer: "/config/groups/1/value/match" }
    }
  ],
  warnings: [
    {
      severity: "warning",
      code: "config.group.no_match",
      message: "分组未匹配到任何节点",
      display_path: "groups.GRP_SG",
      locator: { section: "groups", index: 2, json_pointer: "/config/groups/2" }
    }
  ],
  infos: [
    {
      severity: "info",
      code: "config.fallback.unset",
      message: "fallback 未设置",
      display_path: "fallback",
      locator: { section: "fallback", json_pointer: "/config/fallback" }
    }
  ]
};

export const CLASH_PREVIEW = `# Generated by subconverter
mixed-port: 7890
allow-lan: true
mode: rule
proxies:
  - name: "🇭🇰 香港 IEPL 01"
    type: ss
    server: hk01.example.com
    port: 443
proxy-groups:
  - name: GRP_HK
    type: url-test
    proxies: ["🇭🇰 香港 IEPL 01", "🇭🇰 香港 IEPL 02"]
rules:
  - GEOIP,CN,DIRECT
  - MATCH,SVC_PROXY
`;

export const SURGE_PREVIEW = `# Generated by subconverter
[General]
loglevel = notify
[Proxy]
🇭🇰 香港 IEPL 01 = ss, hk01.example.com, 443
[Proxy Group]
GRP_HK = url-test, 🇭🇰 香港 IEPL 01, 🇭🇰 香港 IEPL 02
[Rule]
GEOIP,CN,DIRECT
FINAL,SVC_PROXY
`;

export const SUBSCRIPTION_LINK_WITH_TOKEN = "https://sub.example.com/generate?format=clash&token=server-token-123";
export const SUBSCRIPTION_LINK_NO_TOKEN = "https://sub.example.com/generate?format=clash";
