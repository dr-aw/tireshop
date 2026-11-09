const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);
test("HTML contains crawlable catalog and real error status without JS", async ({
  browser,
  request,
}) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  const page = await context.newPage();
  await page.goto("/");
  await expect(page.locator(".product")).toHaveCount(24);
  await page.getByRole("link", { name: "Далее →" }).click();
  await expect(page.locator(".product")).toHaveCount(6);
  await page.locator(".product h3 a").first().click();
  await expect(page.locator("h1")).toContainText("Синтетический");
  const missing = await request.get("/missing");
  expect(missing.status()).toBe(404);
  expect(await missing.text()).toContain("Страница не найдена");
  expect((await request.get("/api/missing")).status()).toBe(404);
  const sitemap = await (await request.get("/sitemap.xml")).text();
  expect(sitemap).not.toContain("/cart/");
  expect(sitemap).toContain("/products/");
  await context.close();
});
test("hydration preserves content, updates offers, cart persists across pages", async ({
  page,
}) => {
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error" && /hydration|Minified React/.test(m.text()))
      errors.push(m.text());
  });
  const id = "10000000-0000-4000-8000-000000000002";
  await page.route("**/api/products/" + id, (route) =>
    route.fulfill({
      json: {
        id,
        name: "Синтетический диск 2",
        available: true,
        stock: 3,
        unit_price_rub: 12345,
        unit_label: "шт.",
        images: [],
      },
    }),
  );
  await page.goto("/products/" + id);
  await expect(page.locator(".product-page-price")).toContainText("12");
  await page.getByRole("button", { name: "В корзину", exact: true }).click();
  await page.getByRole("link", { name: /О нас/ }).click();
  await expect(page.locator(".cart-link b")).toHaveText("1");
  await page.getByRole("link", { name: /Корзина/ }).click();
  await expect(page.locator(".basket")).toContainText("Синтетический диск 2");
  await page.reload();
  await expect(page.locator(".basket")).toContainText("Синтетический диск 2");
  expect(errors).toEqual([]);
});
test("new pages responsive, menu visible and no horizontal overflow", async ({
  page,
}) => {
  for (const width of [320, 390, 768, 1280]) {
    await page.setViewportSize({ width, height: 844 });
    for (const path of ["/about/", "/contacts/", "/blog/", "/missing"]) {
      await page.goto(path);
      await expect(page.locator("h1")).toBeVisible();
      await expect(
        page.getByRole("navigation", { name: "Главное меню" }),
      ).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
    }
  }
  await page.goto("/about/");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: ".tmp/ssg-about.png", fullPage: true });
});

test("API outage retains static product and prevents using stale stock", async ({
  page,
}) => {
  await page.route("**/api/**", (route) => route.abort());
  await page.goto("/products/10000000-0000-4000-8000-000000000002");
  await expect(page.locator("h1")).toContainText("Синтетический");
  await expect(page.getByRole("alert")).toContainText("не подтверждены");
  await expect(
    page.getByRole("button", { name: "В корзину", exact: true }),
  ).toBeDisabled();
});

test("new checkout rechecks stock before sending a request", async ({
  page,
}) => {
  const id = "10000000-0000-4000-8000-000000000002";
  let stock = 3,
    posts = 0;
  await page.route("**/api/products/" + id, (route) =>
    route.fulfill({
      json: {
        id,
        name: "Синтетический диск 2",
        available: true,
        stock,
        unit_price_rub: 12345,
        unit_label: "шт.",
        images: [],
      },
    }),
  );
  await page.route("**/api/requests", (route) => {
    posts++;
    return route.fulfill({ status: 500, json: {} });
  });
  await page.goto("/products/" + id);
  await page.getByRole("button", { name: "В корзину", exact: true }).click();
  await page.getByLabel("Имя", { exact: true }).fill("Проверка");
  await page.getByLabel("Телефон", { exact: true }).fill("+79000000000");
  stock = 0;
  await page
    .getByRole("button", { name: "Оформить заявку", exact: true })
    .click();
  await expect(
    page.getByRole("status", { name: "Статус оформления заявки" }),
  ).toContainText("Наличие изменилось");
  expect(posts).toBe(0);
});

test("catalog text slots align and long titles retain their full text", async ({
  page,
}) => {
  const names = [
    "Короткое название",
    "Очень длинное название грузовой шины ".repeat(12),
  ];
  await page.route(/\/api\/products\?/, (route) =>
    route.fulfill({
      json: {
        total: 2,
        items: names.map((name, i) => ({
          id: `10000000-0000-4000-8000-00000000000${i + 1}`,
          name,
          unit_price_rub: 10000,
          stock: 4,
          available: true,
          unit_label: i ? "комплект (2 шт.)" : "шт.",
          details: { sku: `ALIGN-${i}`, kind: "tyre" },
          images: i ? [{ url: "/media/alignment.svg" }] : [],
        })),
      },
    }),
  );
  await page.route("**/media/alignment.svg", (route) =>
    route.fulfill({
      contentType: "image/svg+xml",
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>',
    }),
  );
  for (const width of [390, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");
    await expect(page.locator(".product")).toHaveCount(2);
    const cards = page.locator(".product");
    await expect(cards.nth(1).locator("h3 a")).toHaveAttribute(
      "title",
      names[1],
    );
    await expect(cards.nth(1).locator("h3 a")).toHaveText(names[1].trim());
    await expect(cards.first().locator(".category")).toContainText(
      "Грузовая шина · ALIGN-0",
    );
    await expect(cards.first().locator(".product-stock")).not.toContainText(
      "ALIGN",
    );
    await expect(cards.nth(1).locator("img")).toHaveAttribute(
      "alt",
      names[1] + " — фото 1",
    );
    const metrics = await cards.evaluateAll((cards) =>
      cards.map((c) =>
        Object.fromEntries(
          [
            ".product-image",
            "h3",
            ".price",
            ".product-stock",
            ".product-specifications",
            ".purchase-row",
          ].map((sel) => {
            const r = c.querySelector(sel).getBoundingClientRect();
            return [sel, { top: r.top, height: r.height }];
          }),
        ),
      ),
    );
    for (const selector of Object.keys(metrics[0])) {
      expect(
        Math.abs(metrics[0][selector].top - metrics[1][selector].top),
      ).toBeLessThan(1);
      expect(
        Math.abs(metrics[0][selector].height - metrics[1][selector].height),
      ).toBeLessThan(1);
    }
    expect(
      await cards
        .nth(1)
        .locator("h3 a")
        .evaluate((e) => e.scrollHeight > e.clientHeight),
    ).toBe(true);
  }
});
