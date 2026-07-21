import {
  fileExists,
  directoryExists,
  createFile,
  createDirectory,
  readFileText,
  writeFileText,
} from "@/utils/workspace/fsHelpers";

const HITTABLE_DIR = "hittable";
const ENV_FILE = "env.json";
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

export async function validateHittable(
  root: FileSystemDirectoryHandle
): Promise<FileSystemDirectoryHandle> {
  const hittableDir = await root.getDirectoryHandle(HITTABLE_DIR);
  await scaffoldHittable(root);
  return hittableDir;
}

export async function getHittableDir(
  root: FileSystemDirectoryHandle
): Promise<FileSystemDirectoryHandle | null> {
  try {
    return await root.getDirectoryHandle(HITTABLE_DIR);
  } catch {
    return null;
  }
}

export async function readEnvFile(
  hittableDir: FileSystemDirectoryHandle
): Promise<Record<string, string>> {
  const handle = await hittableDir.getFileHandle(ENV_FILE);
  const content = await readFileText(handle);
  try {
    return JSON.parse(content);
  } catch {
    return {};
  }
}

export async function writeEnvFile(
  hittableDir: FileSystemDirectoryHandle,
  env: Record<string, string>
): Promise<void> {
  const handle = await hittableDir.getFileHandle(ENV_FILE, { create: true });
  await writeFileText(handle, JSON.stringify(env, null, 2) + "\n");
}

export async function readHitFile(
  handle: FileSystemFileHandle
): Promise<string> {
  return readFileText(handle);
}

export async function writeHitFile(
  handle: FileSystemFileHandle,
  content: string
): Promise<void> {
  await writeFileText(handle, content);
}

export async function readMarkdownFile(
  handle: FileSystemFileHandle
): Promise<string> {
  return readFileText(handle);
}

export async function writeMarkdownFile(
  handle: FileSystemFileHandle,
  content: string
): Promise<void> {
  await writeFileText(handle, content);
}
