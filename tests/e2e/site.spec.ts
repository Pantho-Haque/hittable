import { test, expect } from "./fixtures/test";

test.describe("landing page", () => {
  test("loads with its hero, sections and no console errors", async ({ page }) => {
    const errors: string[] = [];
    page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
    const res = await page.goto("/");
    expect(res?.status()).toBe(200);
    await expect(page).toHaveTitle(/Hittable/);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Built for developers who move fast." })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Stay in the flow." })).toBeVisible();
    await expect(page.getByText("Send request")).toBeVisible();
    await page.waitForLoadState("networkidle");
    // Third-party images can fail offline; anything the app itself logs cannot.
    expect(errors.filter((e) => !/Failed to load resource/.test(e))).toEqual([]);
  });

  test("Launch App opens the client", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "Launch App" }).first().click();
    await expect(page).toHaveURL(/\/hittable$/);
    await expect(page.getByRole("complementary", { name: "Collections" })).toBeVisible();
  });

  test("top bar links go to sections, docs and the app", async ({ page }) => {
    await page.goto("/");
    const nav = page.getByRole("banner");
    await nav.getByRole("link", { name: "Docs" }).click();
    await expect(page).toHaveURL(/\/docs$/);
    await expect(page.getByRole("heading", { level: 1, name: "Hittable documentation" })).toBeVisible();

    await page.goto("/");
    await nav.getByRole("link", { name: "Features" }).click();
    await expect(page).toHaveURL(/#features$/);
    await expect(page.locator("#features")).toBeInViewport();

    await nav.getByRole("link", { name: "Open App" }).click();
    await expect(page).toHaveURL(/\/hittable$/);
  });

  test("external links open in a new tab without leaking the opener", async ({ page }) => {
    await page.goto("/");
    const external = page.locator('a[href^="http"]');
    const count = await external.count();
    expect(count).toBeGreaterThan(0);
    for (let i = 0; i < count; i++) {
      await expect(external.nth(i)).toHaveAttribute("target", "_blank");
      await expect(external.nth(i)).toHaveAttribute("rel", /noopener/);
    }
  });

  test("the logo goes home from any page", async ({ page }) => {
    await page.goto("/docs/faq");
    await page.getByRole("banner").getByRole("link", { name: "Hittable" }).first().click();
    await expect(page).toHaveURL(/\/$/);
  });
});

test.describe("static routes and headers", () => {
  for (const path of ["/", "/hittable", "/docs", "/docs/web-app"]) {
    test(`${path} sends the security headers`, async ({ request }) => {
      const res = await request.get(path);
      expect(res.status()).toBe(200);
      const h = res.headers();
      expect(h["x-content-type-options"]).toBe("nosniff");
      expect(h["x-frame-options"]).toBe("DENY");
      expect(h["content-security-policy"]).toContain("frame-ancestors 'none'");
      expect(h["referrer-policy"]).toBe("strict-origin-when-cross-origin");
      expect(h["x-powered-by"]).toBeUndefined();
    });
  }

  test("robots.txt allows the site and hides the API", async ({ request }) => {
    const res = await request.get("/robots.txt");
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).toContain("User-Agent: *");
    expect(body).toContain("Disallow: /api/");
    expect(body).toMatch(/Sitemap: https:\/\/hittable\.vercel\.app\/sitemap\.xml/);
  });

  test("sitemap.xml lists the app and every docs page", async ({ request }) => {
    const res = await request.get("/sitemap.xml");
    expect(res.status()).toBe(200);
    expect(res.headers()["content-type"]).toContain("xml");
    const xml = await res.text();
    for (const loc of ["https://hittable.vercel.app/hittable", "https://hittable.vercel.app/docs", "https://hittable.vercel.app/docs/web-app", "https://hittable.vercel.app/docs/faq"]) {
      expect(xml).toContain(`<loc>${loc}</loc>`);
    }
  });

  test("the web manifest is served", async ({ request }) => {
    const res = await request.get("/manifest.json");
    expect(res.status()).toBe(200);
    expect(await res.json()).toHaveProperty("name");
  });

  test("an unknown page shows the 404 page with a way back", async ({ page }) => {
    const res = await page.goto("/definitely-not-a-page");
    expect(res?.status()).toBe(404);
    await expect(page.getByRole("heading", { name: "404" })).toBeVisible();
    await expect(page.getByText("Page not found")).toBeVisible();
    await page.getByRole("main").getByRole("link", { name: "Docs" }).click();
    await expect(page).toHaveURL(/\/docs$/);
  });

  test("GET on the proxy is not allowed", async ({ request }) => {
    const res = await request.get("/api/proxy");
    expect(res.status()).toBe(405);
  });
});
