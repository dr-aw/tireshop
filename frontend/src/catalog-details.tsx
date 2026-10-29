import { useRef, useState, type ReactNode } from "react";
export type CatalogDetails = {
  tyre_width: number | null;
  tyre_width_unit: string;
  aspect_ratio: number | null;
  rim_diameter: number | null;
  sku: string;
  kind: string;
  description: string;
  brand: string;
  model: string;
  production_year: number | null;
  size: string;
  season: string;
  tread_percent: number | null;
  load_index: string;
  speed_index: string;
  location: string;
  condition: string;
  diameter: number | null;
  width: number | null;
  wheel_offset: number | null;
  pcd: string;
  wheel_type: string;
  center_bore: number | null;
  units_per_sale: number;
  review_notes: string;
};
export function emptyDetails(): CatalogDetails {
  return {
    tyre_width: null,
    tyre_width_unit: "",
    aspect_ratio: null,
    rim_diameter: null,
    sku: "",
    kind: "tyre",
    description: "",
    brand: "",
    model: "",
    production_year: null,
    size: "",
    season: "",
    tread_percent: null,
    load_index: "",
    speed_index: "",
    location: "",
    condition: "",
    diameter: null,
    width: null,
    wheel_offset: null,
    pcd: "",
    wheel_type: "",
    center_bore: null,
    units_per_sale: 1,
    review_notes: "",
  };
}
export const unitLabel = (d?: CatalogDetails) =>
  (d?.units_per_sale || 1) > 1 ? `комплект (${d!.units_per_sale} шт.)` : "шт.";
