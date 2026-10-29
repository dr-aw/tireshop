// Deterministic media preparation, separate from serving processes.
import fs from "node:fs/promises";
import path from "node:path";
import crypto from "node:crypto";
import { createRequire } from "node:module";
import { thumbnail } from "./catalog-thumbnail.mjs";
const require = createRequire(
  new URL("../frontend/package.json", import.meta.url),
);
const sharp = require("sharp");
sharp.concurrency(1);
sharp.cache({ memory: 32, files: 0, items: 20 });
const [batchFile, outputFile, mode] = process.argv.slice(2);
if (!batchFile || !outputFile)
  throw new Error("usage: catalog-media.mjs STAGED_JSON MANIFEST_JSON [pilot]");
const batch = JSON.parse(await fs.readFile(batchFile, "utf8"));
const root = path.resolve(".local/foto");
const files = [];
async function walk(dir) {
  for (const e of await fs.readdir(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) await walk(p);
    else if (e.isFile() && /\.jpe?g$/i.test(e.name)) files.push(p);
  }
}
await walk(root);
files.sort();
const manifest = [],
  issues = [],
  converted = new Map(),
  used = new Set();
async function convert(source, relative) {
  const data = await fs.readFile(source);
  const hash = crypto.createHash("sha256").update(data).digest("hex");
  const url = `/media/${relative}/${hash}-1280-v1.webp`;
  const target = path.join("frontend/public", url);
  await fs.mkdir(path.dirname(target), { recursive: true });
  // Auto-orient first, never crop or enlarge the source, then pad to the common
  // canvas. Sharp removes metadata by default; do not call withMetadata().
  const resized = await sharp(data, { limitInputPixels: 50000000 })
    .rotate()
    .toColourspace("srgb")
    .flatten({ background: "#ffffff" })
    .resize(1280, 1280, { fit: "inside", withoutEnlargement: true })
    .png()
    .toBuffer();
  const meta = await sharp(resized).metadata();
  const left = Math.floor((1280 - meta.width) / 2),
    top = Math.floor((1280 - meta.height) / 2);
  await sharp(resized)
    .extend({
      left,
      right: 1280 - meta.width - left,
      top,
      bottom: 1280 - meta.height - top,
      background: "#ffffff",
    })
    .webp({ quality: 85, effort: 5 })
    .toFile(target);
  const checked = await sharp(target).metadata();
  if (relative.startsWith("products/"))
    await thumbnail(source, target.replace(/-1280-v1\.webp$/, "-320-v1.webp"));
  if (
    checked.width !== 1280 ||
    checked.height !== 1280 ||
    checked.format !== "webp" ||
    checked.exif ||
    checked.icc ||
    checked.iptc ||
    checked.xmp
  )
    throw new Error("Unexpected output dimensions or metadata");
  converted.set(source, url);
  return {
    source_path: path.relative(process.cwd(), source),
    sha256: hash,
    webp_path: url,
    width: 1280,
    height: 1280,
  };
}
const products =
  mode === "pilot"
    ? [
        batch.products[0],
        batch.products.find((p) => p.details.sku === "AU-2207"),
      ]
    : batch.products;
for (const p of products) {
  const sku = p.details.sku,
    selected = [];
  for (let name of p.photos) {
    if (sku === "SU-1702" && name === "SU-1703-03.jpg") name = "SU-1702-03.jpg";
    if (sku === "SU-1904") name = name.replace("SU-1904-", "SN-1904-") + ".jpg";
    if (!name.startsWith(sku + "-") && sku !== "SU-1904") {
      issues.push({ sku, name, reason: "foreign SKU, not linked" });
      continue;
    }
    const candidates = files.filter((f) => path.basename(f) === name);
    if (candidates.length !== 1) {
      issues.push({
        sku,
        name,
        reason: candidates.length ? "ambiguous" : "missing",
      });
      continue;
    }
    selected.push(candidates[0]);
  }
  selected.push(
    ...files.filter(
      (f) =>
        path.basename(path.dirname(f)) === sku &&
        path.basename(f).startsWith(sku + "-") &&
        !selected.includes(f),
    ),
  );
  const hashes = new Set();
  for (const source of [...new Set(selected)]) {
    try {
      const image = await convert(source, `products/${p.id}`);
      used.add(source);
      if (!hashes.has(image.sha256))
        manifest.push({ product_id: p.id, ...image });
      hashes.add(image.sha256);
    } catch (e) {
      issues.push({ sku, source, reason: String(e) });
    }
  }
}
// Convert all remaining source images too, but keep unassigned assets out of
// the public directory and product gallery until their ownership is resolved.
if (mode !== "pilot")
  for (const source of files.filter((f) => !used.has(f))) {
    try {
      const image = await convert(source, "unassigned");
      const from = path.join("frontend/public", image.webp_path);
      const dest = path.join(
        ".tmp/catalog-import/unassigned",
        path.basename(from),
      );
      await fs.mkdir(path.dirname(dest), { recursive: true });
      await fs.rename(from, dest);
    } catch (e) {
      issues.push({ source, reason: String(e) });
    }
  }
await fs.writeFile(outputFile, JSON.stringify(manifest, null, 2));
const report = {
  source_images: files.length,
  converted: converted.size,
  linked_images: manifest.length,
  products_with_images: new Set(manifest.map((m) => m.product_id)).size,
  issues,
};
await fs.writeFile(
  outputFile + ".report.json",
  JSON.stringify(report, null, 2),
);
console.log(JSON.stringify({ ...report, issues: issues.length }));
