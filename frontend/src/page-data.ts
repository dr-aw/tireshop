import type { CatalogDetails } from "./catalog-details";
export type Product = {
  id: string;
  name: string;
  unit_price_rub: number;
  available: boolean;
  stock?: number;
  unit_label?: string;
  details?: CatalogDetails;
  images?: { url: string }[];
};
export type PageData = {
  path: string;
  products: Product[];
  total: number;
  generated?: boolean;
};
export function categoryPath(kind = "", page = 1) {
  const base =
    kind === "tyre"
      ? "/catalog/tires/"
      : kind === "wheel"
        ? "/catalog/wheels/"
        : "/catalog/";
  return page > 1 ? `${base}page/${page}/` : kind ? base : "/";
}
export function routeFilters(path: string): Record<string, string> {
  const kind = path.includes("/tires/")
    ? "tyre"
    : path.includes("/wheels/")
      ? "wheel"
      : "";
  const page = /\/page\/(\d+)\//.exec(path)?.[1];
  return { ...(kind ? { kind } : {}), ...(page ? { page } : {}) };
}
export function isStore(path: string) {
  return (
    path === "/" ||
    path === "/cart/" ||
    /^\/products\/[0-9a-f-]{36}\/?$/i.test(path) ||
    /^\/catalog\/(?:tires\/|wheels\/)?(?:page\/[1-9]\d*\/)?$/.test(path)
  );
}
export function productSchema(p: Product, origin: string) {
  return {
    "@context": "https://schema.org",
    "@type": "Product",
    name: p.name,
    description: p.details?.description || p.name,
    sku: p.details?.sku || p.id,
    image: p.images?.map((i) => new URL(i.url, origin).href),
    offers: {
      "@type": "Offer",
      url: new URL("/products/" + p.id, origin).href,
      priceCurrency: "RUB",
      price: p.unit_price_rub,
      availability:
        "https://schema.org/" +
        (p.available && (p.stock ?? 0) > 0 ? "InStock" : "OutOfStock"),
    },
  };
}
export const jsonScript = (value: unknown) =>
  JSON.stringify(value).replace(/</g, "\\u003c");
