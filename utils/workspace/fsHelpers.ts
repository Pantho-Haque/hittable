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
  content: string | Uint8Array<ArrayBuffer>
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
  // A non-empty folder is refused without `recursive`.
  await dir.removeEntry(name, { recursive: true });
}

/** Moves an entry, possibly into another directory, copying then deleting. */
export async function moveEntry(
  srcDir: FileSystemDirectoryHandle,
  oldName: string,
  destDir: FileSystemDirectoryHandle,
  newName: string
): Promise<void> {
  // The copy would overwrite the target (or, on a case-insensitive disk, be
  // the source itself and get deleted), so an existing name is refused.
  if ((await fileExists(destDir, newName)) || (await directoryExists(destDir, newName))) {
    throw new Error(`"${newName}" already exists`);
  }
  if (await fileExists(srcDir, oldName)) {
    // Write the copy before removing the original, so a failure loses nothing.
    await copyFile(await srcDir.getFileHandle(oldName), destDir, newName);
  } else {
    const oldDir = await srcDir.getDirectoryHandle(oldName);
    await copyDirectoryContents(oldDir, await createDirectory(destDir, newName));
  }
  await removeEntry(srcDir, oldName);
}

// Copies bytes, not text, so images and other binary files survive.
async function copyFile(
  src: FileSystemFileHandle,
  dest: FileSystemDirectoryHandle,
  name: string
): Promise<void> {
  const writable = await (await dest.getFileHandle(name, { create: true })).createWritable();
  await writable.write(await src.getFile());
  await writable.close();
}

async function copyDirectoryContents(
  src: FileSystemDirectoryHandle,
  dest: FileSystemDirectoryHandle
): Promise<void> {
  for await (const [name, handle] of src.entries()) {
    if (handle.kind === "file") {
      await copyFile(handle as FileSystemFileHandle, dest, name);
    } else {
      const newSubDir = await createDirectory(dest, name);
      await copyDirectoryContents(
        handle as FileSystemDirectoryHandle,
        newSubDir
      );
    }
  }
}
