// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SaveFromHistoryModal from "@/components/modals/SaveFromHistoryModal";
import type { THittableCollections, THittableSelectorResponse } from "@/types";
import { curlConverter } from "@/utils/curlConverter";
import { renderWithData, stored } from "./renderWithData";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

const seed: THittableCollections = [
  {
    collectionName: "api",
    env: {},
    secrets: {},
    items: [
      { type: "route", name: "ping", curl: "curl -X GET https://x.dev/ping", response: "" },
      { type: "folder", name: "users", items: [{ type: "folder", name: "admin", items: [] }] },
    ],
  },
  { collectionName: "shop", env: {}, secrets: {}, items: [] },
];

const fromHistory: THittableSelectorResponse = {
  collectionName: "",
  folderPath: [],
  curlName: "History: GET https://x.dev/me",
  curlJson: { method: "POST", url: "https://x.dev/me", headers: '{"A":"1"}', body: '{"x":1}', params: "{}" },
  responseJson: { status: 200, data: { ok: true } },
};

function setup(collections = seed, selector: THittableSelectorResponse = fromHistory) {
  const utils = renderWithData(<SaveFromHistoryModal />, { collections, selector });
  return { ...utils, user: userEvent.setup() };
}

const dialog = () => within(screen.getByRole("dialog", { name: "Save from History" }));
const routeInput = () => dialog().getByLabelText("e.g. get-users");
const saveBtn = () => dialog().getByRole("button", { name: "Save" });

beforeEach(() => {
  localStorage.clear();
  push.mockClear();
});

describe("SaveFromHistoryModal", () => {
  it("is not offered for a request that already belongs to a collection", () => {
    setup(seed, { ...fromHistory, collectionName: "api", curlName: "ping" });
    expect(screen.queryByTitle("Save to collection")).not.toBeInTheDocument();
  });

  it("saves into the first collection by default and opens the saved route", async () => {
    const { user, ctx } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.type(routeInput(), " me {Enter}");

    const route = stored()[0].items[2];
    expect(route).toMatchObject({ type: "route", name: "me" });
    // The stored curl reads back as the same request.
    const back = curlConverter((route as { curl: string }).curl);
    expect(back.method).toBe("POST");
    expect(back.url).toBe("https://x.dev/me");
    expect(JSON.parse(back.headers)).toEqual({ A: "1" });
    // Value types are curlConverter's concern (tested there); here: the body made it.
    expect(Object.keys(JSON.parse(back.body))).toEqual(["x"]);
    expect(push).toHaveBeenCalledWith("/hittable?c=api&r=me");
    expect(ctx().selectorResponse).toMatchObject({ collectionName: "api", folderPath: [], curlName: "me" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("saves into a chosen collection's nested folder", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.click(dialog().getByRole("button", { name: "users" }));
    await user.click(dialog().getByRole("button", { name: "admin" }));
    await user.type(routeInput(), "me");
    await user.click(saveBtn());
    const users = stored()[0].items[1];
    expect(users).toMatchObject({ items: [{ name: "admin", items: [{ type: "route", name: "me" }] }] });
    expect(push).toHaveBeenCalledWith("/hittable?c=api&r=me&p=users%2Fadmin");
  });

  it("can go back up a folder level before saving", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.click(dialog().getByRole("button", { name: "users" }));
    await user.click(dialog().getByTitle("Go up one level"));
    await user.type(routeInput(), "me{Enter}");
    expect(stored()[0].items.map((i) => i.name)).toEqual(["ping", "users", "me"]);
  });

  it("does not overwrite a route with the same name", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.type(routeInput(), "ping{Enter}");
    expect(stored()[0].items.map((i) => i.name)).toEqual(["ping", "users", "ping - New"]);
  });

  it("requires a route name and saves nothing without one", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.click(saveBtn());
    expect(dialog().getByText("Route name is required")).toBeInTheDocument();
    expect(stored()).toEqual(seed);
    expect(push).not.toHaveBeenCalled();
    await user.type(routeInput(), "x");
    expect(dialog().queryByText("Route name is required")).not.toBeInTheDocument();
  });

  it("creates a new collection and saves the route in it", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.click(dialog().getByRole("button", { name: "+ Create new collection" }));
    await user.type(dialog().getByLabelText("New collection name"), "billing");
    await user.type(routeInput(), "me");
    await user.click(saveBtn());
    expect(stored()[2]).toMatchObject({ collectionName: "billing", items: [{ type: "route", name: "me" }] });
    expect(push).toHaveBeenCalledWith("/hittable?c=billing&r=me");
  });

  it("does not leave an empty collection behind when the route name is missing", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.click(dialog().getByRole("button", { name: "+ Create new collection" }));
    await user.type(dialog().getByLabelText("New collection name"), "billing");
    await user.click(saveBtn());
    await user.click(saveBtn());
    expect(dialog().getByText("Route name is required")).toBeInTheDocument();
    expect(stored().map((c) => c.collectionName)).toEqual(["api", "shop"]);
  });

  it("puts the route at the top of a new collection even after browsing another one's folders", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.click(dialog().getByRole("button", { name: "users" }));
    await user.click(dialog().getByRole("button", { name: "+ Create new collection" }));
    await user.type(dialog().getByLabelText("New collection name"), "billing");
    await user.type(routeInput(), "me{Enter}");
    expect(stored()[2]).toMatchObject({ collectionName: "billing", items: [{ type: "route", name: "me" }] });
    expect(push).toHaveBeenCalledWith("/hittable?c=billing&r=me");
  });

  it("requires a collection name when creating one", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.click(dialog().getByRole("button", { name: "+ Create new collection" }));
    await user.type(routeInput(), "me{Enter}");
    expect(dialog().getByText("Collection name is required")).toBeInTheDocument();
    expect(stored()).toEqual(seed);
  });

  it("renames a new collection that clashes with an existing one", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.click(dialog().getByRole("button", { name: "+ Create new collection" }));
    await user.type(dialog().getByLabelText("New collection name"), "shop");
    await user.type(routeInput(), "me{Enter}");
    expect(stored().map((c) => c.collectionName)).toEqual(["api", "shop", "shop - New"]);
    expect(stored()[2].items.map((i) => i.name)).toEqual(["me"]);
  });

  it("asks for a collection when there is none to save into", async () => {
    const { user } = setup([]);
    await user.click(screen.getByTitle("Save to collection"));
    await user.type(routeInput(), "me{Enter}");
    expect(dialog().getByText("Choose a collection")).toBeInTheDocument();
    expect(stored()).toEqual([]);
    expect(push).not.toHaveBeenCalled();
  });

  it("creates a folder from the picker and moves into it", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.click(dialog().getByRole("button", { name: "+ New folder" }));
    await user.type(dialog().getByPlaceholderText("Folder name"), "users{Enter}");
    // "users" is taken at the top level, so the new folder gets a distinct name.
    expect(stored()[0].items.map((i) => i.name)).toEqual(["ping", "users", "users - New"]);
    await user.type(routeInput(), "me{Enter}");
    expect(stored()[0].items[2]).toMatchObject({ name: "users - New", items: [{ type: "route", name: "me" }] });
  });

  it("closes on Escape without saving", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Save to collection"));
    await user.type(routeInput(), "me{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(stored()).toEqual(seed);
  });
});
