import type { Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { test, expect, API, dialog, fillTab, openRequest, openLauncher, response, send, statusBadge, urlInput, methodSelect, FOLDER_ONLY } from "./fixtures/test";
import { seedPrivateFolder, openWorkspace, openFile, expand, node, tree } from "./fixtures/directory";

// People at the edges: blocked storage, no network, other scripts, keyboard
// only, the smallest phones, huge folders, screen readers.

const folderOnly = (browserName: string) => test.skip(browserName !== "chromium", FOLDER_ONLY);

test.describe("browsers with storage blocked", () => {
  // "Block all cookies", some private modes and locked-down profiles make every
  // localStorage access throw. The app must still open and send requests.
  for (const mode of ["throws on access", "throws on write"] as const) {
    const block = (page: Page) =>
      page.addInitScript((m) => {
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

    test(`the launcher and the palette work when localStorage ${mode}`, async ({ page }) => {
      await block(page);
      await openLauncher(page);
      await expect(page.getByRole("heading", { level: 1, name: "Open a workspace" })).toBeVisible();
      await page.keyboard.press("ControlOrMeta+k");
      await page.keyboard.type("keyboard shortcuts");
      await page.keyboard.press("Enter"); // the palette remembers recent picks in localStorage
      await expect(dialog(page, "Keyboard shortcuts")).toBeVisible();
    });

    test(`a folder request can be sent when localStorage ${mode}`, async ({ page, browserName }) => {
      folderOnly(browserName);
      await block(page);
      await openRequest(page, { url: `${API}/users/1` });
      await send(page);
      await expect(statusBadge(page)).toHaveText("200 OK");
    });
  }
});

test.describe("unreliable networks", () => {
  test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

  test("going offline mid-session shows an error, and the next send works again", async ({ page, context }) => {
    await openRequest(page, { url: `${API}/users` });
    await context.setOffline(true);
    await send(page);
    await expect(response(page).getByText(/fail|network|offline|load/i).first()).toBeVisible();
    await context.setOffline(false);
    await send(page);
    await expect(statusBadge(page)).toContainText("200");
  });

  test("double-clicking Send does not break the response", async ({ page }) => {
    await openRequest(page, { url: `${API}/slow` });
    await urlInput(page).click();
    const sendBtn = page.getByRole("button", { name: /^(Send|Sending…)( \(|$)/ });
    await sendBtn.dblclick();
    await expect(statusBadge(page)).toContainText("200", { timeout: 15_000 });
  });
});

test.describe("any language, any text", () => {
  test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

  const names = ["Café ☕ API", "مرحبا بالعالم", "日本語のリクエスト", "👩🏽‍💻 emoji team"];

  test("files named in any script are listed, open in the Runner, and survive a reload", async ({ page }) => {
    const files = Object.fromEntries(
      names.map((n, i) => [`hittable/${n}/${n}.hit`, JSON.stringify({ method: "GET", url: `${API}/users/${i + 1}`, headers: {}, params: {}, body: "", response: null })]),
    );
    await seedPrivateFolder(page, files);
    await openWorkspace(page);
    for (const [i, n] of names.entries()) {
      await openFile(page, `hittable/${n}/${n}.hit`);
      await expect(urlInput(page)).toHaveValue(`${API}/users/${i + 1}`);
    }
    await page.reload();
    await expect(tree(page)).toBeVisible();
    for (const n of names) {
      await expand(page, `hittable/${n}`);
      await expect(node(page, `hittable/${n}/${n}.hit`)).toBeVisible();
    }
  });

  test("a new file can be named in any script", async ({ page }) => {
    await seedPrivateFolder(page);
    await openWorkspace(page);
    await page.getByRole("button", { name: "New File" }).click();
    await page.getByPlaceholder("File name").fill("طلب ☕.hit");
    await page.getByPlaceholder("File name").press("Enter");
    await expect(node(page, "طلب ☕.hit")).toBeVisible();
  });

  test("unicode in the URL, headers and body reaches the server intact", async ({ page }) => {
    await openRequest(page);
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
  test("open a request from the palette and send it without a mouse", async ({ page, browserName }) => {
    folderOnly(browserName);
    await openRequest(page, { files: { "hittable/users/list.hit": JSON.stringify({ method: "GET", url: `${API}/users`, headers: {}, params: {}, body: "", response: null }) } });
    await page.keyboard.press("ControlOrMeta+k");
    await page.keyboard.type("list.hit");
    await page.keyboard.press("Enter");
    await expect(urlInput(page)).toHaveValue(`${API}/users`);
    await page.keyboard.press("ControlOrMeta+Enter");
    await expect(statusBadge(page)).toContainText("200");
  });

  test("a dialog closes with Escape and gives focus back", async ({ page }) => {
    await openLauncher(page);
    const opener = page.getByRole("button", { name: "Search and commands" });
    await opener.focus();
    await page.keyboard.press("Enter");
    await expect(dialog(page, "Command palette")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog(page, "Command palette")).toBeHidden();
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

  test("a request can be sent on a 320px screen", async ({ page, browserName }) => {
    folderOnly(browserName);
    await openRequest(page, { url: `${API}/users` });
    await send(page);
    await expect(statusBadge(page)).toContainText("200");
  });
});

test.describe("power users", () => {
  test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

  test("a folder with 1,000 requests opens, and the palette finds one quickly", async ({ page }) => {
    const files = Object.fromEntries(
      Array.from({ length: 1000 }, (_, i) => [
        `hittable/many/route-${String(i).padStart(4, "0")}.hit`,
        JSON.stringify({ method: "GET", url: `${API}/users/${i}`, headers: {}, params: {}, body: "", response: null }),
      ]),
    );
    await seedPrivateFolder(page, files);
    await openLauncher(page);
    const start = Date.now();
    await page.getByRole("button", { name: "Open a local folder" }).click(); // writes the 1,000 files, then opens
    await expand(page, "hittable");
    await expand(page, "hittable/many");
    await expect(node(page, "hittable/many/route-0000.hit")).toBeVisible();
    expect(Date.now() - start).toBeLessThan(15_000);

    await page.keyboard.press("ControlOrMeta+k");
    await page.keyboard.type("route-0999");
    await expect(dialog(page, "Command palette").getByRole("option").first()).toContainText("route-0999.hit");
    await page.keyboard.press("Enter");
    await expect(urlInput(page)).toHaveValue(`${API}/users/999`);
  });
});

test.describe("reduced motion", () => {
  test("with reduce motion on, animations are off and transitions are instant", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/");
    // The hero's request → commit sequence is simply shown.
    await expect(page.locator(".hit-reveal").first()).toHaveCSS("animation-name", "none");
    const duration = await page.locator("[class*=transition]").first().evaluate((el) => getComputedStyle(el).transitionDuration);
    expect(parseFloat(duration)).toBeLessThan(0.01);
  });
});

test.describe("screen-reader users", () => {
  // Serious and critical WCAG A/AA problems block someone using a screen reader
  // or keyboard. Colour contrast is checked too (NFR-1).
  // Checked with motion reduced (set before the page loads), so a fade-in
  // caught half way, like the landing page's hero, is not reported as low contrast.
  test.beforeEach(({ page }) => page.emulateMedia({ reducedMotion: "reduce" }));

  const blockingViolations = async (page: Page) => {
    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
    return results.violations
      .filter((v) => v.impact === "serious" || v.impact === "critical")
      .map((v) => `${v.id}: ${v.help} (${v.nodes.length}× e.g. ${v.nodes[0]?.target.join(" ")})`);
  };

  for (const path of ["/", "/docs", "/docs/getting-started", "/hittable"]) {
    test(`${path} has no serious accessibility violations`, async ({ page }) => {
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      expect(await blockingViolations(page)).toEqual([]);
    });
  }

  test("the launcher with the old-collections banner and a sign-in error has no serious accessibility violations", async ({ page, seed }) => {
    await seed({ hittable: JSON.stringify([{ collectionName: "Old", items: [] }]) });
    await page.goto("/hittable?auth_error=denied");
    await expect(page.getByRole("region", { name: /Your old browser collections/ })).toBeVisible();
    expect(await blockingViolations(page)).toEqual([]);
  });

  test("a folder request with its response has no serious accessibility violations", async ({ page, browserName }) => {
    folderOnly(browserName);
    await openRequest(page, { url: `${API}/users` });
    await send(page);
    await expect(statusBadge(page)).toBeVisible();
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();
    expect(await blockingViolations(page)).toEqual([]);
  });
});
