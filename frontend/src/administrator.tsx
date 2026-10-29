import { useEffect, useState, type FormEvent } from "react";
import "./administrator.css";
import {
  CatalogFilters,
  changeKind,
  type FilterValues,
} from "./catalog-filters";
import {
  ColumnPicker,
  TableHeaders,
  TableCells,
  readSettings,
  saveSettings,
  type TableSettings,
} from "./admin-table";
import {
  DetailsEditor,
  emptyDetails,
  unitLabel,
  type CatalogDetails,
} from "./catalog-details";

type Product = {
  image_url?: string;
  details?: CatalogDetails;
  id: string;
  name: string;
  unit_price_rub: number;
  stock: number;
  available: boolean;
  version: number;
};
type Item = {
  unit_label?: string;
  product_id: string;
  name: string;
  quantity: number;
  unit_price_rub: number;
};
type Request = {
  id: string;
  customer_name: string;
  phone: string;
  comment: string;
  status: string;
  version: number;
  total_rub: number;
  created_at: string;
  items: Item[];
};
type Detail = {
  request: Request;
  history: {
    actor: string;
    created_at: string;
    comment: string;
    before: Request;
    after: Request;
  }[];
};
const statuses: Record<string, string> = {
  created: "Создано",
  delivery: "Доставка",
  delivered: "Доставлено",
  cancelled: "Отменено",
};
const money = (n: number) =>
  new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency: "RUB",
    maximumFractionDigits: 0,
  }).format(n);
