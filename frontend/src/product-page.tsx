import {
  ProductPhotos,
  labelsFor,
  detailValue,
  type CatalogDetails,
} from "./catalog-details";
type Product = {
  id: string;
  name: string;
  unit_price_rub: number;
  available: boolean;
  stock?: number;
  unit_label?: string;
  details?: CatalogDetails;
  images?: { url: string }[];
};
export function ProductPage({
  product,
  quantity,
  setQuantity,
  add,
  buy,
  locked,
  fallback,
}: {
  product: Product;
  quantity: number;
  setQuantity: (n: number) => void;
  add: () => void;
  buy: () => void;
  locked: boolean;
  fallback: React.ReactNode;
}) {
  const p = product;
  return (
    <section className="product-page">
      <nav className="breadcrumbs">
        <a href="/">Каталог</a>
        <span>/</span>
        {p.details?.sku || p.name}
      </nav>
      <div className="product-page-grid">
        <div className="product-page-gallery">
          <ProductPhotos
            key={p.id}
            images={p.images}
            name={p.name}
            fallback={fallback}
          />
          {!!p.images?.length && (
            <p>
              <a href={p.images[0].url} target="_blank" rel="noreferrer">
                Открыть полный кадр ↗
              </a>
            </p>
          )}
        </div>
        <div className="product-page-info">
          <p className="eyebrow">
            {p.details?.kind === "wheel" ? "ГРУЗОВЫЕ ДИСКИ" : "ГРУЗОВЫЕ ШИНЫ"}
          </p>
          <h1>{p.name}</h1>
          <p className="muted">Артикул: {p.details?.sku || "—"}</p>
          <p className="product-page-price">
            {new Intl.NumberFormat("ru-RU").format(p.unit_price_rub)} ₽{" "}
            <small>/ {p.unit_label || "шт."}</small>
          </p>
          <p>
            {p.available
              ? `В наличии: ${p.stock ?? "уточняйте"} ${p.unit_label || "шт."}`
              : "Товар снят с продажи"}
          </p>
          <label>
            Количество
            <input
              type="number"
              aria-label={`Количество: ${p.name}`}
              min={1}
              max={10000}
              step={1}
              value={quantity}
              disabled={locked || !p.available}
              onChange={(e) => {
                const n = e.target.valueAsNumber;
                if (Number.isInteger(n) && n >= 1 && n <= 10000) setQuantity(n);
              }}
            />
          </label>
          <div className="product-page-actions">
            <button
              className="add-button"
              disabled={locked || !p.available}
              onClick={add}
            >
              В корзину
            </button>
            <button
              className="buy-now"
              disabled={locked || !p.available}
              onClick={buy}
            >
              Купить сейчас ↗
            </button>
          </div>
          <p className="muted">
            Оставьте заявку — сотрудник свяжется с вами для согласования.
          </p>
        </div>
      </div>
      <div className="product-page-specifications">
        <h2>Характеристики и описание</h2>
        <div className="product-page-details-grid">
          <table
            className="product-specification-table"
            aria-label="Характеристики товара"
          >
            <tbody>
              {p.details &&
                labelsFor(p.details.kind)
                  .filter(
                    ([key]) =>
                      key !== "location" &&
                      p.details![key] !== null &&
                      p.details![key] !== "",
                  )
                  .map(([key, label]) => (
                    <tr key={key}>
                      <th scope="row">{label}</th>
                      <td>{detailValue(p.details!, key)}</td>
                    </tr>
                  ))}
            </tbody>
          </table>
          <section className="product-description">
            <h3>Описание товара</h3>
            <p>{p.details?.description || "Описание пока не добавлено."}</p>
          </section>
        </div>
      </div>
    </section>
  );
}
