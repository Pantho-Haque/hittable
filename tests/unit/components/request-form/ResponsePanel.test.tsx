// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ResponsePanel from "@/components/RequestForm/ResponsePanel";
import HtmlPreview from "@/components/RequestForm/ResponsePanelComponents/HtmlPreview";
import { mountData } from "../../helpers/dataHarness";
import type { TResponseJson } from "@/types";
import type { DataSource } from "@/types/workspace";

function setup(res?: TResponseJson) {
  const user = userEvent.setup();
  const r = mountData(<ResponsePanel />);
  if (res !== undefined) act(() => r.ctx().setProxyResponse(res));
  return { user, ...r, respond: (next: TResponseJson) => act(() => r.ctx().setProxyResponse(next)) };
}

const json = (data: unknown, extra: Partial<NonNullable<TResponseJson>> = {}): TResponseJson => ({
  status: 200, statusText: "OK", ok: true, data, headers: { "content-type": "application/json; charset=utf-8" }, ...extra,
});
const status = () => screen.getByText((_, el) => el?.getAttribute("aria-live") === "polite");
const panel = () => screen.getByRole("region", { name: "Response" });

beforeEach(() => {
  localStorage.clear();
  Element.prototype.scrollIntoView = () => {}; // not in jsdom
});

describe("Response panel", () => {
  it("asks you to send a request before there is a response", () => {
    setup();
    expect(screen.getByText("Send a request to see the response")).toBeInTheDocument();
  });

  it.each([
    [200, "#4ade80"], [204, "#4ade80"],
    [301, "#fb923c"], [404, "#fb923c"], [499, "#fb923c"],
    [500, "#f87171"], [100, "#f87171"],
  ])("status %i is coloured %s", (code, colour) => {
    setup(json({}, { status: code, statusText: "X" }));
    expect(status()).toHaveTextContent(`${code} X`);
    expect(status().style.color).toBe(hexToRgb(colour));
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
    setup({ error: "Error: Install the Hittable Extension to make localhost requests" });
    expect(screen.getByText(/Install the Hittable Extension/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /headers/i })).not.toBeInTheDocument();
  });

  describe("headers tab", () => {
    const withHeaders = () =>
      setup(json({ ok: 1 }, { headers: { "content-type": "application/json", "x-request-id": "abc123", "cache-control": "no-store" } }));

    it("shows the header count and every header", async () => {
      const { user } = withHeaders();
      const headersTab = screen.getByRole("button", { name: /headers/i });
      expect(headersTab).toHaveTextContent("(3)");
      await user.click(headersTab);
      expect(screen.getByText("x-request-id")).toBeInTheDocument();
      expect(screen.getByText("abc123")).toBeInTheDocument();
    });

    it("copies one header or all of them", async () => {
      const { user } = withHeaders();
      await user.click(screen.getByRole("button", { name: /headers/i }));
      await user.click(screen.getByTitle("Copy x-request-id"));
      expect(await navigator.clipboard.readText()).toBe("x-request-id: abc123");
      await user.click(screen.getByTitle("Copy all headers"));
      expect(JSON.parse(await navigator.clipboard.readText())).toEqual({
        "content-type": "application/json", "x-request-id": "abc123", "cache-control": "no-store",
      });
    });

    it("search filters header rows by name or value", async () => {
      const { user } = withHeaders();
      await user.click(screen.getByRole("button", { name: /headers/i }));
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
      await user.click(screen.getByRole("button", { name: /headers/i }));
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

  it("is sandboxed so the page's scripts never run", () => {
    render(<HtmlPreview html="<script>alert(1)</script>" />);
    const sandbox = iframe().getAttribute("sandbox")!;
    expect(sandbox).not.toBe(null);
    expect(sandbox.split(/\s+/)).not.toContain("allow-scripts");
  });

  it("injects <base href> pointing at the request's directory", () => {
    render(<HtmlPreview html="<html><head><title>t</title></head><body></body></html>" requestUrl="https://site.io/docs/page.html" />);
    expect(iframe().getAttribute("srcdoc")).toContain('<head>\n<base href="https://site.io/docs/">');
  });

  it("adds a head when the page has only <html>, or prepends the tag for a fragment", () => {
    const { unmount } = render(<HtmlPreview html="<html><body>x</body></html>" requestUrl="https://site.io/a" />);
    expect(iframe().getAttribute("srcdoc")).toContain('<html>\n<head><base href="https://site.io/"></head>');
    unmount();
    render(<HtmlPreview html="<p>x</p>" requestUrl="https://site.io/a/b" />);
    expect(iframe().getAttribute("srcdoc")).toBe('<base href="https://site.io/a/">\n<p>x</p>');
  });

  it("a `$` in the URL path is inserted literally", () => {
    render(<HtmlPreview html="<html><head></head></html>" requestUrl="https://site.io/a$&b/page" />);
    expect(iframe().getAttribute("srcdoc")).toContain('<head>\n<base href="https://site.io/a$&b/">');
  });

  it("without a request URL the HTML is used as-is", () => {
    render(<HtmlPreview html="<p>x</p>" />);
    expect(iframe().getAttribute("srcdoc")).toBe("<p>x</p>");
  });

  it("the panel resolves <<KEY>> in the request URL for the base href", () => {
    const { ctx } = mountData(<ResponsePanel />);
    act(() => {
      ctx().setSelectorResponse({
        collectionName: "Users", folderPath: [], curlName: "Page",
        env: { HOST: "env.io" }, secrets: { HOST: "secret.io" },
        curlJson: { method: "GET", url: "https://<<HOST>>/docs/index.html", headers: "{}", body: "", params: "{}" },
      });
      // The URL in the bar is what was sent, so it is the one the preview uses.
      ctx().setFormInput((f) => ({ ...f, url: "https://<<HOST>>/docs/index.html" }));
      ctx().setProxyResponse({ status: 200, data: "<html><head></head></html>", headers: { "content-type": "text/html" } });
    });
    expect(iframe().getAttribute("srcdoc")).toContain('<base href="https://secret.io/docs/">');
  });

  it("in Directory mode the base href comes from the open file, not the Local-mode route", () => {
    const { ctx } = mountData(<span />);
    const fileSource = {
      ...ctx(),
      proxyResponse: { status: 200, data: "<p>x</p>", headers: { "content-type": "text/html" } },
      selectorResponse: {
        collectionName: "", folderPath: [], curlName: "page.hit", env: {}, secrets: {},
        curlJson: { method: "GET", url: "https://file.io/x/y", headers: "{}", body: "", params: "{}" },
      },
      formInput: { method: "GET", url: "https://file.io/x/y", headers: "{}", body: "", params: "{}" },
    } as unknown as DataSource;
    mountData(<ResponsePanel dataSource={fileSource} />);
    expect(iframe().getAttribute("srcdoc")).toBe('<base href="https://file.io/x/">\n<p>x</p>');
  });
});

function hexToRgb(hex: string) {
  const n = parseInt(hex.slice(1), 16);
  return `rgb(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255})`;
}
