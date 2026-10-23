// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import RequestForm from "@/components/hittable/RequestForm";
import { mountData } from "../../helpers/dataHarness";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

let fetchMock: ReturnType<typeof vi.fn>;

function proxyReplies(body: unknown, status = 200) {
  fetchMock.mockResolvedValue({ status, statusText: "", ok: status < 400, json: async () => body });
}

async function setup(opts?: Parameters<typeof mountData>[1]) {
  const user = userEvent.setup();
  const r = mountData(<RequestForm />, opts);
  const url = await screen.findByLabelText("Request URL");
  await waitFor(() => expect(url).not.toHaveValue(""));
  return { user, url: url as HTMLTextAreaElement, ...r };
}

const sentPayload = () => JSON.parse(fetchMock.mock.calls.at(-1)![1].body);

beforeEach(() => {
  localStorage.clear();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe("URL bar", () => {
  it("loads the selected route's method and URL template", async () => {
    await setup();
    expect(screen.getByLabelText("HTTP method")).toHaveValue("GET");
    expect(screen.getByLabelText("Request URL")).toHaveValue("https://<<HOST>>/users");
  });

  it("offers GET POST PUT PATCH DELETE HEAD and switching marks the request unsaved", async () => {
    const { user, ctx } = await setup();
    const select = screen.getByLabelText("HTTP method");
    expect([...(select as HTMLSelectElement).options].map((o) => o.value)).toEqual(["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"]);
    await user.selectOptions(select, "PATCH");
    expect(ctx().formInput.method).toBe("PATCH");
    expect(screen.getByText(/Unsaved · Ctrl\/Cmd\+S/)).toBeInTheDocument();
  });

  it("typing a query string fills Params, decoded", async () => {
    const { user, url, ctx } = await setup();
    await user.clear(url);
    await user.type(url, "https://x.io/s?q=two%20words&page=2");
    expect(JSON.parse(ctx().formInput.params)).toEqual({ q: "two words", page: "2" });
  });

  it("highlights :param path segments", async () => {
    const { user, url } = await setup();
    await user.clear(url);
    await user.type(url, "https://x.io/users/:id/posts");
    const seg = screen.getByText(":id");
    expect(seg).toHaveClass("text-amber-400");
    expect(screen.getByText("/posts")).not.toHaveClass("text-amber-400");
  });

  describe("pasting a curl command", () => {
    const paste = (el: HTMLElement, text: string) =>
      fireEvent.paste(el, { clipboardData: { getData: () => text } });

    it("parses method, URL, headers and body", async () => {
      const { url, ctx } = await setup();
      paste(url, `curl -X PUT https://api.example.com/items/1 -H "Content-Type: application/json" -d '{"title":"Done"}'`);
      await waitFor(() => expect(ctx().formInput.method).toBe("PUT"), { timeout: 2000 });
      expect(ctx().formInput.url).toBe("https://api.example.com/items/1");
      expect(JSON.parse(ctx().formInput.headers)).toEqual({ "Content-Type": "application/json" });
      expect(JSON.parse(ctx().formInput.body)).toEqual({ title: "Done" });
    });

    it("{{var}} and {{ _.var }} become <<var>> and empty variables on the collection, keeping existing values", async () => {
      const { url, ctx } = await setup();
      paste(url, `curl -X POST https://api.example.com/{{version}}/login -H "Authorization: Bearer {{ _.TOKEN }}" -d '{"user":"{{user}}"}'`);
      await waitFor(() => expect(ctx().formInput.method).toBe("POST"), { timeout: 2000 });
      expect(ctx().formInput.url).toBe("https://api.example.com/<<version>>/login");
      expect(JSON.parse(ctx().formInput.headers)).toEqual({ Authorization: "Bearer <<TOKEN>>" });
      expect(JSON.parse(ctx().formInput.body)).toEqual({ user: "<<user>>" });
      const env = ctx().collections.find((c) => c.collectionName === "Users")!.env;
      expect(env).toEqual({ HOST: "api.example.com", TOKEN: "env-token", version: "", user: "" });
    });

    it("pasting plain text is an ordinary paste", async () => {
      const { url, ctx } = await setup();
      const ev = new Event("paste", { bubbles: true, cancelable: true });
      Object.assign(ev, { clipboardData: { getData: () => "https://plain.io" } });
      url.dispatchEvent(ev);
      expect(ev.defaultPrevented).toBe(false);
      expect(ctx().formInput.url).toBe("https://<<HOST>>/users");
    });
  });

  describe("sending", () => {
    it("Send resolves <<KEY>> at send time (secrets win), goes through /api/proxy and shows the response", async () => {
      proxyReplies({ data: { users: [] }, status: 200, statusText: "OK", ok: true, headers: { "content-type": "application/json" } });
      const { user, ctx } = await setup();
      await user.click(screen.getByRole("button", { name: "Send" }));
      expect(fetchMock).toHaveBeenCalledWith("/api/proxy", expect.objectContaining({ method: "POST" }));
      const payload = sentPayload();
      expect(payload.url).toBe("https://api.example.com/users");
      expect(JSON.parse(payload.headers)).toEqual({ Authorization: "Bearer secret-token" });
      expect(await screen.findByText(/200 OK/)).toBeInTheDocument();
      // the template stays in the form
      expect(ctx().formInput.url).toBe("https://<<HOST>>/users");
    });

    it("unknown keys are sent as-is so you notice them", async () => {
      proxyReplies({ status: 404, data: "nope" });
      const { user, url } = await setup();
      await user.clear(url);
      await user.type(url, "https://x.io/<<MISSING>>");
      await user.click(screen.getByRole("button", { name: "Send" }));
      expect(sentPayload().url).toBe("https://x.io/<<MISSING>>");
    });

    it("Ctrl/Cmd+Enter sends", async () => {
      proxyReplies({ status: 204, data: null });
      const { user } = await setup();
      await user.keyboard("{Control>}{Enter}{/Control}");
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    });

    it("every send is logged in history, newest first, with status, duration and size", async () => {
      proxyReplies({ status: 201, statusText: "Created", data: { id: 9 } });
      const { user, ctx } = await setup();
      await user.click(screen.getByRole("button", { name: "Send" }));
      await waitFor(() => expect(ctx().history).toHaveLength(1));
      const [entry] = ctx().history;
      expect(entry).toMatchObject({ method: "GET", url: "https://<<HOST>>/users", status: 201, statusText: "Created" });
      expect(entry.sizeBytes).toBe(JSON.stringify({ id: 9 }).length);
      expect(typeof entry.durationMs).toBe("number");
      await waitFor(() => expect(JSON.parse(localStorage.getItem("hittable_history")!)).toHaveLength(1));
    });

    it("a failed send shows the error and is still logged", async () => {
      fetchMock.mockRejectedValue(new TypeError("Failed to fetch"));
      const { user, ctx } = await setup();
      await user.click(screen.getByRole("button", { name: "Send" }));
      expect(await screen.findByText(/Failed to fetch/)).toBeInTheDocument();
      await waitFor(() => expect(ctx().history).toHaveLength(1));
      expect(ctx().history[0].status).toBeUndefined();
    });

    it("localhost without the extension explains how to fix it and never hits the proxy", async () => {
      const { user, url } = await setup();
      await user.clear(url);
      await user.type(url, "http://localhost:3000/health");
      await user.click(screen.getByRole("button", { name: "Send" }));
      expect(await screen.findByText(/Install the Hittable Extension to make localhost requests/)).toBeInTheDocument();
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it("shows Sending… and blocks a double send while in flight", async () => {
      let release!: (v: unknown) => void;
      fetchMock.mockReturnValue(new Promise((r) => (release = r)));
      const { user } = await setup();
      await user.click(screen.getByRole("button", { name: "Send" }));
      const sending = screen.getByRole("button", { name: "Sending…" });
      expect(sending).toBeDisabled();
      await act(async () => release({ status: 200, ok: true, json: async () => ({ status: 200 }) }));
      expect(await screen.findByRole("button", { name: "Send" })).toBeEnabled();
    });

    it("a JSON error disables Send, and Ctrl/Cmd+Enter explains why instead of sending", async () => {
      const { user } = await setup();
      await user.click(screen.getByRole("button", { name: "Body" }));
      const body = screen.getByLabelText("body JSON");
      await user.clear(body);
      await user.type(body, "{{not json");
      expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
      await user.keyboard("{Control>}{Enter}{/Control}");
      expect(await screen.findByText("Cannot send request")).toBeInTheDocument();
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  describe("saving", () => {
    it("Save is disabled until something changes; Ctrl/Cmd+S saves and clears the Unsaved pill", async () => {
      const { user, url, ctx } = await setup();
      const save = screen.getByRole("button", { name: "Save" });
      expect(save).toBeDisabled();
      await user.type(url, "/42");
      expect(save).toBeEnabled();
      await user.keyboard("{Control>}s{/Control}");
      await waitFor(() => expect(screen.queryByText(/Unsaved/)).not.toBeInTheDocument());
      const route = ctx().collections[0].items[0];
      expect(route.type === "route" && route.curl).toContain("https://<<HOST>>/users/42");
    });

    it("Discard changes restores the saved request", async () => {
      const { user, url } = await setup();
      await user.type(url, "/42");
      await user.click(screen.getByRole("button", { name: "Discard changes" }));
      expect(url).toHaveValue("https://<<HOST>>/users");
      expect(screen.queryByText(/Unsaved/)).not.toBeInTheDocument();
    });
  });

  it("Copy as CURL copies a curl command for the current request", async () => {
    const { user } = await setup();
    await user.click(screen.getByRole("button", { name: "Copy as CURL" }));
    // user-event provides the clipboard
    expect(await navigator.clipboard.readText()).toBe(`curl -X GET 'https://<<HOST>>/users' -H 'Authorization: Bearer <<TOKEN>>'`);
  });
});
