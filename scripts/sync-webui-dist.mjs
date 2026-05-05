import { access, cp, mkdir, rm } from "node:fs/promises";

const root = new URL("../", import.meta.url);
const source = new URL("web/dist/", root);
const target = new URL("internal/webui/dist/", root);

await access(new URL("index.html", source));
await rm(target, { recursive: true, force: true });
await mkdir(target, { recursive: true });
await cp(source, target, { recursive: true });

console.log("Synced web/dist to internal/webui/dist");
