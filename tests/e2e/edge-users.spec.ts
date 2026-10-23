import AxeBuilder from "@axe-core/playwright";
import {
  test,
  expect,
  API,
  blankRequest,
  collection,
  dialog,
  fillTab,
  openRoute,
  response,
  route,
  send,
  sidebar,
  sidebarItem,
  statusBadge,
  urlInput,
  methodSelect,
} from "./fixtures/test";

/** Creating a collection opens it: the URL names it. */
async function expectOpened(page: import("@playwright/test").Page, name: string) {
  await expect.poll(() => new URL(page.url()).searchParams.get("c")).toBe(name);
}

// People at the edges: blocked storage, no network, other scripts, keyboard
// only, the smallest phones, huge collections, screen readers.

test.describe("browsers with storage blocked", () => {
  // "Block all cookies", some private modes and locked-down profiles make every
  // localStorage access throw. The app must still open and send requests.
  for (const mode of ["throws on access", "throws on write"] as const) {
    test(`the app works when localStorage ${mode}`, async ({ page }) => {
      await page.addInitScript((m) => {
        const err = () => {
          throw new DOMException("The operation is insecure.", "SecurityError");
        };
        if (m === "throws on access") {
          Object.defineProperty(window, "localStorage", { get: err, configurable: true });
        } else {
          Storage.prototype.setItem = function () {
            throw new DOMException("Quota exceeded", "QuotaExceededError");
          };
        }
      }, mode);
      await page.goto("/hittable");
      await sidebar(page).getByRole("button", { name: "New Collection" }).click();
      await dialog(page, "Create collection").getByRole("textbox").fill("Private");
      await page.keyboard.press("Enter");
      await expectOpened(page, "Private");
      await expect(sidebar(page).getByRole("button", { name: "Private" })).toBeVisible();
    });
  }
});

test.describe("unreliable networks", () => {
  test("going offline mid-session shows an error, and the next send works again", async ({ page, seed, context }) => {
    await blankRequest(page, seed, `${API}/users`);
    await context.setOffline(true);
    await send(page);
    await expect(response(page).getByText(/fail|network|offline|load/i).first()).toBeVisible();
    await context.setOffline(false);
    await send(page);
    await expect(statusBadge(page)).toContainText("200");
  });

  test("double-clicking Send does not break the response", async ({ page, seed }) => {
    await blankRequest(page, seed, `${API}/slow`);
    await urlInput(page).click();
    const sendBtn = page.getByRole("button", { name: /^(Send|Sending…)( \(|$)/ });
    await sendBtn.dblclick();
    await expect(statusBadge(page)).toContainText("200", { timeout: 15_000 });
  });
});

test.describe("any language, any text", () => {
  const names = ["Café ☕ API", "مرحبا بالعالم", "日本語のコレクション", "👩🏽‍💻 emoji team"];

  test("collections and routes named in any script survive a reload", async ({ page }) => {
    for (const name of names) {
      await page.goto("/hittable"); // creating a collection opens it; start from the list
      await sidebar(page).getByRole("button", { name: "New Collection" }).click();
      await dialog(page, "Create collection").getByRole("textbox").fill(name);
      await page.keyboard.press("Enter");
      await expectOpened(page, name);
    }
    await page.goto("/hittable");
    for (const name of names) await expect(sidebarItem(page, name)).toBeVisible();
    // and each opens by its own name, through the URL
    for (const name of names) {
      await page.goto(`/hittable?c=${encodeURIComponent(name)}`);
      await expect(sidebar(page).getByRole("button", { name })).toBeVisible(); // its header, with a way back
    }
  });

  test("unicode in the URL, headers and body reaches the server intact", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await methodSelect(page).selectOption("POST");
    await urlInput(page).fill(`${API}/echo?q=${encodeURIComponent("naïve 東京")}`);
    await fillTab(page, "body", JSON.stringify({ greeting: "مرحبا", city: "東京", mood: "😀" }));
    await send(page);
    await expect(statusBadge(page)).toContainText("200");
    await expect(response(page).getByText("東京").first()).toBeVisible();
    await expect(response(page).getByText("😀").first()).toBeVisible();
  });
});

