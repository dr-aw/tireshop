// Vite replaces the constant; build-time React reads the same public setting.
const configured =
  typeof __PUBLIC_MEDIA_BASE__ !== "undefined"
    ? __PUBLIC_MEDIA_BASE__
    : process.env.VITE_MEDIA_BASE_URL || "";
export function validateMediaBase(value: string) {
  if (!value) return "";
  const url = new URL(value);
  if (
    url.protocol !== "https:" ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  )
    throw Error(
      "VITE_MEDIA_BASE_URL must be an HTTPS base URL without credentials, query or fragment",
    );
  return url.href.replace(/\/$/, "");
}
const base = validateMediaBase(configured);
export function mediaURL(value: string): string {
  return base && value.startsWith("/media/") ? base + value : value;
}
// Cart data may come from browser storage. Accept local media or this deployment's
// configured media host/path, never arbitrary remote URLs from stored data.
export function safeMediaURL(value: unknown): string | undefined {
  if (typeof value !== "string") return;
  const path =
    base && value.startsWith(base + "/media/")
      ? value.slice(base.length)
      : value;
  if (!path.startsWith("/media/")) return;
  let decoded: string;
  try {
    decoded = decodeURIComponent(path);
  } catch {
    return;
  }
  if (
    decoded.includes("..") ||
    decoded.includes("\\") ||
    /[?#\u0000-\u0020]/.test(decoded)
  )
    return;
  return mediaURL(path);
}
