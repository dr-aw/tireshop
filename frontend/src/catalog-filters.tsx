import { useEffect, useState, type FormEvent } from "react";

export type FilterValues = Record<string, string>;
const common = [
  ["brand", "Производитель"],
  ["model", "Модель"],
  ["condition", "Состояние"],
  ["production_year", "Год выпуска"],
];
const tyre = [
  ["size", "Размер шины"],
  ["tyre_width", "Ширина шины"],
  ["aspect_ratio", "Профиль, %"],
  ["rim_diameter", "Посадочный диаметр, дюймы"],
  ["season", "Сезон"],
  ["load_index", "Индекс нагрузки"],
  ["speed_index", "Индекс скорости"],
];
const wheel = [
  ["diameter", "Диаметр диска, дюймы"],
  ["width", "Ширина диска, дюймы"],
  ["wheel_offset", "Вылет ET, мм"],
  ["pcd", "Сверловка PCD"],
  ["center_bore", "Ступица DIA, мм"],
  ["wheel_type", "Тип диска"],
];
export const characteristicKeys = [...common, ...tyre, ...wheel].map(
  ([key]) => key,
);
const ranges = [
  "price_min",
  "price_max",
  "tread_min",
  "tread_max",
  "stock_min",
  "stock_max",
];
export const filterKeys = [
  ...characteristicKeys,
  ...ranges,
  "in_stock",
  "visibility",
];
export function changeKind(values: FilterValues, kind: string): FilterValues {
  const next: FilterValues = { ...values, kind };
  for (const [key] of [...tyre, ...wheel]) delete next[key];
  delete next.tread_min;
  delete next.tread_max;
  return next;
}

export function CatalogFilters({
  values,
  apply,
  admin = false,
  disabled = false,
}: {
  values: FilterValues;
  apply: (values: FilterValues) => void;
  admin?: boolean;
  disabled?: boolean;
}) {
  const [initiallyOpen] = useState(
    () => admin || window.matchMedia("(min-width: 641px)").matches,
  );
  const [options, setOptions] = useState<Record<string, string[]>>({});
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [revision, setRevision] = useState(0);
  const kind = values.kind || "";
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    setOptions({});
    fetch(
      `/api/${admin ? "admin/" : ""}products/filters?kind=${encodeURIComponent(kind)}`,
      { signal: controller.signal },
    )
      .then(async (response) => {
        if (!response.ok) throw Error();
        return response.json();
      })
      .then((data) => {
        if (!controller.signal.aborted) setOptions(data.options);
      })
      .catch((e) => {
        if (e.name !== "AbortError")
          setError("Не удалось загрузить значения фильтров.");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [admin, kind, revision]);
  const fields = [
    ...common,
    ...(kind === "tyre" ? tyre : kind === "wheel" ? wheel : []),
  ];
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const next = { ...values };
    for (const key of [...filterKeys, ...(admin ? ["q"] : [])]) {
      delete next[key];
      const value = data.get(key);
      if (typeof value === "string" && value.trim()) next[key] = value.trim();
    }
    apply(next);
  }
  function range(key: string, label: string, max: number, step = "1") {
    return (
      <fieldset className="filter-range">
        <legend>{label}</legend>
        <div>
          <input
            aria-label={`${label}: от`}
            name={`${key}_min`}
            type="number"
            min="0"
            max={max}
            step={step}
            placeholder="От"
            defaultValue={values[`${key}_min`] || ""}
          />
          <span aria-hidden="true">—</span>
          <input
            aria-label={`${label}: до`}
            name={`${key}_max`}
            type="number"
            min="0"
            max={max}
            step={step}
            placeholder="До"
            defaultValue={values[`${key}_max`] || ""}
          />
        </div>
      </fieldset>
    );
  }
  return (
    <details
      className={`catalog-filter-panel ${admin ? "admin-filter-panel" : ""}`}
      open={initiallyOpen}
    >
      <summary>
        Фильтры{filterKeys.some((key) => values[key]) ? " · применены" : ""}
      </summary>
      {error && (
        <p role="alert">
          {error}{" "}
          <button
            type="button"
            className="secondary"
            onClick={() => setRevision((r) => r + 1)}
          >
            Повторить
          </button>
        </p>
      )}
      {loading && <p role="status">Загружаем параметры…</p>}
      <form key={JSON.stringify(values)} onSubmit={submit}>
        <fieldset className="filter-fields" disabled={disabled}>
          {admin && (
            <label>
              Поиск
              <input
                name="q"
                type="search"
                placeholder="Название, артикул, размер"
                defaultValue={values.q || ""}
              />
            </label>
          )}
          {fields.map(([key, label]) => (
            <label key={key}>
              {label}
              <select
                name={key}
                aria-label={label}
                defaultValue={values[key] || ""}
                disabled={loading || !!error}
              >
                <option value="">Все</option>
                {values[key] && !options[key]?.includes(values[key]) && (
                  <option value={values[key]}>{values[key]}</option>
                )}
                {(options[key] || []).map((value) => (
                  <option key={value} value={value}>
                    {key === "tyre_width"
                      ? value.replace(/ mm$/, " мм").replace(/ in$/, "″")
                      : value}
                  </option>
                ))}
              </select>
            </label>
          ))}
          {range("price", "Цена, ₽", 1000000000)}
          {kind === "tyre" && range("tread", "Протектор, %", 100, "any")}
          {admin && range("stock", "Запас, ед.", 1000000000)}
          {admin && (
            <label>
              Публикация
              <select name="visibility" defaultValue={values.visibility || ""}>
                <option value="">Все</option>
                <option value="visible">Опубликованные</option>
                <option value="hidden">Скрытые</option>
              </select>
            </label>
          )}
          <label className="filter-checkbox">
            <input
              type="checkbox"
              name="in_stock"
              value="true"
              defaultChecked={values.in_stock === "true"}
            />
            Только в наличии
          </label>
          <div className="filter-actions">
            <button type="submit" disabled={loading || !!error}>
              Применить
            </button>
            <button
              type="button"
              className="secondary"
              onClick={() => {
                const next = { ...values };
                for (const key of [...filterKeys, ...(admin ? ["q"] : [])])
                  delete next[key];
                apply(next);
              }}
            >
              Сбросить фильтры
            </button>
          </div>
        </fieldset>
      </form>
    </details>
  );
}
