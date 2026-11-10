const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);

test("catalog filters paginate on the server, restore URL history and keep the cart", async ({
  page,
}) => {
  await page.goto("/");
  const allFilters = page.locator(".catalog-filter-panel");
  await expect(allFilters.getByRole("combobox")).toHaveCount(0);
  await expect(
    allFilters.getByLabel("Цена, ₽: от", { exact: true }),
  ).toBeVisible();
  await expect(allFilters.getByLabel("Только в наличии")).toBeVisible();
  await page.goto(
    "/?kind=tyre&q=Browser+filter+tyre&brand=obsolete&condition=obsolete&sort=unit_price_rub#catalog",
  );
  await expect(page.locator(".product")).toHaveCount(24);
  await expect(page.locator(".results-heading")).toContainText("30");
  await expect(
    page
      .locator(".catalog-filter-panel")
      .getByLabel("Состояние", { exact: true }),
  ).toHaveCount(0);
  await page
    .locator(".product")
    .first()
    .getByRole("button", { name: "В корзину", exact: true })
    .click();
  await page
    .getByRole("navigation", { name: "Страницы каталога" })
    .getByRole("link", { name: "Далее" })
    .click();
  await expect(page).toHaveURL(/page=2/);
  await expect(page.locator(".product")).toHaveCount(6);
  await expect(page.locator(".product").first()).toContainText(
    "Browser filter tyre 24",
  );
  await page.reload();
  await expect(page.locator(".product")).toHaveCount(6);
  await expect(page.locator(".cart-link b")).toHaveText("1");
  await page
    .getByRole("navigation", { name: "Страницы каталога" })
    .getByRole("link", { name: "Назад" })
    .click();
  await expect(page.locator(".product")).toHaveCount(24);
  await page.goBack();
  await expect(page.locator(".product")).toHaveCount(6);
  await page.goForward();
  await expect(page.locator(".product")).toHaveCount(24);
  const filters = page.locator(".catalog-filter-panel");
  if (!(await filters.getAttribute("open"))) {
    // Empty-string open attributes also mean expanded.
    if (!(await filters.evaluate((el) => el.open)))
      await filters.locator("summary").click();
  }
  await page.getByRole("button", { name: /^R22\.5, / }).click();
  await page.getByRole("button", { name: /^315\/70R22\.5 / }).click();
  await expect(page).toHaveURL(/diameter=22.5/);
  await expect(page).toHaveURL(/tyre_size=/);
  await filters.getByLabel("Цена, ₽: от", { exact: true }).fill("1025");
  await filters.getByRole("button", { name: "Применить", exact: true }).click();
  await expect(page).not.toHaveURL(/page=2/);
  await expect(page.locator(".product")).toHaveCount(5);
  await filters.getByLabel("Цена, ₽: от", { exact: true }).fill("999999");
  await filters.getByRole("button", { name: "Применить", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "По вашему запросу ничего не найдено" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Сбросить подбор" }).click();
  await expect(page.locator(".product")).toHaveCount(24);
  await expect(
    filters.getByRole("combobox", { name: "Размер шины", exact: true }),
  ).toHaveCount(0);
  await expect(
    filters.getByRole("combobox", {
      name: "Диаметр диска, дюймы",
      exact: true,
    }),
  ).toHaveCount(0);
  await page.getByRole("link", { name: "Диски", exact: true }).click();
  await expect(page).toHaveURL(/kind=wheel/);
  for (const label of ["Производитель", "Модель", "Состояние", "Год выпуска"])
    await expect(filters.getByLabel(label, { exact: true })).toHaveCount(0);
  await expect(
    filters.getByLabel("Вылет ET, мм", { exact: true }),
  ).toBeVisible();
  await expect(
    filters.getByLabel("Протектор, %: от", { exact: true }),
  ).toHaveCount(0);
  await expect(page.locator(".cart-link b")).toHaveText("1");
  await page.screenshot({ path: ".tmp/catalog-filters-desktop.png" });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await expect(
    filters.getByRole("button", { name: "Применить", exact: true }),
  ).toBeVisible();
  const diameterBox = await filters
    .getByLabel("Вылет ET, мм", { exact: true })
    .boundingBox();
  const widthBox = await filters
    .getByLabel("Сверловка PCD", { exact: true })
    .boundingBox();
  expect(Math.abs(diameterBox.y - widthBox.y)).toBeLessThan(2);
  expect(diameterBox.x + diameterBox.width).toBeLessThan(widthBox.x);
  await expect(filters.getByLabel("Вылет ET, мм", { exact: true })).toHaveCSS(
    "padding-right",
    "32px",
  );
  const searchFits = await page
    .locator(".catalog-search-form")
    .evaluate((form) => {
      const input = form.querySelector("input").getBoundingClientRect();
      const label = form.querySelector("label").getBoundingClientRect();
      const button = form.querySelector("button").getBoundingClientRect();
      return (
        input.right <= label.right &&
        label.right <= button.left &&
        input.width > label.width - 35
      );
    });
  expect(searchFits).toBe(true);
  await page.screenshot({ path: ".tmp/catalog-filters-mobile.png" });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test("administrator filters the same catalog through the protected endpoint", async ({
  page,
}) => {
  await page.goto("/administrator");
  await page.getByLabel("Логин", { exact: true }).fill("browser-admin");
  await page
    .getByLabel("Пароль", { exact: true })
    .fill("synthetic-test-password");
  await page.getByRole("button", { name: "Войти", exact: true }).click();
  await page
    .getByRole("button", { name: "Каталог и остатки", exact: true })
    .click();
  await page.getByRole("button", { name: "Шины", exact: true }).click();
  const filters = page.locator(".admin-filter-panel");
  await expect(filters.getByLabel("Состояние", { exact: true })).toHaveCount(0);
  await filters
    .getByLabel("Производитель", { exact: true })
    .selectOption("Browser Filter Brand");
  await filters.getByLabel("Цена, ₽: от", { exact: true }).fill("1027");
  await filters.getByRole("button", { name: "Применить", exact: true }).click();
  await expect(page.locator(".admin-result-count")).toHaveText("Найдено: 3");
  await expect(page.locator(".admin-table-wrap tbody tr")).toHaveCount(3);
  await filters
    .getByRole("button", { name: "Сбросить фильтры", exact: true })
    .click();
  await expect(page.locator(".admin-result-count")).not.toHaveText(
    "Найдено: 3",
  );
  await page.getByRole("button", { name: "Диски", exact: true }).click();
  for (const label of ["Производитель", "Модель", "Состояние", "Год выпуска"])
    await expect(filters.getByLabel(label, { exact: true })).toHaveCount(0);
  await expect(
    filters.getByLabel("Сверловка PCD", { exact: true }),
  ).toBeVisible();
  await expect(
    filters.getByLabel("Протектор, %: от", { exact: true }),
  ).toHaveCount(0);
});

test("typed editor derives tyre dimensions and clears them when changing kind", async ({
  page,
}) => {
  await page.goto("/administrator");
  await page.getByLabel("Логин", { exact: true }).fill("browser-admin");
  await page
    .getByLabel("Пароль", { exact: true })
    .fill("synthetic-test-password");
  await page.getByRole("button", { name: "Войти", exact: true }).click();
  await page.getByRole("heading", { name: "Заявки покупателей" }).waitFor();
  await page
    .getByRole("button", { name: "Каталог и остатки", exact: true })
    .click();
  await page.getByRole("button", { name: "Шины", exact: true }).click();
  const filters = page.locator(".admin-filter-panel");
  await filters
    .getByLabel("Поиск", { exact: true })
    .fill("Browser filter tyre 29");
  await filters.getByRole("button", { name: "Применить", exact: true }).click();
  const row = page.locator(".admin-table-wrap tbody tr");
  await expect(row).toHaveCount(1);
  const href = await row.locator(".admin-product-link").getAttribute("href");
  await row.getByRole("button", { name: "Изменить" }).click();
  await page.getByLabel("Размер шины", { exact: true }).fill("205/85R26");
  await page.getByRole("button", { name: "Сохранить товар" }).click();
  await expect(
    page.getByText(
      "Посадочный диаметр должен быть от 11 до 25 дюймов включительно",
      { exact: true },
    ),
  ).toBeVisible();
  await page.getByLabel("Размер шины", { exact: true }).fill("11R22.5");
  await page.getByRole("button", { name: "Сохранить товар" }).click();
  await expect(page.locator(".admin-notice")).toHaveText("Товар сохранён");
  let product = await (await page.request.get("/api" + href)).json();
  expect(product.details).toMatchObject({
    tyre_width: 11,
    tyre_width_unit: "in",
    aspect_ratio: null,
    rim_diameter: 22.5,
    diameter: null,
    width: null,
  });
  await row.getByRole("button", { name: "Изменить" }).click();
  await page.getByLabel("Категория", { exact: true }).selectOption("wheel");
  await expect(page.getByLabel("Размер шины", { exact: true })).toHaveCount(0);
  await page.getByLabel("Диаметр диска, дюймы", { exact: true }).fill("22.5");
  await page.getByLabel("Ширина диска, дюймы", { exact: true }).fill("8.25");
  await page.getByLabel("Вылет ET, мм", { exact: true }).fill("-118.5");
  await page.getByRole("button", { name: "Сохранить товар" }).click();
  await expect(page.locator(".admin-notice")).toHaveText("Товар сохранён");
  product = await (await page.request.get("/api" + href)).json();
  expect(product.details).toMatchObject({
    kind: "wheel",
    tyre_width: null,
    tyre_width_unit: "",
    aspect_ratio: null,
    rim_diameter: null,
    diameter: 22.5,
    width: 8.25,
    wheel_offset: -118.5,
    size: "",
  });
});

test("search fills its wrapper and tablet/desktop filters have enough width", async ({
  page,
}) => {
  for (const width of [390, 768, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/?kind=wheel");
    const geometry = await page
      .locator(".catalog-search-form")
      .evaluate((form) => {
        const input = form.querySelector("input").getBoundingClientRect();
        const label = form.querySelector("label").getBoundingClientRect();
        const aside = document
          .querySelector("aside.filters")
          .getBoundingClientRect();
        return {
          difference: Math.abs(input.width - label.width),
          left: Math.abs(input.left - label.left),
          aside: aside.width,
          overflow: document.documentElement.scrollWidth > innerWidth,
        };
      });
    expect(geometry.difference).toBeLessThan(1);
    expect(geometry.left).toBeLessThan(1);
    expect(geometry.overflow).toBe(false);
    if (width > 640) expect(geometry.aside).toBeGreaterThanOrEqual(280);
    await page.screenshot({
      path: `.tmp/catalog-width-${width}.png`,
      fullPage: false,
    });
  }
});
