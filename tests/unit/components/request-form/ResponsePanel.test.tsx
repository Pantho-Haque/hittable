// @vitest-environment jsdom
import { EXTENSION_URL } from "@/config";
import { describe, it, expect, beforeEach, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import HtmlPreview from "@/components/RequestForm/ResponsePanelComponents/HtmlPreview";
import { mountRunner } from "../../helpers/runnerHarness";
import type { TResponseJson } from "@/types";

function setup(response?: TResponseJson) {
  const user = userEvent.setup();
  const r = mountRunner({ response });
  return { user, ...r, respond: (next: TResponseJson) => act(() => r.respond(next)) };
}

const json = (data: unknown, extra: Partial<NonNullable<TResponseJson>> = {}): TResponseJson => ({
  status: 200, statusText: "OK", ok: true, data, headers: { "content-type": "application/json; charset=utf-8" }, ...extra,
});
const status = () => screen.getByText((_, el) => el?.getAttribute("aria-live") === "polite");
// The whole Runner is mounted, so tab queries are scoped to the panel: the
// request editor has a Headers tab of its own.
const panel = () => screen.getByRole("region", { name: "Response" });

beforeEach(() => {
  Element.prototype.scrollIntoView = () => {}; // not in jsdom
});

describe("Response panel", () => {
  it("asks you to send a request before there is a response", () => {
    setup();
    expect(screen.getByText("Send a request to see the response")).toBeInTheDocument();
  });

  // FR-7: 3xx is no longer lumped in with 4xx as a warning, and 1xx is not an
  // error. The class is exposed as data-status-kind and drawn from the status
  // tokens (--success/--info/--warning/--danger).
  it.each([
    [200, "success"], [204, "success"],
    [301, "info"], [100, "info"],
    [404, "warning"], [499, "warning"],
    [500, "danger"], [503, "danger"],
  ])("status %i is shown as %s", (code, kind) => {
    setup(json({}, { status: code, statusText: "X" }));
    expect(status()).toHaveTextContent(`${code} X`);
    expect(status()).toHaveAttribute("data-status-kind", kind);
    expect(status().className).toContain(`text-(--${kind})`);
  });

  it("fills in the reason phrase when the server sent none (HTTP/2)", () => {
    setup(json({}, { status: 201, statusText: "" }));
    expect(status()).toHaveTextContent("201 Created");
  });

  it("shows duration and size, switching units for large values", () => {
    const { respond } = setup(json({}, { durationMs: 182, sizeBytes: 509 }));
    expect(screen.getByText("182ms")).toBeInTheDocument();
    expect(screen.getByText("509 B")).toBeInTheDocument();
    respond(json({}, { durationMs: 1500, sizeBytes: 1536 }));
    expect(screen.getByText("1.50s")).toBeInTheDocument();
    expect(screen.getByText("1.5 KB")).toBeInTheDocument();
    respond(json({}, { sizeBytes: 3 * 1024 * 1024 }));
    expect(screen.getByText("3.0 MB")).toBeInTheDocument();
  });

  it("a zero size is not shown", () => {
    setup(json(null, { sizeBytes: 0, durationMs: 5 }));
    expect(screen.queryByText("0 B")).not.toBeInTheDocument();
  });

  it("an error response shows the message instead of tabs", () => {
    setup({ error: "Error: Failed to connect to target URL" });
    expect(screen.getByRole("alert")).toHaveTextContent("Failed to connect to target URL");
    expect(within(panel()).queryByRole("tab", { name: /headers/i })).not.toBeInTheDocument();
  });

  // FR-10: the missing-extension case says what to do, with a primary
  // Install button, instead of a raw error string.
  it("a localhost request without the extension explains it and links the install", () => {
    setup({ error: "Error: Install the Hittable Extension to make localhost requests" });
    expect(screen.getByRole("alert")).toHaveTextContent("localhost needs the Hittable browser extension");
    expect(screen.getByRole("link", { name: "Install the extension" })).toHaveAttribute("href", EXTENSION_URL);
  });

  it("a cancelled request is a calm status, not an error", () => {
    setup({ error: "Request cancelled" });
    expect(screen.getByRole("status")).toHaveTextContent("Request cancelled.");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  describe("headers tab", () => {
    const withHeaders = () =>
      setup(json({ ok: 1 }, { headers: { "content-type": "application/json", "x-request-id": "abc123", "cache-control": "no-store" } }));

    it("shows the header count and every header", async () => {
      const { user } = withHeaders();
      const headersTab = within(panel()).getByRole("tab", { name: /headers/i });
      expect(headersTab).toHaveTextContent("(3)");
      await user.click(headersTab);
      expect(screen.getByText("x-request-id")).toBeInTheDocument();
      expect(screen.getByText("abc123")).toBeInTheDocument();
    });

    it("copies one header or all of them", async () => {
      const { user } = withHeaders();
      await user.click(within(panel()).getByRole("tab", { name: /headers/i }));
      await user.click(screen.getByTitle("Copy x-request-id"));
      expect(await navigator.clipboard.readText()).toBe("x-request-id: abc123");
      await user.click(screen.getByTitle("Copy all headers"));
      expect(JSON.parse(await navigator.clipboard.readText())).toEqual({
        "content-type": "application/json", "x-request-id": "abc123", "cache-control": "no-store",
      });
    });

    it("search filters header rows by name or value", async () => {
      const { user } = withHeaders();
      await user.click(within(panel()).getByRole("tab", { name: /headers/i }));
      await user.click(screen.getByTitle("Search (⌘F)"));
      await user.type(screen.getByPlaceholderText("Search keys & values…"), "STORE");
      expect(screen.getByText("cache-control")).toBeInTheDocument();
      expect(screen.queryByText("x-request-id")).not.toBeInTheDocument();
      expect(screen.getByText("1/1")).toBeInTheDocument();
      await user.clear(screen.getByPlaceholderText("Search keys & values…"));
      await user.type(screen.getByPlaceholderText("Search keys & values…"), "zzz");
      expect(screen.getByText(/No headers matching "zzz"/)).toBeInTheDocument();
      expect(screen.getByText("0/0")).toBeInTheDocument();
    });

    it("a response without headers says so", async () => {
      const { user } = setup({ status: 200, data: "x" });
      await user.click(within(panel()).getByRole("tab", { name: /headers/i }));
      expect(screen.getByText("No response headers")).toBeInTheDocument();
    });
  });

  describe("body view", () => {
    it("JSON content type opens the JSON tree", () => {
      setup(json({ user: { name: "Ada" } }));
      expect(screen.getByText('"user"')).toBeInTheDocument();
      expect(screen.getByText('"Ada"')).toBeInTheDocument();
    });

    it("text content shows raw text with whitespace preserved", () => {
      setup({ status: 200, data: "line 1\n  line 2", headers: { "content-type": "text/plain" } });
      const pre = screen.getByText((_, el) => el?.tagName === "PRE");
      expect(pre.textContent).toBe("line 1\n  line 2");
      expect(pre).toHaveClass("whitespace-pre-wrap");
    });

    it("HTML content opens the sandboxed preview with a source pane", () => {
      setup({ status: 200, data: "<html><body><h1>Hi</h1></body></html>", headers: { "content-type": "text/html" } });
      expect(screen.getByTitle("HTML response preview")).toBeInTheDocument();
      expect(screen.getByText("Source")).toBeInTheDocument();
    });

    it("a manual view choice sticks until the next response arrives", async () => {
      const { user, respond } = setup(json({ a: 1 }));
      await user.click(screen.getByTitle("Raw text view"));
      expect(screen.getByText((_, el) => el?.tagName === "PRE")).toHaveTextContent('"a": 1');
      respond(json({ b: 2 }));
      await waitFor(() => expect(screen.getByText('"b"')).toBeInTheDocument());
      expect(screen.queryByText((_, el) => el?.tagName === "PRE")).not.toBeInTheDocument();
    });

    it("JSON in a string body is parsed for the tree", async () => {
      const { user } = setup({ status: 200, data: '{"id":7}', headers: { "content-type": "text/plain" } });
      await user.click(screen.getByTitle("JSON tree view"));
      expect(screen.getByText('"id"')).toBeInTheDocument();
      expect(screen.getByText("7")).toBeInTheDocument();
    });

    it("Copy copies the body as formatted JSON", async () => {
      const { user } = setup(json({ id: 1 }));
      await user.click(screen.getByTitle("Copy JSON"));
      expect(await navigator.clipboard.readText()).toBe('{\n  "id": 1\n}');
      expect(screen.getByText("Copied")).toBeInTheDocument();
    });
  });

  describe("JSON tree", () => {
    const deep = { a: { b: { c: 1, d: 2 } }, list: [1, 2, 3] };

    it("opens two levels deep; deeper nodes show `N keys` / `N items`", async () => {
      const { user } = setup(json({ ...deep, a: { b: { c: 1, d: 2 }, arr: [7, 8] } }));
      expect(screen.getByText('"b"')).toBeInTheDocument();
      expect(screen.queryByText('"c"')).not.toBeInTheDocument();
      expect(screen.getByText("2 keys")).toBeInTheDocument();
      expect(screen.getByText("2 items")).toBeInTheDocument();
      // list is at depth 1, open
      expect(screen.getByText("3")).toBeInTheDocument();

      await user.click(screen.getByText('"b"').previousElementSibling as HTMLElement);
      expect(screen.getByText('"c"')).toBeInTheDocument();
      expect(screen.queryByText("2 keys")).not.toBeInTheDocument();
    });

    it("collapsing a node hides its children and shows its size", async () => {
      const { user } = setup(json(deep));
      await user.click(screen.getByText('"list"').previousElementSibling as HTMLElement);
      expect(screen.getByText("3 items")).toBeInTheDocument();
      expect(screen.queryByText("3")).not.toBeInTheDocument();
    });

    it("strings are quoted, null and booleans shown as literals", () => {
      setup(json({ s: "x", n: null, t: true, z: 0 }));
      expect(screen.getByText('"x"')).toBeInTheDocument();
      expect(screen.getByText("null")).toBeInTheDocument();
      expect(screen.getByText("true")).toBeInTheDocument();
      expect(screen.getByText("0")).toBeInTheDocument();
    });
  });

  describe("search", () => {
    const data = { name: "Ada", friends: [{ name: "Bob" }], note: "named after Ada" };

    it("Ctrl/Cmd+F opens search, counts matches across keys and values, Esc closes and clears", async () => {
      const { user } = setup(json(data));
      await user.keyboard("{Control>}f{/Control}");
      const input = screen.getByPlaceholderText("Search keys & values…");
      expect(input).toHaveFocus();
      await user.type(input, "name");
      // keys: name, name; values: "named after Ada"
      expect(screen.getByText("1/3")).toBeInTheDocument();
      expect(panel().querySelectorAll("mark")).toHaveLength(3);
      await user.keyboard("{Escape}");
      expect(screen.queryByPlaceholderText("Search keys & values…")).not.toBeInTheDocument();
      expect(panel().querySelectorAll("mark")).toHaveLength(0);
    });

    it("Enter and Shift+Enter step through matches, wrapping around", async () => {
      const { user } = setup(json(data));
      await user.click(screen.getByTitle("Search (⌘F)"));
      await user.type(screen.getByPlaceholderText("Search keys & values…"), "ada");
      expect(screen.getByText("1/2")).toBeInTheDocument();
      await user.keyboard("{Enter}");
      await screen.findByText("2/2");
      await user.keyboard("{Enter}");
      await screen.findByText("1/2");
      await user.keyboard("{Shift>}{Enter}{/Shift}");
      await screen.findByText("2/2");
      await user.click(screen.getByTitle("Previous (Shift+Enter)"));
      await screen.findByText("1/2");
      await user.click(screen.getByTitle("Next (Enter)"));
      await screen.findByText("2/2");
    });

    it("no match shows 0/0 with the arrows disabled", async () => {
      const { user } = setup(json(data));
      await user.click(screen.getByTitle("Search (⌘F)"));
      await user.type(screen.getByPlaceholderText("Search keys & values…"), "zzz");
      expect(screen.getByText("0/0")).toBeInTheDocument();
      expect(screen.getByTitle("Next (Enter)")).toBeDisabled();
    });

    it("collapsed nodes open on their own to reveal matches", async () => {
      const { user } = setup(json({ a: { b: { secret: 1 } } }));
      expect(screen.queryByText('"secret"')).not.toBeInTheDocument();
      await user.click(screen.getByTitle("Search (⌘F)"));
      await user.type(screen.getByPlaceholderText("Search keys & values…"), "secr");
      const key = await screen.findByText((_, el) => el?.tagName === "SPAN" && el.textContent === '"secret"');
      expect(within(key).getByText("secr").tagName).toBe("MARK");
    });

    it("raw text view highlights matches too", async () => {
      const { user } = setup(json({ word: "hello" }));
      await user.click(screen.getByTitle("Raw text view"));
      await user.click(screen.getByTitle("Search (⌘F)"));
      await user.type(screen.getByPlaceholderText("Search keys & values…"), "hello");
      expect(panel().querySelector("pre mark")).toHaveTextContent("hello");
    });
  });
});

describe("HTML preview", () => {
  const iframe = () => screen.getByTitle("HTML response preview") as HTMLIFrameElement;
  const srcdoc = () => iframe().getAttribute("srcdoc")!;

  it("runs page scripts only in an opaque-origin sandbox: never allow-same-origin, no dialogs, no top navigation", () => {
    render(<HtmlPreview html="<script>alert(1)</script>" />);
    const flags = iframe().getAttribute("sandbox")!.split(/\s+/);
    expect(flags).toContain("allow-scripts");
    for (const unsafe of ["allow-same-origin", "allow-modals", "allow-top-navigation", "allow-forms"]) expect(flags).not.toContain(unsafe);
  });

  it("injects <base href> at the page's URL, opening links in a new tab", () => {
    render(<HtmlPreview html="<html><head><title>t</title></head><body></body></html>" baseUrl="https://www.site.io/docs/page.html" />);
    expect(srcdoc()).toMatch(/^<html><head><base href="https:\/\/www.site.io\/docs\/page.html" target="_blank"><style>:root\{color-scheme:light\}<\/style><script nonce=/);
  });

  it("does not mistake <header> for <head>, and adds a head when there is none", () => {
    const { unmount } = render(<HtmlPreview html="<html><body><header>x</header></body></html>" baseUrl="https://site.io/a" />);
    expect(srcdoc()).toMatch(/^<html><head><base href="https:\/\/site.io\/a" target="_blank"><style>[^<]*<\/style><script[^>]*>[\s\S]*<\/script><\/head><body><header>x<\/header>/);
    unmount();
    render(<HtmlPreview html="<p>x</p>" baseUrl="https://site.io/a/b" />);
    expect(srcdoc()).toMatch(/^<base href="https:\/\/site.io\/a\/b" target="_blank"><style>[^<]*<\/style><script[\s\S]*<\/script><p>x<\/p>$/);
  });

  it("a quote or `$` in the URL cannot break out of the attribute", () => {
    render(<HtmlPreview html="<html><head></head></html>" baseUrl={'https://site.io/a$&b/"><script>x()</script>'} />);
    expect(srcdoc()).toContain('<base href="https://site.io/a$&b/%22%3E%3Cscript%3Ex()%3C/script%3E" target="_blank">');
  });

  it("without a usable URL there is no base tag", () => {
    const { unmount } = render(<HtmlPreview html="<p>x</p>" />);
    expect(srcdoc()).not.toContain("<base");
    unmount();
    render(<HtmlPreview html="<p>x</p>" baseUrl="not a url" />);
    expect(srcdoc()).not.toContain("<base");
  });

  it("scripts off adds a CSP that only lets the inspector run", () => {
    const { unmount } = render(<HtmlPreview html="<html><head></head></html>" scripts={false} />);
    const nonce = srcdoc().match(/<script nonce="(\w+)">/)![1];
    expect(srcdoc()).toContain(`<meta http-equiv="Content-Security-Policy" content="script-src 'nonce-${nonce}'">`);
    unmount();
    render(<HtmlPreview html="<html><head></head></html>" />);
    expect(srcdoc()).not.toContain("Content-Security-Policy");
  });

  it("a pick posted by the frame is reported; messages from anywhere else are ignored", () => {
    const picked = vi.fn();
    render(<HtmlPreview html="<p>x</p>" inspectMode onElementSelect={picked} />);
    const pick = { type: "hittable:picked", tagName: "p", attributes: { id: "a", n: 1 }, outerHTML: "<p id=a>x</p>" };
    window.dispatchEvent(new MessageEvent("message", { data: pick, source: window }));
    expect(picked).not.toHaveBeenCalled();
    window.dispatchEvent(new MessageEvent("message", { data: pick, source: iframe().contentWindow }));
    expect(picked).toHaveBeenCalledWith({ tagName: "p", attributes: { id: "a" }, outerHTML: "<p id=a>x</p>" });
  });

  it("the panel prefers the final URL the proxy reports over the request URL", () => {
    mountRunner({
      request: { method: "GET", url: "https://site.io/", headers: "{}", body: "", params: "{}" },
      response: { status: 200, url: "https://www.site.io/home", data: "<html><head></head></html>", headers: { "content-type": "text/html" } },
    });
    expect(srcdoc()).toContain('<base href="https://www.site.io/home" target="_blank">');
  });

  it("the panel resolves <<KEY>> from env in the request URL for the base href", () => {
    mountRunner({
      request: { method: "GET", url: "https://<<HOST>>/docs/index.html", headers: "{}", body: "", params: "{}" },
      env: { HOST: "env.io" },
      response: { status: 200, data: "<html><head></head></html>", headers: { "content-type": "text/html" } },
    });
    expect(iframe().getAttribute("srcdoc")).toContain('<base href="https://env.io/docs/index.html" target="_blank">');
  });
});

