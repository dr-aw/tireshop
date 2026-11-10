const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);
const picture =
  '<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>';
const reply = (route) =>
  route.fulfill({
    contentType: "image/svg+xml",
    headers: { "Cache-Control": "public, max-age=3600" },
    body: picture,
  });
async function gallery(page, count = 4) {
  const id = "10000000-0000-4000-8000-000000000002";
  await page.route("**/api/products/" + id, (r) =>
    r.fulfill({
      json: {
        id,
        name: "Preload",
        unit_price_rub: 100,
        stock: 2,
        available: true,
        images: Array.from({ length: count }, (_, i) => ({
          url: `/media/preload-${i}.svg`,
        })),
      },
    }),
  );
  return "/products/" + id;
}
test("neighbors preload sequentially after current image and follow position", async ({
  page,
}) => {
  const requested = [];
  let current, next;
  let releaseCurrent, releaseNext;
  const gateCurrent = new Promise((r) => (releaseCurrent = r)),
    gateNext = new Promise((r) => (releaseNext = r));
  await page.route("**/media/preload-*.svg", async (route) => {
    const n = Number(/preload-(\d+)/.exec(route.request().url())[1]);
    requested.push(n);
    if (n === 0 && !current) {
      current = true;
      await gateCurrent;
    }
    if (n === 1 && !next) {
      next = true;
      await gateNext;
    }
    await reply(route);
  });
  await page.goto(await gallery(page));
  await expect.poll(() => requested).toEqual([0]);
  releaseCurrent();
  await expect.poll(() => requested).toEqual([0, 1]);
  await page.waitForTimeout(200);
  expect(requested).toEqual([0, 1]);
  releaseNext();
  await expect.poll(() => requested).toEqual([0, 1, 3]);
  await page.getByRole("button", { name: "Следующее фото: Preload" }).click();
  await expect.poll(() => requested.includes(2)).toBe(true);
  await page.getByRole("button", { name: "Предыдущее фото: Preload" }).click();
  expect(requested.filter((n) => n === 3).length).toBe(1);
});
test("one photo has no neighbors; two photos request their neighbor once", async ({
  page,
}) => {
  const requested = [];
  await page.route("**/media/preload-*.svg", (r) => {
    requested.push(r.request().url());
    return reply(r);
  });
  await page.goto(await gallery(page, 1));
  await expect(page.locator(".product-page-gallery img")).toBeVisible();
  await page.waitForTimeout(200);
  expect(requested.length).toBe(1);
  await page.goto(await gallery(page, 2));
  await expect.poll(() => requested.length).toBe(3);
  await page.waitForTimeout(200);
  expect(requested.length).toBe(3);
});
test("failed neighbor does not block the other neighbor", async ({ page }) => {
  const requested = [];
  await page.route("**/media/preload-*.svg", (r) => {
    const n = Number(/preload-(\d+)/.exec(r.request().url())[1]);
    requested.push(n);
    return n === 1 ? r.abort() : reply(r);
  });
  await page.goto(await gallery(page));
  await expect.poll(() => requested.includes(3)).toBe(true);
  await expect(page.locator(".product-page-gallery img")).toBeVisible();
  expect(requested).toEqual([0, 1, 3]);
});

test("catalog galleries share one queue and offscreen cards wait for scrolling", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  const requested = [];
  let active = 0,
    maxActive = 0;
  await page.route("**/media/queued-*.svg", async (route) => {
    const [, card, image] = /queued-(\d+)-(\d+)/.exec(route.request().url());
    requested.push(`${card}-${image}`);
    if (image !== "0") {
      active++;
      maxActive = Math.max(maxActive, active);
      await new Promise((r) => setTimeout(r, 60));
    }
    await reply(route);
    if (image !== "0") active--;
  });
  await page.route(/\/api\/products\?/, (r) =>
    r.fulfill({
      json: {
        total: 9,
        items: Array.from({ length: 9 }, (_, i) => ({
          id: `10000000-0000-4000-8000-${String(i + 1).padStart(12, "0")}`,
          name: `Queued ${i}`,
          unit_price_rub: 100,
          stock: 1,
          available: true,
          images: Array.from({ length: 3 }, (_, n) => ({
            url: `/media/queued-${i}-${n}.svg`,
          })),
        })),
      },
    }),
  );
  await page.goto("/");
  await expect
    .poll(
      () =>
        requested.filter((s) => s.endsWith("-1") || s.endsWith("-2")).length,
    )
    .toBeGreaterThanOrEqual(4);
  expect(maxActive).toBe(1);
  expect(requested).not.toContain("8-1");
  expect(requested).not.toContain("8-2");
  await page.locator(".product").nth(8).scrollIntoViewIfNeeded();
  await expect
    .poll(() => requested.includes("8-1") && requested.includes("8-2"))
    .toBe(true);
  expect(maxActive).toBe(1);
});
