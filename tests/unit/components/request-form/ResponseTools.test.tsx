// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ResponsePanel from "@/components/RequestForm/ResponsePanel";
import { mountData } from "../../helpers/dataHarness";
import type { TResponseJson } from "@/types";

// FR-13 (JSONPath filter) and FR-14 (compare with the previous response).

/** user.type reads `[` and `{` as key descriptors; double them to type them. */
const typed = (s: string) => s.replace(/[[{]/g, "$&$&");

function setup(res?: TResponseJson) {
  const user = userEvent.setup();
  const r = mountData(<ResponsePanel />);
  if (res !== undefined) act(() => r.ctx().setProxyResponse(res));
  return { user, ...r, respond: (next: TResponseJson) => act(() => r.ctx().setProxyResponse(next)) };
}

const json = (data: unknown, extra: Partial<NonNullable<TResponseJson>> = {}): TResponseJson => ({
  status: 200, statusText: "OK", ok: true, data, headers: { "content-type": "application/json" }, ...extra,
});

const filterButton = () => screen.getByRole("button", { name: /Filter with JSONPath/ });
const filterInput = () => screen.getByRole("textbox", { name: "JSONPath filter" });
const compareButton = () => screen.getByRole("button", { name: "Compare with previous response" });
const compareView = () => screen.getByRole("region", { name: "Comparison with previous response" });
const changedPaths = () =>
  within(screen.getByRole("list", { name: "Body changes" }))
    .getAllByRole("listitem")
    .map((li) => li.querySelector("code")!.textContent);

beforeEach(() => {
  localStorage.clear();
  Element.prototype.scrollIntoView = () => {};
});

describe("JSONPath filter", () => {
  const users = { users: [{ name: "a", email: "a@x" }, { name: "b", email: "b@x" }], note: "hello" };

  it("shows the matches of a valid path, and the button shows a dot", async () => {
    const { user } = setup(json(users));
    expect(screen.queryByTestId("filter-dot")).not.toBeInTheDocument();
    await user.click(filterButton());
    expect(filterButton()).toHaveAttribute("aria-pressed", "true");
    await user.type(filterInput(), typed("$.users[*].name"));
    expect(screen.getByText('"a"')).toBeInTheDocument();
    expect(screen.getByText('"b"')).toBeInTheDocument();
    expect(screen.queryByText('"a@x"')).not.toBeInTheDocument();
    expect(screen.queryByText('"hello"')).not.toBeInTheDocument();
    expect(screen.getByText("2 matches")).toBeInTheDocument();
    expect(screen.getByTestId("filter-dot")).toBeInTheDocument();
    expect(filterButton()).toHaveAccessibleName("Filter with JSONPath (active)");
  });

  it("a single match is shown as itself; recursive descent finds nested ids", async () => {
    const { user } = setup(json({ a: { id: 7, b: { id: 8 } } }));
    await user.click(filterButton());
    await user.type(filterInput(), "$.a.b");
    expect(screen.getByText("1 match")).toBeInTheDocument();
    expect(screen.getByText("8")).toBeInTheDocument();
    await user.clear(filterInput());
    await user.type(filterInput(), "$..id");
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("8")).toBeInTheDocument();
  });

  it("an invalid path says so and keeps the whole body", async () => {
    const { user } = setup(json(users));
    await user.click(filterButton());
    await user.type(filterInput(), typed("$["));
    expect(screen.getByText(/^Invalid JSONPath: /)).toBeInTheDocument();
    expect(filterInput()).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText('"hello"')).toBeInTheDocument();
    expect(screen.queryByTestId("filter-dot")).not.toBeInTheDocument();
  });

  it("a valid path with no matches says no results", async () => {
    const { user } = setup(json(users));
    await user.click(filterButton());
    await user.type(filterInput(), "$.nope");
    expect(screen.getByText("No results for $.nope")).toBeInTheDocument();
    expect(screen.queryByText('"hello"')).not.toBeInTheDocument();
  });

  it("a new response clears the filter", async () => {
    const { user, respond } = setup(json(users));
    await user.click(filterButton());
    await user.type(filterInput(), "$.users");
    expect(screen.queryByText('"hello"')).not.toBeInTheDocument();
    respond(json(users));
    expect(filterInput()).toHaveValue("");
    expect(screen.getByText('"hello"')).toBeInTheDocument();
  });

  it("Clear empties and closes it; it coexists with search", async () => {
    const { user } = setup(json(users));
    await user.click(screen.getByTitle("Search (⌘F)"));
    await user.click(filterButton());
    await user.type(filterInput(), typed("$.users[0]"));
    await user.type(screen.getByPlaceholderText("Search keys & values…"), "a@x");
    expect(screen.getByText("1/1")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Clear filter" }));
    expect(screen.queryByRole("textbox", { name: "JSONPath filter" })).not.toBeInTheDocument();
    expect(screen.getByText('"hello"')).toBeInTheDocument();
  });

  it("is not offered for a text body", () => {
    setup({ status: 200, data: "plain", headers: { "content-type": "text/plain" } });
    expect(screen.queryByRole("button", { name: /Filter with JSONPath/ })).not.toBeInTheDocument();
  });
});

describe("Compare with the previous response", () => {
  it("is disabled, and says why, with nothing earlier", async () => {
    const { user } = setup(json({ a: 1 }));
    expect(compareButton()).toHaveAttribute("aria-disabled", "true");
    expect(compareButton()).toHaveAttribute("title", "No earlier response for this request yet");
    await user.click(compareButton());
    expect(screen.queryByRole("region", { name: "Comparison with previous response" })).not.toBeInTheDocument();
  });

  it("after a second response lists exactly the changed, added and removed paths", async () => {
    const { user, respond } = setup(json({ id: 1, name: "Ada", old: true }));
    respond(json({ id: 1, name: "Grace", fresh: 1 }, { status: 201, statusText: "Created" }));
    expect(compareButton()).toHaveAttribute("aria-disabled", "false");
    await user.click(compareButton());
    expect(compareView()).toHaveTextContent("3 changes · status 200 → 201");
    expect(changedPaths()).toEqual(["$['fresh']", "$['name']", "$['old']"]);
    expect(within(compareView()).getByText('"Ada"')).toBeInTheDocument();
    expect(within(compareView()).getByText('"Grace"')).toBeInTheDocument();
    expect(within(compareView()).getByText("removed")).toHaveClass("text-(--danger)");
  });

  it("ignore paths re-run the diff; header names can be ignored too", async () => {
    const { user, respond } = setup(json({ id: 1, name: "Ada", old: true, items: [{ id: 1 }] }, { headers: { "X-Rev": "1" } }));
    respond(json({ id: 1, name: "Grace", fresh: 1, items: [{ id: 2 }] }, { headers: { "x-rev": "2" } }));
    await user.click(compareButton());
    const headers = within(screen.getByRole("list", { name: "Header changes" }));
    expect(headers.getByText("x-rev")).toBeInTheDocument();
    await user.type(screen.getByRole("textbox", { name: "Ignore paths" }), typed("$.name, $.old, $.items[*].id, X-Rev"));
    expect(changedPaths()).toEqual(["$['fresh']"]);
    expect(screen.queryByRole("list", { name: "Header changes" })).not.toBeInTheDocument();
    await user.type(screen.getByRole("textbox", { name: "Ignore paths" }), typed(", $["));
    expect(screen.getByText("Invalid ignore path: $[")).toBeInTheDocument();
  });

  it("a reordered object is no difference", async () => {
    const { user, respond } = setup(json({ a: 1, b: { c: 2, d: 3 } }));
    respond(json({ b: { d: 3, c: 2 }, a: 1 }));
    await user.click(compareButton());
    expect(within(compareView()).getByText("No differences")).toBeInTheDocument();
  });

  it("text bodies are compared as text", async () => {
    const text = (data: string): TResponseJson => ({ status: 200, data, headers: { "content-type": "text/plain" } });
    const { user, respond } = setup(text("one"));
    respond(text("one\ntwo"));
    await user.click(compareButton());
    expect(compareView()).toHaveTextContent("Body differs · 1 → 2 lines");
  });

  it("Close returns to the body with focus on Compare; a new response closes it", async () => {
    const { user, respond } = setup(json({ a: 1 }));
    respond(json({ a: 2 }));
    await user.click(compareButton());
    expect(screen.queryByRole("tab", { name: /body/i })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Close compare" }));
    expect(screen.getByRole("tab", { name: /body/i })).toBeInTheDocument();
    expect(compareButton()).toHaveFocus();
    await user.click(compareButton());
    respond(json({ a: 3 }));
    expect(screen.queryByRole("region", { name: "Comparison with previous response" })).not.toBeInTheDocument();
  });

  it("an error in between is skipped: compares with the last real response", async () => {
    const { user, respond } = setup(json({ a: 1 }));
    respond({ error: "Error: boom" });
    respond(json({ a: 2 }));
    await user.click(compareButton());
    expect(changedPaths()).toEqual(["$['a']"]);
  });

  it("opening another request starts over", () => {
    const { ctx, respond } = setup(json({ a: 1 }));
    respond(json({ a: 2 }));
    expect(compareButton()).toHaveAttribute("aria-disabled", "false");
    act(() => {
      ctx().setSelectorResponse({
        collectionName: "Users", folderPath: ["Admin"], curlName: "Create user",
        curlJson: { method: "POST", url: "https://x/admin/users", headers: "{}", body: "", params: "{}" },
      });
    });
    // The new route's saved response lands a render later, as in the app.
    respond(json({ a: 3 }));
    expect(compareButton()).toHaveAttribute("aria-disabled", "true");
  });

  it("local mode falls back to the newest history entry with the same method and URL", async () => {
    const hist = (data: unknown) => ({
      id: String(Math.random()), timestamp: 1, method: "GET", url: "https://<<HOST>>/users",
      curlJson: { method: "GET", url: "https://<<HOST>>/users", headers: "{}", body: "", params: "{}" },
      responseJson: json(data),
    });
    // Newest first: the entry for the response on screen, then the one before it.
    localStorage.setItem("hittable_history", JSON.stringify([hist({ v: 2 }), hist({ v: 1 }), hist({ v: 0 })]));
    const user = userEvent.setup();
    const { ctx } = mountData(<ResponsePanel />);
    act(() => {
      ctx().setFormInput((f) => ({ ...f, method: "GET", url: "https://<<HOST>>/users" }));
      ctx().setProxyResponse(json({ v: 2 }));
    });
    await user.click(compareButton());
    expect(within(compareView()).getByText("1")).toBeInTheDocument();
    expect(within(compareView()).getByText("2")).toBeInTheDocument();
  });
});
