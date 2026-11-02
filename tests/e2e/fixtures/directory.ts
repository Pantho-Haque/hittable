import type { Page } from "@playwright/test";
import { expect } from "@playwright/test";

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

// A file the app is writing is swapped in when the write closes, and reads
// as missing (or unreadable) for that moment; such a read is retried briefly.
export const readFile = (page: Page, path: string) =>
  page.evaluate(async (p) => {
    for (let attempt = 0; ; attempt++) {
      try {
        let d = await (await navigator.storage.getDirectory()).getDirectoryHandle("project");
        const parts = p.split("/");
        for (const part of parts.slice(0, -1)) d = await d.getDirectoryHandle(part);
        return await (await (await d.getFileHandle(parts.at(-1)!)).getFile()).text();
      } catch (err) {
        // Mid-swap the file reads as missing, or (Chromium) as not readable.
        const name = (err as DOMException).name;
        if ((name !== "NotFoundError" && name !== "NotReadableError") || attempt >= 10) throw err;
        await new Promise((r) => setTimeout(r, 50));
      }
    }
  }, path);

export const tree = (page: Page) => page.getByRole("tree", { name: "Workspace files" });
export const node = (page: Page, path: string) => tree(page).getByTitle(path, { exact: true });

/** Open a folder in the explorer unless it already is. */
export async function expand(page: Page, path: string) {
  const item = tree(page).getByRole("treeitem", { name: path.split("/").at(-1), exact: true });
  if ((await item.first().getAttribute("aria-expanded")) !== "true") await node(page, path).click();
  await expect(item.first()).toHaveAttribute("aria-expanded", "true");
}

/** Open a file from the explorer, expanding the folders above it. */
export async function openFile(page: Page, path: string) {
  const parts = path.split("/");
  for (let i = 1; i < parts.length; i++) await expand(page, parts.slice(0, i).join("/"));
  await node(page, path).click();
}

/**
 * Go to the launcher and wait until it is interactive: the top bar's sign-in
 * link only renders client-side, so clicks and shortcuts after it are heard.
 */
export async function openLauncher(page: Page) {
  await page.goto("/hittable");
  await expect(page.getByRole("banner").getByRole("link", { name: /^Sign in/ })).toBeVisible();
}

export async function openWorkspace(page: Page) {
  await openLauncher(page);
  await page.getByRole("button", { name: "Open a local folder" }).click();
  await expect(page).toHaveURL(/\/hittable\/folder$/);
  await expect(tree(page)).toBeVisible();
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
