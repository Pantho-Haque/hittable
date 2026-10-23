// @vitest-environment jsdom
// The companion extension (browserExtension/) loaded as the browser would run
// it: background.js with a mocked `chrome` + `fetch`, content.js against the
// page window, and the page side (utils/hittableProxy) talking to both.
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import fs from "node:fs";
import path from "node:path";
import { fetchViaExtension, hittableProxy } from "@/utils/hittableProxy";

const read = (f: string) => fs.readFileSync(path.join(process.cwd(), "browserExtension", f), "utf8");
const BACKGROUND = read("background.js");
const CONTENT = read("content.js");
const MANIFEST = JSON.parse(read("manifest.json"));

type Listener = (msg: unknown, sender: unknown, sendResponse: (r: unknown) => void) => boolean | void;

function loadBackground(fetchImpl: (url: string, init: RequestInit) => Promise<Response>, cookies: unknown[] = []) {
  let listener: Listener | undefined;
  const chrome = {
    runtime: { onMessage: { addListener: (fn: Listener) => { listener = fn; } } },
    cookies: { getAll: vi.fn((_q: unknown, cb: (c: unknown[]) => void) => cb(cookies)) },
  };
  const fetchMock = vi.fn(fetchImpl);
  new Function("chrome", "fetch", "URL", BACKGROUND)(chrome, fetchMock, URL);
  // Resolves with what the listener returned and what it sent back, whether
  // it answered synchronously, asynchronously, or not at all.
  const send = (message: unknown) =>
    new Promise<{ keptOpen: boolean | void; reply: Record<string, unknown> | undefined }>((resolve) => {
      let answered: ((r: unknown) => void) | undefined;
      const replied = new Promise<unknown>((r) => { answered = r; });
      const keptOpen = listener!(message, {}, (r) => answered!(r));
      if (keptOpen) replied.then((r) => resolve({ keptOpen, reply: r as Record<string, unknown> }));
      else Promise.race([replied, Promise.resolve(undefined)]).then((r) => resolve({ keptOpen, reply: r as Record<string, unknown> | undefined }));
    });
  return { send, fetchMock, chrome };
}

const request = (payload: Record<string, unknown>) => ({ type: "HITTABLE_REQUEST", payload });
const jsonResponse = (body: unknown, init: ResponseInit = {}) =>
  new Response(JSON.stringify(body), { status: 200, headers: { "content-type": "application/json; charset=utf-8" }, ...init });

describe("manifest", () => {
  it("only asks for localhost and 127.0.0.1 hosts", () => {
    expect(MANIFEST.manifest_version).toBe(3);
    expect(MANIFEST.host_permissions.sort()).toEqual(["http://127.0.0.1/*", "http://localhost/*"]);
    expect(MANIFEST.content_scripts[0].js).toEqual(["content.js"]);
  });
});

