import { ContactMessengers } from "./contact-messengers";
import { ContactMap } from "./contact-map";
import { SiteHeader, SiteFooter, useCartCount } from "./site-shell";
export function Information({ path }: { path: string }) {
  const count = useCartCount();
  return (
    <>
      <SiteHeader path={path} count={count} />
      <main className="wrap information">
        <a className="back-link" href="/">
          ← Каталог
        </a>
        {path === "/about/" ? (
          <>
            <p className="eyebrow">О КОМПАНИИ</p>
            <h1>
              Более 20 лет на рынке.
              <br />
              Шины и диски из Японии.
            </h1>
            <p className="information-lead">
              Специализируемся на продаже б/у шин и дисков, прямые поставки из
              Японии.
            </p>
            <ul className="benefits">
              {[
                "20+ лет на рынке",
                "Прямые поставки из Японии",
                "Узкая специализация",
                "Доставка по всей России",
              ].map((x) => (
                <li key={x}>{x}</li>
              ))}
            </ul>
            <p>
              Мы сознательно сконцентрировались на одном направлении — японских
              грузовиках и спецтехнике. Благодаря этому накопили глубокую
              экспертизу и постоянно поддерживаем в наличии все основные размеры
              шин и дисков, которые выпускались с 1990-х годов и продолжают
              производиться сегодня.
            </p>
            <p>
              Это позволяет быстро закрывать потребности как владельцев техники
              90-х–2000-х годов, так и более современных машин.
            </p>
            <p>
              Находимся в Иркутске. Работаем с транспортными компаниями и
              организуем доставку по всей России — от Калининграда до
              Владивостока. Клиенты получают качественный товар по выгодным
              ценам без лишних посредников.
            </p>
            <blockquote>
              Прямые поставки из Японии, многолетний опыт и узкая специализация
              — то, что позволяет нам предлагать оптимальное соотношение цены и
              качества.
            </blockquote>
            <a className="info-action" href="/contacts/">
              Связаться с нами →
            </a>
          </>
        ) : path === "/contacts/" ? (
          <>
            <p className="eyebrow">МЫ НА СВЯЗИ</p>
            <h1>Контакты</h1>
            <div className="contact-grid">
              <section className="contact-card contact-address">
                <span className="contact-icon" aria-hidden="true">
                  ⌖
                </span>
                <div>
                  <h2>Адрес склада</h2>
                  <p>
                    г. Иркутск, ул. Олега Кошевого, 63,
                    <br />
                    база «Саяны»
                  </p>
                </div>
                <ContactMap />
              </section>
              <section className="contact-card">
                <span className="contact-icon" aria-hidden="true">
                  ☎
                </span>
                <div>
                  <h2>Телефон</h2>
                  <p>
                    <a href="tel:+79256340399">+7 925 634-03-99</a>
                    <br />
                    <a href="tel:+79027625164">+7 902 762-51-64</a>
                  </p>
                </div>
              </section>
              <section className="contact-card">
                <span className="contact-icon" aria-hidden="true">
                  ◷
                </span>
                <div>
                  <h2>Режим работы</h2>
                  <p>
                    Пн–Сб: 10:00–17:00
                    <br />
                    Вс: по договорённости
                  </p>
                </div>
              </section>
            </div>
            <ContactMessengers />
            <a className="info-action contact-call" href="tel:+79256340399">
              Позвонить нам
            </a>
          </>
        ) : path === "/blog/" ? (
          <>
            <p className="eyebrow">О ШИНАХ И ДИСКАХ</p>
            <h1>Блог</h1>
            <section className="blog-empty">
              <span aria-hidden="true">▤</span>
              <h2>Полезные материалы — скоро</h2>
              <p>
                Готовим статьи о выборе шин и дисков для грузового транспорта.
              </p>
              <a href="/">Перейти в каталог →</a>
            </section>
          </>
        ) : (
          <>
            <p className="eyebrow">404</p>
            <h1>Страница не найдена</h1>
            <p>Возможно, адрес изменился или страница больше недоступна.</p>
            <a className="info-action" href="/">
              Перейти в каталог →
            </a>
          </>
        )}
      </main>
      <SiteFooter />
    </>
  );
}
