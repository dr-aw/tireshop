const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);

test("SSG category hydrates into selection with keyboard, URL and canonical support", async ({
  page,
  request,
}) => {
  const html = await (await request.get("/catalog/tires/")).text();
  const embedded = JSON.parse(
    html.match(
      /<script id="page-data" type="application\/json">([\s\S]*?)<\/script>/,
    )[1],
  );
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error" && /hydration|Minified React/.test(m.text()))
      errors.push(m.text());
  });
  await page.route("**/api/products?**", (route) =>
    route.fulfill({
      json: { items: embedded.products, total: embedded.total },
    }),
  );
  await page.route("**/api/products/filters?**", (route) =>
    route.fulfill({ json: { options: {} } }),
  );
  await page.route("**/api/products/selection?**", (route) =>
    route.fulfill({
      json: {
        radii: [13, 14, 15, 15.5, 16, 17.5, 19.5, 20, 22.5].map((value) => ({
          value: String(value),
          label: `R${value}`,
          product_count: 2,
          unit_count: value === 17.5 ? "900719925474099100" : "8",
        })),
        sizes: route.request().url().includes("radius=17.5")
          ? [
              {
                value: "195/70R17.5",
                label: "195/70R17.5",
                product_count: 1,
                unit_count: "8",
              },
              {
                value: "205/60R17.5",
                label: "205/60R17.5",
                product_count: 1,
                unit_count: "12",
              },
            ]
          : [],
      },
    }),
  );
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/catalog/tires/");
  await expect(
    page
      .locator(".selection-categories")
      .getByRole("link", { name: "Шины", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  const radius = page.getByRole("button", { name: /^R17\.5,/ });
  await radius.focus();
  await page.keyboard.press("Enter");
  const size = page.getByRole("button", {
    name: "195/70R17.5 8 шт",
    exact: true,
  });
  await size.focus();
  await page.keyboard.press("Space");
  await expect(size).toHaveAttribute("aria-pressed", "true");
  await expect(page).toHaveURL(/tyre_size=/);
  await expect(page.locator('meta[name="robots"]')).toHaveAttribute(
    "content",
    "noindex,follow",
  );
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
    "href",
    /\/catalog\/tires\/$/,
  );
  for (const width of [320, 390, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({ path: `.tmp/selection-grid-${width}.png` });
  }
  expect(errors).toEqual([]);
});
