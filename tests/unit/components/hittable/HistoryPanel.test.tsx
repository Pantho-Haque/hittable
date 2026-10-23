// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import HistoryPanel from "@/components/hittable/HistoryPanel";
import { mountData } from "../../helpers/dataHarness";
import type { THistoryEntry } from "@/types";

const now = Date.now();
const entry = (over: Partial<THistoryEntry>): THistoryEntry => ({
  id: Math.random().toString(36).slice(2),
  timestamp: now,
  method: "GET",
  url: "https://api.example.com/users",
  curlJson: { method: "GET", url: "https://api.example.com/users", headers: "{}", body: "", params: "{}" },
  ...over,
});

function setup(history: THistoryEntry[]) {
  localStorage.setItem("hittable_history", JSON.stringify(history));
  const user = userEvent.setup();
  return { user, ...mountData(<HistoryPanel />) };
}

const dialog = () => screen.getByRole("dialog");
const rows = () => within(dialog()).getAllByRole("button").filter((b) => b.querySelector("[title='Delete entry']"));

beforeEach(() => localStorage.clear());

describe("Request history", () => {
  it("the clock badge shows how many requests are logged, capped at 99+", () => {
    const { unmount } = setup([entry({}), entry({})]);
    expect(screen.getByTitle("History")).toHaveTextContent("2");
    unmount();
    setup(Array.from({ length: 100 }, () => entry({})));
    expect(screen.getByTitle("History")).toHaveTextContent("99+");
  });

  it("an empty log says so", async () => {
    const { user } = setup([]);
    await user.click(screen.getByTitle("History"));
    expect(within(dialog()).getByText("No request history yet")).toBeInTheDocument();
    expect(within(dialog()).getByText("0 requests logged")).toBeInTheDocument();
    expect(within(dialog()).queryByText("Clear all")).not.toBeInTheDocument();
  });

  it("each row shows method, URL, status, duration, size and relative time", async () => {
    const { user } = setup([
      entry({ method: "POST", url: "https://api.example.com/orders", status: 201, statusText: "Created", durationMs: 182, sizeBytes: 2048, timestamp: now - 5 * 60_000 }),
    ]);
    await user.click(screen.getByTitle("History"));
    const d = within(dialog());
    expect(d.getByText("1 request logged")).toBeInTheDocument();
    expect(d.getByText("POST")).toBeInTheDocument();
    expect(d.getByText("https://api.example.com/orders")).toBeInTheDocument();
    expect(d.getByText("201")).toHaveStyle({ color: "rgb(74, 222, 128)" });
    expect(d.getByText("Created")).toBeInTheDocument();
    expect(d.getByText("182ms")).toBeInTheDocument();
    expect(d.getByText("2.0 KB")).toBeInTheDocument();
    expect(d.getByText("5m ago")).toBeInTheDocument();
  });

  it("status colours: 3xx/4xx amber, 5xx red", async () => {
    const { user } = setup([entry({ status: 404 }), entry({ status: 503 })]);
    await user.click(screen.getByTitle("History"));
    expect(within(dialog()).getByText("404")).toHaveStyle({ color: "rgb(251, 146, 60)" });
    expect(within(dialog()).getByText("503")).toHaveStyle({ color: "rgb(248, 113, 113)" });
  });

  it("clicking a row loads it back into the runner as a request with no collection", async () => {
    const saved = entry({
      method: "PUT",
      url: "https://api.example.com/users/3",
      curlJson: { method: "PUT", url: "https://api.example.com/users/3", headers: '{"A":"1"}', body: '{"x":1}', params: "{}" },
      responseJson: { status: 200, data: { ok: true } },
    });
    const { user, ctx } = setup([saved]);
    await user.click(screen.getByTitle("History"));
    await user.click(rows()[0]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(ctx().formInput).toEqual(saved.curlJson);
    expect(ctx().proxyResponse).toEqual({ status: 200, data: { ok: true } });
    expect(ctx().selectorResponse).toMatchObject({ collectionName: "", curlName: "History: PUT /users/3" });
  });

  it("a logged request whose URL is a <<KEY>> template can still be loaded", async () => {
    const templated = entry({
      url: "<<BASE_URL>>/users",
      curlJson: { method: "GET", url: "<<BASE_URL>>/users", headers: "{}", body: "", params: "{}" },
    });
    const { user, ctx } = setup([templated]);
    await user.click(screen.getByTitle("History"));
    await user.click(rows()[0]);
    expect(ctx().formInput.url).toBe("<<BASE_URL>>/users");
    expect(ctx().selectorResponse!.curlName).toBe("History: GET <<BASE_URL>>/users");
  });

  it("a failed request (no status) is loaded with no response", async () => {
    const { user, ctx } = setup([entry({})]);
    await user.click(screen.getByTitle("History"));
    await user.click(rows()[0]);
    expect(ctx().proxyResponse).toBeNull();
  });

  it("deleting a row removes only that entry, and it stays deleted", async () => {
    const { user, ctx } = setup([entry({ url: "https://a.io/1" }), entry({ url: "https://a.io/2" })]);
    await user.click(screen.getByTitle("History"));
    await user.click(within(rows()[0]).getByTitle("Delete entry"));
    expect(ctx().history.map((e) => e.url)).toEqual(["https://a.io/2"]);
    expect(screen.getByRole("dialog")).toBeInTheDocument(); // deleting doesn't load the row
    await waitFor(() => expect(JSON.parse(localStorage.getItem("hittable_history")!)).toHaveLength(1));
  });

  it("Clear all empties the log", async () => {
    const { user, ctx } = setup([entry({}), entry({})]);
    await user.click(screen.getByTitle("History"));
    await user.click(within(dialog()).getByText("Clear all"));
    expect(ctx().history).toEqual([]);
    expect(within(dialog()).getByText("No request history yet")).toBeInTheDocument();
    await waitFor(() => expect(localStorage.getItem("hittable_history")).toBe("[]"));
  });

  it("Close dismisses the dialog", async () => {
    const { user } = setup([entry({})]);
    await user.click(screen.getByTitle("History"));
    await user.click(within(dialog()).getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
