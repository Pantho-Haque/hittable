// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import NoteModal from "@/components/modals/NoteModal";
import type { NotesStore } from "@/types";

const seed: NotesStore = {
  "1": { title: "auth flow", content: "# Login\nPOST /login" },
  "2": { title: "billing", content: "Stripe webhooks" },
};
const saved = (): NotesStore => JSON.parse(localStorage.getItem("notesStore") ?? "{}");

function setup() {
  localStorage.setItem("notesStore", JSON.stringify(seed));
  render(<NoteModal />);
  return userEvent.setup();
}

const notebook = () => screen.getByRole("dialog", { name: "Notebook" });

async function openNote(user: ReturnType<typeof userEvent.setup>, title: string) {
  await user.click(screen.getByTitle("Notes"));
  await user.click(within(notebook()).getByText(title));
  await user.click(screen.getByRole("button", { name: "Switch to edit" }));
  return screen.getByRole("textbox", { name: "Markdown source" });
}

beforeEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

describe("NoteModal", () => {
  it("lists notes and filters them by title", async () => {
    const user = setup();
    await user.click(screen.getByTitle("Notes"));
    const nb = within(notebook());
    expect(nb.getByText("auth flow")).toBeInTheDocument();
    expect(nb.getByText("billing")).toBeInTheDocument();
    await user.type(nb.getByPlaceholderText("Search notes..."), "BILL");
    expect(nb.queryByText("auth flow")).not.toBeInTheDocument();
    expect(nb.getByText("billing")).toBeInTheDocument();
    await user.clear(nb.getByPlaceholderText("Search notes..."));
    await user.type(nb.getByPlaceholderText("Search notes..."), "zzz");
    expect(nb.getByText("No matching notes")).toBeInTheDocument();
  });

  it("saves an edit with Ctrl/Cmd+S and shows word and character counts", async () => {
    const user = setup();
    const editor = await openNote(user, "billing");
    expect(editor).toHaveValue("Stripe webhooks");
    await user.clear(editor);
    await user.type(editor, "three small words");
    expect(screen.getByText("17 chars · 3 words")).toBeInTheDocument();
    expect(within(notebook()).getByRole("button", { name: "Save" })).toBeInTheDocument();
    await user.keyboard("{Control>}s{/Control}");
    expect(saved()["2"].content).toBe("three small words");
    // The footer button flips back from "Save" to "Close" once saved.
    expect(within(notebook()).queryByRole("button", { name: "Save" })).not.toBeInTheDocument();
  });

  it("saves with the footer Save button", async () => {
    const user = setup();
    const editor = await openNote(user, "billing");
    await user.type(editor, "!");
    await user.click(within(notebook()).getByRole("button", { name: "Save" }));
    expect(saved()["2"].content).toBe("Stripe webhooks!");
  });

  it("asks before closing with unsaved edits, and Discard throws them away", async () => {
    const user = setup();
    const editor = await openNote(user, "billing");
    await user.type(editor, " draft");
    await user.keyboard("{Escape}");
    expect(screen.getByRole("dialog", { name: "Unsaved Changes" })).toBeInTheDocument();
    expect(notebook()).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(saved()["2"].content).toBe("Stripe webhooks");

    const reopened = await openNote(user, "billing");
    expect(reopened).toHaveValue("Stripe webhooks");
  });

  it("Save & Close persists the edit", async () => {
    const user = setup();
    const editor = await openNote(user, "auth flow");
    await user.type(editor, "\nGET /me");
    await user.click(screen.getByTitle("Close"));
    await user.click(screen.getByRole("button", { name: "Save & Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(saved()["1"].content).toBe("# Login\nPOST /login\nGET /me");
  });

  it("Cancel in the prompt returns to the editor with the edit intact", async () => {
    const user = setup();
    const editor = await openNote(user, "billing");
    await user.type(editor, " draft");
    await user.keyboard("{Escape}");
    await user.click(within(screen.getByRole("dialog", { name: "Unsaved Changes" })).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog", { name: "Unsaved Changes" })).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Markdown source" })).toHaveValue("Stripe webhooks draft");
  });

  it("closes straight away on Escape when nothing changed", async () => {
    const user = setup();
    await openNote(user, "billing");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("creates a note, renames it, and deletes it after confirmation", async () => {
    const user = setup();
    await user.click(screen.getByTitle("Notes"));
    await user.click(screen.getByRole("button", { name: "New note" }));
    const rename = await screen.findByDisplayValue("untitled");
    await user.clear(rename);
    await user.type(rename, "  todo  {Enter}");
    const id = Object.keys(saved()).find((k) => !(k in seed))!;
    expect(saved()[id]).toEqual({ title: "todo", content: "" });

    const confirm = vi.spyOn(window, "confirm").mockReturnValueOnce(false).mockReturnValueOnce(true);
    await user.click(screen.getByTitle("Delete"));
    expect(saved()[id]).toBeDefined();
    await user.click(screen.getByTitle("Delete"));
    expect(confirm).toHaveBeenCalledTimes(2);
    expect(saved()[id]).toBeUndefined();
    expect(within(notebook()).queryByText("todo")).not.toBeInTheDocument();
  });

  it("asks before a new note replaces unsaved edits", async () => {
    const user = setup();
    const editor = await openNote(user, "billing");
    await user.type(editor, " draft");
    vi.spyOn(window, "confirm").mockReturnValue(false);
    await user.click(screen.getByRole("button", { name: "New note" }));
    expect(Object.keys(saved())).toEqual(["1", "2"]);
    expect(screen.getByRole("textbox", { name: "Markdown source" })).toHaveValue("Stripe webhooks draft");
  });

  it("refuses to rename a note to a blank title", async () => {
    const user = setup();
    await user.click(screen.getByTitle("Notes"));
    await user.click(within(notebook()).getByText("billing"));
    await user.click(screen.getByTitle("Rename"));
    const input = screen.getByDisplayValue("billing");
    await user.clear(input);
    await user.type(input, "   {Enter}");
    expect(saved()["2"].title).toBe("billing");
  });

  it("asks before switching notes with unsaved edits", async () => {
    const user = setup();
    const editor = await openNote(user, "billing");
    await user.type(editor, " draft");
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
    await user.click(within(notebook()).getByText("auth flow"));
    expect(confirm).toHaveBeenCalledWith("You have unsaved changes. Discard?");
    expect(screen.getByRole("textbox", { name: "Markdown source" })).toHaveValue("Stripe webhooks draft");
  });
});
