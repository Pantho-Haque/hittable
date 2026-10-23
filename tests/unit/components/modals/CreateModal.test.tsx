// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CreateModal from "@/components/modals/CreateModal";
import type { THittableCollections } from "@/types";
import { renderWithData, stored } from "./renderWithData";

const seed: THittableCollections = [
  {
    collectionName: "api",
    env: {},
    secrets: {},
    items: [
      { type: "route", name: "list", curl: "curl -X GET https://x.dev", response: "" },
      { type: "folder", name: "users", items: [] },
    ],
  },
];
const curlList = { api: ["list"] };

function setup(type: "collection" | "route", folderPath?: string[]) {
  const setSelection = vi.fn();
  const utils = renderWithData(
    <CreateModal
      type={type}
      selection={{ collectionName: "api", folderPath: [], curlName: "" }}
      setSelection={setSelection}
      collectionCurlList={curlList}
      folderPath={folderPath}
    />,
    { collections: seed },
  );
  return { ...utils, setSelection, user: userEvent.setup() };
}

beforeEach(() => localStorage.clear());

describe("CreateModal — collection", () => {
  it("creates a trimmed collection and selects it", async () => {
    const { user, setSelection } = setup("collection");
    await user.click(screen.getByTitle("New Collection"));
    expect(screen.getByRole("dialog", { name: "Create collection" })).toBeInTheDocument();
    await user.type(screen.getByLabelText("my-api"), "  payments  ");
    await user.click(screen.getByRole("button", { name: "Create" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(stored().map((c) => c.collectionName)).toEqual(["api", "payments"]);
    expect(setSelection).toHaveBeenCalledWith({ collectionName: "payments", folderPath: [], curlName: "" });
  });

  it("confirms with Enter", async () => {
    const { user } = setup("collection");
    await user.click(screen.getByTitle("New Collection"));
    await user.type(screen.getByLabelText("my-api"), "billing{Enter}");
    expect(stored().map((c) => c.collectionName)).toContain("billing");
  });

  it("closes on Escape without creating anything", async () => {
    const { user } = setup("collection");
    await user.click(screen.getByTitle("New Collection"));
    await user.type(screen.getByLabelText("my-api"), "nope");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(stored()).toHaveLength(1);
  });

  it("closes on Cancel without creating anything", async () => {
    const { user } = setup("collection");
    await user.click(screen.getByTitle("New Collection"));
    await user.type(screen.getByLabelText("my-api"), "nope");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(stored()).toHaveLength(1);
  });

  it("rejects an empty or whitespace-only name", async () => {
    const { user, setSelection } = setup("collection");
    await user.click(screen.getByTitle("New Collection"));
    await user.type(screen.getByLabelText("my-api"), "   ");
    await user.click(screen.getByRole("button", { name: "Create" }));
    expect(stored()).toHaveLength(1);
    expect(setSelection).not.toHaveBeenCalled();
  });

  it("never creates two collections with the same name", async () => {
    const { user } = setup("collection");
    for (let i = 0; i < 3; i++) {
      await user.click(screen.getByTitle("New Collection"));
      await user.type(screen.getByLabelText("my-api"), "api{Enter}");
    }
    const names = stored().map((c) => c.collectionName);
    expect(names).toHaveLength(4);
    expect(new Set(names).size).toBe(4);
    expect(names[1]).toBe("api - New");
  });
});

describe("CreateModal — route", () => {
  it("adds a route with the pasted curl inside the current folder", async () => {
    const { user, setSelection } = setup("route", ["users"]);
    await user.click(screen.getByTitle("New Route"));
    expect(screen.getByText("Adding to api / users")).toBeInTheDocument();
    await user.type(screen.getByLabelText("get-users"), "get-one");
    await user.click(screen.getByPlaceholderText("Paste you curl here"));
    await user.paste("curl https://x.dev/users/1");
    await user.click(screen.getByRole("button", { name: "Create" }));

    const folder = stored()[0].items[1];
    expect(folder).toMatchObject({
      type: "folder",
      items: [{ type: "route", name: "get-one", curl: "curl https://x.dev/users/1", response: "" }],
    });
    expect(setSelection).toHaveBeenCalledWith({ collectionName: "api", folderPath: ["users"], curlName: "get-one" });
  });

  it("renames a duplicate route in the same folder instead of overwriting it", async () => {
    const { user } = setup("route");
    await user.click(screen.getByTitle("New Route"));
    await user.type(screen.getByLabelText("get-users"), "list{Enter}");
    await user.click(screen.getByTitle("New Route"));
    await user.type(screen.getByLabelText("get-users"), "list{Enter}");
    const names = stored()[0].items.map((i) => i.name);
    expect(names).toEqual(["list", "users", "list - New", "list - New - New"]);
  });

  it("allows the same route name in a different folder", async () => {
    const { user } = setup("route", ["users"]);
    await user.click(screen.getByTitle("New Route"));
    await user.type(screen.getByLabelText("get-users"), "list{Enter}");
    const folder = stored()[0].items[1];
    expect(folder.type === "folder" && folder.items.map((i) => i.name)).toEqual(["list"]);
  });
});
