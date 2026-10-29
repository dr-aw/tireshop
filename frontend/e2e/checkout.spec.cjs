const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);

test("lost committed response survives reload and retries the exact attempt", async ({
  page,
}) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  const card = page
    .locator("article")
    .filter({ hasText: "Browser synthetic tyre" });

  await card.getByRole("button", { name: "В корзину", exact: true }).click();
  const basket = page.locator(".basket");
  const minus = basket.getByRole("button", { name: /^Уменьшить:/ });
  const plus = basket.getByRole("button", { name: /^Увеличить:/ });
  await expect(minus).toBeDisabled();
  await plus.click();
  await plus.click();
  await minus.click();
  await expect(basket.getByRole("spinbutton")).toHaveValue("2");
  await expect(page.locator(".total")).toContainText(
    "36 000".replace(" ", "\u00a0"),
  );

  await page.getByLabel("Имя", { exact: true }).fill("Browser Buyer");
  await page.getByLabel("Телефон", { exact: true }).fill("+79000000000");
  let original, requestID;
  await page.route("**/api/requests", async (route) => {
    original = {
      key: route.request().headers()["idempotency-key"],
      body: route.request().postData(),
    };
    // Commit through the real API, then drop only the browser-facing response.
    const response = await route.fetch();
    expect(response.status()).toBe(201);
    requestID = (await response.json()).request_id;
    await route.abort("failed");
  });
  await page
    .getByRole("button", { name: "Оформить заявку", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Ответ не получен");
  expect(JSON.parse(original.body).items[0].quantity).toBe(2);
  await expect(plus).toBeDisabled();
  await expect(minus).toBeDisabled();
  await page.unroute("**/api/requests");
  await page.reload();
  await expect(page.getByLabel("Имя", { exact: true })).toBeDisabled();
  const retry = page.waitForRequest("**/api/requests");
  await page.getByRole("button", { name: "Повторить отправку" }).click();
  const sent = await retry;
  expect(sent.headers()["idempotency-key"]).toBe(original.key);
  expect(sent.postData()).toBe(original.body);
  await expect(
    page.getByRole("heading", { name: "Заявка получена" }),
  ).toBeVisible();
  await expect(page.locator("code")).toHaveText(requestID);
  expect(
    await page.evaluate(() => sessionStorage.getItem("tireshop.checkout.v1")),
  ).not.toContain("Browser Buyer");
  await page.screenshot({ path: ".tmp/browser-desktop.png", fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: ".tmp/browser-mobile.png", fullPage: true });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
});

test("buy now confirms an updated price with a new key and keeps the basket", async ({
  page,
}) => {
  await page.route(/\/api\/products(?:\?|$)/, async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    // Simulate a page loaded before a catalog price update. The DB remains real.
    data.items.find((p) => p.name === "Browser synthetic tyre").unit_price_rub =
      17000;
    await route.fulfill({ response, json: data });
  });
  await page.goto("/");
  const card = page
    .locator("article")
    .filter({ hasText: "Browser synthetic tyre" });
  await card.getByRole("spinbutton").fill("2");
  await card.getByRole("button", { name: "В корзину", exact: true }).click();
  await card.getByRole("spinbutton").fill("1");
  await card.getByRole("button", { name: "Купить сейчас" }).click();
  await page.getByLabel("Имя", { exact: true }).fill("Price Buyer");
  await page.getByLabel("Телефон", { exact: true }).fill("+79000000000");
  const sent = [];
  page.on("request", (request) => {
    if (request.url().endsWith("/api/requests"))
      sent.push({
        key: request.headers()["idempotency-key"],
        body: request.postDataJSON(),
      });
  });
  await page
    .getByRole("button", { name: "Оформить заявку", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Обновлённые цены" }),
  ).toBeVisible();
  expect(sent).toHaveLength(1);
  await page.getByRole("button", { name: /Подтвердить .*18/ }).click();
  await expect(
    page.getByRole("heading", { name: "Заявка получена" }),
  ).toBeVisible();
  expect(sent).toHaveLength(2);
  expect(sent[0].key).not.toBe(sent[1].key);
  expect(sent[1].body.items[0].expected_unit_price_rub).toBe(18000);
  expect(sent[1].body.items[0].quantity).toBe(1);
  await page.getByRole("button", { name: "Вернуться к покупкам" }).click();
  await expect(page.locator(".basket").getByRole("spinbutton")).toHaveValue(
    "2",
  );
  await page
    .locator(".basket")
    .getByRole("button", { name: /^Уменьшить:/ })
    .click();
  await expect(page.locator(".basket").getByRole("spinbutton")).toHaveValue(
    "1",
  );
  await expect(
    page.locator(".basket").getByRole("button", { name: /^Уменьшить:/ }),
  ).toBeDisabled();
  await page.locator(".basket").getByRole("spinbutton").fill("10000");
  await expect(
    page.locator(".basket").getByRole("button", { name: /^Увеличить:/ }),
  ).toBeDisabled();
  await page
    .locator(".basket")
    .getByRole("button", { name: /^Убрать / })
    .click();
  await expect(page.getByText("Пока ничего не добавлено")).toBeVisible();
});
