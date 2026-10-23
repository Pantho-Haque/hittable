import { test as base, expect, type Page, type Locator } from "@playwright/test";

// The fixture API (tests/e2e/fixtures/api-server.mjs) listens dual-stack.
// `[::1]` is not one of the hostnames the app hands to the extension, so
// requests to it go through the server-side /api/proxy like any remote URL.
// `127.0.0.1` is routed to the extension instead.
const API_PORT = 4100;
export const API = `http://[::1]:${API_PORT}`;
export const LOCAL_API = `http://127.0.0.1:${API_PORT}`;

export type Route = { type: "route"; name: string; curl: string; response?: string };
export type Folder = { type: "folder"; name: string; items: (Route | Folder)[] };
export type Collection = {
  collectionName: string;
  items: (Route | Folder)[];
  env?: Record<string, string>;
  secrets?: Record<string, string>;
};

export const route = (name: string, curl: string, response = ""): Route => ({ type: "route", name, curl, response });
export const folder = (name: string, items: (Route | Folder)[]): Folder => ({ type: "folder", name, items });
export const collection = (
  collectionName: string,
  items: (Route | Folder)[] = [],
  env: Record<string, string> = {},
  secrets: Record<string, string> = {},
): Collection => ({ collectionName, items, env, secrets });

type Seed = {
  collections?: Collection[];
  history?: unknown[];
  notes?: Record<string, { title: string; content: string }>;
  sidebarCollapsed?: boolean;
};

type Fixtures = {
  /** Answer the app's extension handshake like the real extension does. Default true. */
  extension: boolean;
  /** Uncaught page errors collected during the test; the test fails if any remain. */
  pageErrors: Error[];
  /** Seed localStorage once, before the app's first load (reloads keep what the app wrote). */
  seed: (data: Seed) => Promise<void>;
  /** Every alert/confirm/prompt the page opened. Dialogs are accepted. */
  dialogs: string[];
};

// Mirrors browserExtension/content.js + background.js: a ping gets a ready
// signal, a request is fetched and answered with the same id.
function fakeExtension() {
  window.addEventListener("message", async (event) => {
    if (event.source !== window || event.origin !== window.location.origin) return;
    const msg = event.data;
    if (msg?.type === "HITTABLE_PING") {
      window.postMessage({ type: "HITTABLE_EXTENSION_READY" }, window.location.origin);
    }
    if (msg?.type === "HITTABLE_REQUEST") {
      (window as unknown as { __extensionRequests: unknown[] }).__extensionRequests ??= [];
      (window as unknown as { __extensionRequests: unknown[] }).__extensionRequests.push(msg.payload);
      const { url, method, headers, body } = msg.payload;
      try {
        const res = await fetch(url, {
          method,
          headers,
          body: ["GET", "HEAD"].includes(method) ? undefined : body,
        });
        const type = res.headers.get("content-type") ?? "";
        const data = type.includes("application/json") ? await res.json() : await res.text();
        const responseHeaders: Record<string, string> = {};
        res.headers.forEach((v, k) => (responseHeaders[k] = v));
        window.postMessage(
          { type: "HITTABLE_RESPONSE", id: msg.id, success: true, data, status: res.status, statusText: res.statusText, ok: res.ok, headers: responseHeaders, cookies: [] },
          window.location.origin,
        );
      } catch (err) {
        window.postMessage({ type: "HITTABLE_RESPONSE", id: msg.id, success: false, error: String(err) }, window.location.origin);
      }
    }
  });
}

// Remember what the app copies. Only Chromium lets a test read the system
// clipboard back; recording the write checks the same thing in every engine.
function recordClipboard() {
  const clip = navigator.clipboard;
  if (!clip?.writeText) return;
  const write = clip.writeText.bind(clip);
  clip.writeText = (text: string) => {
    (window as unknown as { __copied?: string }).__copied = text;
    return write(text).catch(() => undefined); // headless WebKit may refuse the real write
  };
}

