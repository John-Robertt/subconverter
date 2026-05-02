import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const nginxConfig = readFileSync("nginx.conf", "utf8");

describe("nginx proxy config", () => {
  it("preserves the browser Host header including port", () => {
    const hostHeaders = nginxConfig.match(/proxy_set_header Host \$http_host;/g) ?? [];

    expect(hostHeaders).toHaveLength(3);
    expect(nginxConfig).not.toContain("proxy_set_header Host $host;");
  });
});
