// @vitest-environment jsdom
import { useEffect } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, renderHook } from "@testing-library/react";
import { DataProvider, useDataContext } from "@/context/dataContext";
import { collections as seedCollections } from "@/constants";
import type { THittableCollections } from "@/types";

type Ctx = ReturnType<typeof useDataContext>;

function mount() {
  const ctxRef: { current: Ctx | null } = { current: null };
  function Probe() {
    const ctx = useDataContext();
    useEffect(() => { ctxRef.current = ctx; });
    return null;
  }
  render(<DataProvider><Probe /></DataProvider>);
  return () => ctxRef.current!;
}

const stored: THittableCollections = [
  {
    collectionName: "Users",
    items: [{ type: "route", name: "List", curl: "curl -X GET https://api.example.com/users", response: "" }],
    env: { BASE: "https://api.example.com" },
    secrets: {},
  },
];

function selectRoute(ctx: () => Ctx) {
  const curlJson = { method: "GET", url: "https://api.example.com/users", headers: "{}", body: '{"a":1}', params: "{}" };
  act(() => {
    ctx().setSelectorResponse({ collectionName: "Users", folderPath: [], curlName: "List", curlJson, responseJson: null });
    ctx().setFormInput({ ...curlJson });
  });
  return curlJson;
}

beforeEach(() => localStorage.clear());

describe("DataProvider", () => {
  it("useDataContext outside the provider throws a clear error", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => renderHook(() => useDataContext())).toThrow(/within a DataProvider/);
  });

  it("a first visit starts with the sample collections", () => {
    const ctx = mount();
    expect(ctx().collections.map((c) => c.collectionName)).toEqual(seedCollections.map((c) => c.collectionName));
    expect(ctx().hasCollections).toBe(true);
  });

  it("loads collections saved in localStorage", () => {
    localStorage.setItem("hittable", JSON.stringify(stored));
    const ctx = mount();
    expect(ctx().collections).toEqual(stored);
  });

  it("corrupt stored data falls back to the samples instead of crashing", () => {
    localStorage.setItem("hittable", "{not json");
    const ctx = mount();
    expect(ctx().collections.length).toBe(seedCollections.length);
  });

  it("upgrades the legacy flat `curls` format and writes the upgrade back", () => {
    localStorage.setItem(
      "hittable",
      JSON.stringify([{ collectionName: "Old", curls: [{ name: "Ping", curl: "curl https://x.io", response: "" }], env: { A: "1" } }]),
    );
    const ctx = mount();
    expect(ctx().collections[0]).toEqual({
      collectionName: "Old",
      items: [{ type: "route", name: "Ping", curl: "curl https://x.io", response: "" }],
      env: { A: "1" },
      secrets: {},
    });
    expect(JSON.parse(localStorage.getItem("hittable")!)[0].items).toBeDefined();
  });

  it("persists every collection change to localStorage", () => {
    localStorage.setItem("hittable", JSON.stringify(stored));
    const ctx = mount();
    act(() => ctx().setCollections((prev) => [...prev, { collectionName: "New", items: [], env: {}, secrets: {} }]));
    expect(JSON.parse(localStorage.getItem("hittable")!).map((c: { collectionName: string }) => c.collectionName)).toEqual(["Users", "New"]);
  });

  it("loads request history from localStorage", () => {
    const entry = { id: "1", timestamp: 1, method: "GET", url: "https://x.io", curlJson: { method: "GET", url: "https://x.io", headers: "{}", body: "", params: "{}" } };
    localStorage.setItem("hittable_history", JSON.stringify([entry]));
    const ctx = mount();
    expect(ctx().history).toEqual([entry]);
  });

  describe("unsaved changes", () => {
    it("a freshly selected route is not unsaved", () => {
      const ctx = mount();
      selectRoute(ctx);
      expect(ctx().isUnsaved()).toBe(false);
    });

    it("editing the URL marks the request unsaved", () => {
      const ctx = mount();
      selectRoute(ctx);
      act(() => ctx().setFormInput((f) => ({ ...f, url: f.url + "/2" })));
      expect(ctx().isUnsaved()).toBe(true);
    });

    it("reformatting JSON alone is not a change", () => {
      const ctx = mount();
      selectRoute(ctx);
      act(() => ctx().setFormInput((f) => ({ ...f, body: '{\n\t"a": 1\n}', headers: "{ }", params: "{\n}" })));
      expect(ctx().isUnsaved()).toBe(false);
    });

    it("changing a JSON value is a change", () => {
      const ctx = mount();
      selectRoute(ctx);
      act(() => ctx().setFormInput((f) => ({ ...f, body: '{"a":2}' })));
      expect(ctx().isUnsaved()).toBe(true);
    });

    it("the browser warns before closing the tab only while something is unsaved", () => {
      const ctx = mount();
      selectRoute(ctx);
      const clean = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(clean);
      expect(clean.defaultPrevented).toBe(false);

      act(() => ctx().setFormInput((f) => ({ ...f, method: "POST" })));
      const dirty = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(dirty);
      expect(dirty.defaultPrevented).toBe(true);
    });
  });

  it("Save writes the edited request back into its collection as curl, with the response", () => {
    localStorage.setItem("hittable", JSON.stringify(stored));
    const ctx = mount();
    selectRoute(ctx);
    act(() => {
      ctx().setFormInput((f) => ({ ...f, method: "POST", headers: '{"X-Id":"7"}', body: '{"name":"Ada"}' }));
      ctx().setProxyResponse({ status: 201, data: { id: 1 } });
    });
    act(() => ctx().handleSaveCollection());

    const route = ctx().collections[0].items[0];
    expect(route.type === "route" && route.curl).toBe(
      `curl -X POST 'https://api.example.com/users' -H 'X-Id: 7' -d '{"name":"Ada"}'`,
    );
    expect(route.type === "route" && JSON.parse(route.response)).toEqual({ status: 201, data: { id: 1 } });
    expect(ctx().isUnsaved()).toBe(false);
    expect(JSON.parse(localStorage.getItem("hittable")!)[0].items[0].curl).toContain("-X POST");
  });

  it("Discard changes restores the last saved request and response", () => {
    const ctx = mount();
    const saved = selectRoute(ctx);
    act(() => {
      ctx().setFormInput((f) => ({ ...f, url: "https://elsewhere.io" }));
      ctx().setProxyResponse({ status: 500 });
    });
    act(() => ctx().handleRevert());
    expect(ctx().formInput.url).toBe(saved.url);
    expect(JSON.parse(ctx().formInput.body)).toEqual({ a: 1 });
    expect(ctx().proxyResponse).toBeNull();
    expect(ctx().isUnsaved()).toBe(false);
  });

  it("Save and Discard do nothing when no route is selected", () => {
    localStorage.setItem("hittable", JSON.stringify(stored));
    const ctx = mount();
    act(() => ctx().setSelectorResponse(null));
    act(() => ctx().handleSaveCollection());
    act(() => ctx().handleRevert());
    expect(ctx().collections).toEqual(stored);
    expect(ctx().isUnsaved()).toBe(false);
  });
});
