// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import Selector from "@/components/hittable/Selector";
import RequestForm from "@/components/hittable/RequestForm";
import { DataProvider, useDataContext } from "@/context/dataContext";
import { NotificationProvider } from "@/context/notifyContext";
import { ShortcutProvider } from "@/context/ShortcutKeypressProvider";
import { sampleCollections, type DataCtx } from "../../helpers/dataHarness";
import type { THittableCollections } from "@/types";

// The router is the boundary: a tiny URL store so pushes re-render readers.
const nav = vi.hoisted(() => {
  let url = "/hittable";
  const subs = new Set<() => void>();
  return {
    get: () => url,
    push: (u: string) => { url = u; subs.forEach((f) => f()); },
    reset: (u = "/hittable") => { url = u; },
    subscribe: (f: () => void) => { subs.add(f); return () => { subs.delete(f); }; },
  };
});
vi.mock("next/navigation", async () => {
  const React = await import("react");
  return {
    useRouter: () => ({ push: nav.push }),
    useSearchParams: () => {
      const url = React.useSyncExternalStore(nav.subscribe, nav.get);
      return React.useMemo(() => new URLSearchParams(url.split("?")[1] ?? ""), [url]);
    },
  };
});

function mount(collections: THittableCollections = sampleCollections(), url = "/hittable") {
  nav.reset(url);
  localStorage.setItem("hittable", JSON.stringify(collections));
  const ctxRef: { current: DataCtx | null } = { current: null };
  function Probe() {
    const ctx = useDataContext();
    useEffect(() => { ctxRef.current = ctx; });
    return null;
  }
  render(
    <NotificationProvider>
      <DataProvider>
        <ShortcutProvider>
          <Probe />
          <Selector />
          <main><RequestForm /></main>
        </ShortcutProvider>
      </DataProvider>
    </NotificationProvider>,
  );
  return { user: userEvent.setup(), ctx: () => ctxRef.current! };
}

const sidebar = () => within(screen.getByRole("complementary", { name: "Collections" }));
const query = () => Object.fromEntries(new URLSearchParams(nav.get().split("?")[1] ?? ""));

beforeEach(() => {
  localStorage.clear();
  Element.prototype.scrollIntoView = () => {};
});

