import { useEffect, useState, type ReactNode } from "react";
export function thumbnailURL(url: string) {
  return url.replace(/-1280-v1\.webp$/, "-320-v1.webp");
}
function safeURL(value: unknown): string | undefined {
  return typeof value === "string" &&
    value.startsWith("/media/") &&
    !value.includes("..") &&
    !value.includes("\\")
    ? value
    : undefined;
}
export function BasketPhoto({
  id,
  name,
  url,
  fallback,
  lookup = true,
}: {
  id: string;
  name: string;
  url?: string;
  fallback: ReactNode;
  lookup?: boolean;
}) {
  const [source, setSource] = useState<string | undefined>(safeURL(url));
  const [original, setOriginal] = useState(false);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    if (safeURL(url)) {
      setSource(safeURL(url));
      return;
    }
    const controller = new AbortController();
    if (!lookup) return;
    fetch(`/api/products/${encodeURIComponent(id)}`, {
      signal: controller.signal,
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((p) => setSource(safeURL(p?.images?.[0]?.url)))
      .catch(() => {});
    return () => controller.abort();
  }, [id, url, lookup]);
  if (!source || failed) return <>{fallback}</>;
  return (
    <img
      src={original ? source : thumbnailURL(source)}
      alt={name}
      width={320}
      height={320}
      loading="lazy"
      onError={() => (original ? setFailed(true) : setOriginal(true))}
    />
  );
}
