import { useEffect, useState } from "react";
import {
  characteristicKeys,
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
function read(): FilterValues {
  return Object.fromEntries(
    [...new URLSearchParams(location.search)].filter(
      ([key, value]) => keys.has(key) && value !== "",
    ),
  );
}
export function useCatalogNavigation() {
  const [values, setValues] = useState(read);
  useEffect(() => {
    const restore = () => setValues(read());
    window.addEventListener("popstate", restore);
    return () => window.removeEventListener("popstate", restore);
  }, []);
  function navigate(next: FilterValues, page = 1) {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(next))
      if (key !== "page" && value) params.set(key, value);
    if (page > 1) params.set("page", String(page));
    const query = params.toString();
    history.pushState(
      null,
      "",
      `${location.pathname}${query ? `?${query}` : ""}#catalog`,
    );
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
