import {
  detailLabels,
  unitLabel,
  type CatalogDetails,
} from "./catalog-details";
import { BasketPhoto } from "./basket-photo";
export type TableSettings = {
  columns: string[];
  sort: string;
  direction: "asc" | "desc";
};
export const defaultSettings = (): TableSettings => ({
  columns: ["name", "unit_price_rub", "stock", "is_available"],
  sort: "",
  direction: "asc",
});
export const columns = [
  { key: "name", label: "Товар" },
  { key: "unit_price_rub", label: "Цена за единицу" },
  { key: "stock", label: "Остаток" },
  { key: "is_available", label: "На витрине" },
  ...detailLabels.map(([key, label]) => ({ key, label })),
  { key: "kind", label: "Категория" },
  { key: "units_per_sale", label: "Штук в единице продажи" },
  { key: "description", label: "Описание" },
  { key: "review_notes", label: "Замечания" },
];
export function readSettings(user: string): TableSettings {
  try {
    const v = JSON.parse(
      localStorage.getItem(`tireshop.admin.table.v1:${user}`) || "null",
    );
    if (!v) return defaultSettings();
    const selected = columns
      .filter((c) => Array.isArray(v.columns) && v.columns.includes(c.key))
      .map((c) => c.key);
    return {
      columns: ["name", ...selected.filter((c) => c !== "name")],
      sort: columns.some((c) => c.key === v.sort) ? v.sort : "",
      direction: v.direction === "desc" ? "desc" : "asc",
    };
  } catch {
    return defaultSettings();
  }
}
export function saveSettings(user: string, value: TableSettings) {
  localStorage.setItem(
    `tireshop.admin.table.v1:${user}`,
    JSON.stringify(value),
  );
}
export function ColumnPicker({
  settings,
  change,
}: {
  settings: TableSettings;
  change: (s: TableSettings) => void;
}) {
  return (
    <details className="admin-column-picker">
      <summary>Столбцы таблицы</summary>
      <p>Настройки сохраняются для вашего логина в этом браузере.</p>
      <div>
        {columns.map((c) => (
          <label key={c.key}>
            <input
              type="checkbox"
              checked={settings.columns.includes(c.key)}
              disabled={c.key === "name"}
              onChange={(e) =>
                change({
                  ...settings,
                  columns: e.target.checked
                    ? [...settings.columns, c.key]
                    : settings.columns.filter((k) => k !== c.key),
                })
              }
            />
            {c.label}
          </label>
        ))}
      </div>
      <button
        type="button"
        onClick={() =>
          change({ ...settings, columns: defaultSettings().columns })
        }
      >
        Стандартные столбцы
      </button>
    </details>
  );
}
export function TableHeaders({
  settings,
  busy,
  sort,
}: {
  settings: TableSettings;
  busy: boolean;
  sort: (key: string) => void;
}) {
  return (
    <>
      {settings.columns.map((key) => (
        <th
          key={key}
          aria-sort={
            settings.sort === key
              ? settings.direction === "asc"
                ? "ascending"
                : "descending"
              : "none"
          }
        >
          <button type="button" disabled={busy} onClick={() => sort(key)}>
            {columns.find((c) => c.key === key)?.label}{" "}
            <span aria-hidden="true">
              {settings.sort === key
                ? settings.direction === "asc"
                  ? "↑"
                  : "↓"
                : "↕"}
            </span>
          </button>
        </th>
      ))}
      <th>
        <span className="sr-only">Действия</span>
      </th>
    </>
  );
}
type Product = {
  id: string;
  name: string;
  image_url?: string;
  stock: number;
  unit_price_rub: number;
  available: boolean;
  details?: CatalogDetails;
};
export function TableCells({
  product: p,
  settings,
}: {
  product: Product;
  settings: TableSettings;
}) {
  return (
    <>
      {settings.columns.map((key) => {
        let content;
        if (key === "name")
          content = (
            <div className="admin-product-name">
              <span className="admin-thumbnail">
                <BasketPhoto
                  id={p.id}
                  name={p.name}
                  url={p.image_url}
                  lookup={false}
                  fallback={<span aria-hidden="true">◇</span>}
                />
              </span>
              <div>
                {p.name}
                <small>{p.details?.sku}</small>
                {p.details?.review_notes && (
                  <small>{p.details.review_notes}</small>
                )}
              </div>
            </div>
          );
        else if (key === "unit_price_rub")
          content =
            new Intl.NumberFormat("ru-RU").format(p.unit_price_rub) + " ₽";
        else if (key === "stock")
          content = (
            <>
              {p.stock}
              {(p.details?.units_per_sale || 1) > 1 && (
                <small>{unitLabel(p.details)}</small>
              )}
            </>
          );
        else if (key === "is_available") content = p.available ? "Да" : "Скрыт";
        else if (key === "kind")
          content = p.details?.kind === "wheel" ? "Диск" : "Шина";
        else content = p.details?.[key as keyof CatalogDetails] ?? "—";
        return (
          <td key={key} data-column={key}>
            {content === "" ? "—" : content}
          </td>
        );
      })}
    </>
  );
}
