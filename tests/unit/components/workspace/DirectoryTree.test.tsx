// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { WorkspaceProvider, useWorkspace } from "@/context/workspaceContext";
import { NotificationProvider } from "@/context/notifyContext";
import DirectoryTree from "@/components/workspace/DirectoryTree";
import { FakeDir, FakeFile, asDir } from "../../helpers/fakeFs";

vi.mock("@/services/workspace/HandleStore", () => ({
  saveDirectoryHandle: vi.fn(async () => {}),
  loadDirectoryHandle: vi.fn(async () => null),
  removeDirectoryHandle: vi.fn(async () => {}),
  verifyPermission: vi.fn(async () => true),
}));

let root: FakeDir;

function Opener() {
  const { openFolder, activeFile } = useWorkspace();
  useEffect(() => { openFolder(); }, [openFolder]);
  return <output aria-label="open file">{activeFile?.path.join("/") ?? ""}</output>;
}

async function mount() {
  window.showDirectoryPicker = vi.fn(async () => asDir(root)) as unknown as typeof window.showDirectoryPicker;
  render(
    <WorkspaceProvider>
      <NotificationProvider>
        <Opener />
        <DirectoryTree />
      </NotificationProvider>
    </WorkspaceProvider>,
  );
  await waitFor(() => expect(screen.getByRole("treeitem", { name: "hittable" })).toHaveAttribute("aria-expanded", "true"));
  return userEvent.setup();
}

const item = (name: string) => screen.getByRole("treeitem", { name });
const selected = () => screen.getAllByRole("treeitem").find((el) => el.getAttribute("aria-selected") === "true")?.getAttribute("aria-label");
// The context menu, found through the Rename action every menu has.
const menuButton = (name: string) =>
  within(screen.getByRole("button", { name: "Rename" }).parentElement!).getByRole("button", { name });
const row = (name: string) => item(name).firstElementChild as HTMLElement;

beforeEach(() => {
  Element.prototype.scrollIntoView = () => {}; // not in jsdom
  root = new FakeDir("my-api", {
    hittable: {
      "env.json": "{}",
      notes: { "sample.md": "# hi" },
      users: { "list.hit": '{"method":"GET"}', "create.hit": '{"method":"POST"}' },
    },
    "README.md": "# readme",
  });
});

