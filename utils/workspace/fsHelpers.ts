export async function getFileHandle(
  dir: FileSystemDirectoryHandle,
  name: string
): Promise<FileSystemFileHandle | null> {
  try {
    return await dir.getFileHandle(name);
  } catch {
    return null;
  }
}

export async function getDirectoryHandle(
  dir: FileSystemDirectoryHandle,
  name: string
): Promise<FileSystemDirectoryHandle | null> {
  try {
    return await dir.getDirectoryHandle(name);
  } catch {
    return null;
  }
}

export async function fileExists(
  dir: FileSystemDirectoryHandle,
  name: string
): Promise<boolean> {
  try {
    await dir.getFileHandle(name);
    return true;
  } catch {
    return false;
  }
}

export async function directoryExists(
  dir: FileSystemDirectoryHandle,
  name: string
): Promise<boolean> {
  try {
    await dir.getDirectoryHandle(name);
    return true;
  } catch {
    return false;
  }
}

export async function readFileText(
  handle: FileSystemFileHandle
): Promise<string> {
  const file = await handle.getFile();
  return file.text();
}

export async function writeFileText(
  handle: FileSystemFileHandle,
  content: string
): Promise<void> {
  const writable = await handle.createWritable();
  await writable.write(content);
  await writable.close();
}

export async function createFile(
  dir: FileSystemDirectoryHandle,
  name: string,
  content: string = ""
): Promise<FileSystemFileHandle> {
  const handle = await dir.getFileHandle(name, { create: true });
  await writeFileText(handle, content);
  return handle;
}

export async function createDirectory(
  dir: FileSystemDirectoryHandle,
  name: string
): Promise<FileSystemDirectoryHandle> {
  return dir.getDirectoryHandle(name, { create: true });
}

export async function removeEntry(
  dir: FileSystemDirectoryHandle,
  name: string
): Promise<void> {
  await dir.removeEntry(name);
}

export async function renameEntry(
  dir: FileSystemDirectoryHandle,
  oldName: string,
  newName: string,
  kind: "file" | "directory"
): Promise<void> {
  if (kind === "file") {
    const oldHandle = await dir.getFileHandle(oldName);
    const content = await readFileText(oldHandle);
    await removeEntry(dir, oldName);
    await createFile(dir, newName, content);
  } else {
    const oldDir = await dir.getDirectoryHandle(oldName);
    const newDir = await createDirectory(dir, newName);
    await copyDirectoryContents(oldDir, newDir);
    await removeEntry(dir, oldName);
  }
}

async function copyDirectoryContents(
  src: FileSystemDirectoryHandle,
  dest: FileSystemDirectoryHandle
): Promise<void> {
  for await (const [name, handle] of src.entries()) {
    if (handle.kind === "file") {
      const content = await readFileText(handle as FileSystemFileHandle);
      await createFile(dest, name, content);
    } else {
      const newSubDir = await createDirectory(dest, name);
      await copyDirectoryContents(
        handle as FileSystemDirectoryHandle,
        newSubDir
      );
    }
  }
}

export async function listDirectory(
  dir: FileSystemDirectoryHandle
): Promise<{ name: string; kind: "file" | "directory" }[]> {
  const entries: { name: string; kind: "file" | "directory" }[] = [];
  for await (const [name, handle] of dir.entries()) {
    entries.push({ name, kind: handle.kind as "file" | "directory" });
  }
  return entries;
}