const date = (s: string) => new Date(s).toLocaleString("ru-RU");
class APIError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
  }
}
async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const response = await fetch(`/api/admin/${path}`, {
    method,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-Admin-Request": "1" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await response.json();
  if (!response.ok)
    throw new APIError(
      data.error?.message || "Не удалось выполнить запрос",
      response.status,
    );
  return data;
}

export function Administrator() {
  const [user, setUser] = useState<string | null>(null);
  const [checking, setChecking] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [tab, setTab] = useState("requests");
  const [kind, setKind] = useState("");
  const [productFilters, setProductFilters] = useState<FilterValues>({});
  const [productTotal, setProductTotal] = useState(0);
  const [requests, setRequests] = useState<Request[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [choices, setChoices] = useState<Product[]>([]);
  const [offset, setOffset] = useState(0);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [product, setProduct] = useState<Product | null>(null);
  const [settingsByUser, setSettingsByUser] = useState<
    Record<string, TableSettings>
  >({});
  const settings = settingsByUser[user || ""] || readSettings(user || "");
  function changeSettings(next: TableSettings) {
    setSettingsByUser((old) => ({ ...old, [user!]: next }));
    try {
      saveSettings(user!, next);
    } catch {
      setError("Не удалось сохранить настройки таблицы в браузере");
    }
  }

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await fn();
    } catch (e) {
      if (e instanceof APIError && e.status === 401) {
        setUser(null);
        setDetail(null);
        setProduct(null);
        setRequests([]);
        setProducts([]);
        setChoices([]);
      }
      setError(
        e instanceof Error
          ? e.message
          : "Нет связи с сервером. Обновите данные перед повтором.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function load(
    which: string,
    start: number,
    category = kind,
    order = settings,
    filters = productFilters,
  ) {
    if (which === "requests")
      setRequests(
        (await api<{ items: Request[] }>(`requests?offset=${start}`)).items,
      );
    else {
      const params = new URLSearchParams({
        ...filters,
        kind: category,
        offset: String(start),
        sort: order.sort,
        direction: order.direction,
      });
      const data = await api<{ items: Product[]; total: number }>(
        `products?${params}`,
      );
      setProducts(data.items);
      setProductTotal(data.total);
    }
    setOffset(start);
    setTab(which);
  }
  useEffect(() => {
    let active = true;
    api<{ username: string }>("session")
      .then((s) => {
        if (active) setUser(s.username);
      })
      .catch((e) => {
        if (active && (!(e instanceof APIError) || e.status !== 401))
          setError("Не удалось проверить сессию. Обновите страницу.");
      })
      .finally(() => {
        if (active) setChecking(false);
      });
    return () => {
      active = false;
    };
  }, []);
  useEffect(() => {
    if (user) void run(() => load("requests", 0));
  }, [user]);

  async function openRequest(id: string) {
    const d = await api<Detail>(`requests/${id}`);
    setDetail(d);
    setChoices([]);
    const all: Product[] = [];
    for (let start = 0; start <= 100000; start += 50) {
      const page = (await api<{ items: Product[] }>(`products?offset=${start}`))
        .items;
      all.push(...page);
      if (page.length < 50) break;
    }
    setChoices(all);
  }
  function login(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const data = new FormData(form);
    void run(async () => {
      const s = await api<{ username: string }>("login", "POST", {
        username: data.get("username"),
        password: data.get("password"),
      });
      form.reset();
      setUser(s.username);
    });
  }
  return (
    <div className="administrator">
      <header className="admin-header">
        <a href="/">
          tire<span>shop</span>
        </a>
        <strong>Администратор</strong>
        {user && (
          <div>
            {user}{" "}
            <button
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  await api("logout", "POST", {});
                  setUser(null);
                  setDetail(null);
                  setProduct(null);
                  setRequests([]);
                  setProducts([]);
                  setChoices([]);
                })
              }
            >
              Выйти
            </button>
          </div>
        )}
      </header>
      <main className="admin-main">
        {error && (
          <div className="admin-error" role="alert">
            {error}
          </div>
        )}
        {notice && (
          <div className="admin-notice" role="status">
            {notice}
          </div>
        )}
        {checking ? (
          <p>Проверяем сессию…</p>
        ) : !user ? (
          <form className="admin-card admin-login" onSubmit={login}>
            <h1>Вход для сотрудников</h1>
            <p>Заявки, каталог и остатки магазина</p>
            <label>
              Логин
              <input
                name="username"
                autoComplete="username"
                required
                maxLength={80}
              />
            </label>
            <label>
              Пароль
              <input
                name="password"
                type="password"
                autoComplete="current-password"
                required
                maxLength={1024}
              />
            </label>
            <button className="admin-primary" disabled={busy}>
              Войти
            </button>
          </form>
        ) : (
          <>
            <nav className="admin-nav">
              <button
                aria-pressed={tab === "requests"}
                disabled={busy}
                onClick={() =>
                  void run(async () => {
                    await load("requests", 0);
                    setProduct(null);
                  })
                }
              >
                Заявки
              </button>
              <button
                aria-pressed={tab === "products"}
                disabled={busy}
                onClick={() =>
                  void run(async () => {
                    await load("products", 0);
                    setDetail(null);
                  })
                }
              >
                Каталог и остатки
              </button>
              <button
                className="admin-refresh"
                disabled={busy}
                onClick={() =>
                  void run(async () => {
                    await load(tab, offset);
                    if (detail) await openRequest(detail.request.id);
                    setProduct(null);
                  })
                }
              >
                <span aria-hidden="true">⟳</span> Обновить данные
              </button>
            </nav>
            <div className="admin-heading">
              <div>
                <h1>
                  {tab === "requests"
                    ? "Заявки покупателей"
                    : "Каталог и остатки"}
                </h1>
                <p>
                  {tab === "requests"
                    ? "Передача в доставку списывает весь состав заявки со склада."
                    : "Скрытая позиция исчезает с витрины. История заявок сохраняется."}
                </p>
              </div>
              {tab === "products" && (
                <button
                  className="admin-primary"
                  disabled={busy}
                  onClick={() =>
                    setProduct({
                      id: crypto.randomUUID(),
                      name: "",
                      unit_price_rub: 0,
                      stock: 0,
                      available: true,
                      version: 0,
                      details: emptyDetails(),
                    })
                  }
                >
                  Добавить товар
                </button>
              )}
            </div>
            {tab === "products" && !product && (
              <div className="admin-categories" aria-label="Категория каталога">
                {[
                  ["", "Все товары"],
                  ["tyre", "Шины"],
                  ["wheel", "Диски"],
                ].map(([value, label]) => (
                  <button
                    key={value}
                    aria-pressed={kind === value}
                    disabled={busy}
                    onClick={() =>
                      void run(async () => {
                        const next = changeKind(productFilters, value);
                        await load("products", 0, value, settings, next);
                        setProductFilters(next);
                        setKind(value);
                      })
                    }
                  >
                    {label}
                  </button>
                ))}
              </div>
            )}
            {tab === "products" && !product && (
              <>
                <CatalogFilters
                  admin
                  disabled={busy}
                  values={{ ...productFilters, kind }}
                  apply={(next) =>
                    void run(async () => {
                      await load("products", 0, kind, settings, next);
                      setProductFilters(next);
                    })
                  }
                />
                <p className="admin-result-count">Найдено: {productTotal}</p>
                <ColumnPicker settings={settings} change={changeSettings} />
              </>
            )}
            <div
              className="admin-card admin-table-wrap"
              hidden={!!detail || !!product}
            >
              <table>
                <thead>
                  <tr>
                    {tab === "requests" ? (
                      [
                        "Заявка / дата",
                        "Покупатель",
                        "Статус",
                        "Сумма",
                        "",
                      ].map((h, i) => <th key={i}>{h}</th>)
                    ) : (
                      <TableHeaders
                        settings={settings}
                        busy={busy}
                        sort={(key) =>
                          void run(async () => {
                            const next = {
                              ...settings,
                              sort: key,
                              direction:
                                settings.sort === key &&
                                settings.direction === "asc"
                                  ? ("desc" as const)
                                  : ("asc" as const),
                            };
                            await load("products", 0, kind, next);
                            changeSettings(next);
                          })
                        }
                      />
                    )}
                  </tr>
                </thead>
                <tbody>
                  {tab === "requests"
                    ? requests.map((r) => (
                        <tr key={r.id}>
                          <td>
                            <strong>№ {r.id.slice(0, 8)}</strong>
                            <small>{date(r.created_at)}</small>
                          </td>
                          <td>
                            {r.customer_name}
                            <small>{r.phone}</small>
                          </td>
                          <td>
                            <span className={`admin-status ${r.status}`}>
                              {statuses[r.status]}
                            </span>
                          </td>
                          <td>{money(r.total_rub)}</td>
                          <td>
                            <button
                              disabled={busy}
                              onClick={() => void run(() => openRequest(r.id))}
                            >
                              Открыть
                            </button>
                          </td>
                        </tr>
                      ))
                    : products.map((p) => (
                        <tr
                          key={p.id}
                          className={
                            p.stock === 0 ? "admin-zero-stock" : undefined
                          }
                        >
                          <TableCells product={p} settings={settings} />
                          <td>
                            <button
                              disabled={busy}
                              onClick={() => setProduct({ ...p })}
                            >
                              Изменить
                            </button>
                          </td>
                        </tr>
                      ))}
                </tbody>
              </table>
              {(tab === "requests" ? requests : products).length === 0 && (
                <p className="admin-empty">Записей пока нет</p>
              )}
            </div>
            <div className="admin-pagination" hidden={!!detail || !!product}>
              <button
                disabled={busy || offset === 0}
                onClick={() => void run(() => load(tab, offset - 50))}
              >
                ← Назад
              </button>
              <span>Страница {offset / 50 + 1}</span>
              <button
                disabled={
                  busy ||
                  (tab === "requests"
                    ? requests.length < 50
                    : offset + 50 >= productTotal)
                }
                onClick={() => void run(() => load(tab, offset + 50))}
              >
                Далее →
              </button>
            </div>
            {tab === "requests" && detail && (
              <RequestEditor
                key={`${detail.request.id}:${detail.request.version}`}
                detail={detail}
                products={choices}
                busy={busy}
                close={() => setDetail(null)}
                save={(body) =>
                  void run(async () => {
                    await api(`requests/${detail.request.id}`, "PUT", body);
                    await openRequest(detail.request.id);
                    await load("requests", offset);
                    setNotice("Заявка сохранена");
                  })
                }
              />
            )}
            {tab === "products" && product && (
              <form
                className="admin-card admin-editor"
                onSubmit={(e) => {
                  e.preventDefault();
                  void run(async () => {
                    await api(
                      product.version ? `products/${product.id}` : "products",
                      product.version ? "PUT" : "POST",
                      product,
                    );
                    setProduct(null);
                    await load("products", offset);
                    setNotice("Товар сохранён");
                  });
                }}
              >
                <div className="admin-editor-heading">
                  <h2>
                    {product.version ? "Редактирование товара" : "Новый товар"}
                  </h2>
                  {product.version > 0 && (
                    <a
                      className="admin-card-link"
                      href={`/products/${product.id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Карточка товара <span aria-hidden="true">↗</span>
                    </a>
                  )}
                </div>
                <fieldset disabled={busy}>
                  <label>
                    Название
                    <input
                      required
                      maxLength={300}
                      value={product.name}
                      onChange={(e) =>
                        setProduct({ ...product, name: e.target.value })
                      }
                    />
                  </label>
                  <div className="admin-fields">
                    <label>
                      {(product.details?.units_per_sale || 1) > 1
                        ? `Цена за ${unitLabel(product.details)}, ₽`
                        : "Цена за штуку, ₽"}
                      <input
                        type="number"
                        required
                        min={0}
                        max={Number.MAX_SAFE_INTEGER}
                        step={1}
                        value={product.unit_price_rub}
                        onChange={(e) =>
                          setProduct({
                            ...product,
                            unit_price_rub: e.target.valueAsNumber,
                          })
                        }
                      />
                    </label>
                    <label>
                      {(product.details?.units_per_sale || 1) > 1
                        ? "Текущий остаток, комплектов"
                        : "Текущий остаток, шт."}
                      <input
                        type="number"
                        required
                        min={0}
                        max={Number.MAX_SAFE_INTEGER}
                        step={1}
                        value={product.stock}
                        onChange={(e) =>
                          setProduct({
                            ...product,
                            stock: e.target.valueAsNumber,
                          })
                        }
                      />
                    </label>
                  </div>
                  {product.details && (
                    <DetailsEditor
                      value={product.details}
                      existing={product.version > 0}
                      onChange={(details) =>
                        setProduct({ ...product, details })
                      }
                    />
                  )}
                  <label className="admin-check">
                    <input
                      type="checkbox"
                      checked={product.available}
                      onChange={(e) =>
                        setProduct({ ...product, available: e.target.checked })
                      }
                    />
                    Показывать на витрине
                  </label>
                  <div className="admin-actions">
                    <button className="admin-primary">Сохранить товар</button>
                    <button type="button" onClick={() => setProduct(null)}>
                      Закрыть
                    </button>
                  </div>
                </fieldset>
              </form>
            )}
          </>
        )}
      </main>
    </div>
  );
}

function RequestEditor({
  detail,
  products,
  busy,
  close,
  save,
}: {
  detail: Detail;
  products: Product[];
  busy: boolean;
  close: () => void;
  save: (body: unknown) => void;
}) {
  const r = detail.request;
  const [items, setItems] = useState(r.items);
  const [status, setStatus] = useState(r.status);
  const [comment, setComment] = useState("");
  const [add, setAdd] = useState("");
  const editable = r.status === "created";
  const allowed = [
    r.status,
    ...(r.status === "created"
      ? ["delivery"]
      : r.status === "delivery"
        ? ["delivered"]
        : []),
    ...(r.status !== "cancelled" ? ["cancelled"] : []),
  ];
  return (
    <section className="admin-card admin-editor">
      <h2>Заявка № {r.id.slice(0, 8)}</h2>
      <p>
        {r.customer_name} · <a href={`tel:${r.phone}`}>{r.phone}</a> ·{" "}
        {date(r.created_at)}
      </p>
      <p className="admin-comment">
        Комментарий покупателя: {r.comment || "—"}
      </p>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          save({
            version: r.version,
            status,
            comment,
            ...(editable
              ? {
                  items: items.map(({ product_id, quantity }) => ({
                    product_id,
                    quantity,
                  })),
                }
              : {}),
          });
        }}
      >
        <fieldset disabled={busy}>
          <h3>Состав заявки</h3>
          {items.map((item) => (
            <div className="admin-line" key={item.product_id}>
              <div>
                <a
                  className="admin-product-link"
                  href={`/products/${item.product_id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {item.name}
                </a>
                <small>
                  {money(item.unit_price_rub)} / {item.unit_label || "шт."}
                </small>
              </div>
              <label>
                Количество
                <input
                  aria-label={`Количество: ${item.name}`}
                  type="number"
                  required
                  min={1}
                  max={10000}
                  step={1}
                  disabled={!editable}
                  value={item.quantity}
                  onChange={(e) =>
                    setItems(
                      items.map((i) =>
                        i.product_id === item.product_id
                          ? { ...i, quantity: e.target.valueAsNumber }
                          : i,
                      ),
                    )
                  }
                />
              </label>
              <strong>{money(item.quantity * item.unit_price_rub)}</strong>
              {editable && (
                <button
                  type="button"
                  onClick={() =>
                    setItems(
                      items.filter((i) => i.product_id !== item.product_id),
                    )
                  }
                >
                  Убрать
                </button>
              )}
            </div>
          ))}
          {editable ? (
            <div className="admin-add">
              <label>
                Добавить позицию
                <select value={add} onChange={(e) => setAdd(e.target.value)}>
                  <option value="">Выберите товар</option>
                  {products
                    .filter(
                      (p) =>
                        p.available &&
                        !items.some((i) => i.product_id === p.id),
                    )
                    .map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name} · {money(p.unit_price_rub)} · {p.stock}{" "}
                        {unitLabel(p.details)}
                      </option>
                    ))}
                </select>
              </label>
              <button
                type="button"
                disabled={!add}
                onClick={() => {
                  const p = products.find((p) => p.id === add);
                  if (p)
                    setItems([
                      ...items,
                      {
                        product_id: p.id,
                        name: p.name,
                        unit_price_rub: p.unit_price_rub,
                        unit_label: unitLabel(p.details),
                        quantity: 1,
                      },
                    ]);
                  setAdd("");
                }}
              >
                Добавить
              </button>
            </div>
          ) : (
            <p>После отправки состав зафиксирован.</p>
          )}
          <p className="admin-total">
            Итого:{" "}
            {money(
              items.reduce((sum, i) => sum + i.quantity * i.unit_price_rub, 0),
            )}
          </p>
          <p>
            Сохранённые позиции сохраняют цену заявки. Новые позиции
            рассчитываются по текущей цене каталога при сохранении.
          </p>
          <label>
            Статус
            <select value={status} onChange={(e) => setStatus(e.target.value)}>
              {allowed.map((s) => (
                <option key={s} value={s}>
                  {statuses[s]}
                </option>
              ))}
            </select>
          </label>
          {status === "delivery" && editable && (
            <p className="admin-notice">
              При сохранении все позиции будут списаны со склада. При нехватке
              хотя бы одной позиции изменения не сохранятся.
            </p>
          )}
          {status === "cancelled" && (
            <p>
              Отмена не возвращает отправленный товар на склад автоматически.
            </p>
          )}
          <label>
            Комментарий сотрудника{" "}
            {status === "cancelled" ? "(обязательно)" : "(в историю)"}
            <textarea
              value={comment}
              maxLength={2000}
              required={status === "cancelled"}
              onChange={(e) => setComment(e.target.value)}
            />
          </label>
          <div className="admin-actions">
            <button className="admin-primary" disabled={items.length === 0}>
              Сохранить заявку
            </button>
            <button type="button" onClick={close}>
              Закрыть
            </button>
          </div>
        </fieldset>
      </form>
      <h3>История изменений</h3>
      {detail.history.length === 0 ? (
        <p>Изменений пока нет.</p>
      ) : (
        detail.history.map((event) => (
          <details key={event.after.version}>
            <summary>
              {date(event.created_at)} · {event.actor} ·{" "}
              {statuses[event.before.status]} → {statuses[event.after.status]} ·
              версия {event.after.version}
            </summary>
            <p className="admin-comment">
              {event.comment || "Без комментария"}
            </p>
            <div className="admin-fields">
              {[event.before, event.after].map((snapshot, i) => (
                <div key={i}>
                  <strong>
                    {i === 0 ? "До" : "После"}: {money(snapshot.total_rub)}
                  </strong>
                  <ul>
                    {snapshot.items.map((item) => (
                      <li key={item.product_id}>
                        <a
                          className="admin-product-link"
                          href={`/products/${item.product_id}`}
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          {item.name}
                        </a>{" "}
                        — {item.quantity} {item.unit_label || "шт."} ×{" "}
                        {money(item.unit_price_rub)}
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          </details>
        ))
      )}
      <small>
        Последние 100 изменений. Полная история хранится на сервере.
      </small>
    </section>
  );
}