test.describe("keyboard-only users", () => {
  test("create a collection, open a request and send it without a mouse", async ({ page, seed }) => {
    await seed({ collections: [collection("Keys", [route("list", `curl -X GET ${API}/users`)])] });
    await page.goto("/hittable");
    const newCollection = sidebar(page).getByRole("button", { name: "New Collection" });
    await newCollection.focus();
    await page.keyboard.press("Enter");
    await page.keyboard.type("Typed");
    await page.keyboard.press("Enter");
    await expectOpened(page, "Typed");

    await openRoute(page, "Keys", "list");
    await urlInput(page).focus();
    await page.keyboard.press("ControlOrMeta+Enter");
    await expect(statusBadge(page)).toContainText("200");
  });

  test("every dialog closes with Escape and gives focus back", async ({ page }) => {
    await page.goto("/hittable");
    const opener = sidebar(page).getByRole("button", { name: "New Collection" });
    await opener.focus();
    await page.keyboard.press("Enter");
    await expect(dialog(page, "Create collection")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog(page, "Create collection")).toBeHidden();
    await expect(opener).toBeFocused();
  });
});

test.describe("the smallest screens", () => {
  test.use({ viewport: { width: 320, height: 568 }, isMobile: true, hasTouch: true }); // iPhone SE (1st gen)

  for (const path of ["/", "/docs", "/docs/web-app", "/hittable"]) {
    test(`${path} fits a 320px screen without sideways scrolling`, async ({ page }) => {
      await page.goto(path);
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
      expect(overflow).toBeLessThanOrEqual(1);
    });
  }

  test("a request can be sent on a 320px screen", async ({ page, seed }) => {
    await blankRequest(page, seed, `${API}/users`);
    await send(page);
    await expect(statusBadge(page)).toContainText("200");
  });
});

test.describe("power users", () => {
  test("a collection with 1,000 routes opens, filters and stays responsive", async ({ page, seed }) => {
    const items = Array.from({ length: 1000 }, (_, i) => route(`route-${String(i).padStart(4, "0")}`, `curl -X GET ${API}/users/${i}`));
    await seed({ collections: [collection("Huge", items)] });
    const start = Date.now();
    await page.goto("/hittable?c=Huge");
    await expect(sidebarItem(page, "route-0000")).toBeVisible();
    expect(Date.now() - start).toBeLessThan(10_000);
    await sidebar(page).getByRole("textbox").fill("route-0999");
    await expect(sidebarItem(page, "route-0999")).toBeVisible();
    await expect(sidebarItem(page, "route-0001")).toBeHidden();
  });
});

test.describe("reduced motion", () => {
  test("with reduce motion on, transitions are instant", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/hittable");
    const duration = await sidebar(page).evaluate((el) => {
      const panel = el.querySelector<HTMLElement>("[class*=transition]") ?? el;
      return getComputedStyle(panel).transitionDuration;
    });
    expect(parseFloat(duration)).toBeLessThan(0.01);
  });
});

test.describe("screen-reader users", () => {
  // Serious and critical WCAG A/AA problems block someone using a screen reader
  // or keyboard; colour-contrast of decorative dim text is reported separately.
  for (const path of ["/", "/docs", "/docs/getting-started", "/hittable"]) {
    test(`${path} has no serious accessibility violations`, async ({ page }) => {
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).disableRules(["color-contrast"]).analyze();
      const blocking = results.violations
        .filter((v) => v.impact === "serious" || v.impact === "critical")
        .map((v) => `${v.id}: ${v.help} (${v.nodes.length}× e.g. ${v.nodes[0]?.target.join(" ")})`);
      expect(blocking).toEqual([]);
    });
  }
});
