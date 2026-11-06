const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);

test("phone formatting and numeric limits in shop and administration", async ({
  page,
}) => {
  await page.goto("/?kind=tyre");
  const phone = page.getByLabel("Телефон", { exact: true });
  await phone.fill("+79876543210");
  await expect(phone).toHaveValue("+7(987)654-32-10");
  expect(await phone.evaluate((input) => input.checkValidity())).toBe(true);
  await phone.fill("+7987");
  expect(await phone.evaluate((input) => input.checkValidity())).toBe(false);
  await phone.fill("+79876543210");
  await phone.press("End");
  await phone.press("Backspace");
  await expect(phone).toHaveValue("+7(987)654-32-1");
  const filters = page.locator(".catalog-filter-panel");
  await filters.getByLabel("Цена, ₽: от", { exact: true }).fill("-20");
  await expect(filters.getByLabel("Цена, ₽: от", { exact: true })).toHaveValue(
    "0",
  );
  await filters.getByLabel("Протектор, %: до", { exact: true }).fill("150");
  await expect(
    filters.getByLabel("Протектор, %: до", { exact: true }),
  ).toHaveValue("100");
  await page.goto("/administrator");
  await page.getByLabel("Логин", { exact: true }).fill("browser-admin");
  await page
    .getByLabel("Пароль", { exact: true })
    .fill("synthetic-test-password");
  await page.getByRole("button", { name: "Войти", exact: true }).click();
  await page
    .getByRole("button", { name: "Каталог и остатки", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Добавить товар", exact: true })
    .click();
  const price = page.getByLabel("Цена за штуку, ₽", { exact: true });
  await price.fill("-100");
  await expect(price).toHaveValue("0");
  const tread = page.getByLabel("Протектор, %", { exact: true });
  await tread.fill("101");
  await expect(tread).toHaveValue("100");
  await tread.fill("-1");
  await expect(tread).toHaveValue("0");
});

test("phone caret stays after typed digits with and without country code", async ({
  page,
}) => {
  await page.goto("/");
  const phone = page.getByLabel("Телефон", { exact: true });
  await phone.pressSequentially("7");
  await expect(phone).toHaveValue("+7");
  expect(await phone.evaluate((input) => input.selectionStart)).toBe(2);
  await phone.pressSequentially("9876543210");
  await expect(phone).toHaveValue("+7(987)654-32-10");
  await phone.fill("");
  await phone.pressSequentially("9");
  await expect(phone).toHaveValue("+7(9");
  expect(await phone.evaluate((input) => input.selectionStart)).toBe(4);
  await phone.pressSequentially("876543210");
  await expect(phone).toHaveValue("+7(987)654-32-10");
  await phone.fill("");
  await phone.pressSequentially("79876543210", { delay: 25 });
  await expect(phone).toHaveValue("+7(987)654-32-10");
  await phone.fill("");
  await phone.pressSequentially("8");
  await expect(phone).toHaveValue("+7");
  expect(await phone.evaluate((input) => input.selectionStart)).toBe(2);
  await phone.pressSequentially("9888888888");
  await expect(phone).toHaveValue("+7(988)888-88-88");
  await phone.fill("89876543210");
  await expect(phone).toHaveValue("+7(987)654-32-10");
});
