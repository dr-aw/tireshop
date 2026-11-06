const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);

test("theme follows system until overridden and shares preference with administration", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto("/");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.emulateMedia({ colorScheme: "light" });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await page.getByRole("button", { name: "Включить тёмную тему" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.screenshot({ path: ".tmp/theme-shop-dark.png", fullPage: true });
  await page.goto("/administrator");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator(".administrator")).toHaveCSS(
    "background-color",
    "rgb(32, 39, 51)",
  );
  await expect(page.locator(".administrator")).toHaveCSS(
    "color",
    "rgb(229, 233, 240)",
  );
  await page.screenshot({ path: ".tmp/theme-admin-dark.png", fullPage: true });
  await page.getByRole("button", { name: "Включить светлую тему" }).click();
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
});
