import { useEffect, useState } from "react";
import { CatalogFilters, type FilterValues } from "./catalog-filters";

type Option = {
  value: string;
  label: string;
  product_count: number;
  unit_count: string;
};
type Selection = { diameters: Option[]; sizes: Option[] };
export function DiameterIcon({ wheel = false }: { wheel?: boolean }) {
  return (
    <svg
      viewBox="0 0 48 48"
      aria-hidden="true"
      fill="none"
      stroke="currentColor"
      strokeWidth="4"
    >
      <circle cx="24" cy="24" r="19" />
      <circle cx="24" cy="24" r={wheel ? 6 : 11} />
      {wheel ? (
        <path
          d="M24 6v9m0 18v9M6 24h9m18 0h9M11 11l7 7m12 12 7 7m-26 0 7-7m12-12 7-7"
          strokeWidth="3"
        />
      ) : (
        <circle cx="24" cy="24" r="3" fill="currentColor" stroke="none" />
      )}
    </svg>
  );
}
const legacyLabels: Record<string, string> = {
  size: "Исходный размер",
  rim_diameter: "Диаметр шины",
  radius: "Прежний диаметр",
};
export function StorefrontSelection({
  values,
  apply,
}: {
  values: FilterValues;
  apply: (v: FilterValues) => void;
}) {
  const params = new URLSearchParams(values);
  for (const key of ["page", "sort", "direction"]) params.delete(key);
  params.sort();
  const query = params.toString();
  const kind = values.kind || "";
  const [revision, setRevision] = useState(0);
  const [result, setResult] = useState<{
    query: string;
    data?: Selection;
    error?: string;
  }>();
  useEffect(() => {
    if (!kind) return;
    const controller = new AbortController();
    setResult(undefined);
    fetch(`/api/products/selection?${query}`, { signal: controller.signal })
      .then(async (r) => {
        if (!r.ok) throw Error();
        return r.json();
      })
      .then((data: Selection) => {
        if (
          !Array.isArray(data.diameters) ||
          !Array.isArray(data.sizes) ||
          ![...data.diameters, ...data.sizes].every(
            (o) =>
              typeof o.value === "string" &&
              typeof o.label === "string" &&
              /^\d+$/.test(o.unit_count),
          )
        )
          throw Error();
        if (!controller.signal.aborted) setResult({ query, data });
      })
      .catch((e) => {
        if (e.name !== "AbortError" && !controller.signal.aborted)
          setResult({ query, error: "Не удалось загрузить подбор." });
      });
    return () => controller.abort();
  }, [kind, query, revision]);
  const current = result?.query === query ? result : undefined;
  const data = current?.data;
  const sizeKey = kind === "tyre" ? "tyre_size" : "width";
  function chooseDiameter(value: string) {
    const next = { ...values };
    for (const key of [
      "diameter",
      "rim_diameter",
      "radius",
      "size",
      "tyre_size",
      ...(kind === "wheel" ? ["width"] : []),
    ])
      delete next[key];
    if (value !== values.diameter) next.diameter = value;
    apply(next);
  }
  function chooseSize(value: string) {
    const next = { ...values };
    delete next.size;
    if (next[sizeKey] === value) delete next[sizeKey];
    else next[sizeKey] = value;
    apply(next);
  }
  function optionsWithSelection(
    options: Option[],
    value: string | undefined,
    label: string,
  ) {
    return value && !options.some((o) => o.value === value)
      ? [...options, { value, label, product_count: 0, unit_count: "0" }]
      : options;
  }
  const legacy = Object.keys(legacyLabels).filter((key) => values[key]);
  if (kind === "wheel" && values.width && !values.diameter)
    legacy.push("width");
  return (
    <>
      {kind && (
        <section
          className="storefront-selection"
          aria-label="Подбор по размеру"
          aria-busy={!current}
        >
          <h3>Диаметр, дюймы</h3>
          {!current && <p role="status">Загружаем размеры…</p>}
          {current?.error && (
            <p role="alert">
              {current.error}{" "}
              <button
                type="button"
                className="secondary"
                onClick={() => setRevision((r) => r + 1)}
              >
                Повторить
              </button>
            </p>
          )}
          {data && (
            <>
              <div className="diameter-options">
                {optionsWithSelection(
                  data.diameters,
                  values.diameter,
                  kind === "wheel"
                    ? `${values.diameter}″`
                    : `R${values.diameter}`,
                ).map((option) => (
                  <button
                    type="button"
                    key={option.value}
                    className="diameter-option"
                    aria-pressed={values.diameter === option.value}
                    aria-label={`${option.label}, ${BigInt(option.unit_count).toLocaleString("ru-RU")} шт`}
                    onClick={() => chooseDiameter(option.value)}
                  >
                    <DiameterIcon wheel={kind === "wheel"} />
                    <strong>{option.label}</strong>
                    <small>
                      {BigInt(option.unit_count).toLocaleString("ru-RU")} шт
                    </small>
                  </button>
                ))}
              </div>
              {!data.diameters.length && !values.diameter && (
                <p>Нет диаметров для этого подбора.</p>
              )}
              {values.diameter && (
                <>
                  <h3>Размеры {values.diameter}″</h3>
                  <div
                    className="size-options"
                    role="group"
                    aria-label={`Размеры ${values.diameter}″`}
                  >
                    {optionsWithSelection(
                      data.sizes,
                      values[sizeKey],
                      kind === "wheel"
                        ? `${values.width}×${values.diameter}`
                        : values.tyre_size,
                    ).map((option) => (
                      <button
                        type="button"
                        key={option.value}
                        className="size-option"
                        aria-pressed={values[sizeKey] === option.value}
                        onClick={() => chooseSize(option.value)}
                      >
                        <strong>{option.label}</strong>
                        <span>
                          {BigInt(option.unit_count).toLocaleString("ru-RU")} шт
                        </span>
                      </button>
                    ))}
                  </div>
                  {!data.sizes.length && !values[sizeKey] && (
                    <p>Нет размеров для этого подбора.</p>
                  )}
                </>
              )}
            </>
          )}
          {(values.diameter || values[sizeKey]) && (
            <button
              className="secondary selection-clear"
              type="button"
              onClick={() => {
                const next = { ...values };
                for (const key of [
                  "diameter",
                  "tyre_size",
                  "width",
                  "radius",
                  "rim_diameter",
                  "size",
                ])
                  delete next[key];
                apply(next);
              }}
            >
              Сбросить размер
            </button>
          )}
        </section>
      )}
      {legacy.length > 0 && (
        <div className="selection-legacy" aria-label="Применённые ограничения">
          {legacy.map((key) => (
            <button
              type="button"
              className="secondary"
              key={key}
              onClick={() => {
                const next = { ...values };
                delete next[key];
                apply(next);
              }}
            >
              {legacyLabels[key] || "Ширина диска"}: {values[key]} ×
            </button>
          ))}
        </div>
      )}
      {values.tyre_size && !values.diameter && (
        <button
          type="button"
          className="secondary"
          onClick={() => {
            const next = { ...values };
            delete next.tyre_size;
            apply(next);
          }}
        >
          Размер: {values.tyre_size} ×
        </button>
      )}
      <CatalogFilters values={values} apply={apply} storefront />
    </>
  );
}
