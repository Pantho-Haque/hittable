// File uploads in a .hit body. The rules are shared with the terminal app;
// see docs/file-uploads.md and tests/unit/fixtures/fileBody.cases.json.

export type FileSpec = { path: string; type?: string; filename?: string };
export type MultipartPart = { name: string; value: string } | { name: string; file: FileSpec };
export type FileRefs =
  | { kind: "raw"; body: string }
  | { kind: "binary"; file: FileSpec }
  | { kind: "multipart"; parts: MultipartPart[] };

export const MULTIPART = "multipart/form-data";
export const isMultipart = (contentType: string) => contentType.trim().toLowerCase().startsWith(MULTIPART);

/** `path[;type=T][;filename=F]`: only trailing type/filename options are options. */
export function parseFileSpec(spec: string): FileSpec {
  let rest = spec;
  const out: { type?: string; filename?: string } = {};
  for (let m = /;(type|filename)=([^;]*)$/.exec(rest); m; m = /;(type|filename)=([^;]*)$/.exec(rest)) {
    if (out[m[1] as "type" | "filename"] === undefined) out[m[1] as "type" | "filename"] = m[2];
    rest = rest.slice(0, m.index);
  }
  const path = rest.trim();
  if (!path) throw new Error("A file field needs a path after @");
  return { path, ...out };
}

/** The file references in a body, or null when it is sent as it is. Throws on a malformed multipart body. */
export function parseFileRefs(body: string, contentType: string): FileRefs | null {
  if (isMultipart(contentType)) {
    if (!body.trim()) return { kind: "multipart", parts: [] };
    let obj: unknown;
    try {
      obj = JSON.parse(body);
    } catch {
      throw new Error("A multipart body is a JSON object of fields");
    }
    if (!obj || typeof obj !== "object" || Array.isArray(obj)) throw new Error("A multipart body is a JSON object of fields");
    return {
      kind: "multipart",
      parts: Object.entries(obj).map(([name, v]): MultipartPart => {
        if (typeof v !== "string") return { name, value: v === null ? "" : JSON.stringify(v) };
        if (v.startsWith("@@")) return { name, value: v.slice(1) };
        if (v.startsWith("@")) return { name, file: parseFileSpec(v.slice(1)) };
        return { name, value: v };
      }),
    };
  }
  const t = body.trim();
  if (t.length < 2 || !t.startsWith("@") || t.includes("\n")) return null;
  if (t.startsWith("@@")) return { kind: "raw", body: t.slice(1) };
  return { kind: "binary", file: parseFileSpec(t.slice(1)) };
}

/** Every file a body refers to (none for a malformed body). */
export function fileSpecsOf(refs: FileRefs | null): FileSpec[] {
  if (!refs || refs.kind === "raw") return [];
  if (refs.kind === "binary") return [refs.file];
  return refs.parts.flatMap((p) => ("file" in p ? [p.file] : []));
}

/**
 * Workspace path segments for a file a .hit refers to: relative to the .hit
 * file's folder, never above the workspace root, never absolute.
 */
export function resolveRefPath(hitPath: string[], path: string): string[] {
  const p = path.replace(/\\/g, "/");
  if (p.startsWith("/") || p.startsWith("~") || /^[A-Za-z]:\//.test(p)) {
    throw new Error(`${path}: absolute paths only work in the terminal app`);
  }
  const out = hitPath.slice(0, -1);
  for (const seg of p.split("/")) {
    if (!seg || seg === ".") continue;
    if (seg === "..") {
      if (!out.length) throw new Error(`${path}: the path leaves the workspace`);
      out.pop();
    } else out.push(seg);
  }
  if (!out.length) throw new Error(`${path}: not a file`);
  return out;
}

/** `path` written relative to the .hit file's folder, for storing in the body. */
export function relativeRefPath(hitPath: string[], target: string[]): string {
  const from = hitPath.slice(0, -1);
  let i = 0;
  while (i < from.length && i < target.length - 1 && from[i] === target[i]) i++;
  return [...Array(from.length - i).fill(".."), ...target.slice(i)].join("/");
}

/** The body text for a file spec, as stored in a multipart value or a binary body. */
export function fileSpecText(f: FileSpec): string {
  return `@${f.path}${f.type ? `;type=${f.type}` : ""}${f.filename ? `;filename=${f.filename}` : ""}`;
}

const MIME: Record<string, string> = {
  png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg", gif: "image/gif", webp: "image/webp", svg: "image/svg+xml",
  avif: "image/avif", ico: "image/x-icon", bmp: "image/bmp", pdf: "application/pdf", json: "application/json",
  txt: "text/plain", csv: "text/csv", html: "text/html", htm: "text/html", xml: "application/xml", md: "text/markdown",
  zip: "application/zip", gz: "application/gzip", tar: "application/x-tar", mp3: "audio/mpeg", wav: "audio/wav",
  mp4: "video/mp4", webm: "video/webm", mov: "video/quicktime", js: "text/javascript", css: "text/css",
  yaml: "application/yaml", yml: "application/yaml", doc: "application/msword",
  docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  xlsx: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
};

/** A content type from the file name; octet-stream when unknown. */
export function guessType(name: string): string {
  const ext = /\.([^./]+)$/.exec(name)?.[1]?.toLowerCase();
  return (ext && MIME[ext]) || "application/octet-stream";
}

export const baseName = (path: string) => path.replace(/\\/g, "/").split("/").filter(Boolean).pop() ?? path;
