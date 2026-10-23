import type { Page } from "@playwright/test";
import { expect } from "./test";

// Directory mode needs a real folder from the OS picker. The Origin Private
// File System gives real FileSystemDirectoryHandles, so the picker is pointed
// at a folder there and everything else is the app's own code.
export async function seedPrivateFolder(page: Page, files: Record<string, string> = {}) {
  await page.addInitScript((seed) => {
    const root = async () => (await navigator.storage.getDirectory()).getDirectoryHandle("project", { create: true });
    (window as unknown as { showDirectoryPicker: () => Promise<FileSystemDirectoryHandle> }).showDirectoryPicker = async () => {
      const dir = await root();
      for (const [path, content] of Object.entries(seed)) {
        const parts = path.split("/");
        let d = dir;
        for (const p of parts.slice(0, -1)) d = await d.getDirectoryHandle(p, { create: true });
        const f = await (await d.getFileHandle(parts.at(-1)!, { create: true })).createWritable();
        // "base64:..." seeds binary files (images and the like) byte for byte.
        await f.write(content.startsWith("base64:") ? Uint8Array.from(atob(content.slice(7)), (c) => c.charCodeAt(0)) : content);
        await f.close();
      }
      return dir;
    };
  }, files);
}

export const readFile = (page: Page, path: string) =>
  page.evaluate(async (p) => {
    let d = await (await navigator.storage.getDirectory()).getDirectoryHandle("project");
    const parts = p.split("/");
    for (const part of parts.slice(0, -1)) d = await d.getDirectoryHandle(part);
    return (await (await d.getFileHandle(parts.at(-1)!)).getFile()).text();
  }, path);

export const tree = (page: Page) => page.getByRole("tree", { name: "Workspace files" });
export const node = (page: Page, path: string) => tree(page).getByTitle(path, { exact: true });

/** Open a folder in the explorer unless it already is. */
export async function expand(page: Page, path: string) {
  const item = tree(page).getByRole("treeitem", { name: path.split("/").at(-1), exact: true });
  if ((await item.first().getAttribute("aria-expanded")) !== "true") await node(page, path).click();
  await expect(item.first()).toHaveAttribute("aria-expanded", "true");
}

export async function openWorkspace(page: Page) {
  await page.goto("/hittable");
  await page.getByRole("button", { name: "Open a directory workspace" }).click();
  await expect(tree(page)).toBeVisible();
  await expect(page.getByRole("button", { name: "Switch to local workspace" })).toHaveText("Directory");
}

/** A file's exact bytes, base64-encoded, to prove nothing rewrote it. */
export const readBytes = (page: Page, path: string) =>
  page.evaluate(async (p) => {
    let d = await (await navigator.storage.getDirectory()).getDirectoryHandle("project");
    const parts = p.split("/");
    for (const part of parts.slice(0, -1)) d = await d.getDirectoryHandle(part);
    const buf = new Uint8Array(await (await (await d.getFileHandle(parts.at(-1)!)).getFile()).arrayBuffer());
    return btoa(String.fromCharCode(...buf));
  }, path);
