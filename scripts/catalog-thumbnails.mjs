// Prepare only the small derivatives without changing the import or stock.
import fs from "node:fs/promises";
import path from "node:path";
import { thumbnail } from "./catalog-thumbnail.mjs";
const manifest = JSON.parse(
  await fs.readFile(
    process.argv[2] || ".tmp/catalog-import/media.json",
    "utf8",
  ),
);
let bytes = 0;
for (const m of manifest) {
  if (
    !/^\/media\/products\/[0-9a-f-]+\/[a-f0-9]{64}-1280-v1\.webp$/.test(
      m.webp_path,
    )
  )
    throw Error("Unexpected media path");
  const target = path.join(
    "frontend/public",
    m.webp_path.replace(/-1280-v1\.webp$/, "-320-v1.webp"),
  );
  await thumbnail(m.source_path, target);
  bytes += (await fs.stat(target)).size;
}
console.log(
  JSON.stringify({ thumbnails: manifest.length, total_bytes: bytes }),
);
