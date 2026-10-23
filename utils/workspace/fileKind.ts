// What a file in the workspace is, decided the same way as the terminal app
// (shellapp/ui/screens/helpers_open.go): images by extension, binaries by a
// NUL byte in the first 8 KB, and nothing over 1 MiB goes into the editor.
// Anything that is not editable text is never written back, so an accidental
// keypress can't destroy it.

export const MAX_EDITABLE_BYTES = 1024 * 1024;

const IMAGE_EXTENSIONS = ["png", "jpg", "jpeg", "gif", "webp", "avif", "bmp", "ico"];

export type FileView =
  | { kind: "text" }
  | { kind: "image"; label: string }
  | { kind: "binary" }
  | { kind: "toolarge" };

const extensionOf = (name: string) => (name.includes(".") ? name.split(".").pop()!.toLowerCase() : "");

export const isSvg = (name: string) => extensionOf(name) === "svg";

export function humanSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export async function classifyFile(name: string, file: Blob): Promise<FileView> {
  const ext = extensionOf(name);
  if (IMAGE_EXTENSIONS.includes(ext)) {
    return { kind: "image", label: `${ext === "jpeg" ? "JPG" : ext.toUpperCase()} image` };
  }
  // Requests and env.json always open (a saved response can make a .hit big);
  // the limit is for other files, as in the terminal app.
  if (ext === "hit" || name === "env.json" || name === "env") return { kind: "text" };
  if (file.size > MAX_EDITABLE_BYTES) return { kind: "toolarge" };
  const head = new Uint8Array(await file.slice(0, 8192).arrayBuffer());
  return head.includes(0) ? { kind: "binary" } : { kind: "text" };
}
