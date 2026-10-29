const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);

test("catalog filters paginate on the server, restore URL history and keep the cart", async ({
  page,
}) => {
  await page.goto(
    "/?kind=tyre&brand=Browser+Filter+Brand&sort=unit_price_rub#catalog",
  );
  await expect(page.locator(".product")).toHaveCount(24);
  await expect(page.locator(".results-heading")).toContainText("30");
  await page
    .locator(".product")
    .first()
    .getByRole("button", { name: "В корзину", exact: true })
    .click();
  await page
    .getByRole("navigation", { name: "Страницы каталога" })
    .getByRole("button", { name: "Далее" })
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
    .getByRole("button", { name: "Назад" })
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
  await page.getByRole("button", { name: "Грузовые диски" }).click();
  await expect(page).toHaveURL(/kind=wheel/);
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
  await filters.locator("summary").click();
  await expect(
    filters.getByRole("button", { name: "Применить", exact: true }),
  ).toBeVisible();
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
  const filters = page.locator(".admin-filter-panel");
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
  await expect(
    filters.getByLabel("Сверловка PCD", { exact: true }),
  ).toBeVisible();
  await expect(
    filters.getByLabel("Протектор, %: от", { exact: true }),
  ).toHaveCount(0);
});