describe("DirectoryTree (explorer)", () => {
  it("shows the folder name and expands the first top-level folder", async () => {
    await mount();
    expect(screen.getByText("my-api")).toBeInTheDocument();
    expect(item("hittable")).toHaveAttribute("aria-expanded", "true");
    expect(item("users")).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("treeitem", { name: "list.hit" })).not.toBeInTheDocument();
  });

  it("clicking a folder toggles it, clicking a file opens it", async () => {
    const user = await mount();
    await user.click(row("users"));
    expect(item("users")).toHaveAttribute("aria-expanded", "true");
    await user.click(row("list.hit"));
    expect(screen.getByLabelText("open file")).toHaveTextContent("hittable/users/list.hit");
    await user.click(row("users"));
    expect(screen.queryByRole("treeitem", { name: "list.hit" })).not.toBeInTheDocument();
  });

  it("keyboard: ↑ ↓ move, → expands, ← collapses or jumps to the parent, Enter opens", async () => {
    const user = await mount();
    act(() => screen.getByRole("tree").focus());
    expect(selected()).toBe("hittable");

    await user.keyboard("{ArrowDown}");
    expect(selected()).toBe("env.json");
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(selected()).toBe("users");

    await user.keyboard("{ArrowRight}");
    expect(item("users")).toHaveAttribute("aria-expanded", "true");
    await user.keyboard("{ArrowDown}");
    expect(selected()).toBe("list.hit");

    await user.keyboard("{Enter}");
    expect(screen.getByLabelText("open file")).toHaveTextContent("hittable/users/list.hit");

    await user.keyboard("{ArrowLeft}");
    expect(selected()).toBe("users");
    await user.keyboard("{ArrowLeft}");
    expect(item("users")).toHaveAttribute("aria-expanded", "false");
    await user.keyboard("{ArrowUp}");
    expect(selected()).toBe("notes");
    await user.keyboard("{Enter}");
    expect(item("notes")).toHaveAttribute("aria-expanded", "true");
  });

  it("↑ at the top and ↓ at the bottom stay put", async () => {
    const user = await mount();
    act(() => screen.getByRole("tree").focus());
    await user.keyboard("{ArrowUp}");
    expect(selected()).toBe("hittable");
    for (let i = 0; i < 10; i++) await user.keyboard("{ArrowDown}");
    expect(selected()).toBe("README.md");
  });

  it("F2 renames the selected file on disk", async () => {
    const user = await mount();
    act(() => screen.getByRole("tree").focus());
    await user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}{ArrowDown}");
    expect(selected()).toBe("README.md");
    await user.keyboard("{F2}");
    const input = screen.getByLabelText("Rename README.md");
    expect(input).toHaveValue("README.md");
    await user.clear(input);
    await user.type(input, "GUIDE.md{Enter}");
    await screen.findByRole("treeitem", { name: "GUIDE.md" });
    expect(root.at("README.md")).toBeUndefined();
    expect((root.at("GUIDE.md") as FakeFile).content).toBe("# readme");
  });

  it("Escape cancels a rename", async () => {
    const user = await mount();
    await user.pointer({ keys: "[MouseRight]", target: row("README.md") });
    await user.click(screen.getByRole("button", { name: "Rename" }));
    await user.type(screen.getByLabelText("Rename README.md"), "x{Escape}");
    expect(screen.queryByLabelText("Rename README.md")).not.toBeInTheDocument();
    expect(root.at("README.md")).toBeDefined();
  });

  it("Delete offers to delete the selected item; confirming removes it from disk", async () => {
    const user = await mount();
    await user.click(row("users"));
    await user.keyboard("{Delete}");
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.queryByRole("treeitem", { name: "users" })).not.toBeInTheDocument());
    expect(root.at("hittable/users")).toBeUndefined();
  });

  it("deleting the open file closes it", async () => {
    const user = await mount();
    await user.click(row("README.md"));
    expect(screen.getByLabelText("open file")).toHaveTextContent("README.md");
    await user.pointer({ keys: "[MouseRight]", target: row("README.md") });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.getByLabelText("open file")).toHaveTextContent(""));
  });

  describe("reserved names inside hittable/", () => {
    it("env.json and notes have no context menu", async () => {
      const user = await mount();
      await user.pointer({ keys: "[MouseRight]", target: row("env.json") });
      await user.pointer({ keys: "[MouseRight]", target: row("notes") });
      expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
    });

    it("F2 and Delete do nothing on env.json", async () => {
      const user = await mount();
      await user.click(row("env.json"));
      await user.keyboard("{F2}");
      await user.keyboard("{Delete}");
      expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
      expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    });

    it("a new file named env.json cannot be created in hittable/", async () => {
      const user = await mount();
      await user.pointer({ keys: "[MouseRight]", target: row("hittable") });
      await user.click(menuButton("New File"));
      await user.type(screen.getByPlaceholderText("File name"), "env.json{Enter}");
      expect(root.at("hittable/env.json")).toBeDefined();
      expect((root.at("hittable/env.json") as FakeFile).content).toBe("{}");
    });

    it("a file cannot be renamed to env.json in hittable/", async () => {
      root = new FakeDir("my-api", { hittable: { "env.json": '{"KEEP":"1"}', "a.json": "{}" } });
      const user = await mount();
      await user.pointer({ keys: "[MouseRight]", target: row("a.json") });
      await user.click(screen.getByRole("button", { name: "Rename" }));
      const input = screen.getByLabelText("Rename a.json");
      await user.clear(input);
      await user.type(input, "env.json{Enter}");
      expect(await screen.findByText(/reserved name/)).toBeInTheDocument();
      expect((root.at("hittable/env.json") as FakeFile).content).toBe('{"KEEP":"1"}');
      expect(root.at("hittable/a.json")).toBeDefined();
    });
  });

  it("New File at the root creates it and shows it in the tree", async () => {
    const user = await mount();
    await user.click(screen.getByRole("button", { name: "New File" }));
    await user.type(screen.getByPlaceholderText("File name"), "todo.txt{Enter}");
    await screen.findByRole("treeitem", { name: "todo.txt" });
    expect((root.at("todo.txt") as FakeFile).content).toBe("");
  });

  it("New Folder inside a folder creates it there and expands the parent", async () => {
    const user = await mount();
    await user.pointer({ keys: "[MouseRight]", target: row("users") });
    await user.click(menuButton("New Folder"));
    // the input lives inside the expanded parent
    await user.click(row("users"));
    await user.type(screen.getByPlaceholderText("Folder name"), "admin{Enter}");
    await screen.findByRole("treeitem", { name: "admin" });
    expect(root.at("hittable/users/admin")!.kind).toBe("directory");
    expect(item("users")).toHaveAttribute("aria-expanded", "true");
  });

  it("Escape or an empty name cancels creating", async () => {
    const user = await mount();
    await user.click(screen.getByRole("button", { name: "New File" }));
    await user.keyboard("{Escape}");
    expect(screen.queryByPlaceholderText("File name")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "New File" }));
    await user.keyboard("   {Enter}");
    expect([...root.children.keys()].sort()).toEqual(["README.md", "hittable"]);
  });

  it("creating a file whose name is taken shows an error and keeps the original", async () => {
    const user = await mount();
    await user.click(screen.getByRole("button", { name: "New File" }));
    await user.type(screen.getByPlaceholderText("File name"), "README.md{Enter}");
    expect(await screen.findByText("Could not create")).toBeInTheDocument();
    expect((root.at("README.md") as FakeFile).content).toBe("# readme");
  });

  it("renaming onto an existing name shows an error and loses nothing", async () => {
    const user = await mount();
    await user.click(row("users"));
    await user.pointer({ keys: "[MouseRight]", target: row("create.hit") });
    await user.click(screen.getByRole("button", { name: "Rename" }));
    const input = screen.getByLabelText("Rename create.hit");
    await user.clear(input);
    await user.type(input, "list.hit{Enter}");
    expect(await screen.findByText("Could not rename")).toBeInTheDocument();
    expect((root.at("hittable/users/list.hit") as FakeFile).content).toBe('{"method":"GET"}');
    expect((root.at("hittable/users/create.hit") as FakeFile).content).toBe('{"method":"POST"}');
  });

  it("Refresh picks up files created outside the app", async () => {
    const user = await mount();
    root.children.set("new.hit", new FakeFile("new.hit", "{}"));
    await user.click(screen.getByRole("button", { name: "Refresh" }));
    await screen.findByRole("treeitem", { name: "new.hit" });
  });

  it("a folder menu offers New File/New Folder; a file menu only Rename/Delete", async () => {
    const user = await mount();
    await user.pointer({ keys: "[MouseRight]", target: row("README.md") });
    const fileMenu = screen.getByRole("button", { name: "Rename" }).parentElement!;
    expect(within(fileMenu).queryByRole("button", { name: "New File" })).not.toBeInTheDocument();
    await user.keyboard("{Escape}");
    await user.click(fileMenu.previousElementSibling as HTMLElement); // backdrop closes it
    await user.pointer({ keys: "[MouseRight]", target: row("users") });
    expect(menuButton("New File")).toBeInTheDocument();
    expect(menuButton("New Folder")).toBeInTheDocument();
  });
});
