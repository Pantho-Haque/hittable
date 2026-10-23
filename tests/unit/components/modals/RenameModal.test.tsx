// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect, useState } from "react";
import RenameModal from "@/components/modals/RenameModal";
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
          { type: "route", name: "create", curl: "curl https://x.dev/u", response: "" },
        ],
      },
      { type: "folder", name: "admin", items: [] },
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
}) {
  const { collections, setCollections } = useDataContext();
  const [selection, setSelection] = useState<THittableSelectorSelection>({
    collectionName: "api",
    folderPath: props.folderPath ?? [],
    curlName: props.type === "route" ? props.currentName : "",
  });
  useEffect(() => {
    lastSelection = selection;
  });
  const curlList = Object.fromEntries(collections.map((c) => [c.collectionName, []]));
  return <RenameModal {...props} collectionCurlList={curlList} setCollections={setCollections} setSelection={setSelection} />;
}

function setup(props: Parameters<typeof Harness>[0]) {
  renderWithData(<Harness {...props} />, { collections: seed });
  return userEvent.setup();
}

async function rename(user: ReturnType<typeof userEvent.setup>, to: string) {
  await user.click(screen.getByRole("button", { name: "Rename" }));
  const input = screen.getByRole("textbox");
  await user.clear(input);
  if (to) await user.type(input, to);
  return input;
}

beforeEach(() => localStorage.clear());

describe("RenameModal", () => {
  it("opens prefilled with the current name", async () => {
    const user = setup({ currentName: "api", type: "collection" });
    await user.click(screen.getByRole("button", { name: "Rename" }));
    expect(screen.getByRole("dialog", { name: "Rename collection" })).toBeInTheDocument();
    expect(screen.getByText("Currently: api")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toHaveValue("api");
  });

  it("renames a collection and follows the selection to the new name", async () => {
    const user = setup({ currentName: "api", type: "collection" });
    await rename(user, " v2 ");
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Rename" }));
    expect(stored().map((c) => c.collectionName)).toEqual(["v2", "other"]);
    expect(lastSelection.collectionName).toBe("v2");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("refuses a collection name that is already taken and keeps the dialog open", async () => {
    const user = setup({ currentName: "api", type: "collection" });
    await rename(user, "other{Enter}");
    expect(screen.getByText('"other" already exists')).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(stored().map((c) => c.collectionName)).toEqual(["api", "other"]);

    await user.type(screen.getByRole("textbox"), "-2");
    expect(screen.queryByText(/already exists/)).not.toBeInTheDocument();
  });

  it("treats an empty name or an unchanged name as a no-op", async () => {
    const user = setup({ currentName: "api", type: "collection" });
    await rename(user, "");
    await user.keyboard("{Enter}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await rename(user, "api{Enter}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(stored().map((c) => c.collectionName)).toEqual(["api", "other"]);
  });

  it("treats a name that only differs by surrounding spaces as unchanged", async () => {
    const user = setup({ currentName: "api", type: "collection" });
    await rename(user, " api {Enter}");
    expect(screen.queryByText(/already exists/)).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(stored().map((c) => c.collectionName)).toEqual(["api", "other"]);
  });

  it("renames a route inside a folder and keeps it selected", async () => {
    const user = setup({ currentName: "get", type: "route", collectionName: "api", folderPath: ["users"] });
    await rename(user, "get-one{Enter}");
    const folder = stored()[0].items[1];
    expect(folder.type === "folder" && folder.items.map((i) => i.name)).toEqual(["get-one", "create"]);
    expect(lastSelection.curlName).toBe("get-one");
  });

  it("refuses a route name already used in the same folder", async () => {
    const user = setup({ currentName: "get", type: "route", collectionName: "api", folderPath: ["users"] });
    await rename(user, "create{Enter}");
    expect(screen.getByText('"create" already exists')).toBeInTheDocument();
    const folder = stored()[0].items[1];
    expect(folder.type === "folder" && folder.items.map((i) => i.name)).toEqual(["get", "create"]);
  });

  it("renames a folder and refuses a sibling's name", async () => {
    const user = setup({ currentName: "admin", type: "folder", collectionName: "api", folderPath: [] });
    await rename(user, "users{Enter}");
    expect(screen.getByText('"users" already exists')).toBeInTheDocument();
    await user.clear(screen.getByRole("textbox"));
    await user.type(screen.getByRole("textbox"), "staff{Enter}");
    expect(stored()[0].items.map((i) => i.name)).toEqual(["list", "users", "staff"]);
  });

  it("closes on Escape without renaming", async () => {
    const user = setup({ currentName: "api", type: "collection" });
    await rename(user, "zzz");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(stored()[0].collectionName).toBe("api");
  });

  it("does not open the parent row when the Rename menu item is clicked", async () => {
    const onRow = vi.fn();
    localStorage.clear();
    renderWithData(
      <div onClick={onRow}>
        <Harness currentName="api" type="collection" />
      </div>,
      { collections: seed },
    );
    await userEvent.setup().click(screen.getByRole("button", { name: "Rename" }));
    expect(onRow).not.toHaveBeenCalled();
  });
});
