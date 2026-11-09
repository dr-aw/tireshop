import { renderToString } from "react-dom/server";
import { App } from "../src/store";
import { Information } from "../src/information";
import { emptyDetails } from "../src/catalog-details";
import {
  categoryPath,
  jsonScript,
  productSchema,
  type PageData,
  type Product,
} from "../src/page-data";
import { mkdir, readFile, writeFile, rm, rename } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const target = resolve(root, "dist-next");
const origin = process.env.SITE_ORIGIN || "http://localhost:5173";
const url = new URL(origin);
if (url.pathname !== "/" || !["http:", "https:"].includes(url.protocol))
  throw Error("SITE_ORIGIN must be an HTTP(S) origin");
const api = process.env.SSG_CATALOG_URL;
const file = process.env.SSG_SNAPSHOT_FILE;
if (api && file) throw Error("Choose only one snapshot source");
if (process.env.RENDER && (!api || !process.env.SITE_ORIGIN))
  throw Error("Render requires SSG_CATALOG_URL and SITE_ORIGIN");
async function snapshot() {
  if (file) return JSON.parse(await readFile(file, "utf8"));
  if (api) {
    const response = await fetch(api, {
      signal: AbortSignal.timeout(20000),
      redirect: "error",
    });
    if (!response.ok) throw Error(`Snapshot HTTP ${response.status}`);
    const reader = response.body!.getReader();
    const chunks: Uint8Array[] = [];
    let length = 0;
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      length += value.length;
      if (length > 32 * 1024 * 1024) {
        await reader.cancel();
        throw Error("Snapshot exceeds 32 MiB");
      }
      chunks.push(value);
    }
    return JSON.parse(Buffer.concat(chunks).toString());
  }
  const items = Array.from(
    { length: Number(process.env.SSG_FIXTURE_COUNT || 30) },
    (_, i) => ({
      id: `10000000-0000-4000-8000-${String(i + 1).padStart(12, "0")}`,
      name: `Синтетический ${i % 2 ? "диск" : "товар"} ${i + 1}`,
      unit_price_rub: 10000 + i,
      stock: i % 5,
      available: true,
      unit_label: "шт.",
      details: {
        ...emptyDetails(),
        kind: i % 2 ? "wheel" : "tyre",
        sku: `DEMO-${i + 1}`,
        description: "Синтетические данные для проверки сборки.",
      },
      images: [],
    }),
  );
  return {
    schema_version: 1,
    version: createHash("sha256").update(JSON.stringify(items)).digest("hex"),
    generated_at: "2026-01-01T00:00:00Z",
    items,
  };
}
const snap = await snapshot();
if (
  snap.schema_version !== 1 ||
  !/^[a-f0-9]{64}$/.test(snap.version) ||
  !Number.isFinite(Date.parse(snap.generated_at)) ||
  !Array.isArray(snap.items) ||
  snap.items.length > 10000 ||
  (!snap.items.length && process.env.SSG_ALLOW_EMPTY !== "true")
)
  throw Error("Invalid/empty snapshot");
