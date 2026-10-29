import fs from "node:fs/promises";
import path from "node:path";
import { createRequire } from "node:module";
const sharp = createRequire(
  new URL("../frontend/package.json", import.meta.url),
)("sharp");
export async function thumbnail(source, target) {
  await fs.mkdir(path.dirname(target), { recursive: true });
  await sharp(source, { limitInputPixels: 50000000 })
    .rotate()
    .toColourspace("srgb")
    .flatten({ background: "#ffffff" })
    .resize(320, 320, { fit: "cover", position: "centre" })
    .webp({ quality: 80, effort: 5 })
    .toFile(target);
  const m = await sharp(target).metadata();
  if (
    m.width !== 320 ||
    m.height !== 320 ||
    m.format !== "webp" ||
    m.exif ||
    m.icc ||
    m.xmp ||
    m.iptc
  )
    throw Error("Invalid thumbnail output");
}
