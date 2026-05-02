export interface OrderedEntry<TValue> {
  key: string;
  value: TValue;
}

export type ProxyStrategy = "select" | "url-test";
export type FetchSourceKind = "subscriptions" | "snell" | "vless";
export type GenerateFormat = "clash" | "surge";

export interface FetchSource {
  url: string;
}

export interface RelayThrough {
  type: "group" | "select" | "all";
  strategy: ProxyStrategy;
  name?: string;
  match?: string;
}

export interface CustomProxy {
  name: string;
  url: string;
  relay_through?: RelayThrough;
}

export interface SourcesConfig {
  subscriptions: FetchSource[];
  snell: FetchSource[];
  vless: FetchSource[];
  custom_proxies: CustomProxy[];
  fetch_order: FetchSourceKind[];
}

export interface FiltersConfig {
  exclude?: string;
}

export interface GroupConfig {
  match: string;
  strategy: ProxyStrategy;
}

export interface TemplatesConfig {
  clash?: string;
  surge?: string;
}

export interface Config {
  base_url?: string;
  sources?: Partial<SourcesConfig>;
  filters?: FiltersConfig;
  groups?: OrderedEntry<GroupConfig>[];
  routing?: OrderedEntry<string[]>[];
  rulesets?: OrderedEntry<string[]>[];
  rules?: string[];
  fallback?: string;
  templates?: TemplatesConfig;
}

export interface ConfigSnapshot {
  config_revision: string;
  config: Config;
}

export interface AuthStatus {
  authed: boolean;
  setup_required: boolean;
  setup_token_required: boolean;
  locked_until: string;
}

export interface LoginRequest {
  username: string;
  password: string;
  remember: boolean;
}

export interface LoginResponse {
  redirect?: string;
}

export interface SetupRequest {
  username: string;
  password: string;
  setup_token: string;
}

export interface ConfigSourceStatus {
  location: string;
  type: "local" | "remote" | string;
  writable: boolean;
}

export interface LastReloadStatus {
  time: string;
  success: boolean;
  duration_ms: number;
  error?: string;
}

export interface StatusResponse {
  version: string;
  commit?: string;
  build_date?: string;
  config_source: ConfigSourceStatus;
  config_revision: string;
  runtime_config_revision: string;
  config_loaded_at?: string;
  config_dirty: boolean;
  capabilities: {
    config_write: boolean;
    reload: boolean;
  };
  last_reload?: LastReloadStatus;
}

export interface ReloadResult {
  success: boolean;
  duration_ms: number;
}

export interface DiagnosticLocator {
  section?: string;
  index?: number;
  key?: string;
  value_path?: string;
  json_pointer?: string;
}

export interface Diagnostic {
  severity: "error" | "warning" | "info" | string;
  code: string;
  message: string;
  display_path?: string;
  locator?: DiagnosticLocator;
}

export interface ValidateResult {
  valid: boolean;
  errors: Diagnostic[];
  warnings: Diagnostic[];
  infos: Diagnostic[];
}

export interface NodePreview {
  name: string;
  type: string;
  kind: string;
  server?: string;
  port?: number;
  filtered: boolean;
}

export interface NodePreviewResponse {
  nodes: NodePreview[];
  total: number;
  active_count: number;
  filtered_count: number;
}

export interface ExpandedMember {
  value: string;
  origin: "literal" | "auto_expanded" | "all_expanded" | string;
}

export interface PreviewGroup {
  name: string;
  strategy: string;
  members: string[];
  expanded_members?: ExpandedMember[];
}

export interface GroupPreviewResponse {
  node_groups: PreviewGroup[];
  chained_groups: PreviewGroup[];
  service_groups: PreviewGroup[];
  all_proxies: string[];
}

export interface GenerateLinkResponse {
  url: string;
  token_included: boolean;
}

export interface ApiErrorPayload {
  code?: string;
  message?: string;
  details?: unknown;
  remaining?: number;
  until?: string;
  current_config_revision?: string;
}
