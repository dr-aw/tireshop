const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);

test("selection follows the mockup, counts physical units, and restores history", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await expect(page.locator(".product")).toHaveCount(24);
  const categories = page.locator(".selection-categories");
  await expect(
    categories.getByRole("link", { name: "Все товары", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  const all = await categories
    .getByRole("link", { name: "Все товары", exact: true })
    .boundingBox();
  const tyre = await categories
    .getByRole("link", { name: "Шины", exact: true })
    .boundingBox();
  const wheel = await categories
    .getByRole("link", { name: "Диски", exact: true })
    .boundingBox();
  expect(all.width).toBeGreaterThan(tyre.width * 1.9);
  expect(all.height).toBeLessThan(tyre.height);
  expect(all.y + all.height).toBeLessThan(tyre.y);
  expect(tyre.y).toBe(wheel.y);
  await categories.getByRole("link", { name: "Диски", exact: true }).click();
  await page.getByRole("button", { name: "R17.5, 2 шт", exact: true }).click();
  const size = page.getByRole("button", {
    name: "6.75×17.5 2 шт",
    exact: true,
  });
  await size.click();
  await expect(size).toHaveAttribute("aria-pressed", "true");
  await expect(page).toHaveURL(/width=6.75/);
  await expect(page.locator(".product")).toHaveCount(1);
  await page.reload();
  await expect(
    page.getByRole("button", { name: "6.75×17.5 2 шт", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await page
    .getByRole("button", { name: "Сбросить размер", exact: true })
    .click();
  await expect(page).not.toHaveURL(/radius=/);
  await page.goBack();
  await expect(size).toHaveAttribute("aria-pressed", "true");
  await categories.getByRole("link", { name: "Шины", exact: true }).click();
  await expect(page).not.toHaveURL(/radius=|width=/);
  await page.getByRole("button", { name: /^R22\.5, / }).click();
  await expect(
    page.getByRole("group", { name: "Размеры R22.5", exact: true }),
  ).toBeVisible();
  for (const width of [320, 390, 768, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `.tmp/selection-${width}.png`,
      fullPage: false,
    });
  }
  await page.emulateMedia({ colorScheme: "dark", reducedMotion: "reduce" });
  await page.reload();
  await expect(page.getByRole("button", { name: /^R22\.5, / })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await page.screenshot({ path: ".tmp/selection-dark.png" });
});

test("selection handles failures, stale replies and legacy restrictions", async ({
  page,
}) => {
  let fail = true;
  await page.route("**/api/products/selection?**", async (route) => {
    if (fail) await route.fulfill({ status: 503, body: "{}" });
    else await route.continue();
  });
  await page.goto(
    "/?kind=tyre&rim_diameter=22.5&size=315%2F70R22.5&tyre_width=315+mm",
  );
  await expect(page.getByText("Не удалось загрузить подбор.")).toBeVisible();
  await expect(
    page.getByRole("button", {
      name: "Исходный размер: 315/70R22.5 ×",
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: /Ширина шины:/ })).toHaveCount(
    0,
  );
  fail = false;
  await page
    .locator(".storefront-selection")
    .getByRole("button", { name: "Повторить" })
    .click();
  await expect(page.getByRole("button", { name: /^R22\.5, / })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await page
    .getByRole("button", {
      name: "Исходный размер: 315/70R22.5 ×",
      exact: true,
    })
    .click();
  await expect(page).not.toHaveURL(/[?&]size=/);
  await page.unroute("**/api/products/selection?**");
  await page.route("**/api/products/selection?**", async (route) => {
    if (route.request().url().includes("kind=tyre")) {
      await new Promise((resolve) => setTimeout(resolve, 350));
      await route.fulfill({
        json: {
          radii: [
            { value: "99", label: "R99", product_count: 1, unit_count: "999" },
          ],
          sizes: [],
        },
      });
    } else await route.continue();
  });
  const categories = page.locator(".selection-categories");
  await categories
    .getByRole("link", { name: "Все товары", exact: true })
    .click();
  await categories.getByRole("link", { name: "Шины", exact: true }).click();
  await categories.getByRole("link", { name: "Диски", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "R17.5, 2 шт", exact: true }),
  ).toBeVisible();
  await page.waitForTimeout(450);
  await expect(page.getByRole("button", { name: /^R99/ })).toHaveCount(0);
  await page.goto("/?kind=wheel&radius=99&width=123");
  await expect(
    page.getByRole("button", { name: "R99, 0 шт", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByRole("button", { name: "123×99 0 шт", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await page
    .getByRole("button", { name: "Сбросить размер", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "R17.5, 2 шт", exact: true }),
  ).toBeVisible();
});

test("storefront keeps price and stock, removes old fields without changing admin", async ({
  page,
}) => {
  const queries = [];
  page.on("request", (r) => {
    if (new URL(r.url()).pathname === "/api/products")
      queries.push(new URL(r.url()).searchParams);
  });
  for (const kind of ["", "tyre", "wheel"]) {
    await page.goto(
      `/?kind=${kind}&brand=obsolete&model=obsolete&season=obsolete&production_year=2001&tread_min=99&load_index=x&speed_index=x&wheel_type=x&aspect_ratio=99&tyre_width=999+mm`,
    );
    const filters = page.locator(".catalog-filter-panel");
    await expect(
      filters.getByLabel("Цена, ₽: от", { exact: true }),
    ).toBeVisible();
    await expect(
      filters.getByLabel("Только в наличии", { exact: true }),
    ).toBeVisible();
    await expect(filters.getByRole("combobox")).toHaveCount(
      kind === "wheel" ? 3 : 0,
    );
    for (const label of [
      "Производитель",
      "Модель",
      "Сезон",
      "Год выпуска",
      "Протектор, %: от",
      "Индекс нагрузки",
      "Индекс скорости",
      "Тип диска",
      "Ширина шины",
      "Профиль, %",
    ])
      await expect(filters.getByLabel(label, { exact: true })).toHaveCount(0);
    if (kind === "wheel")
      for (const label of ["Сверловка PCD", "Вылет ET, мм", "Ступица DIA, мм"])
        await expect(filters.getByLabel(label, { exact: true })).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Найти", exact: true }),
    ).toBeEnabled();
    await expect.poll(() => queries.length).toBeGreaterThan(0);
    const query = queries.at(-1);
    for (const key of [
      "brand",
      "model",
      "season",
      "production_year",
      "tread_min",
      "load_index",
      "speed_index",
      "wheel_type",
      "aspect_ratio",
      "tyre_width",
    ])
      expect(query.has(key)).toBe(false);
    queries.length = 0;
  }
});
