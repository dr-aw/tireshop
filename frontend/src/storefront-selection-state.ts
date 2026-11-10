import {
  cleanCategoryFilters,
  changeKind,
  type FilterValues,
} from "./catalog-filters";

export const selectionKeys = ["diameter", "radius", "tyre_size"];
export function cleanStorefrontFilters(values: FilterValues): FilterValues {
  const next = cleanCategoryFilters(values);
  // The shared admin cleaner still treats diameter as a wheel editor field.
  if (values.diameter) next.diameter = values.diameter;
  const allowed = new Set([
    "kind",
    "q",
    "sort",
    "direction",
    "page",
    "price_min",
    "price_max",
    "in_stock",
    ...(values.kind ? ["diameter", "radius"] : []),
    ...(values.kind === "tyre" ? ["tyre_size", "size", "rim_diameter"] : []),
    ...(values.kind === "wheel"
      ? ["width", "pcd", "wheel_offset", "center_bore"]
      : []),
  ]);
  for (const key of Object.keys(next)) if (!allowed.has(key)) delete next[key];
  for (const legacy of ["radius", "rim_diameter"]) {
    if (
      next[legacy] &&
      Number.isFinite(Number(next[legacy])) &&
      Number(next[legacy]) > 0
    ) {
      if (!next.diameter) next.diameter = String(Number(next[legacy]));
      if (Number(next.diameter) === Number(next[legacy])) delete next[legacy];
    }
  }
  if (next.diameter && Number.isFinite(Number(next.diameter)))
    next.diameter = String(Number(next.diameter));
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
