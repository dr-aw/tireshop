import { BrandLogo } from "./brand-logo";
import { useEffect, useState } from "react";
import { ThemeToggle } from "./theme-toggle";
export function SiteHeader({
  path,
  count = 0,
}: {
  path: string;
  count?: number;
}) {
  const items = [
    ["/about/", "О нас", "M3 10 12 3l9 7M5 9v12h5v-6h4v6h5V9"],
    ["/blog/", "Блог", "M5 3h14v18H5zM8 7h8M8 11h8M8 15h5"],
    ["/", "Каталог", "M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z"],
    ["/cart/", "Корзина", "M3 3h2l3 13h11l2-9H6M9 20h1M17 20h1"],
    [
      "/contacts/",
      "Контакты",
      "M20 10c0 6-8 12-8 12S4 16 4 10a8 8 0 1 1 16 0ZM12 7v6M9 10h6",
    ],
  ];
  const active =
    path.startsWith("/products/") || path.startsWith("/catalog/")
      ? "/"
      : path.startsWith("/blog/")
        ? "/blog/"
        : path;
  return (
    <>
      <div className="topline">
        <div className="wrap">
          <span>Шины и диски для грузового транспорта</span>
          <a href="/contacts/">Связаться с нами ↗</a>
        </div>
      </div>
      <header className="site-header">
        <div className="wrap header-inner">
          <BrandLogo />
          <nav className="site-navigation" aria-label="Главное меню">
            {items.map(([href, label, icon]) => (
              <a
                key={href}
                href={href}
                aria-current={active === href ? "page" : undefined}
                className={
                  (active === href ? "active " : "") +
                  (href === "/cart/"
                    ? "cart-link" + (count ? " cart-filled" : "")
                    : "")
                }
              >
                <span className="nav-icon" aria-hidden="true">
                  <svg
                    width="22"
                    height="22"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.6"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d={icon} />
                  </svg>
                </span>
                <span>{label}</span>
                {href === "/cart/" && (
                  <b key={count} className={count ? "cart-added" : ""}>
                    {count}
                  </b>
                )}
              </a>
            ))}
          </nav>
          <ThemeToggle />
        </div>
      </header>
    </>
  );
}
export function useCartCount() {
  const [count, setCount] = useState(0);
  useEffect(() => {
    const read = () => {
      try {
        const a = JSON.parse(localStorage.getItem("tireshop.cart.v1") || "[]");
        setCount(
          Array.isArray(a)
            ? a.reduce(
                (n: number, x: { quantity: number }) =>
                  n +
                  (Number.isSafeInteger(x.quantity) && x.quantity > 0
                    ? x.quantity
                    : 0),
                0,
              )
            : 0,
        );
      } catch {
        setCount(0);
      }
    };
    read();
    window.addEventListener("storage", read);
    return () => window.removeEventListener("storage", read);
  }, []);
  return count;
}
export function SiteFooter() {
  return (
    <footer>
      <div className="wrap footer-inner">
        <BrandLogo />
        <span>Грузовые шины и диски</span>
        <a href="/contacts/">Контакты</a>
      </div>
    </footer>
  );
}
