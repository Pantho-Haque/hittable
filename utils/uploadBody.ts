// Turns a resolved .hit body with file references (`@path`) into the bytes to
// send and the Content-Type to send them with. See docs/file-uploads.md.

import { baseName, guessType, parseFileRefs, resolveRefPath, type FileSpec } from "./fileBody";

export type ReadFile = (path: string[]) => Promise<Uint8Array>;
export type UploadBody =
  | { kind: "text"; body: string }
  | { kind: "bytes"; bytes: Uint8Array<ArrayBuffer>; headers: Record<string, string> };

const ctKey = (h: Record<string, string>) => Object.keys(h).find((k) => k.toLowerCase() === "content-type");

/** Headers with one Content-Type, keeping the user's spelling of the key. */
function withContentType(h: Record<string, string>, value: string): Record<string, string> {
  const key = ctKey(h) ?? "Content-Type";
  return { ...h, [key]: value };
}

async function read(readFile: ReadFile | undefined, hitPath: string[] | undefined, f: FileSpec) {
  if (!readFile || !hitPath) throw new Error(`${f.path}: files can only be sent from a request in a workspace`);
  const at = resolveRefPath(hitPath, f.path);
  try {
    return await readFile(at);
  } catch (err) {
    const e = err as Error & { code?: string };
    // Folder: NotFoundError (or TypeMismatchError for a folder); GitHub: code "not_found".
    const missing = e?.name === "NotFoundError" || e?.name === "TypeMismatchError" || e?.code === "not_found";
    throw new Error(missing ? `File not found: ${f.path}` : `Can't read ${f.path}: ${e?.message || String(err)}`);
  }
}

/** True when the body refers to files or needs unescaping (checked without awaiting). */
export function needsUploadBody(body: string, headers: Record<string, string>): boolean {
  const key = ctKey(headers);
  try {
    return parseFileRefs(body ?? "", key ? headers[key] : "") !== null;
  } catch {
    return true; // let buildUploadBody report the malformed body
  }
}

/**
 * The body to send for a resolved request, or null when it is plain text sent as is.
 * Throws with a message naming the file when one can't be read.
 */
export async function buildUploadBody(
  body: string,
  headers: Record<string, string>,
  ctx: { hitPath?: string[]; readFile?: ReadFile },
): Promise<UploadBody | null> {
  const key = ctKey(headers);
  const refs = parseFileRefs(body ?? "", key ? headers[key] : "");
  if (!refs) return null;
  if (refs.kind === "raw") return { kind: "text", body: refs.body };

  if (refs.kind === "binary") {
    const bytes = await read(ctx.readFile, ctx.hitPath, refs.file);
    const type = (key && headers[key]) || refs.file.type || guessType(refs.file.path);
    return { kind: "bytes", bytes: new Uint8Array(bytes), headers: withContentType(headers, type) };
  }

  const form = new FormData();
  for (const p of refs.parts) {
    if ("value" in p) form.append(p.name, p.value);
    else {
      const bytes = await read(ctx.readFile, ctx.hitPath, p.file);
      const type = p.file.type || guessType(p.file.filename ?? p.file.path);
      form.append(p.name, new Blob([new Uint8Array(bytes)], { type }), p.file.filename ?? baseName(p.file.path));
    }
  }
  // The platform writes the multipart body and picks the boundary.
  const encoded = new Response(form);
  const bytes = new Uint8Array(await encoded.arrayBuffer());
  return { kind: "bytes", bytes, headers: withContentType(headers, encoded.headers.get("content-type")!) };
}

export function bytesToBase64(bytes: Uint8Array): string {
  let bin = "";
  for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(bin);
}

export const formatBytes = (n: number) =>
  n >= 1024 * 1024 ? `${(n / 1024 / 1024).toFixed(1)} MB` : n >= 1024 ? `${(n / 1024).toFixed(1)} KB` : `${n} B`;
