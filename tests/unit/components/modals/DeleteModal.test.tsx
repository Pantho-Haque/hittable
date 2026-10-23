// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect, useState } from "react";
import DeleteModal from "@/components/modals/DeleteModal";
import { useDataContext } from "@/context/dataContext";
import type { THittableCollections, THittableSelectorSelection } from "@/types";
import { renderWithData, stored } from "./renderWithData";

const seed: THittableCollections = [
  {
    collectionName: "api",
    env: {},
    secrets: {},
    items: [
      { type: "route", name: "list", curl: "curl https://x.dev", response: "" },
      {
        type: "folder",
        name: "users",
        items: [
          { type: "route", name: "get", curl: "curl https://x.dev/u", response: "" },
          { type: "folder", name: "admin", items: [{ type: "route", name: "ban", curl: "curl https://x.dev/b", response: "" }] },
        ],
      },
      { type: "folder", name: "empty", items: [] },
    ],
  },
  { collectionName: "other", env: {}, secrets: {}, items: [] },
];

let lastSelection: THittableSelectorSelection;

function Harness(props: {
  currentName: string;
  type: "collection" | "route" | "folder";
  collectionName?: string;
  folderPath?: string[];
  initial: THittableSelectorSelection;
}) {
  const { setCollections } = useDataContext();
  const [selection, setSelection] = useState(props.initial);
  useEffect(() => {
    lastSelection = selection;
  });
  return <DeleteModal {...props} setCollections={setCollections} setSelection={setSelection} />;
}

function setup(props: Parameters<typeof Harness>[0]) {
  renderWithData(<Harness {...props} />, { collections: seed });
  return userEvent.setup();
}

beforeEach(() => localStorage.clear());

describe("DeleteModal", () => {
  it("warns about a nested folder's own contents, not its parent's", async () => {
    const user = setup({ currentName: "admin", type: "folder", collectionName: "api", folderPath: ["users"], initial: { collectionName: "api", folderPath: ["users"], curlName: "" } });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText('This will permanently remove "admin" and its 1 route(s), 0 sub-folder(s).')).toBeInTheDocument();
  });

  it("deletes a collection and clears the selection", async () => {
    const user = setup({ currentName: "api", type: "collection", initial: { collectionName: "api", folderPath: ["users"], curlName: "get" } });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByRole("dialog", { name: "Delete collection" })).toBeInTheDocument();
    expect(screen.getByText('This will permanently remove "api" and all its routes.')).toBeInTheDocument();
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
    expect(stored().map((c) => c.collectionName)).toEqual(["other"]);
    expect(lastSelection).toEqual({ collectionName: "", folderPath: [], curlName: "" });
  });

  it("warns how much a non-empty folder contains, counting nested items", async () => {
    const user = setup({ currentName: "users", type: "folder", collectionName: "api", folderPath: [], initial: { collectionName: "api", folderPath: [], curlName: "" } });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText('This will permanently remove "users" and its 2 route(s), 1 sub-folder(s).')).toBeInTheDocument();
  });

  it("uses the plain warning for an empty folder", async () => {
    const user = setup({ currentName: "empty", type: "folder", collectionName: "api", folderPath: [], initial: { collectionName: "api", folderPath: [], curlName: "" } });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText('This will permanently remove "empty".')).toBeInTheDocument();
  });

  it("deletes a route inside a folder and deselects it", async () => {
    const user = setup({ currentName: "get", type: "route", collectionName: "api", folderPath: ["users"], initial: { collectionName: "api", folderPath: ["users"], curlName: "get" } });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.keyboard("{Enter}");
    const folder = stored()[0].items[1];
    expect(folder.type === "folder" && folder.items.map((i) => i.name)).toEqual(["admin"]);
    expect(lastSelection).toEqual({ collectionName: "api", folderPath: ["users"], curlName: "" });
  });

  it("leaves the other routes alone when deleting a top-level route", async () => {
    const user = setup({ currentName: "list", type: "route", collectionName: "api", folderPath: [], initial: { collectionName: "api", folderPath: [], curlName: "list" } });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
    expect(stored()[0].items.map((i) => i.name)).toEqual(["users", "empty"]);
  });

  it("stays in the parent folder after deleting a sub-folder", async () => {
    const user = setup({ currentName: "admin", type: "folder", collectionName: "api", folderPath: ["users"], initial: { collectionName: "api", folderPath: ["users"], curlName: "" } });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
    const folder = stored()[0].items[1];
    expect(folder.type === "folder" && folder.items.map((i) => i.name)).toEqual(["get"]);
    expect(lastSelection).toEqual({ collectionName: "api", folderPath: ["users"], curlName: "" });
  });

  it("keeps everything on Cancel or Escape", async () => {
    const user = setup({ currentName: "api", type: "collection", initial: { collectionName: "api", folderPath: [], curlName: "" } });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(stored()).toHaveLength(2);
  });

  it("does not react to Enter while closed", async () => {
    const user = setup({ currentName: "api", type: "collection", initial: { collectionName: "api", folderPath: [], curlName: "" } });
    await user.keyboard("{Enter}");
    expect(stored()).toHaveLength(2);
  });
});
