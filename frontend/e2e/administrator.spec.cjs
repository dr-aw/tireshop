const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);

test("administrator edits stock and request, dispatches once and retains history", async ({
  page,
  request,
}) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/administrator");
  await page.getByLabel("Логин", { exact: true }).fill("browser-admin");
  await page
    .getByLabel("Пароль", { exact: true })
    .fill("synthetic-test-password");
  await page.getByRole("button", { name: "Войти", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Заявки покупателей" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Каталог и остатки", exact: true })
    .click();
  const row = page
    .getByRole("row")
    .filter({ hasText: "Browser synthetic tyre" });
  await row.getByRole("button", { name: "Изменить" }).click();
  await page.getByLabel("Текущий остаток, шт.").fill("8");
  await page.getByRole("button", { name: "Сохранить товар" }).click();
  await expect(page.locator(".admin-notice")).toHaveText("Товар сохранён");

  const response = await request.post("/api/requests", {
    headers: { "Idempotency-Key": "00000000-0000-4000-8000-000000000090" },
    data: {
      customer_name: "Admin Browser Buyer",
      phone: "+79000000000",
      comment: "<script>window.injected = true</script>",
      items: [
        {
          product_id: "00000000-0000-4000-8000-000000000000",
          quantity: 2,
          expected_unit_price_rub: 18000,
        },
      ],
    },
  });
  expect(response.status()).toBe(201);
  await page.getByRole("button", { name: "Заявки", exact: true }).click();
  await page
    .getByRole("row")
    .filter({ hasText: "Admin Browser Buyer" })
    .getByRole("button", { name: "Открыть" })
    .click();
  const editor = page.locator(".admin-editor");
  await expect(editor).toContainText("<script>window.injected = true</script>");
  expect(await page.evaluate(() => window.injected)).toBeUndefined();
  await page
    .getByRole("spinbutton", { name: "Количество: Browser synthetic tyre" })
    .fill("3");
  await page
    .getByLabel("Комментарий сотрудника", { exact: false })
    .fill("Уточнено по телефону");
  await page.getByRole("button", { name: "Сохранить заявку" }).click();
  await expect(page.locator(".admin-notice")).toHaveText("Заявка сохранена");
  await page
    .getByRole("combobox", { name: "Статус", exact: true })
    .selectOption("delivery");
  await page
    .getByLabel("Комментарий сотрудника", { exact: false })
    .fill("Передано курьеру");
  await page.getByRole("button", { name: "Сохранить заявку" }).click();
  await expect(
    page.getByRole("spinbutton", {
      name: "Количество: Browser synthetic tyre",
    }),
  ).toBeDisabled();
  await expect(editor.locator("summary")).toHaveCount(2);
  await page.screenshot({
    path: ".tmp/administrator-desktop.png",
    fullPage: true,
  });
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Заявки покупателей" }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Каталог и остатки", exact: true })
    .click();
  await expect(row.getByRole("cell").nth(2)).toHaveText("5");
  await page.getByRole("button", { name: "Добавить товар" }).click();
  await page
    .getByLabel("Название", { exact: true })
    .fill("Browser added wheel");
  await page.getByLabel("Цена за штуку, ₽").fill("2500");
  await page.getByLabel("Текущий остаток, шт.").fill("4");
  await page.getByRole("button", { name: "Сохранить товар" }).click();
  await page
    .locator(".admin-filter-panel")
    .getByLabel("Поиск", { exact: true })
    .fill("Browser added wheel");
  await page
    .locator(".admin-filter-panel")
    .getByRole("button", { name: "Применить", exact: true })
    .click();
  const added = page
    .getByRole("row")
    .filter({ hasText: "Browser added wheel" });
  await added.getByRole("button", { name: "Изменить" }).click();
  await page.getByLabel("Показывать на витрине").uncheck();
  await page.getByRole("button", { name: "Сохранить товар" }).click();
  await expect(added).toContainText("Скрыт");
  const publicList = await (
    await request.get("/api/products?limit=100")
  ).json();
  expect(publicList.items.some((p) => p.name === "Browser added wheel")).toBe(
    false,
  );
  await page
    .locator(".admin-filter-panel")
    .getByRole("button", { name: "Сбросить фильтры", exact: true })
    .click();
  await page.locator(".admin-column-picker summary").click();
  await page.getByRole("checkbox", { name: "Бренд", exact: true }).check();
  await page
    .getByRole("checkbox", { name: "Вылет ET, мм", exact: true })
    .check();
  await expect(
    page.getByRole("columnheader").filter({ hasText: "Вылет ET" }),
  ).toBeVisible();
  const sorted = page.waitForResponse(
    (r) => r.url().includes("sort=unit_price_rub") && r.status() === 200,
  );
  await page
    .getByRole("columnheader")
    .filter({ hasText: "Цена за единицу" })
    .getByRole("button")
    .click();
  await sorted;
  await expect(
    page.getByRole("columnheader").filter({ hasText: "Цена за единицу" }),
  ).toHaveAttribute("aria-sort", "ascending");
  await page.reload();
  await page
    .getByRole("button", { name: "Каталог и остатки", exact: true })
    .click();
  await expect(
    page.getByRole("columnheader").filter({ hasText: "Вылет ET" }),
  ).toBeVisible();
  await expect(
    page.getByRole("columnheader").filter({ hasText: "Цена за единицу" }),
  ).toHaveAttribute("aria-sort", "ascending");
  const zero = page.locator("tr.admin-zero-stock").first();
  await expect(zero.locator('td[data-column="stock"]')).toHaveText("0");
  await expect(zero.locator("td").first()).toHaveCSS(
    "background-color",
    "rgb(255, 241, 242)",
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({
    path: ".tmp/administrator-mobile.png",
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.getByRole("button", { name: "Выйти", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Вход для сотрудников" }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});
