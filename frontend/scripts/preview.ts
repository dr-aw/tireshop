// Strict artifact server for QA: no SPA fallback, no production runtime dependency.
import { resolve, sep } from "node:path";
const root = resolve(import.meta.dir, "../dist");
Bun.serve({
  port: Number(process.env.PORT || 5175),
  hostname: "127.0.0.1",
  async fetch(req) {
    let path: string;
    try {
      path = decodeURIComponent(new URL(req.url).pathname);
    } catch {
      return new Response("Bad request", { status: 400 });
    }
    if (path.startsWith("/api/") || path.startsWith("/media/"))
      return new Response("Not found", { status: 404 });
    const base = resolve(root, "." + path);
    if (!base.startsWith(root + sep) && base !== root)
      return new Response("Forbidden", { status: 403 });
    for (const candidate of [
      base.endsWith(sep) ? base + "index.html" : base,
      base + ".html",
      resolve(base, "index.html"),
    ]) {
      const f = Bun.file(candidate);
      if (await f.exists()) {
        try {
          if ((await f.stat()).isFile()) return new Response(f);
        } catch {}
      }
    }
    return new Response(Bun.file(resolve(root, "404.html")), {
      status: 404,
      headers: { "Content-Type": "text/html; charset=utf-8" },
    });
  },
});
console.log("SSG preview on http://127.0.0.1:" + (process.env.PORT || 5175));
