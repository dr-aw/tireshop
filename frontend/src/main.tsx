import { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import "./style.css";

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

function Wheel({ rim = false }: { rim?: boolean }) {
  return (
    <svg viewBox="0 0 200 180" aria-hidden="true">
      <ellipse cx="100" cy="158" rx="64" ry="9" fill="#e5e7eb" />
      <g transform="translate(100 84) rotate(-15)">
        <ellipse rx="60" ry="73" fill="#24272b" />
        <ellipse
          rx="51"
          ry="65"
          fill="none"
          stroke="#42464b"
          strokeWidth="5"
          strokeDasharray="3 5"
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

  return (
    <>
      <div className="topline">
        <div className="wrap">
          Шины и диски для коммерческого транспорта{" "}
          <span>Без онлайн-оплаты</span>
        </div>
      </div>
      <header className="wrap">
        <a className="logo" href="/">
          tire<span>shop</span>
          <small>ШИНЫ И ДИСКИ</small>
        </a>
        <nav>
          <a href="#catalog">Каталог</a>
          <a href="#checkout">
            Оформить заявку <b>{cart.reduce((s, l) => s + l.quantity, 0)}</b>
          </a>
        </nav>
      </header>
      <main className="wrap">
        <div className="intro">
          <div>
            <p className="eyebrow">КАТАЛОГ / ГРУЗОВОЙ ТРАНСПОРТ</p>
            <h1>
              Надёжность в каждом
              <br />
              километре.
            </h1>
            <p>
              Выберите товары и оставьте контакты.
              <br />
              Мы уточним наличие и условия доставки.
            </p>
          </div>
          <div className="intro-mark" aria-hidden="true">
            22.5<span>ШИНЫ · ДИСКИ</span>
          </div>
        </div>
        <p className="demo">
          Демонстрационный каталог · синтетические товары и цены
        </p>
        <div className="layout">
          <section id="catalog">
            <div className="section-title">
              <h2>Шины и диски</h2>
              <span>{products.length} товара</span>
            </div>
            {catalogError && <p role="alert">{catalogError}</p>}
            {!products.length && !catalogError && <p>Загружаем каталог…</p>}
            <div className="products">
              {products.map((p) => (
                <article className="product" key={p.id}>
                  <div className="product-image">
                    <span className="tag">ДЕМО</span>
                    <Wheel rim={p.name.startsWith("Диск")} />
                  </div>
                  <div className="product-body">
                    <p className="category">
                      {p.name.startsWith("Диск")
                        ? "Грузовой диск"
                        : "Грузовая шина"}
                    </p>
                    <h3>{p.name.replace(" — демонстрационный товар", "")}</h3>
                    <p className="price">
                      {rub(p.unit_price_rub)} <small>/ шт.</small>
                    </p>
                    <label className="quantity">
                      Количество, шт.
                      <input
                        aria-label={`Количество: ${p.name}`}
                        type="number"
                        min="1"
                        max="10000"
                        step="1"
                        disabled={locked}
                        value={quantities[p.id] || 1}
                        onChange={(e) =>
                          setQuantities({
                            ...quantities,
                            [p.id]: Math.max(
                              1,
                              Math.min(
                                10000,
                                Math.trunc(Number(e.target.value)) || 1,
                              ),
                            ),
                          })
                        }
                      />
                    </label>
                    <button disabled={locked} onClick={() => add(p, false)}>
                      В корзину
                    </button>
                    <button
                      className="text-button"
                      disabled={locked}
                      onClick={() => add(p, true)}
                    >
                      Купить сейчас →
                    </button>
                  </div>
                </article>
              ))}
            </div>
            <div className="assurances">
              <div>
                <b>Цена за штуку</b>
                <p>Все суммы в рублях, без копеек.</p>
              </div>
              <div>
                <b>Одна заявка на корзину</b>
                <p>Детали согласуем лично.</p>
              </div>
            </div>
          </section>
          <aside id="checkout">
            <h2>
              {quick || attempt?.source === "quick"
                ? "Быстрая заявка"
                : "Ваша корзина"}
            </h2>
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
              <>
                {!selected.length ? (
                  <p className="muted">
                    Добавьте товары из каталога — они появятся здесь.
                  </p>
                ) : (
                  <ul className="basket">
                    {selected.map((l) => (
                      <li key={l.product_id}>
                        <b>{l.name.replace(" — демонстрационный товар", "")}</b>
                        <div>
                          <span>
                            {l.quantity} шт. × {rub(l.unit_price_rub)}
                          </span>
                          {!locked && (
                            <button
                              className="remove"
                              aria-label={`Убрать ${l.name}`}
                              onClick={() =>
                                quick
                                  ? setQuick(null)
                                  : setCart(
                                      cart.filter(
                                        (x) => x.product_id !== l.product_id,
                                      ),
                                    )
                              }
                            >
                              ×
                            </button>
                          )}
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
                <div className="total">
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
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    submit();
                  }}
                >
                  <fieldset disabled={locked}>
                    <legend>Контакты для связи</legend>
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
                          {l.name}: {l.quantity} × {rub(l.unit_price_rub)}
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
                    Это заявка на покупку, без оплаты и автоматического резерва.
                    Наличие и доставку подтвердит сотрудник.
                  </p>
                </form>
              </>
            )}
          </aside>
        </div>
      </main>
      <footer className="wrap">
        <b>tireshop</b>
        <span>Шины и диски · демонстрационная версия</span>
      </footer>
    </>
  );
}
createRoot(document.getElementById("root")!).render(<App />);
