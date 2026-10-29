const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);
test("catalog gallery and bundle price survive checkout", async ({ page }) => {
  await page.route("**/media/test-*.webp", (route) =>
    route.fulfill({
      contentType: "image/svg+xml",
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="1280"><rect width="1280" height="1280" fill="white"/><circle cx="640" cy="640" r="300" fill="gray"/></svg>',
    }),
  );
  await page.goto("/");
  const card = page
    .locator("article")
    .filter({ hasText: "Browser wheel bundle" });
  await expect(card.getByRole("img")).toHaveAttribute(
    "src",
    "/media/test-0-1280-v1.webp",
  );
  await expect
    .poll(() =>
      card
        .getByRole("img")
        .evaluate(
          (img) =>
            img.complete &&
            img.naturalWidth > 0 &&
            img.getBoundingClientRect().height > 100,
        ),
    )
    .toBe(true);
  await card
    .getByRole("button", { name: "Следующее фото: Browser wheel bundle" })
    .click();
  await expect(card.getByRole("img")).toHaveAttribute(
    "src",
    "/media/test-1-1280-v1.webp",
  );
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true });
  async function swipe(dx, dy = 0) {
    const gallery = card.locator(".catalog-photo-link");
    await gallery.scrollIntoViewIfNeeded();
    const box = await gallery.boundingBox();
    const x = box.x + box.width / 2,
      y = box.y + box.height / 2;
    await cdp.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [{ x, y }],
    });
    for (let step = 1; step <= 6; step++)
      await cdp.send("Input.dispatchTouchEvent", {
        type: "touchMove",
        touchPoints: [{ x: x + (dx * step) / 6, y: y + (dy * step) / 6 }],
      });
    await cdp.send("Input.dispatchTouchEvent", {
      type: "touchEnd",
      touchPoints: [],
    });
  }
  await swipe(90);
  await expect(card.getByRole("img")).toHaveAttribute(
    "src",
    "/media/test-0-1280-v1.webp",
  );
  await swipe(-90);
  await expect(card.getByRole("img")).toHaveAttribute(
    "src",
    "/media/test-1-1280-v1.webp",
  );
  await expect(page).toHaveURL(/\/$/);
  await swipe(5, 100);
  await expect(card.getByRole("img")).toHaveAttribute(
    "src",
    "/media/test-1-1280-v1.webp",
  );
  await cdp.detach();
  await expect(card.locator(".price")).toContainText("комплект (2 шт.)");
  await card.locator("summary").click();
  await expect(card).toContainText("Synthetic gallery fixture");
  await card.getByRole("button", { name: "В корзину", exact: true }).click();
  await expect(page.locator(".basket")).toContainText("комплект (2 шт.)");
  await expect(page.locator(".basket img")).toHaveAttribute(
    "src",
    "/media/test-0-320-v1.webp",
  );
  await page.locator(".basket img").scrollIntoViewIfNeeded();
  await expect
    .poll(() =>
      page
        .locator(".basket img")
        .evaluate(
          (img) =>
            img.complete &&
            img.naturalWidth > 0 &&
            img.clientWidth === img.clientHeight,
        ),
    )
    .toBe(true);
  await page
    .locator(".basket")
    .getByRole("link", { name: "Browser wheel bundle", exact: true })
    .click();
  await expect(page).toHaveURL(/\/products\/[0-9a-f-]+$/);
  await expect(page.locator(".product-page h1")).toHaveText(
    "Browser wheel bundle",
  );
  await expect(page.locator(".basket")).toContainText("Browser wheel bundle");
  await page.reload();
  await expect(page.locator(".basket")).toContainText("Browser wheel bundle");
  await expect(page.locator(".basket img")).toHaveAttribute(
    "src",
    "/media/test-0-320-v1.webp",
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await expect
    .poll(() =>
      page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    )
    .toBe(true);
  await page.screenshot({
    path: ".tmp/product-page-mobile.png",
    fullPage: true,
  });
  await page
    .locator(".product-page .breadcrumbs")
    .getByRole("link", { name: "Каталог", exact: true })
    .click();
  await expect(page.locator(".basket")).toContainText("Browser wheel bundle");
  await page.getByLabel("Имя", { exact: true }).fill("Browser Bundle Buyer");
  await page.getByLabel("Телефон", { exact: true }).fill("+79000000000");
  await page
    .getByRole("button", { name: "Оформить заявку", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Заявка получена" }),
  ).toBeVisible();
});
