import { test as base, expect, type Page } from "@playwright/test";
import { seedPrivateFolder, openWorkspace, openFile } from "./directory";
export { openLauncher } from "./directory";

// The fixture API (tests/e2e/fixtures/api-server.mjs) listens dual-stack.
// `[::1]` is not one of the hostnames the app hands to the extension, so
// requests to it go through the server-side /api/proxy like any remote URL.
// `127.0.0.1` is routed to the extension instead.
const API_PORT = 4100;
export const API = `http://[::1]:${API_PORT}`;
export const LOCAL_API = `http://127.0.0.1:${API_PORT}`;

/** Raw localStorage entries, e.g. the old Local mode's `hittable` key. */
type Seed = Record<string, string>;

type Fixtures = {
  /** Answer the app's extension handshake like the real extension does. Default true. */
  extension: boolean;
  /** Uncaught page errors collected during the test; the test fails if any remain. */
  pageErrors: Error[];
  /** Set localStorage keys once, before the app's first load (reloads keep what the app wrote). */
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
      // WebKit reports a fetch that a navigation cancelled (a Next.js prefetch,
      // the top bar's session check) as an uncaught "… due to access control
      // checks", even when the app catches it; nothing breaks.
      const real = errors.filter((e) => !/(\?_rsc=\S+|\/api\/github\/session) due to access control checks/.test(e.message));
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
          for (const [k, v] of Object.entries(d)) localStorage.setItem(k, v);
        },
        { d: data, n },
      );
    });
  },
});

export { expect };

// ─── Page helpers ─────────────────────────────────────────────────────────

export const response = (page: Page) => page.getByRole("region", { name: "Response" });
export const urlInput = (page: Page) => page.getByRole("textbox", { name: "Request URL" });
export const methodSelect = (page: Page) => page.getByRole("combobox", { name: "HTTP method" });
// Icon-only on the narrowest screens, where the tooltip "Send (Ctrl/Cmd+Enter)" names it.
export const sendButton = (page: Page) => page.getByRole("button", { name: /^(Send|Sending…)( \(|$)/ });
export const editor = (page: Page, tab: "params" | "body" | "headers") =>
  page.getByRole("textbox", { name: `${tab} JSON` });
export const dialog = (page: Page, name: string | RegExp) => page.getByRole("dialog", { name });

export const FOLDER_ONLY = "Folder mode needs the File System Access API (Chromium only)";

type RequestFile = {
  url?: string;
  method?: string;
  headers?: Record<string, string>;
  params?: Record<string, string>;
  body?: string;
  /** hittable/env.json */
  env?: Record<string, string>;
  /** hittable/env.local.json, left out when not given */
  envLocal?: Record<string, string>;
  /** Any other files to put in the folder. */
  files?: Record<string, string>;
};

/**
 * Open a local folder holding hittable/req.hit (and env.json) and show that
 * request in the Runner. Folder mode is Chromium only: skip other engines.
 */
export async function openRequest(page: Page, r: RequestFile = {}) {
  const { url = "", method = "GET", headers = {}, params = {}, body = "" } = r;
  await seedPrivateFolder(page, {
    "hittable/env.json": JSON.stringify(r.env ?? {}),
    ...(r.envLocal ? { "hittable/env.local.json": JSON.stringify(r.envLocal) } : {}),
    "hittable/req.hit": JSON.stringify({ method, url, headers, params, body, response: null }, null, 2),
    ...r.files,
  });
  await openWorkspace(page);
  await openFile(page, "hittable/req.hit");
  await expect(page.getByRole("button", { name: "Runner", exact: true })).toHaveAttribute("aria-pressed", "true");
  await expect(urlInput(page)).toHaveValue(url);
}

/** Replace the whole content of a request editor tab. */
export async function fillTab(page: Page, tab: "params" | "body" | "headers", value: string) {
  await page.getByRole("tab", { name: tab[0].toUpperCase() + tab.slice(1), exact: true }).click();
  await editor(page, tab).fill(value);
}

export async function send(page: Page) {
  await sendButton(page).click();
}

export const statusBadge = (page: Page) => response(page).locator("[aria-live=polite]");

/** Read the text the app last put on the clipboard. */
export async function clipboardText(page: Page) {
  const copied = await page.waitForFunction(() => (window as unknown as { __copied?: string }).__copied);
  return (await copied.jsonValue()) as string;
}
