import { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import "./style.css";
import { Administrator } from "./administrator";

type Product = {
  id: string;
  name: string;
  unit_price_rub: number;
  available: boolean;
};
type Line = {
  product_id: string;
  name: string;
  quantity: number;
  unit_price_rub: number;
};
type Payload = {
  customer_name: string;
  phone: string;
  comment: string;
  items: {
    product_id: string;
    quantity: number;
    expected_unit_price_rub: number;
  }[];
};
type Attempt = {
  key: string;
  payload: Payload;
  lines: Line[];
  source: "cart" | "quick";
};
type Saved = { attempt: Attempt } | { receipt: string };
const storageKey = "tireshop.checkout.v1";
const rub = (n: number) => `${new Intl.NumberFormat("ru-RU").format(n)} ₽`;
const total = (lines: Line[]) =>
  lines.reduce((sum, line) => sum + line.quantity * line.unit_price_rub, 0);

function readSaved(): { saved?: Saved; error?: string } {
  try {
    const raw = sessionStorage.getItem(storageKey);
    if (!raw) return {};
    const saved = JSON.parse(raw);
    if (typeof saved.receipt === "string") return { saved };
    const a = saved.attempt;
    if (
      typeof a?.key !== "string" ||
      !Array.isArray(a.lines) ||
      !Array.isArray(a.payload?.items) ||
      typeof a.payload.customer_name !== "string" ||
      typeof a.payload.phone !== "string" ||
      typeof a.payload.comment !== "string" ||
      !["cart", "quick"].includes(a.source) ||
      !a.lines.every(
        (l: Line) =>
          typeof l.name === "string" &&
          Number.isSafeInteger(l.quantity) &&
          Number.isSafeInteger(l.unit_price_rub),
      )
    )
      throw Error();
    return { saved };
  } catch {
    // Never silently replace a potentially committed attempt after storage fails.
    return {
      error:
        "Не удалось восстановить отправку. Не отправляйте заявку повторно: свяжитесь с магазином для уточнения.",
    };
  }
}

const productName = (name: string) =>
  name.replace(" — демонстрационный товар", "");

// Both catalog and checkout use the same limits. Reaching one never removes a
// line: deletion is a separate action, so a stray click cannot lose the item.
function QuantityControl({
  value,
  disabled,
  label,
  onChange,
}: {
  value: number;
  disabled: boolean;
  label: string;
  onChange: (value: number) => void;
}) {
  return (
    <div className="stepper" role="group" aria-label={label}>
      <button
        type="button"
        aria-label={`Уменьшить: ${label}`}
        disabled={disabled || value <= 1}
        onClick={() => onChange(value - 1)}
      >
        −
      </button>
      <input
        aria-label={label}
        type="number"
        min="1"
        max="10000"
        step="1"
        value={value}
        disabled={disabled}
        onChange={(e) =>
          onChange(
            Math.max(
              1,
              Math.min(10000, Math.trunc(Number(e.target.value)) || 1),
            ),
          )
        }
      />
      <button
        type="button"
        aria-label={`Увеличить: ${label}`}
        disabled={disabled || value >= 10000}
        onClick={() => onChange(value + 1)}
      >
        +
      </button>
    </div>
  );
}

function Wheel({ rim = false }: { rim?: boolean }) {
  return (
    <svg viewBox="0 0 200 180" aria-hidden="true">
      <ellipse cx="100" cy="158" rx="64" ry="9" fill="#e5e7eb" />
      <g transform="translate(100 84) rotate(-15)">
        <ellipse rx="60" ry="73" fill={rim ? "#b6bdc5" : "#24272b"} />
        <ellipse
          rx="51"
          ry="65"
          fill="none"
          stroke={rim ? "#e7e9ec" : "#42464b"}
          strokeWidth="5"
          strokeDasharray={rim ? undefined : "3 5"}
        />
        <ellipse rx="35" ry="46" fill={rim ? "#c7cbd1" : "#51565d"} />
        <ellipse rx="27" ry="37" fill="#dce0e5" />
        {[0, 60, 120, 180, 240, 300].map((a) => (
          <ellipse
            key={a}
            cy="-23"
            rx="5"
            ry="9"
            fill="#42464b"
            transform={`rotate(${a})`}
          />
        ))}
        <circle r="11" fill="#8e969f" />
        <circle r="4" fill="#292d32" />
      </g>
    </svg>
  );
}

function App() {
  const [initial] = useState(readSaved);
  const [products, setProducts] = useState<Product[]>([]);
  const [catalogError, setCatalogError] = useState("");
  const [category, setCategory] = useState("all");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState("default");
  const [quantities, setQuantities] = useState<Record<string, number>>({});
  const [cart, setCart] = useState<Line[]>([]);
  const [quick, setQuick] = useState<Line[] | null>(null);
  const [attempt, setAttempt] = useState<Attempt | undefined>(
    initial.saved && "attempt" in initial.saved
      ? initial.saved.attempt
      : undefined,
  );
  const [receipt, setReceipt] = useState(
    initial.saved && "receipt" in initial.saved ? initial.saved.receipt : "",
  );
  const [name, setName] = useState(attempt?.payload.customer_name || "");
  const [phone, setPhone] = useState(attempt?.payload.phone || "");
  const [comment, setComment] = useState(attempt?.payload.comment || "");
  const [message, setMessage] = useState(
    initial.error ||
      (attempt
        ? "Есть незавершённая отправка. Повторите её с прежними данными."
        : ""),
  );
  const [blocked, setBlocked] = useState(Boolean(initial.error));
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);
  const [newPrices, setNewPrices] = useState<Line[] | null>(null);
  const [canEdit, setCanEdit] = useState(false);
  const selected = attempt?.lines || quick || cart;
  const locked = Boolean(attempt) || busy || blocked;

  useEffect(() => {
    const controller = new AbortController();
    fetch("/api/products", { signal: controller.signal })
      .then(async (r) => {
        if (!r.ok) throw Error();
        const data = await r.json();
        setProducts(data.items);
      })
      .catch((e) => {
        if (e.name !== "AbortError")
          setCatalogError(
            "Каталог временно недоступен. Обновите страницу чуть позже.",
          );
      });
    return () => controller.abort();
  }, []);

  function persist(value: Saved) {
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(value));
      return true;
    } catch {
      setBlocked(true);
      setMessage(
        "Браузер не сохраняет отправку. Разрешите хранилище этой вкладки перед оформлением.",
      );
      return false;
    }
  }

  async function send(a: Attempt) {
    // Save BEFORE the network call. A timeout or reload must reuse both the key
    // and the exact payload: generating a new key could duplicate a committed order.
    if (inFlight.current || !persist({ attempt: a })) return;
    inFlight.current = true;
    setAttempt(a);
    setBusy(true);
    setCanEdit(false);
    setNewPrices(null);
    setMessage("Отправляем заявку…");
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 10000);
    try {
      const response = await fetch("/api/requests", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": a.key,
        },
        body: JSON.stringify(a.payload),
        signal: controller.signal,
      });
      const data = await response.json();
      if (response.status === 201 && typeof data.request_id === "string") {
        // Remove contacts from browser storage once a durable receipt is known.
        if (!persist({ receipt: data.request_id })) return;
        setReceipt(data.request_id);
        setAttempt(undefined);
        setName("");
        setPhone("");
        setComment("");
        if (a.source === "cart") setCart([]);
        setQuick(null);
        setMessage("");
      } else if (
        response.status === 409 &&
        data.error?.code === "price_changed"
      ) {
        setNewPrices(data.items);
        setCanEdit(true);
        setMessage(
          "Цена изменилась. Проверьте новую сумму и подтвердите отправку.",
        );
      } else {
        // Only definite business/validation failures allow editing. Unknown
        // responses, rate limits and 5xx retain the original attempt for retries.
        setCanEdit(
          ["validation_error", "product_unavailable", "invalid_json"].includes(
            data.error?.code,
          ),
        );
        setMessage(
          data.error?.message ||
            "Приём не подтверждён. Повторите ту же отправку.",
        );
      }
    } catch {
      setMessage(
        "Ответ не получен. Заявка могла сохраниться. Повторите отправку — номер попытки останется прежним.",
      );
    } finally {
      clearTimeout(timer);
      inFlight.current = false;
      setBusy(false);
    }
  }

  function submit(lines = selected) {
    if (attempt && !newPrices) {
      void send(attempt);
      return;
    }
    const payload: Payload = {
      customer_name: name,
      phone,
      comment,
      items: lines.map((l) => ({
        product_id: l.product_id,
        quantity: l.quantity,
        expected_unit_price_rub: l.unit_price_rub,
      })),
    };
    // A confirmed new quote is a NEW business attempt; the old key retains its
    // original price_changed result and must never be reused with changed data.
    void send({
      key: crypto.randomUUID(),
      payload,
      lines,
      source: attempt?.source || (quick ? "quick" : "cart"),
    });
  }

  function add(p: Product, buyNow: boolean) {
    const line = {
      product_id: p.id,
      name: p.name,
      quantity: quantities[p.id] || 1,
      unit_price_rub: p.unit_price_rub,
    };
    setReceipt("");
    setMessage("");
    if (buyNow) {
      setQuick([line]);
      document
        .getElementById("checkout")
        ?.scrollIntoView({ behavior: "smooth" });
    } else {
      setQuick(null);
      setCart((old) => [
        ...old.filter((l) => l.product_id !== p.id),
        {
          ...line,
          quantity: Math.min(
            10000,
            line.quantity +
              (old.find((l) => l.product_id === p.id)?.quantity || 0),
          ),
        },
      ]);
    }
  }

  function edit() {
    try {
      sessionStorage.removeItem(storageKey);
    } catch {
      setBlocked(true);
      return;
    }
    if (attempt?.source === "quick") setQuick(attempt.lines);
    else setCart(attempt?.lines || []);
    setAttempt(undefined);
    setNewPrices(null);
    setCanEdit(false);
    setMessage("");
  }

  // Keep basket edits local until submission. Once an attempt exists, its
  // saved payload is immutable even if the previous HTTP response was lost.
  function changeQuantity(id: string, quantity: number) {
    if (locked) return;
    const update = (lines: Line[]) =>
      lines.map((line) =>
        line.product_id === id ? { ...line, quantity } : line,
      );
    if (quick) setQuick((previous) => (previous ? update(previous) : null));
    else setCart(update);
  }
  const visible = products
    .filter(
      (p) =>
        (category === "all" ||
          (category === "rims"
            ? p.name.startsWith("Диск")
            : !p.name.startsWith("Диск"))) &&
        productName(p.name)
          .toLocaleLowerCase("ru")
          .includes(search.trim().toLocaleLowerCase("ru")),
    )
    .sort((a, b) =>
      sort === "low"
        ? a.unit_price_rub - b.unit_price_rub
        : sort === "high"
          ? b.unit_price_rub - a.unit_price_rub
          : 0,
    );
  const cartCount = cart.reduce((sum, line) => sum + line.quantity, 0);

  return (
    <>
      <div className="topline">
        <div className="wrap">
          <span>Шины и диски для грузового транспорта</span>
          <a href="#checkout">Заказать с обратным звонком ↗</a>
        </div>
      </div>
      <header className="site-header">
        <div className="wrap header-inner">
          <a className="logo" href="/" aria-label="tireshop — главная">
            <span className="logo-symbol" aria-hidden="true">
              ▰
            </span>
            tire<span>shop</span>
            <small>ГРУЗОВЫЕ ШИНЫ И ДИСКИ</small>
          </a>
          <nav aria-label="Главное меню">
            <a className="active" href="#catalog">
              Каталог
            </a>
            <a href="#how-to-order">Как заказать</a>
          </nav>
          <a className="cart-link" href="#checkout">
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.7"
              aria-hidden="true"
            >
              <path d="M3 3h2l3 13h11l2-9H6" />
              <circle cx="9" cy="20" r="1" />
              <circle cx="18" cy="20" r="1" />
            </svg>
            Корзина <b>{cartCount}</b>
          </a>
        </div>
      </header>
      <main className="wrap">
        <div className="breadcrumbs">
          <a href="/">Главная</a>
          <span>/</span>Каталог
        </div>
        <div className="page-heading">
          <div>
            <p className="eyebrow">ДЛЯ ГРУЗОВОГО ТРАНСПОРТА</p>
            <h1>
              Шины и диски<span>.</span>
            </h1>
          </div>
          <p>
            Выберите размер.
            <br />
            Остальное согласуем при звонке.
          </p>
        </div>
        <div className="catalog-layout" id="catalog">
          <aside className="filters" aria-label="Категории каталога">
            <h2>Каталог</h2>
            {[
              ["all", "Все товары"],
              ["tyres", "Грузовые шины"],
              ["rims", "Грузовые диски"],
            ].map(([value, label]) => (
              <button
                key={value}
                className={
                  category === value
                    ? "category-link selected"
                    : "category-link"
                }
                aria-pressed={category === value}
                onClick={() => setCategory(value)}
              >
                <span>{label}</span>
                <span aria-hidden="true">↗</span>
              </button>
            ))}
            <div className="selection-note">
              <span className="note-icon" aria-hidden="true">
                ↗
              </span>
              <h3>
                Нужна помощь
                <br />с выбором?
              </h3>
              <p>
                Добавьте вопрос в комментарий к заявке — разберём детали вместе.
              </p>
              <a href="#checkout">К оформлению →</a>
            </div>
          </aside>
          <section className="catalog-results" aria-label="Товары">
            <div className="catalog-toolbar">
              <label className="search">
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.7"
                  aria-hidden="true"
                >
                  <circle cx="10" cy="10" r="7" />
                  <path d="m15 15 6 6" />
                </svg>
                <input
                  type="search"
                  aria-label="Поиск по каталогу"
                  placeholder="Название или размер, например 315/70"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </label>
              <label className="sort">
                <span>Сортировка</span>
                <select
                  aria-label="Сортировка"
                  value={sort}
                  onChange={(e) => setSort(e.target.value)}
                >
                  <option value="default">По умолчанию</option>
                  <option value="low">Сначала дешевле</option>
                  <option value="high">Сначала дороже</option>
                </select>
              </label>
            </div>
            <div className="results-heading">
              <span>
                Найдено: <b>{visible.length}</b>
              </span>
              <span>Цена за 1 шт. · ₽</span>
            </div>
            {catalogError && (
              <p role="alert" className="notice">
                {catalogError}
              </p>
            )}
            {!products.length && !catalogError && <p>Загружаем каталог…</p>}
            {products.length > 0 && !visible.length && (
              <div className="empty-results">
                <h3>По вашему запросу ничего не найдено</h3>
                <button
                  className="secondary"
                  onClick={() => {
                    setSearch("");
                    setCategory("all");
                  }}
                >
                  Сбросить поиск
                </button>
              </div>
            )}
            <div className="products">
              {visible.map((p) => (
                <article className="product" key={p.id}>
                  <div className="product-image">
                    <span className="product-type">
                      {p.name.startsWith("Диск") ? "ДИСК" : "ШИНА"}
                    </span>
                    <Wheel rim={p.name.startsWith("Диск")} />
                  </div>
                  <div className="product-body">
                    <p className="category">
                      {p.name.startsWith("Диск")
                        ? "Грузовой диск"
                        : "Грузовая шина"}
                    </p>
                    <h3>{productName(p.name)}</h3>
                    <p className="price">
                      {rub(p.unit_price_rub)} <small>/ шт.</small>
                    </p>
                    <div className="purchase-row">
                      <QuantityControl
                        value={quantities[p.id] || 1}
                        disabled={locked}
                        label={`Количество: ${productName(p.name)}`}
                        onChange={(value) =>
                          setQuantities((prev) => ({ ...prev, [p.id]: value }))
                        }
                      />
                      <button
                        className="add-button"
                        disabled={locked}
                        onClick={() => add(p, false)}
                      >
                        В корзину
                      </button>
                    </div>
                    <button
                      className="buy-now"
                      disabled={locked}
                      onClick={() => add(p, true)}
                    >
                      Купить сейчас <span aria-hidden="true">↗</span>
                    </button>
                  </div>
                </article>
              ))}
            </div>
          </section>
        </div>
        <section id="checkout" className="checkout">
          <div className="checkout-heading">
            <div>
              <p className="eyebrow">ВАШ ВЫБОР</p>
              <h2>
                {quick || attempt?.source === "quick"
                  ? "Быстрая заявка"
                  : "Ваша корзина"}
              </h2>
            </div>
            <span>
              {selected.reduce((sum, line) => sum + line.quantity, 0)} шт.
            </span>
          </div>
          {receipt ? (
            <div className="receipt" role="status">
              <span className="check">✓</span>
              <h3>Заявка получена</h3>
              <p>Мы свяжемся с вами по указанному телефону.</p>
              <small>Номер заявки</small>
              <code>{receipt}</code>
              <button
                className="secondary"
                onClick={() => {
                  try {
                    sessionStorage.removeItem(storageKey);
                    setReceipt("");
                  } catch {
                    setMessage("Не удалось очистить подтверждение.");
                  }
                }}
              >
                Вернуться к покупкам
              </button>
            </div>
          ) : (
            <div className="checkout-content">
              <div className="basket-panel">
                {!selected.length ? (
                  <div className="empty-basket">
                    <span aria-hidden="true">＋</span>
                    <h3>Пока ничего не добавлено</h3>
                    <p>Выберите шины или диски в каталоге.</p>
                    <a href="#catalog">Перейти к товарам ↑</a>
                  </div>
                ) : (
                  <ul className="basket">
                    {selected.map((l) => (
                      <li key={l.product_id}>
                        <div className="basket-image">
                          <Wheel rim={l.name.startsWith("Диск")} />
                        </div>
                        <div className="basket-description">
                          <b>{productName(l.name)}</b>
                          <span>{rub(l.unit_price_rub)} / шт.</span>
                        </div>
                        <div className="basket-actions">
                          <QuantityControl
                            value={l.quantity}
                            disabled={locked}
                            label={`В корзине: ${productName(l.name)}`}
                            onChange={(quantity) =>
                              changeQuantity(l.product_id, quantity)
                            }
                          />
                          <strong>{rub(l.quantity * l.unit_price_rub)}</strong>
                          <button
                            className="remove"
                            disabled={locked}
                            aria-label={`Убрать ${productName(l.name)}`}
                            onClick={() =>
                              quick
                                ? setQuick(null)
                                : setCart((prev) =>
                                    prev.filter(
                                      (x) => x.product_id !== l.product_id,
                                    ),
                                  )
                            }
                          >
                            Удалить
                          </button>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
                <div className="total" aria-live="polite">
                  <span>Итого</span>
                  <strong>{rub(total(selected))}</strong>
                </div>
                {quick && !locked && (
                  <button
                    className="text-button"
                    onClick={() => setQuick(null)}
                  >
                    Вернуться к корзине
                  </button>
                )}
                <p className="basket-caption">
                  Без предоплаты на сайте. Стоимость доставки уточним отдельно.
                </p>
              </div>{" "}
              <form
                className="contact-form"
                onSubmit={(e) => {
                  e.preventDefault();
                  submit();
                }}
              >
                <fieldset disabled={locked}>
                  <legend>Оформление заявки</legend>
                  <label>
                    Имя
                    <input
                      autoComplete="name"
                      required
                      maxLength={100}
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                    />
                  </label>
                  <label>
                    Телефон
                    <input
                      type="tel"
                      autoComplete="tel"
                      placeholder="+7 999 123-45-67"
                      pattern="\+[0-9 \(\)\-]{8,30}"
                      required
                      value={phone}
                      onChange={(e) => setPhone(e.target.value)}
                    />
                  </label>
                  <label>
                    Комментарий <span className="muted">· необязательно</span>
                    <textarea
                      maxLength={2000}
                      rows={2}
                      value={comment}
                      onChange={(e) => setComment(e.target.value)}
                    />
                  </label>
                </fieldset>
                {message && (
                  <p className="notice" role="status">
                    {message}
                  </p>
                )}
                {newPrices ? (
                  <div className="quote">
                    <h3>Обновлённые цены</h3>
                    {newPrices.map((l) => (
                      <p key={l.product_id}>
                        {productName(l.name)}: {l.quantity} ×{" "}
                        {rub(l.unit_price_rub)}
                      </p>
                    ))}
                    <strong>Итого: {rub(total(newPrices))}</strong>
                    <button
                      type="button"
                      disabled={busy || blocked}
                      onClick={() => submit(newPrices)}
                    >
                      Подтвердить {rub(total(newPrices))}
                    </button>
                  </div>
                ) : (
                  <button
                    type="submit"
                    disabled={!selected.length || busy || blocked}
                  >
                    {busy
                      ? "Отправляем…"
                      : attempt
                        ? "Повторить отправку"
                        : "Оформить заявку"}
                  </button>
                )}
                {canEdit && (
                  <button
                    type="button"
                    className="text-button"
                    disabled={busy || blocked}
                    onClick={edit}
                  >
                    Изменить заявку
                  </button>
                )}
                <p className="fineprint">
                  После отправки мы свяжемся с вами, чтобы подтвердить наличие и
                  согласовать доставку.
                </p>
              </form>
            </div>
          )}
        </section>
        <section className="how-to-order" id="how-to-order">
          <h2>От выбора до заказа</h2>
          <div>
            <span>01</span>
            <h3>Соберите корзину</h3>
            <p>Выберите товары и нужное количество.</p>
          </div>
          <div>
            <span>02</span>
            <h3>Оставьте контакты</h3>
            <p>Имя, телефон и пожелания к заказу.</p>
          </div>
          <div>
            <span>03</span>
            <h3>Дождитесь звонка</h3>
            <p>Подтвердим наличие и обсудим доставку.</p>
          </div>
        </section>
      </main>
      <footer>
        <div className="wrap footer-inner">
          <a className="logo" href="/">
            tire<span>shop</span>
          </a>
          <span>Грузовые шины и диски</span>
          <a href="#catalog">Каталог ↑</a>
        </div>
      </footer>
    </>
  );
}
createRoot(document.getElementById("root")!).render(
  /^\/administrator\/?$/.test(window.location.pathname) ? (
    <Administrator />
  ) : (
    <App />
  ),
);
