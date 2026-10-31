import { THittableCurlJson } from "@/types";
import { resolveEnv } from "./hittableCollectionModifier";

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
  const body = method === "GET" || method === "HEAD" ? "" : r.body ?? "";
  const contentType = Object.entries(headers).find(([k]) => k.toLowerCase() === "content-type")?.[1] ?? "";
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
