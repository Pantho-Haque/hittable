import {
  fileExists,
  directoryExists,
  createFile,
  createDirectory,
  readFileText,
} from "@/utils/workspace/fsHelpers";
import { HITTABLE_DIR } from "./backend";

// Folder-only setup: a GitHub repo is initialised through its own staged changes.
const ENV_FILE = "env.json";
// FR-17: listed in hittable/.gitignore so it is never committed.
const LOCAL_ENV_FILE = "env.local.json";
const GITIGNORE = ".gitignore";
const NOTES_DIR = "notes";
const EMPTY_ENV = "{}";

const SAMPLE_HIT_CONTENT = JSON.stringify({
  method: "GET",
  url: "https://jsonplaceholder.typicode.com/todos/1",
  headers: {},
  params: {},
  body: "",
  response: null,
}, null, 2) + "\n";

const SAMPLE_MARKDOWN_CONTENT = `# Sample Note

This is a sample markdown note in your Hittable workspace.

## Features

- Edit in plain text mode
- Switch to Runner mode for .hit files
- Environment variables with \`<<KEY>>\` syntax
- Auto-save to disk

## Environment Variables

Use \`<<KEY>>\` placeholders in your requests. Set values in the \`env.json\` file.
`;

export async function pickFolder(): Promise<FileSystemDirectoryHandle> {
  return window.showDirectoryPicker({ mode: "readwrite" });
}

export async function scaffoldHittable(
  root: FileSystemDirectoryHandle
): Promise<FileSystemDirectoryHandle> {
  const hittableDir = await root.getDirectoryHandle(HITTABLE_DIR, {
    create: true,
  });

  if (!(await fileExists(hittableDir, ENV_FILE))) {
    await createFile(hittableDir, ENV_FILE, EMPTY_ENV);
  }

  await ensureLocalEnvIgnored(hittableDir);

  if (!(await directoryExists(hittableDir, NOTES_DIR))) {
    await createDirectory(hittableDir, NOTES_DIR);
    const notesDir = await hittableDir.getDirectoryHandle(NOTES_DIR);
    await createFile(notesDir, "sample.md", SAMPLE_MARKDOWN_CONTENT);
  }

  if (!(await directoryExists(hittableDir, "testcollection"))) {
    const testDir = await hittableDir.getDirectoryHandle("testcollection", { create: true });
    await createFile(testDir, "test.hit", SAMPLE_HIT_CONTENT);
  }

  return hittableDir;
}

/**
 * Adds env.local.json to hittable/.gitignore unless a line already names it.
 * Appends; never rewrites what the user has there.
 */
export async function ensureLocalEnvIgnored(hittableDir: FileSystemDirectoryHandle): Promise<void> {
  try {
    let current = "";
    if (await fileExists(hittableDir, GITIGNORE)) {
      current = await readFileText(await hittableDir.getFileHandle(GITIGNORE));
    }
    const listed = current.split(/\r?\n/).some((l) => l.trim().replace(/^\//, "") === LOCAL_ENV_FILE);
    if (listed) return;
    const sep = current && !current.endsWith("\n") ? "\n" : "";
    await createFile(hittableDir, GITIGNORE, `${current}${sep}${LOCAL_ENV_FILE}\n`);
  } catch (err) {
    // A read-only folder still opens; it just cannot be protected.
    console.error("Could not update hittable/.gitignore:", err);
  }
}
