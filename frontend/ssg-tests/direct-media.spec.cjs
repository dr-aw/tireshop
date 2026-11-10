const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);
test("direct media handles gallery, original links and old cart paths without proxy", async ({
  page,
}) => {
  const base = process.env.VITE_MEDIA_BASE_URL;
  test.skip(!base, "Run against a build with VITE_MEDIA_BASE_URL");
  const requests = [];
  page.on("request", (r) => {
    if (r.resourceType() === "image") requests.push(r.url());
  });
  await page.route(base + "/**", (r) =>
    r.fulfill({
      contentType: "image/svg+xml",
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>',
    }),
  );
  const id = "10000000-0000-4000-8000-000000000002";
  const images = [
    { url: "/media/a-1280-v1.webp" },
    { url: "/media/b-1280-v1.webp" },
  ];
  await page.route("**/api/products/" + id, (r) =>
    r.fulfill({
      json: {
        id,
        name: "Direct photo",
        stock: 4,
        unit_price_rub: 123,
        available: true,
        images,
      },
    }),
  );
  await page.goto("/products/" + id);
  await expect(page.locator(".product-page-gallery img")).toHaveAttribute(
    "src",
    base + images[0].url,
  );
  await expect(
    page.getByRole("link", { name: "Открыть полный кадр" }),
  ).toHaveAttribute("href", base + images[0].url);
  await page
    .getByRole("button", { name: "Следующее фото: Direct photo" })
    .click();
  await expect(page.locator(".product-page-gallery img")).toHaveAttribute(
    "src",
    base + images[1].url,
  );
  await page.getByRole("button", { name: "В корзину", exact: true }).click();
  expect(
    await page.evaluate(
      () => JSON.parse(localStorage.getItem("tireshop.cart.v1"))[0].image_url,
    ),
  ).toBe(images[0].url);
  await page.goto("/cart/");
  await expect(page.locator(".basket img")).toHaveAttribute(
    "src",
    base + "/media/a-320-v1.webp",
  );
  expect(
    requests.some(
      (u) =>
        new URL(u).origin === "http://127.0.0.1:5175" &&
        new URL(u).pathname.startsWith("/media/"),
    ),
  ).toBe(false);
  expect(
    requests.filter((u) => u.startsWith(base + "/media/")).length,
  ).toBeGreaterThan(0);
});
