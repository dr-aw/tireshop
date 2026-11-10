const { test, expect } = require(process.env.PLAYWRIGHT_TEST_MODULE);
test("contact messenger links and SVGs work on mobile and desktop", async ({
  page,
}) => {
  await page.route("https://api-maps.yandex.ru/**", (r) => r.abort());
  await page.goto("/contacts/");
  const section = page.locator(".contact-messengers");
  await expect(
    section.getByRole("link", { name: "Написать в WhatsApp" }),
  ).toHaveAttribute("href", "https://wa.me/79027625164");
  await expect(
    section.getByRole("link", { name: "Написать в Telegram" }),
  ).toHaveAttribute("href", "https://t.me/marksv95");
  for (const image of await section.locator("img").all())
    await expect
      .poll(() => image.evaluate((i) => i.complete && i.naturalWidth > 0))
      .toBe(true);
  await expect(
    section.getByRole("link", { name: "Написать в MAX" }),
  ).toHaveAttribute(
    "href",
    "https://max.ru/u/f9LHodD0cOIhCr6cQVaeSQA4LhkcMxoaO7IRakvnCvEfSosUO3AtwjGvlCw",
  );
  for (const width of [320, 390, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  }
  await page.emulateMedia({ colorScheme: "dark" });
  await page.setViewportSize({ width: 390, height: 900 });
  await section.scrollIntoViewIfNeeded();
  await page.screenshot({ path: ".tmp/contact-messengers.png" });
});