export const detailLabels: [keyof CatalogDetails, string, boolean?][] = [
  ["sku", "Артикул"],
  ["brand", "Бренд"],
  ["model", "Модель"],
  ["production_year", "Год выпуска", true],
  ["size", "Размер / маркировка"],
  ["tyre_width", "Ширина шины", true],
  ["aspect_ratio", "Профиль шины, %", true],
  ["rim_diameter", "Посадочный диаметр шины, дюймы", true],
  ["season", "Сезонность"],
  ["tread_percent", "Протектор, %", true],
  ["load_index", "Нагрузка / PR"],
  ["speed_index", "Индекс скорости"],
  ["condition", "Состояние"],
  ["diameter", "Диаметр диска, дюймы", true],
  ["width", "Ширина диска, дюймы", true],
  ["wheel_offset", "Вылет ET, мм", true],
  ["pcd", "Сверловка PCD"],
  ["wheel_type", "Тип диска"],
  ["center_bore", "Диаметр ЦО, мм", true],
  ["location", "Место хранения"],
];
const tyreKeys = [
  "season",
  "tread_percent",
  "load_index",
  "speed_index",
  "tyre_width",
  "tyre_width_unit",
  "aspect_ratio",
  "rim_diameter",
];
const wheelKeys = [
  "diameter",
  "width",
  "wheel_offset",
  "pcd",
  "wheel_type",
  "center_bore",
];
export function labelsFor(kind: string) {
  return detailLabels
    .filter(([key]) => !(kind === "wheel" ? tyreKeys : wheelKeys).includes(key))
    .map(([key, label, numeric]): [keyof CatalogDetails, string, boolean?] => [
      key,
      key === "size"
        ? kind === "wheel"
          ? "Маркировка диска"
          : "Размер шины"
        : label,
      numeric,
    ]);
}
export function detailValue(d: CatalogDetails, key: keyof CatalogDetails) {
  if (key === "tyre_width" && d.tyre_width != null)
    return (
      String(d.tyre_width) +
      (d.tyre_width_unit === "mm"
        ? " мм"
        : d.tyre_width_unit === "in"
          ? "″"
          : "")
    );
  return d[key];
}
export function changeProductKind(
  value: CatalogDetails,
  kind: string,
): CatalogDetails {
  const next = { ...value, kind, size: "" };
  const defaults = emptyDetails();
  for (const key of [...tyreKeys, ...wheelKeys] as (keyof CatalogDetails)[])
    Object.assign(next, { [key]: defaults[key] });
  return next;
}
export function ProductPhotos({
  images,
  name,
  fallback,
  href,
}: {
  images?: { url: string }[];
  name: string;
  fallback: ReactNode;
  href?: string;
}) {
  const [index, setIndex] = useState(0);
  const [failed, setFailed] = useState(false);
  const gesture = useRef<{ id: number; x: number; y: number } | null>(null);
  const suppressClickUntil = useRef(0);
  if (!images?.length || failed) return <>{fallback}</>;
  return (
    <div
      className="catalog-photos"
      onPointerDown={(e) => {
        if (
          e.isPrimary &&
          e.pointerType !== "mouse" &&
          !(e.target as HTMLElement).closest("button")
        )
          gesture.current = { id: e.pointerId, x: e.clientX, y: e.clientY };
      }}
      onPointerMove={(e) => {
        const g = gesture.current;
        if (
          g?.id === e.pointerId &&
          Math.abs(e.clientX - g.x) > 12 &&
          Math.abs(e.clientX - g.x) > Math.abs(e.clientY - g.y) * 1.3
        )
          e.currentTarget.setPointerCapture(e.pointerId);
      }}
      onPointerUp={(e) => {
        const g = gesture.current;
        gesture.current = null;
        if (!g || g.id !== e.pointerId) return;
        const dx = e.clientX - g.x,
          dy = e.clientY - g.y;
        if (
          images.length > 1 &&
          Math.abs(dx) >= 45 &&
          Math.abs(dx) > Math.abs(dy) * 1.3
        ) {
          setIndex(
            (i) => (i + (dx < 0 ? 1 : images.length - 1)) % images.length,
          );
          suppressClickUntil.current = performance.now() + 500;
          e.preventDefault();
        }
      }}
      onPointerCancel={() => {
        gesture.current = null;
      }}
      onClickCapture={(e) => {
        if (performance.now() < suppressClickUntil.current) {
          e.preventDefault();
          e.stopPropagation();
        }
      }}
    >
      <a
        href={href || images[index]?.url}
        target={href ? undefined : "_blank"}
        rel={href ? undefined : "noreferrer"}
        aria-label={href ? `Открыть: ${name}` : undefined}
        className="catalog-photo-link"
      >
        <img
          src={images[index]?.url}
          alt={`${name} — фото ${index + 1}`}
          width={1280}
          height={1280}
          loading="lazy"
          draggable={false}
          onError={() => setFailed(true)}
        />
      </a>
      {images.length > 1 && (
        <div className="catalog-photo-controls">
          <button
            type="button"
            aria-label={`Предыдущее фото: ${name}`}
            onClick={() =>
              setIndex((index + images.length - 1) % images.length)
            }
          >
            <svg
              viewBox="0 0 24 24"
              aria-hidden="true"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="m15 6-6 6 6 6" />
            </svg>
          </button>
          <span>
            {index + 1} / {images.length}
          </span>
          <button
            type="button"
            aria-label={`Следующее фото: ${name}`}
            onClick={() => setIndex((index + 1) % images.length)}
          >
            <svg
              viewBox="0 0 24 24"
              aria-hidden="true"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="m9 6 6 6-6 6" />
            </svg>
          </button>
        </div>
      )}
    </div>
  );
}
export function ProductSpecifications({
  details,
}: {
  details?: CatalogDetails;
}) {
  if (!details?.sku) return null;
  return (
    <details className="catalog-specs">
      <summary>Характеристики и описание</summary>
      <dl>
        {labelsFor(details.kind)
          .filter(
            ([key]) =>
              key !== "location" &&
              details[key] !== null &&
              details[key] !== "",
          )
          .map(([key, label]) => (
            <div key={key}>
              <dt>{label}</dt>
              <dd>{detailValue(details, key)}</dd>
            </div>
          ))}
      </dl>
      {details.description && <p>{details.description}</p>}
    </details>
  );
}
export function DetailsEditor({
  value,
  onChange,
  existing,
}: {
  value: CatalogDetails;
  onChange: (d: CatalogDetails) => void;
  existing: boolean;
}) {
  return (
    <details open className="admin-characteristics">
      <summary>Редактировать характеристики товара</summary>
      <label>
        Категория
        <select
          aria-label="Категория"
          value={value.kind}
          onChange={(e) => onChange(changeProductKind(value, e.target.value))}
        >
          <option value="tyre">Шина</option>
          <option value="wheel">Диск</option>
        </select>
      </label>
      <div className="admin-fields">
        {labelsFor(value.kind)
          .filter(
            ([key]) =>
              !["tyre_width", "aspect_ratio", "rim_diameter"].includes(key),
          )
          .map(([key, label, numeric]) => (
            <label key={key}>
              {label}
              <input
                type={numeric ? "number" : "text"}
                step="any"
                value={value[key] ?? ""}
                onChange={(e) =>
                  onChange({
                    ...value,
                    [key]: numeric
                      ? e.target.value === ""
                        ? null
                        : e.target.valueAsNumber
                      : e.target.value,
                  })
                }
              />
            </label>
          ))}
      </div>
      {value.kind === "tyre" && (
        <p className="fineprint">
          Ширина, профиль и посадочный диаметр определяются по размеру шины. Для
          неизвестного формата параметры остаются пустыми.
        </p>
      )}
      <label>
        Штук в единице продажи
        <input
          type="number"
          min={1}
          max={100}
          step={1}
          value={value.units_per_sale}
          disabled={existing}
          onChange={(e) =>
            onChange({ ...value, units_per_sale: e.target.valueAsNumber })
          }
        />
      </label>
      <label>
        Описание
        <textarea
          value={value.description}
          onChange={(e) => onChange({ ...value, description: e.target.value })}
        />
      </label>
      <label>
        Замечания для проверки
        <textarea
          value={value.review_notes}
          onChange={(e) => onChange({ ...value, review_notes: e.target.value })}
        />
      </label>
    </details>
  );
}
