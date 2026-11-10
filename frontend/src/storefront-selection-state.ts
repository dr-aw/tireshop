import {
  cleanCategoryFilters,
  changeKind,
  type FilterValues,
} from "./catalog-filters";

export const selectionKeys = ["radius", "tyre_size"];
export function cleanStorefrontFilters(values: FilterValues): FilterValues {
  const next = cleanCategoryFilters(values);
  const allowed = new Set([
    "kind",
    "q",
    "sort",
    "direction",
    "page",
    "price_min",
    "price_max",
    "in_stock",
    "radius",
    ...(values.kind === "tyre" ? ["tyre_size", "size", "rim_diameter"] : []),
    ...(values.kind === "wheel"
      ? ["width", "diameter", "pcd", "wheel_offset", "center_bore"]
      : []),
  ]);
  for (const key of Object.keys(next)) if (!allowed.has(key)) delete next[key];
  if (!next.kind) delete next.radius;
  if (next.kind !== "tyre") delete next.tyre_size;
  const legacy = next.kind === "tyre" ? "rim_diameter" : "diameter";
  if (
    next[legacy] &&
    Number.isFinite(Number(next[legacy])) &&
    Number(next[legacy]) > 0
  ) {
    if (!next.radius) next.radius = String(Number(next[legacy]));
    if (Number(next.radius) === Number(next[legacy])) delete next[legacy];
  }
  if (next.radius && Number.isFinite(Number(next.radius)))
    next.radius = String(Number(next.radius));
  return next;
}
export function changeStorefrontKind(
  values: FilterValues,
  kind: string,
): FilterValues {
  const next = changeKind(values, kind);
  for (const key of selectionKeys) delete next[key];
  return next;
}