export const test = base.extend<Fixtures>({
  extension: [true, { option: true }],

  page: async ({ page, extension }, provide) => {
    if (extension) await page.addInitScript(fakeExtension);
    await page.addInitScript(recordClipboard);
    await provide(page);
  },

  pageErrors: [
    async ({ page }, provide) => {
      const errors: Error[] = [];
      page.on("pageerror", (err) => errors.push(err));
      await provide(errors);
      // WebKit reports a Next.js prefetch that a navigation cancelled as an
      // uncaught "…?_rsc=… due to access control checks"; nothing breaks.
      const real = errors.filter((e) => !/\?_rsc=\S+ due to access control checks/.test(e.message));
      expect(real.map((e) => e.message), "uncaught errors in the page").toEqual([]);
    },
    { auto: true },
  ],

  dialogs: [
    async ({ page }, provide) => {
      const seen: string[] = [];
      page.on("dialog", (d) => {
        seen.push(`${d.type()}: ${d.message()}`);
        d.accept().catch(() => {});
      });
      await provide(seen);
    },
    { auto: true },
  ],

  seed: async ({ page }, provide) => {
    // Each call writes once, in call order; later reloads keep what the app wrote.
    let calls = 0;
    await provide(async (data) => {
      const n = ++calls;
      await page.addInitScript(
        ({ d, n }) => {
          if (Number(sessionStorage.getItem("__e2e_seed") ?? 0) >= n) return;
          sessionStorage.setItem("__e2e_seed", String(n));
          if (d.collections) localStorage.setItem("hittable", JSON.stringify(d.collections));
          if (d.history) localStorage.setItem("hittable_history", JSON.stringify(d.history));
          if (d.notes) localStorage.setItem("notesStore", JSON.stringify(d.notes));
          if (d.sidebarCollapsed !== undefined) localStorage.setItem("hittable_sidebar_collapsed", String(d.sidebarCollapsed));
        },
        { d: data, n },
      );
    });
  },
});

export { expect };

// ─── Page helpers ─────────────────────────────────────────────────────────

export const sidebar = (page: Page) => page.getByRole("complementary", { name: "Collections" });
export const response = (page: Page) => page.getByRole("region", { name: "Response" });
export const urlInput = (page: Page) => page.getByRole("textbox", { name: "Request URL" });
export const methodSelect = (page: Page) => page.getByRole("combobox", { name: "HTTP method" });
// Icon-only on the narrowest screens, where the tooltip "Send (Ctrl/Cmd+Enter)" names it.
export const sendButton = (page: Page) => page.getByRole("button", { name: /^(Send|Sending…)( \(|$)/ });
export const editor = (page: Page, tab: "params" | "body" | "headers") =>
  page.getByRole("textbox", { name: `${tab} JSON` });
export const dialog = (page: Page, name: string | RegExp) => page.getByRole("dialog", { name });

/** Open /hittable with one collection selected and its route open. */
export async function openRoute(page: Page, collectionName: string, routeName: string, folderPath: string[] = []) {
  const p = folderPath.length ? `&p=${encodeURIComponent(folderPath.join("/"))}` : "";
  await page.goto(`/hittable?c=${encodeURIComponent(collectionName)}&r=${encodeURIComponent(routeName)}${p}`);
  await expect(urlInput(page)).toBeVisible();
}

/** A fresh workspace with one empty request open, ready to be filled. */
export async function blankRequest(page: Page, seed: Fixtures["seed"], url = "") {
  await seed({ collections: [collection("Workbench", [route("scratch", url ? `curl -X GET ${url}` : "")])] });
  await openRoute(page, "Workbench", "scratch");
}

/** Replace the whole content of a request editor tab. */
export async function fillTab(page: Page, tab: "params" | "body" | "headers", value: string) {
  await page.getByRole("button", { name: tab[0].toUpperCase() + tab.slice(1), exact: true }).click();
  await editor(page, tab).fill(value);
}

export async function send(page: Page) {
  await sendButton(page).click();
}

export const statusBadge = (page: Page) => response(page).locator("[aria-live=polite]");

/** Row in the sidebar list for a collection, folder or route. */
export const sidebarItem = (page: Page, name: string): Locator =>
  sidebar(page).locator(`div[title=${JSON.stringify(name)}]`);

/** Open the ⋮ menu of a sidebar row and pick an action. */
export async function rowAction(page: Page, name: string, action: "Rename" | "Delete" | "Export") {
  const row = sidebarItem(page, name);
  await row.hover();
  await row.getByRole("button", { name: `Actions for ${name}` }).click();
  await page.getByRole("button", { name: action, exact: true }).click();
}

/** Read the text the app last put on the clipboard. */
export async function clipboardText(page: Page) {
  const copied = await page.waitForFunction(() => (window as unknown as { __copied?: string }).__copied);
  return (await copied.jsonValue()) as string;
}