describe("background worker", () => {
  it("ignores messages that are not Hittable requests", async () => {
    const { send, fetchMock } = loadBackground(async () => jsonResponse({}));
    expect(await send({ type: "OTHER" })).toEqual({ keptOpen: undefined, reply: undefined });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("refuses non-localhost URLs without fetching them", async () => {
    const { send, fetchMock } = loadBackground(async () => jsonResponse({}));
    for (const url of ["https://example.com", "http://localhost.evil.com/", "http://127.0.0.2/", "http://user@evil.com/?localhost"]) {
      expect((await send(request({ url, method: "GET" }))).reply).toEqual({ success: false, error: "The extension only makes localhost requests" });
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects invalid or missing URLs with a clear error", async () => {
    const { send } = loadBackground(async () => jsonResponse({}));
    expect((await send(request({ url: "not a url" }))).reply).toEqual({ success: false, error: "Invalid URL: not a url" });
    expect((await send({ type: "HITTABLE_REQUEST" })).reply).toMatchObject({ success: false, error: expect.stringContaining("Invalid URL") });
  });

  it("forwards method, headers and body, and keeps the channel open for the async reply", async () => {
    const { send, fetchMock } = loadBackground(async () => jsonResponse({ id: 1 }, { status: 201, statusText: "Created" }));
    const { keptOpen, reply } = await send(request({ url: "http://localhost:3000/users", method: "post", headers: { "X-Id": "7" }, body: '{"a":1}' }));
    expect(keptOpen).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith("http://localhost:3000/users", { method: "POST", headers: { "X-Id": "7" }, body: '{"a":1}' });
    expect(reply).toMatchObject({ success: true, data: { id: 1 }, status: 201, statusText: "Created", ok: true });
  });

  it("never sends a body with GET or HEAD, and defaults the method to GET", async () => {
    const { send, fetchMock } = loadBackground(async () => new Response(""));
    await send(request({ url: "http://127.0.0.1/", method: "GET", body: "x" }));
    await send(request({ url: "http://127.0.0.1/", method: "head", body: "x" }));
    await send(request({ url: "http://0.0.0.0/", body: "x" }));
    await send(request({ url: "http://localhost/", method: "DELETE", body: "x" }));
    expect(fetchMock.mock.calls.map(([, init]) => [init.method, init.body])).toEqual([["GET", undefined], ["HEAD", undefined], ["GET", undefined], ["DELETE", "x"]]);
  });

  it("returns text for non-JSON responses, flattened headers, and the origin's cookies", async () => {
    const cookies = [{ name: "sid", value: "abc" }];
    const { send, chrome } = loadBackground(async () => new Response("<h1>hi</h1>", { status: 404, headers: { "content-type": "text/html", "x-a": "1" } }), cookies);
    const { reply } = await send(request({ url: "http://localhost:8080/a/b?c=1", method: "GET" }));
    expect(reply).toMatchObject({ success: true, data: "<h1>hi</h1>", status: 404, ok: false, cookies });
    expect(reply!.headers).toMatchObject({ "content-type": "text/html", "x-a": "1" });
    expect(chrome.cookies.getAll).toHaveBeenCalledWith({ url: "http://localhost:8080" }, expect.any(Function));
  });

  it("reports network failures and broken JSON as errors instead of hanging", async () => {
    const down = loadBackground(async () => { throw new TypeError("Failed to fetch"); });
    expect((await down.send(request({ url: "http://localhost:1/" }))).reply).toEqual({ success: false, error: "Failed to fetch" });
    const broken = loadBackground(async () => new Response("{not json", { headers: { "content-type": "application/json" } }));
    expect((await broken.send(request({ url: "http://localhost/" }))).reply).toMatchObject({ success: false });
  });
});

// A window stand-in for content.js: records listeners and what it posts.
function loadContent(sendMessage: (msg: unknown, cb: (r: unknown) => void) => void, lastError?: { message: string }) {
  const listeners: ((e: MessageEvent) => void)[] = [];
  const win = {
    location: { origin: "https://hittable.vercel.app" },
    addEventListener: (_: string, fn: (e: MessageEvent) => void) => listeners.push(fn),
    postMessage: vi.fn(),
  };
  const chrome = { runtime: { sendMessage: vi.fn(sendMessage), lastError } };
  new Function("chrome", "window", CONTENT)(chrome, win);
  const deliver = (data: unknown, source: unknown = win, origin = win.location.origin) =>
    listeners.forEach((fn) => fn({ data, source, origin } as unknown as MessageEvent));
  return { win, chrome, deliver };
}

describe("content script", () => {
  it("answers a ping from the page with READY, to the page's own origin only", () => {
    const { win, deliver } = loadContent(() => {});
    deliver({ type: "HITTABLE_PING" });
    expect(win.postMessage).toHaveBeenCalledWith({ type: "HITTABLE_EXTENSION_READY" }, "https://hittable.vercel.app");
  });

  it("ignores messages from other windows or origins (iframes, other tabs)", () => {
    const { win, chrome, deliver } = loadContent(() => {});
    deliver({ type: "HITTABLE_PING" }, {});
    deliver({ type: "HITTABLE_REQUEST", id: "1", payload: {} }, {});
    deliver({ type: "HITTABLE_REQUEST", id: "1", payload: {} }, win, "https://evil.example");
    expect(win.postMessage).not.toHaveBeenCalled();
    expect(chrome.runtime.sendMessage).not.toHaveBeenCalled();
  });

  it("relays a request to the worker and echoes the request id on the reply", () => {
    const { win, chrome, deliver } = loadContent((_msg, cb) => cb({ success: true, data: 1, status: 200 }));
    const msg = { type: "HITTABLE_REQUEST", id: "42", payload: { url: "http://localhost/" } };
    deliver(msg);
    expect(chrome.runtime.sendMessage).toHaveBeenCalledWith(msg, expect.any(Function));
    expect(win.postMessage).toHaveBeenCalledWith({ type: "HITTABLE_RESPONSE", id: "42", success: true, data: 1, status: 200 }, "https://hittable.vercel.app");
  });

  it("turns a missing worker answer into an error reply instead of silence", () => {
    const { win, deliver } = loadContent((_msg, cb) => cb(undefined), { message: "Could not establish connection" });
    deliver({ type: "HITTABLE_REQUEST", id: "7", payload: {} });
    expect(win.postMessage).toHaveBeenCalledWith({ type: "HITTABLE_RESPONSE", id: "7", success: false, error: "Could not establish connection" }, "https://hittable.vercel.app");
  });

  it("ignores unrelated message types", () => {
    const { win, chrome, deliver } = loadContent(() => {});
    deliver({ type: "SOMETHING_ELSE" });
    deliver(null);
    expect(win.postMessage).not.toHaveBeenCalled();
    expect(chrome.runtime.sendMessage).not.toHaveBeenCalled();
  });
});

// jsdom's postMessage leaves `source` null and `origin` empty; browsers set
// both. Deliver page messages the way a browser does.
function browserPostMessage() {
  return vi.spyOn(window, "postMessage").mockImplementation((data: unknown) => {
    setTimeout(() => window.dispatchEvent(new MessageEvent("message", { data, source: window, origin: window.location.origin })), 0);
  });
}
const reply = (data: Record<string, unknown>, source: MessageEventSource | null = window) =>
  window.dispatchEvent(new MessageEvent("message", { data: { type: "HITTABLE_RESPONSE", ...data }, source, origin: window.location.origin }));
const sentRequests = (spy: ReturnType<typeof browserPostMessage>) =>
  spy.mock.calls.map(([d]) => d as { type: string; id: string; payload: Record<string, unknown> }).filter((d) => d.type === "HITTABLE_REQUEST");

describe("page side: fetchViaExtension", () => {
  let post: ReturnType<typeof browserPostMessage>;
  beforeEach(() => { post = browserPostMessage(); });
  afterEach(() => vi.useRealTimers());

  it("posts the request to its own origin, not '*'", async () => {
    const p = fetchViaExtension("http://localhost/", "GET", { A: "1" }, null);
    const [req] = sentRequests(post);
    expect(post).toHaveBeenCalledWith(expect.anything(), window.location.origin);
    expect(req.payload).toEqual({ url: "http://localhost/", method: "GET", headers: { A: "1" }, body: null });
    reply({ id: req.id, success: true, data: "ok", status: 200, ok: true });
    await expect(p).resolves.toMatchObject({ data: "ok", status: 200 });
  });

  it("two requests in flight get their own responses, even when answered out of order", async () => {
    const a = fetchViaExtension("http://localhost/a", "GET", {}, null);
    const b = fetchViaExtension("http://localhost/b", "GET", {}, null);
    const [ra, rb] = sentRequests(post);
    expect(ra.id).not.toBe(rb.id);
    reply({ id: rb.id, success: true, data: "B" });
    reply({ id: ra.id, success: true, data: "A" });
    expect((await a).data).toBe("A");
    expect((await b).data).toBe("B");
  });

  it("accepts a reply without an id from an older extension", async () => {
    const p = fetchViaExtension("http://localhost/", "GET", {}, null);
    reply({ success: true, data: "legacy" });
    expect((await p).data).toBe("legacy");
  });

  it("ignores replies posted by another window", async () => {
    vi.useFakeTimers();
    const p = fetchViaExtension("http://localhost/", "GET", {}, null, 1000);
    const [req] = sentRequests(post);
    reply({ id: req.id, success: true, data: "forged" }, {} as MessageEventSource);
    reply({ id: req.id, success: true, data: "forged" }, null);
    const settled = vi.fn();
    p.then(settled, settled);
    await vi.advanceTimersByTimeAsync(10);
    expect(settled).not.toHaveBeenCalled();
    reply({ id: req.id, success: true, data: "real" });
    expect((await p).data).toBe("real");
  });

  it("rejects with the extension's error, or a default one", async () => {
    const p1 = fetchViaExtension("http://localhost/", "GET", {}, null);
    reply({ id: sentRequests(post)[0].id, success: false, error: "ECONNREFUSED" });
    await expect(p1).rejects.toThrow("ECONNREFUSED");
    const p2 = fetchViaExtension("http://localhost/", "GET", {}, null);
    reply({ id: sentRequests(post)[1].id, success: false });
    await expect(p2).rejects.toThrow("Extension request failed");
  });

  it("times out with the documented message and stops listening", async () => {
    vi.useFakeTimers();
    const remove = vi.spyOn(window, "removeEventListener");
    const p = fetchViaExtension("http://localhost/", "GET", {}, null, 500);
    const assertion = expect(p).rejects.toThrow("Extension request timed out");
    await vi.advanceTimersByTimeAsync(500);
    await assertion;
    expect(remove).toHaveBeenCalledWith("message", expect.any(Function));
  });

  it("removes its listener and timer once answered", async () => {
    vi.useFakeTimers();
    const remove = vi.spyOn(window, "removeEventListener");
    const p = fetchViaExtension("http://localhost/", "GET", {}, null, 500);
    await vi.advanceTimersByTimeAsync(1);
    reply({ id: sentRequests(post)[0].id, success: true, data: 1 });
    await p;
    expect(remove).toHaveBeenCalledWith("message", expect.any(Function));
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("page side: hittableProxy routing", () => {
  const form = (url: string, over = {}) => ({ method: "POST", url, headers: '{"Authorization":"Bearer <<TOKEN>>"}', body: '{"u":"<<USER>>"}', params: "{}", ...over });
  let fetchMock: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    fetchMock = vi.fn(async () => jsonResponse({ data: { ok: 1 }, status: 200, ok: true }));
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => vi.unstubAllGlobals());

  it("sends remote URLs through /api/proxy with variables resolved, secrets winning", async () => {
    const out = await hittableProxy(form("<<BASE>>/users"), { BASE: "https://api.dev", TOKEN: "env-token", USER: "ada" }, { TOKEN: "secret-token" }, true);
    expect(out).toEqual({ data: { ok: 1 }, status: 200, ok: true });
    expect(fetchMock).toHaveBeenCalledWith("/api/proxy", expect.objectContaining({ method: "POST" }));
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      url: "https://api.dev/users", method: "POST", headers: '{"Authorization":"Bearer secret-token"}', body: '{"u":"ada"}',
    });
  });

  it("a host that merely starts with localhost still goes through the proxy", async () => {
    await hittableProxy(form("http://localhost.evil.com/"), {}, {}, false);
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("explains a proxy answer that is not JSON", async () => {
    fetchMock.mockResolvedValueOnce(new Response("<html>502</html>", { status: 502, statusText: "Bad Gateway" }));
    expect(await hittableProxy(form("https://x.dev"), {}, {}, false)).toEqual({
      data: null, status: 502, statusText: "Bad Gateway", ok: false, error: "Failed to parse proxy response",
    });
  });

  it("localhost without the extension fails with the documented message", async () => {
    for (const url of ["http://localhost:3000/a", "http://127.0.0.1/a", "http://0.0.0.0:8080/", "<<LOCAL>>/x"]) {
      await expect(hittableProxy(form(url), { LOCAL: "http://localhost:4000" }, {}, false)).rejects.toThrow(
        "Install the Hittable Extension to make localhost requests",
      );
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("localhost with the extension goes through it with parsed headers, never the proxy", async () => {
    const post = browserPostMessage();
    const p = hittableProxy(form("http://localhost:3000/a"), { TOKEN: "t", USER: "u" }, {}, true);
    const [req] = sentRequests(post);
    expect(req.payload).toEqual({ url: "http://localhost:3000/a", method: "POST", headers: { Authorization: "Bearer t" }, body: '{"u":"u"}' });
    reply({ id: req.id, success: true, data: "local" });
    expect((await p).data).toBe("local");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("invalid header JSON on a localhost request is sent as no headers", async () => {
    const post = browserPostMessage();
    const p = hittableProxy(form("http://localhost/", { headers: "{oops" }), {}, {}, true);
    const [req] = sentRequests(post);
    expect(req.payload.headers).toEqual({});
    reply({ id: req.id, success: true });
    await p;
  });
});

describe("page ↔ content script ↔ worker, end to end", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("a localhost request made in the page comes back from the worker's fetch", async () => {
    // Real browser-style messaging on the jsdom window, with content.js
    // attached to it and background.js behind chrome.runtime.
    browserPostMessage();
    const bg = loadBackground(async (url) => jsonResponse({ echoed: url }), [{ name: "sid", value: "1" }]);
    const chrome = {
      runtime: {
        sendMessage: (msg: unknown, cb: (r: unknown) => void) => { bg.send(msg).then(({ reply }) => cb(reply)); },
        lastError: undefined,
      },
    };
    new Function("chrome", "window", CONTENT)(chrome, window);

    const [a, b] = await Promise.all([
      hittableProxy({ method: "GET", url: "http://localhost:3000/a", headers: "{}", body: "", params: "{}" }, {}, {}, true),
      hittableProxy({ method: "GET", url: "http://127.0.0.1:3000/b", headers: "{}", body: "", params: "{}" }, {}, {}, true),
    ]);
    expect(a).toMatchObject({ success: true, data: { echoed: "http://localhost:3000/a" }, cookies: [{ name: "sid", value: "1" }] });
    expect(b).toMatchObject({ data: { echoed: "http://127.0.0.1:3000/b" } });
  });
});
