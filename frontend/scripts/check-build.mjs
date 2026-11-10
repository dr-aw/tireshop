import { readFile, writeFile, mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import assert from "node:assert/strict";
const temp = await mkdtemp(join(tmpdir(), "tireshop-ssg-"));
const original = await readFile("frontend/dist/index.html", "utf8");
const data = JSON.parse(
  /id="page-data" type="application\/json">(.*?)<\/script>/.exec(original)[1],
);
const product = data.products[0];
const snap = {
  schema_version: 1,
  version: "a".repeat(64),
  generated_at: "2026-01-01T00:00:00Z",
  items: [product],
};
function run(env) {
  return spawnSync("bun", ["frontend/scripts/build.tsx"], {
    env: {
      ...process.env,
      SSG_CATALOG_URL: "",
      SSG_SNAPSHOT_FILE: join(temp, "snapshot.json"),
      ...env,
    },
    stdio: "pipe",
  });
}
try {
  for (const bad of [
    { ...snap, items: [] },
    { ...snap, items: [product, product] },
    { ...snap, items: [{ ...product, available: false }] },
    {
      ...snap,
      items: [
        { ...product, details: { ...product.details, location: "private" } },
      ],
    },
  ]) {
    await writeFile(join(temp, "snapshot.json"), JSON.stringify(bad));
    assert.notEqual(run({}).status, 0);
    assert.equal(await readFile("frontend/dist/index.html", "utf8"), original);
  }
  const photographed = {
    ...product,
    id: "eeeeeeee-0000-4000-8000-000000000003",
    images: [{ url: "/media/synthetic.webp" }],
  };
  await writeFile(
    join(temp, "snapshot.json"),
    JSON.stringify({
      ...snap,
      items: [{ ...product, images: [] }, photographed],
    }),
  );
  const mediaBase = "https://storage.googleapis.com/tireshop-test";
  const ordered = run({ VITE_MEDIA_BASE_URL: mediaBase });
  assert.equal(ordered.status, 0, ordered.stderr?.toString());
  const catalog = await readFile("frontend/dist/index.html", "utf8");
  const initial = JSON.parse(
    /id="page-data" type="application\/json">(.*?)<\/script>/.exec(catalog)[1],
  );
  assert.equal(initial.products[0].id, photographed.id);
  assert(catalog.includes(`src="${mediaBase}/media/synthetic.webp"`));
  const productHTML = await readFile(
    `frontend/dist/products/${photographed.id}.html`,
    "utf8",
  );
  const schema = JSON.parse(
    /id="product-schema" type="application\/ld\+json">(.*?)<\/script>/.exec(
      productHTML,
    )[1],
  );
  assert.equal(schema.image[0], mediaBase + "/media/synthetic.webp");
  const attack = "$& </script><script>window.PWNED=true</script>";
  await writeFile(
    join(temp, "snapshot.json"),
    JSON.stringify({
      ...snap,
      items: [
        {
          ...product,
          name: attack,
          details: { ...product.details, description: attack },
        },
      ],
    }),
  );
  const built = run({});
  assert.equal(built.status, 0, built.stderr?.toString());
  const html = await readFile(
    `frontend/dist/products/${product.id}.html`,
    "utf8",
  );
  assert(!html.includes(attack));
  assert(html.includes("\\u003c/script>"));
  assert(html.includes("&lt;/script&gt;"));
  const saved = JSON.parse(
    /id="page-data" type="application\/json">(.*?)<\/script>/.exec(html)[1],
  );
  assert.equal(saved.products[0].name, attack);
  console.log(
    "PASS: invalid snapshots preserve dist; HTML and JSON escape script input",
  );
} finally {
  await rm(temp, { recursive: true, force: true });
  const restored = spawnSync("bun", ["frontend/scripts/build.tsx"], {
    env: { ...process.env, SSG_SNAPSHOT_FILE: "", SSG_CATALOG_URL: "" },
    stdio: "pipe",
  });
  if (restored.status !== 0) throw Error("Fixture rebuild failed");
}
