import { THittableCurlJson } from "@/types";
import { resolveEnv } from "./resolveEnv";
import { baseName, guessType, parseFileRefs, type FileRefs, type FileSpec } from "./fileBody";

export type CodeLanguage = "curl" | "fetch" | "python" | "httpie";

export const CODE_LANGUAGES: { id: CodeLanguage; label: string }[] = [
  { id: "curl", label: "cURL" },
  { id: "fetch", label: "JavaScript fetch" },
  { id: "python", label: "Python requests" },
  { id: "httpie", label: "HTTPie" },
];

// Single-quotes a word for a POSIX shell; $, ` and \ stay literal.
const sh = (s: string) => `'${s.replace(/'/g, `'\\''`)}'`;
const str = (s: string) => JSON.stringify(s);

function parseMap(s: string): Record<string, string> {
  try {
    const o = JSON.parse(s || "{}");
    if (!o || typeof o !== "object" || Array.isArray(o)) return {};
    return Object.fromEntries(Object.entries(o).map(([k, v]) => [k, String(v)]));
  } catch {
    return {};
  }
}

function parseJson(s: string): { ok: true; value: unknown } | { ok: false } {
  try {
    return { ok: true, value: JSON.parse(s) };
  } catch {
    return { ok: false };
  }
}

// Python literal for a parsed JSON value. JSON string escapes are valid Python.
function toPy(v: unknown, pad = ""): string {
  if (v === null) return "None";
  if (v === true) return "True";
  if (v === false) return "False";
  if (typeof v === "string") return str(v);
  if (typeof v !== "object") return String(v);
  const inner = pad + "    ";
  if (Array.isArray(v)) {
    return v.length ? `[\n${v.map((x) => inner + toPy(x, inner)).join(",\n")},\n${pad}]` : "[]";
  }
  const entries = Object.entries(v);
  return entries.length
    ? `{\n${entries.map(([k, x]) => `${inner}${str(k)}: ${toPy(x, inner)}`).join(",\n")},\n${pad}}`
    : "{}";
}

export function generateCode(lang: CodeLanguage, req: THittableCurlJson, vars?: Record<string, string>): string {
  const r = vars ? resolveEnv(req, vars) : req;
  const method = (r.method || "GET").toUpperCase();
  const url = r.url;
  const headers = parseMap(r.headers);
  let body = method === "GET" || method === "HEAD" ? "" : r.body ?? "";
  const contentType = Object.entries(headers).find(([k]) => k.toLowerCase() === "content-type")?.[1] ?? "";
  let refs: FileRefs | null = null;
  try {
    refs = body ? parseFileRefs(body, contentType) : null;
  } catch {
    refs = null; // a malformed multipart body is shown as the text it is
  }
  if (refs?.kind === "raw") {
    body = refs.body;
    refs = null;
  }
  if (refs) return uploadCode(lang, method, url, headers, refs);
  const parsed = body ? parseJson(body) : { ok: false as const };
  // ponytail: JSON.parse rounds integers past 2^53; emit the raw text if that ever matters.
  const json = /json/i.test(contentType) && parsed.ok ? parsed.value : undefined;
  const hasJson = json !== undefined;

  switch (lang) {
    case "curl": {
      const parts = [`curl ${method === "HEAD" ? "-I" : `-X ${method}`} ${sh(url)}`];
      for (const [k, v] of Object.entries(headers)) parts.push(`-H ${sh(`${k}: ${v}`)}`);
      if (body) parts.push(`--data-raw ${sh(body)}`);
      return parts.join(" \\\n  ");
    }
    case "httpie": {
      // `Name;` sends an empty header; `Name:` would remove it.
      const parts = [`http ${method} ${sh(url)}`];
      for (const [k, v] of Object.entries(headers)) parts.push(sh(v === "" ? `${k};` : `${k}:${v}`));
      if (body) parts.push(`--raw ${sh(body)}`);
      return parts.join(" \\\n  ");
    }
    case "fetch": {
      const opts = [`  method: ${str(method)},`];
      if (Object.keys(headers).length) opts.push(`  headers: ${JSON.stringify(headers, null, 2).replace(/\n/g, "\n  ")},`);
      if (body) {
        opts.push(
          hasJson
            ? `  body: JSON.stringify(${JSON.stringify(json, null, 2).replace(/\n/g, "\n  ")}),`
            : `  body: ${str(body)},`,
        );
      }
      return [
        `const response = await fetch(${str(url)}, {`,
        ...opts,
        `});`,
        `console.log(response.status, await response.text());`,
      ].join("\n");
    }
    case "python": {
      const lines = ["import requests", "", `url = ${str(url)}`];
      const args = ["url"];
      if (Object.keys(headers).length) {
        lines.push(`headers = ${toPy(headers)}`);
        args.push("headers=headers");
      }
      if (body) {
        lines.push(hasJson ? `payload = ${toPy(json)}` : `payload = ${str(body)}`);
        args.push(hasJson ? "json=payload" : "data=payload");
      }
      lines.push(
        "",
        `response = requests.request(${str(method)}, ${args.join(", ")})`,
        "print(response.status_code)",
        "print(response.text)",
      );
      return lines.join("\n");
    }
  }
}

// Copies the generated snippet; for the Ctrl/Cmd+Shift+C binding and the menu.
export async function copyAs(lang: CodeLanguage, req: THittableCurlJson, vars?: Record<string, string>): Promise<string> {
  const code = generateCode(lang, req, vars);
  await navigator.clipboard.writeText(code);
  return code;
}

const fileType = (f: FileSpec) => f.type || guessType(f.filename ?? f.path);
const fileName = (f: FileSpec) => f.filename ?? baseName(f.path);

/** Snippets for a request that uploads files; paths are relative to the request's folder. */
function uploadCode(lang: CodeLanguage, method: string, url: string, headers: Record<string, string>, refs: Exclude<FileRefs, { kind: "raw" }>): string {
  const multipart = refs.kind === "multipart";
  // Multipart: the tool writes the Content-Type with its boundary.
  const hdrs = Object.fromEntries(Object.entries(headers).filter(([k]) => !(multipart && k.toLowerCase() === "content-type")));
  if (refs.kind === "binary" && !Object.keys(hdrs).some((k) => k.toLowerCase() === "content-type")) hdrs["Content-Type"] = fileType(refs.file);
  const parts = refs.kind === "multipart" ? refs.parts : [];

  switch (lang) {
    case "curl": {
      const out = [`curl -X ${method} ${sh(url)}`];
      for (const [k, v] of Object.entries(hdrs)) out.push(`-H ${sh(`${k}: ${v}`)}`);
      if (refs.kind === "binary") out.push(`--data-binary ${sh(`@${refs.file.path}`)}`);
      for (const p of parts) {
        // --form-string: a value starting with @ or < stays text.
        if ("value" in p) out.push(`--form-string ${sh(`${p.name}=${p.value}`)}`);
        else out.push(`-F ${sh(`${p.name}=@${p.file.path}${p.file.type ? `;type=${p.file.type}` : ""}${p.file.filename ? `;filename=${p.file.filename}` : ""}`)}`);
      }
      return out.join(" \\\n  ");
    }
    case "httpie": {
      const out = [`http${multipart ? " --form" : ""} ${method} ${sh(url)}`];
      for (const [k, v] of Object.entries(hdrs)) out.push(sh(v === "" ? `${k};` : `${k}:${v}`));
      for (const p of parts) out.push("value" in p ? sh(`${p.name}=${p.value}`) : sh(`${p.name}@${p.file.path}${p.file.type ? `;type=${p.file.type}` : ""}`));
      if (refs.kind === "binary") out.push(`< ${sh(refs.file.path)}`);
      return out.join(" \\\n  ");
    }
    case "fetch": {
      const lines = [`import { openAsBlob } from "node:fs"; // Node 20+; in a browser use a File from <input type="file">`, ""];
      const opts = [`  method: ${str(method)},`];
      if (Object.keys(hdrs).length) opts.push(`  headers: ${JSON.stringify(hdrs, null, 2).replace(/\n/g, "\n  ")},`);
      if (refs.kind === "binary") opts.push(`  body: await openAsBlob(${str(refs.file.path)}),`);
      else {
        lines.push("const form = new FormData();");
        for (const p of parts) {
          lines.push("value" in p
            ? `form.append(${str(p.name)}, ${str(p.value)});`
            : `form.append(${str(p.name)}, await openAsBlob(${str(p.file.path)}, { type: ${str(fileType(p.file))} }), ${str(fileName(p.file))});`);
        }
        lines.push("");
        opts.push("  body: form,");
      }
      return [...lines, `const response = await fetch(${str(url)}, {`, ...opts, `});`, `console.log(response.status, await response.text());`].join("\n");
    }
    case "python": {
      const lines = ["import requests", "", `url = ${str(url)}`];
      const args = ["url"];
      if (Object.keys(hdrs).length) {
        lines.push(`headers = ${toPy(hdrs)}`);
        args.push("headers=headers");
      }
      if (refs.kind === "binary") {
        lines.push(`payload = open(${str(refs.file.path)}, "rb")`);
        args.push("data=payload");
      } else {
        const data = parts.filter((p) => "value" in p) as { name: string; value: string }[];
        const files = parts.filter((p) => "file" in p) as { name: string; file: FileSpec }[];
        if (data.length) {
          lines.push(`data = ${toPy(Object.fromEntries(data.map((p) => [p.name, p.value])))}`);
          args.push("data=data");
        }
        lines.push(`files = {\n${files.map((p) => `    ${str(p.name)}: (${str(fileName(p.file))}, open(${str(p.file.path)}, "rb"), ${str(fileType(p.file))}),`).join("\n")}\n}`);
        args.push("files=files");
      }
      lines.push("", `response = requests.request(${str(method)}, ${args.join(", ")})`, "print(response.status_code)", "print(response.text)");
      return lines.join("\n");
    }
  }
}