describe("Collections sidebar and request form", () => {
  it("with nothing selected the form asks you to pick a route", () => {
    mount();
    expect(sidebar().getByText("Collections")).toBeInTheDocument();
    expect(screen.getByText("Select a route")).toBeInTheDocument();
  });

  it("with no collections at all it says so", () => {
    mount([]);
    expect(sidebar().getByText("No collections")).toBeInTheDocument();
    expect(screen.getByText("No collections yet")).toBeInTheDocument();
  });

  it("opening a collection selects its first route and loads it into the form", async () => {
    const { user, ctx } = mount();
    await user.click(sidebar().getByText("Users"));
    expect(query()).toEqual({ c: "Users", r: "List users" });
    await waitFor(() => expect(screen.getByLabelText("Request URL")).toHaveValue("https://<<HOST>>/users"));
    expect(ctx().selectorResponse).toMatchObject({
      collectionName: "Users", curlName: "List users",
      env: { HOST: "api.example.com", TOKEN: "env-token" }, secrets: { TOKEN: "secret-token" },
    });
    expect(screen.queryByText(/Unsaved/)).not.toBeInTheDocument();
  });

  it("lists folders (with their size) before routes, routes with method badges", async () => {
    const { user } = mount();
    await user.click(sidebar().getByText("Users"));
    const folder = sidebar().getByTitle("Admin");
    const route = sidebar().getByTitle("List users");
    expect(folder.compareDocumentPosition(route) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(within(folder).getByText("1")).toBeInTheDocument();
    expect(within(route).getByText("GET")).toBeInTheDocument();
  });

  it("drilling into a folder, the breadcrumb and Back navigate the tree", async () => {
    const { user } = mount();
    await user.click(sidebar().getByText("Users"));
    await user.click(sidebar().getByTitle("Admin"));
    expect(query()).toEqual({ c: "Users", p: "Admin" });
    const create = sidebar().getByTitle("Create user");
    expect(within(create).getByText("POST")).toBeInTheDocument();

    await user.click(create);
    expect(query()).toEqual({ c: "Users", r: "Create user", p: "Admin" });
    await waitFor(() => expect(screen.getByLabelText("HTTP method")).toHaveValue("POST"));

    // breadcrumb: collection name jumps to the root of the collection
    const crumbs = sidebar().getAllByRole("button", { name: "Users" });
    await user.click(crumbs[crumbs.length - 1]);
    expect(query()).toEqual({ c: "Users" });

    await user.click(sidebar().getByTitle("Admin"));
    await user.click(sidebar().getAllByRole("button", { name: /Users/ })[0]); // ← back
    expect(query()).toEqual({ c: "Users" });
    await user.click(sidebar().getAllByRole("button", { name: /Users/ })[0]);
    expect(nav.get()).toBe("/hittable");
  });

  it("a deep link restores the selection", async () => {
    mount(sampleCollections(), "/hittable?c=Users&r=Create%20user&p=Admin");
    await waitFor(() => expect(screen.getByLabelText("Request URL")).toHaveValue("https://<<HOST>>/admin/users"));
    expect(screen.getByLabelText("HTTP method")).toHaveValue("POST");
  });

  it("empty collections and folders say so", async () => {
    const cols = sampleCollections();
    cols[0].items.push({ type: "folder", name: "Empty", items: [] });
    const { user } = mount(cols);
    await user.click(sidebar().getByText("Other"));
    expect(sidebar().getByText("No routes yet")).toBeInTheDocument();
    act(() => nav.push("/hittable?c=Users&p=Empty"));
    expect(sidebar().getByText("Empty folder")).toBeInTheDocument();
  });

  it("Shift+T adds an Untitled route to the current folder and opens it; names never clash", async () => {
    const { user, ctx } = mount();
    await user.click(sidebar().getByText("Users"));
    await user.click(sidebar().getByTitle("Admin"));
    await user.keyboard("{Shift>}T{/Shift}");
    expect(query()).toEqual({ c: "Users", r: "Untitled", p: "Admin" });
    await user.keyboard("{Shift>}T{/Shift}");
    expect(query().r).toBe("Untitled - New");
    const admin = ctx().collections[0].items.find((i) => i.name === "Admin");
    expect(admin?.type === "folder" && admin.items.map((i) => i.name)).toEqual(["Create user", "Untitled", "Untitled - New"]);
  });

  it("Shift+T does nothing outside a collection or while typing", async () => {
    const { user, ctx } = mount();
    await user.keyboard("{Shift>}T{/Shift}");
    expect(nav.get()).toBe("/hittable");
    await user.click(sidebar().getByText("Users"));
    await user.click(screen.getByLabelText("Request URL"));
    await user.keyboard("{Shift>}T{/Shift}");
    expect(ctx().collections[0].items).toHaveLength(2);
  });

  it("New Folder adds a uniquely named folder", async () => {
    const { user, ctx } = mount();
    await user.click(sidebar().getByText("Users"));
    await user.click(sidebar().getByRole("button", { name: "New Folder" }));
    await user.click(sidebar().getByRole("button", { name: "New Folder" }));
    expect(ctx().collections[0].items.map((i) => i.name)).toEqual(["List users", "Admin", "New Folder", "New Folder - New"]);
  });

  it("the filter appears for more than three items and narrows the list", async () => {
    const cols = sampleCollections();
    cols[0].items.push(
      { type: "route", name: "Get user", curl: "curl https://a.io/1", response: "" },
      { type: "route", name: "Delete user", curl: "curl -X DELETE https://a.io/1", response: "" },
    );
    const { user } = mount(cols);
    await user.click(sidebar().getByText("Users"));
    const filter = sidebar().getByPlaceholderText("Filter...");
    await user.type(filter, "DELETE");
    expect(sidebar().getByTitle("Delete user")).toBeInTheDocument();
    expect(sidebar().queryByTitle("Get user")).not.toBeInTheDocument();
    await user.clear(filter);
    await user.type(filter, "nothing");
    expect(sidebar().getByText("No matching items")).toBeInTheDocument();
  });

  it("collapse and expand the sidebar", async () => {
    const { user } = mount();
    await user.click(sidebar().getByRole("button", { name: "Collapse sidebar" }));
    expect(sidebar().queryByText("Users")).not.toBeInTheDocument();
    await user.click(sidebar().getByRole("button", { name: "Expand sidebar" }));
    expect(sidebar().getByText("Users")).toBeInTheDocument();
  });

  describe("form breadcrumbs", () => {
    it("the collection crumb switches collection", async () => {
      const { user } = mount(sampleCollections(), "/hittable?c=Users&r=List%20users");
      await waitFor(() => expect(screen.getByLabelText("Request URL")).not.toHaveValue(""));
      const form = within(screen.getByRole("main"));
      await user.click(form.getByRole("button", { name: "Users" }));
      await user.click(form.getByRole("button", { name: "Other" }));
      expect(query()).toEqual({ c: "Other" });
    });

    it("the route crumb lists sibling routes with their methods", async () => {
      const cols = sampleCollections();
      cols[0].items.push({ type: "route", name: "Remove", curl: "curl -X DELETE https://a.io/1", response: "" });
      const { user } = mount(cols, "/hittable?c=Users&r=List%20users");
      await waitFor(() => expect(screen.getByLabelText("Request URL")).not.toHaveValue(""));
      const form = within(screen.getByRole("main"));
      await user.click(form.getByRole("button", { name: "List users" }));
      const remove = form.getByRole("button", { name: /Remove/ });
      expect(within(remove).getByText("DELETE")).toBeInTheDocument();
      await user.click(remove);
      expect(query()).toEqual({ c: "Users", r: "Remove" });
    });

    it("a folder crumb lists the items beside that folder", async () => {
      const { user } = mount(sampleCollections(), "/hittable?c=Users&r=Create%20user&p=Admin");
      await waitFor(() => expect(screen.getByLabelText("HTTP method")).toHaveValue("POST"));
      const form = within(screen.getByRole("main"));
      await user.click(form.getByRole("button", { name: "Admin" }));
      await user.click(form.getByRole("button", { name: /List users/ }));
      expect(query()).toEqual({ c: "Users", r: "List users" });
    });
  });

  it("editing then switching routes discards nothing silently: the new route loads clean", async () => {
    const { user } = mount(sampleCollections(), "/hittable?c=Users&r=List%20users");
    await waitFor(() => expect(screen.getByLabelText("Request URL")).not.toHaveValue(""));
    await user.type(screen.getByLabelText("Request URL"), "/9");
    expect(screen.getByText(/Unsaved/)).toBeInTheDocument();
    act(() => nav.push("/hittable?c=Users&r=Create%20user&p=Admin"));
    await waitFor(() => expect(screen.getByLabelText("Request URL")).toHaveValue("https://<<HOST>>/admin/users"));
    expect(screen.queryByText(/Unsaved/)).not.toBeInTheDocument();
  });
});
