import type { Page } from "@playwright/test";
import { test, expect, dialog } from "./fixtures/test";

async function openNotes(page: Page) {
  await page.goto("/hittable");
  await page.getByRole("button", { name: "Notes" }).click();
  const nb = dialog(page, "Notebook");
  await expect(nb).toBeVisible();
  return nb;
}

const source = (page: Page) => dialog(page, "Notebook").getByRole("textbox", { name: "Markdown source" });
const preview = (page: Page) => dialog(page, "Notebook").locator(".note-preview").first();

test.describe("notes", () => {
  test("the sample notes are there on first run", async ({ page }) => {
    const nb = await openNotes(page);
    await expect(nb.getByText("micro", { exact: true })).toBeVisible();
    await expect(nb.getByText("nano", { exact: true })).toBeVisible();
    await expect(nb.getByText("Select a note")).toBeVisible();
  });

  test("create, rename, write markdown, preview, save, and find it after reload", async ({ page, seed }) => {
    await seed({ notes: {} });
    const nb = await openNotes(page);
    await expect(nb.getByText("No notes yet")).toBeVisible();
    await nb.getByRole("button", { name: "New note" }).click();
    const rename = nb.locator("input:not([placeholder])");
    await expect(rename).toBeFocused();
    await expect(rename).toHaveValue("untitled");
    await rename.fill("API checklist");
    await rename.press("Enter");
    await expect(nb.getByText("API checklist").first()).toBeVisible();

    await nb.getByRole("button", { name: "Switch to edit" }).click();
    await source(page).fill("# Release\n\n- **auth** works\n- `GET /users`\n\nline one\nline two");
    await expect(nb.getByText("unsaved")).toBeVisible();
    await expect(nb.getByText(/\d+ chars · 12 words/)).toBeVisible();

    await nb.getByRole("button", { name: "Switch to preview" }).click();
    await expect(preview(page).getByRole("heading", { level: 1, name: "Release" })).toBeVisible();
    await expect(preview(page).locator("strong")).toHaveText("auth");
    await expect(preview(page).locator("code")).toHaveText("GET /users");
    await expect(preview(page).getByRole("listitem")).toHaveCount(2);
    // GitHub-flavoured with line breaks kept.
    await expect(preview(page).locator("br")).toHaveCount(1);

    await page.keyboard.press("ControlOrMeta+s");
    await expect(nb.getByText("unsaved")).toBeHidden();
    await nb.getByRole("button", { name: "Close" }).last().click();
    await expect(nb).toBeHidden();

    await page.reload();
    const again = await openNotes(page);
    await again.getByText("API checklist").first().click();
    await expect(preview(page).getByRole("heading", { name: "Release" })).toBeVisible();
  });

  test("split view shows source and preview together", async ({ page }) => {
    const nb = await openNotes(page);
    await nb.getByText("micro", { exact: true }).click();
    await nb.getByRole("button", { name: "Split", exact: true }).click();
    await expect(nb.getByRole("button", { name: "Split", exact: true })).toHaveAttribute("aria-pressed", "true");
    await source(page).fill("## Live");
    await expect(preview(page).getByRole("heading", { level: 2, name: "Live" })).toBeVisible();
    await expect(nb.getByText("Split mode")).toBeVisible();
  });

  test("Escape with unsaved edits asks: cancel, save & close, or discard", async ({ page }) => {
    let nb = await openNotes(page);
    await nb.getByText("nano", { exact: true }).click();
    await nb.getByRole("button", { name: "Switch to edit" }).click();
    await source(page).fill("first edit");
    await page.keyboard.press("Escape");
    const ask = dialog(page, "Unsaved Changes");
    await expect(ask).toBeVisible();
    await ask.getByRole("button", { name: "Cancel" }).click();
    await expect(ask).toBeHidden();
    await expect(source(page)).toHaveValue("first edit");

    await page.keyboard.press("Escape");
    await dialog(page, "Unsaved Changes").getByRole("button", { name: "Save & Close" }).click();
    await expect(nb).toBeHidden();

    await page.getByRole("button", { name: "Notes" }).click();
    nb = dialog(page, "Notebook");
    await nb.getByText("nano", { exact: true }).click();
    await expect(preview(page)).toHaveText("first edit");
    await nb.getByRole("button", { name: "Switch to edit" }).click();
    await source(page).fill("thrown away");
    await page.keyboard.press("Escape");
    await dialog(page, "Unsaved Changes").getByRole("button", { name: "Discard" }).click();
    await expect(nb).toBeHidden();
    await page.getByRole("button", { name: "Notes" }).click();
    await dialog(page, "Notebook").getByText("nano", { exact: true }).click();
    await expect(preview(page)).toHaveText("first edit");
  });

  test("switching notes with unsaved edits asks first; delete asks too", async ({ page, dialogs }) => {
    const nb = await openNotes(page);
    await nb.getByText("micro", { exact: true }).click();
    await nb.getByRole("button", { name: "Switch to edit" }).click();
    await source(page).fill("unsaved micro");
    await nb.getByText("nano", { exact: true }).click();
    expect(dialogs).toContain("confirm: You have unsaved changes. Discard?");
    await expect(nb.getByText("nano").first()).toBeVisible();

    await nb.getByRole("button", { name: "Delete" }).click();
    expect(dialogs).toContain("confirm: Delete this note?");
    await expect(nb.getByText("nano", { exact: true })).toHaveCount(0);
    await expect(nb.getByText("Select a note")).toBeVisible();
  });

  test("search filters notes by title", async ({ page }) => {
    const nb = await openNotes(page);
    await nb.getByPlaceholder("Search notes...").fill("mic");
    await expect(nb.getByText("micro", { exact: true })).toBeVisible();
    await expect(nb.getByText("nano", { exact: true })).toHaveCount(0);
    await nb.getByPlaceholder("Search notes...").fill("zzz");
    await expect(nb.getByText("No matching notes")).toBeVisible();
  });

  test("HTML in a note is sanitized: no script runs", async ({ page, seed, dialogs }) => {
    await seed({
      notes: {
        "1": {
          title: "evil",
          content:
            '# Hi\n\n<script>window.__noteXss = "script"; alert(1)</script>\n<img src="x" onerror="window.__noteXss = \'onerror\'; alert(2)">\n<a href="javascript:alert(3)">click</a>\n<iframe src="javascript:alert(4)"></iframe>',
        },
      },
    });
    const nb = await openNotes(page);
    await nb.getByText("evil", { exact: true }).click();
    await expect(preview(page).getByRole("heading", { name: "Hi" })).toBeVisible();
    await expect(preview(page).locator("script, iframe")).toHaveCount(0);
    await expect(preview(page).locator("img")).not.toHaveAttribute("onerror", /.*/);
    const link = preview(page).getByText("click");
    if (await link.count()) await expect(link).not.toHaveAttribute("href", /javascript:/);
    await nb.getByRole("button", { name: "Split", exact: true }).click();
    await nb.getByRole("button", { name: "Split", exact: true }).click();
    expect(await page.evaluate(() => (window as unknown as { __noteXss?: string }).__noteXss)).toBeUndefined();
    expect(dialogs).toEqual([]);
  });
});
