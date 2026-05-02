import type { Config, FetchSourceKind, SourcesConfig } from "../api/types";

export const fetchSourceKinds: FetchSourceKind[] = ["subscriptions", "snell", "vless"];

export function cloneConfig(config: Config): Config {
  return JSON.parse(JSON.stringify(config)) as Config;
}

export function ensureConfig(config: Config | undefined): Config {
  const next = cloneConfig(config ?? {});
  next.sources = ensureSources(next.sources);
  next.filters = next.filters ?? {};
  next.groups = next.groups ?? [];
  next.routing = next.routing ?? [];
  next.rulesets = next.rulesets ?? [];
  next.rules = next.rules ?? [];
  next.templates = next.templates ?? {};
  return next;
}

export function ensureSources(sources: Config["sources"]): SourcesConfig {
  return {
    subscriptions: sources?.subscriptions ?? [],
    snell: sources?.snell ?? [],
    vless: sources?.vless ?? [],
    custom_proxies: sources?.custom_proxies ?? [],
    fetch_order: normalizeFetchOrder(sources?.fetch_order)
  };
}

export function normalizeFetchOrder(value: unknown): FetchSourceKind[] {
  if (!Array.isArray(value)) {
    return [...fetchSourceKinds];
  }

  const order = value.filter((item): item is FetchSourceKind => fetchSourceKinds.includes(item as FetchSourceKind));
  if (new Set(order).size !== fetchSourceKinds.length) {
    return [...fetchSourceKinds];
  }
  return order;
}

export function maskUrl(url: string): string {
  if (!url) return "";

  try {
    const parsed = new URL(url);
    for (const key of ["token", "access_token", "password", "passwd", "key"]) {
      if (parsed.searchParams.has(key)) {
        parsed.searchParams.set(key, "...");
      }
    }
    if (parsed.username) parsed.username = "...";
    if (parsed.password) parsed.password = "...";
    return parsed.toString();
  } catch {
    return url.length > 96 ? `${url.slice(0, 40)}...${url.slice(-28)}` : url;
  }
}

export function moveItem<T>(items: T[], from: number, to: number): T[] {
  const next = [...items];
  const [item] = next.splice(from, 1);
  if (item === undefined) return items;
  next.splice(to, 0, item);
  return next;
}

export function getRoutingMemberOptions(config: Config): string[] {
  const groupNames = config.groups?.map((entry) => entry.key) ?? [];
  const routingNames = config.routing?.map((entry) => entry.key) ?? [];
  return Array.from(new Set([...groupNames, ...routingNames, "DIRECT", "REJECT", "@all", "@auto"]));
}

export function getPolicyOptions(config: Config): string[] {
  const routingNames = config.routing?.map((entry) => entry.key).filter(Boolean) ?? [];
  return Array.from(new Set([...routingNames, "DIRECT", "REJECT"]));
}

export function splitRulePolicy(rule: string): { body: string; policy: string; parseable: boolean } {
  const index = rule.lastIndexOf(",");
  if (index < 0) {
    return { body: rule, policy: "", parseable: false };
  }
  return {
    body: rule.slice(0, index),
    policy: rule.slice(index + 1).trim(),
    parseable: true
  };
}

export function replaceRulePolicy(rule: string, policy: string): string {
  const parsed = splitRulePolicy(rule);
  if (!parsed.parseable) {
    return rule;
  }
  return `${parsed.body},${policy}`;
}

export function isConfigChanged(a: Config | undefined, b: Config | undefined): boolean {
  return JSON.stringify(a ?? {}) !== JSON.stringify(b ?? {});
}
