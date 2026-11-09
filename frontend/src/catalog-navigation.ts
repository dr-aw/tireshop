import { routeFilters, categoryPath } from "./page-data";
import { useEffect, useState } from "react";
import {
  characteristicKeys,
  cleanCategoryFilters,
  filterKeys,
  type FilterValues,
} from "./catalog-filters";
const keys = new Set([
  ...characteristicKeys,
  ...filterKeys,
  "q",
  "kind",
  "sort",
  "direction",
  "page",
]);
function read(defaults: FilterValues = {}): FilterValues {
  const values = Object.fromEntries(
    [...new URLSearchParams(location.search)].filter(
      ([key, value]) => keys.has(key) && value !== "",
    ),
  );
  return cleanCategoryFilters({ ...defaults, ...values });
}
export function useCatalogNavigation(defaults: FilterValues = {}, live = true) {
  const [values, setValues] = useState(() =>
    live ? read(defaults) : defaults,
  );
  useEffect(() => {
    const restore = () => setValues(read(routeFilters(location.pathname)));
    window.addEventListener("popstate", restore);
    return () => window.removeEventListener("popstate", restore);
  }, []);
  useEffect(() => {
    if (
      !live ||
      location.pathname.startsWith("/products/") ||
      location.pathname === "/cart/"
    )
      return;
    const filtered = Object.keys(values).some(
      (k) => !["kind", "page"].includes(k) && values[k],
    );
    let tag = document.querySelector<HTMLMetaElement>('meta[name="robots"]');
    if (filtered) {
      if (!tag) {
        tag = document.createElement("meta");
        tag.name = "robots";
        document.head.append(tag);
      }
      tag.content = "noindex,follow";
    } else if (
      tag?.content === "noindex,follow" &&
      location.pathname !== "/cart/"
    )
      tag.remove();
    const link = document.querySelector<HTMLLinkElement>(
      'link[rel="canonical"]',
    );
    if (link) {
      const origin = new URL(link.href).origin;
      link.href = new URL(
        categoryPath(values.kind, filtered ? 1 : Number(values.page || 1)),
        origin,
      ).href;
    }
  }, [values, live]);
  function navigate(next: FilterValues, page = 1) {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(cleanCategoryFilters(next)))
      if (key !== "page" && value) params.set(key, value);
    if (page > 1) params.set("page", String(page));
    const query = params.toString();
    history.pushState(null, "", `/${query ? `?${query}` : ""}#catalog`);
    setValues(read());
  }
  const rawPage = Number(values.page || 1);
  const page =
    Number.isInteger(rawPage) && rawPage > 0 && rawPage <= 4167 ? rawPage : 1;
  const params = new URLSearchParams(values);
  params.delete("page");
  params.set("limit", "24");
  params.set("offset", String((page - 1) * 24));
  return { values, navigate, page, query: params.toString() };
}
