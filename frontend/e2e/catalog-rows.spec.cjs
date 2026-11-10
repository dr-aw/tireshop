const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);

test("characteristics expand by visual row on desktop and mobile", async ({
  page,
}) => {
  for (const [width, columns] of [
    [1280, 3],
    [390, 2],
  ]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/?kind=tyre&q=Browser+filter+tyre");
    await expect(page.locator(".product")).toHaveCount(24);
    const details = page.locator(".product .catalog-specs");
    await expect(details).toHaveCount(24);
    await details.first().locator("summary").click();
    for (let i = 0; i < columns; i++)
      await expect(details.nth(i)).toHaveAttribute("open", "");
    await expect(details.nth(columns)).not.toHaveAttribute("open", "");
    await details.nth(1).locator("summary").focus();
    await page.keyboard.press("Enter");
    await expect(page.locator(".product .catalog-specs[open]")).toHaveCount(0);
    await details.nth(columns).locator("summary").click();
    await expect(details.first()).not.toHaveAttribute("open", "");
    for (let i = columns; i < columns * 2; i++)
      await expect(details.nth(i)).toHaveAttribute("open", "");
  }
  await page.setViewportSize({ width: 1280, height: 900 });
  await expect
    .poll(() =>
      page.locator(".products").evaluate((grid) => {
        const groups = new Map();
        for (const card of grid.children) {
          const top = Math.round(card.getBoundingClientRect().top);
          const opened = card.querySelector("details").open;
          if (groups.has(top) && groups.get(top) !== opened) return false;
          groups.set(top, opened);
        }
        return true;
      }),
    )
    .toBe(true);
});