const ids = new Set();
for (const p of snap.items) {
  if (
    !/^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/.test(
      p.id,
    ) ||
    ids.has(p.id) ||
    typeof p.name !== "string" ||
    !p.name.trim() ||
    !p.available ||
    !Number.isSafeInteger(p.unit_price_rub) ||
    p.unit_price_rub < 0 ||
    !Number.isSafeInteger(p.stock) ||
    p.stock < 0 ||
    !["tyre", "wheel"].includes(p.details?.kind) ||
    p.details.location ||
    p.details.review_notes ||
    !Array.isArray(p.images)
  )
    throw Error("Invalid public product");
  for (const image of p.images) {
    const u = new URL(image.url, origin);
    if (!["https:", "http:"].includes(u.protocol))
      throw Error("Invalid image URL");
  }
  ids.add(p.id);
}
// Vite creates client assets and three entry templates in the staging directory.
await rm(target, { recursive: true, force: true });
const build = spawnSync("vite", ["build", root, "--outDir", target], {
  stdio: "inherit",
});
if (build.status !== 0) throw Error("Vite build failed");
const templates = {
  store: await readFile(resolve(target, "index.html"), "utf8"),
  info: await readFile(resolve(target, "information.html"), "utf8"),
};
const urls: string[] = [];
const escape = (s: string) =>
  s
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
async function page(
  data: PageData,
  title: string,
  description: string,
  info = false,
  noindex = false,
) {
  const markup = renderToString(
    info ? <Information path={data.path} /> : <App data={data} />,
  );
  const canonical = new URL(data.path, origin).href;
  let html = templates[info ? "info" : "store"]
    .replace(
      /<title>.*?<\/title>/,
      () => `<title>${escape(title)} — tireshop</title>`,
    )
    .replace(
      '<div id="root"></div>',
      () =>
        `<div id="root">${markup}</div><script id="page-data" type="application/json">${jsonScript(data)}</script>`,
    );
  html = html.replace(
    "</head>",
    () =>
      `<meta name="description" content="${escape(description)}"><link rel="canonical" href="${escape(canonical)}">${noindex ? '<meta name="robots" content="noindex,follow">' : ""}${data.path.startsWith("/products/") ? `<script id="product-schema" type="application/ld+json">${jsonScript(productSchema(data.products[0], origin))}</script>` : ""}</head>`,
  );
  const dest =
    data.path === "/404.html"
      ? "404.html"
      : data.path === "/"
        ? "index.html"
        : data.path.startsWith("/products/")
          ? data.path.slice(1) + ".html"
          : data.path.slice(1) + "index.html";
  await mkdir(dirname(resolve(target, dest)), { recursive: true });
  await writeFile(resolve(target, dest), html);
  if (!noindex) urls.push(canonical);
}
const products: Product[] = [...snap.items].sort(
  (a: Product, b: Product) =>
    Number(Boolean(b.images?.length)) - Number(Boolean(a.images?.length)) ||
    (a.id < b.id ? -1 : a.id > b.id ? 1 : 0),
);
for (const kind of ["", "tyre", "wheel"]) {
  const items = products.filter((p) => !kind || p.details?.kind === kind);
  for (let n = 1; n <= Math.max(1, Math.ceil(items.length / 24)); n++) {
    const title =
      kind === "tyre"
        ? "Грузовые шины"
        : kind === "wheel"
          ? "Грузовые диски"
          : "Шины и диски";
    await page(
      {
        path: categoryPath(kind, n),
        products: items.slice((n - 1) * 24, n * 24),
        total: items.length,
        generated: true,
      },
      title + (n > 1 ? ` — страница ${n}` : ""),
      "Каталог шин и дисков для грузового транспорта. Иркутск, доставка по России.",
    );
  }
}
for (const p of products)
  await page(
    { path: "/products/" + p.id, products: [p], total: 1, generated: true },
    p.name,
    p.details?.description || p.name,
  );
for (const [path, title, description] of [
  [
    "/about/",
    "О нас",
    "Более 20 лет на рынке. Б/у шины и диски, прямые поставки из Японии.",
  ],
  [
    "/contacts/",
    "Контакты",
    "Адрес склада в Иркутске, телефоны и режим работы tireshop.",
  ],
  ["/blog/", "Блог", "Материалы о шинах и дисках для грузового транспорта."],
  ["/404.html", "Страница не найдена", "Вернитесь в каталог шин и дисков."],
])
  await page(
    { path, products: [], total: 0, generated: true },
    title,
    description,
    true,
    path === "/404.html",
  );
await page(
  { path: "/cart/", products: [], total: 0, generated: true },
  "Корзина",
  "Оформление заявки на шины и диски.",
  false,
  true,
);
await rm(resolve(target, "information.html"));
await writeFile(
  resolve(target, "sitemap.xml"),
  `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${urls.map((u) => `<url><loc>${escape(u)}</loc></url>`).join("")}</urlset>`,
);
await writeFile(
  resolve(target, "robots.txt"),
  `User-agent: *\nAllow: /\nSitemap: ${origin}/sitemap.xml\n`,
);
await writeFile(
  resolve(target, "publication.json"),
  JSON.stringify(
    {
      schema_version: 1,
      snapshot: snap.version,
      generated_at: snap.generated_at,
      code: process.env.RENDER_GIT_COMMIT || process.env.GITHUB_SHA || "local",
      pages: urls.length,
      synthetic: !api && !file,
    },
    null,
    2,
  ),
);
if (api && (await snapshot()).version !== snap.version)
  throw Error("Catalog changed during build; retry publication");
const previous = resolve(root, "dist-previous");
await rm(previous, { recursive: true, force: true });
try {
  await rename(resolve(root, "dist"), previous);
} catch (e) {
  if ((e as NodeJS.ErrnoException).code !== "ENOENT") throw e;
}
try {
  await rename(target, resolve(root, "dist"));
} catch (e) {
  await rename(previous, resolve(root, "dist"));
  throw e;
}
await rm(previous, { recursive: true, force: true });
console.log(
  `SSG: ${urls.length} indexable pages, snapshot ${snap.version.slice(0, 12)}${!api && !file ? " (synthetic)" : ""}`,
);
