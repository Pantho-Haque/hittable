import { THittableCurlJson } from "@/types";
import { formatJson } from "./formatJson";

// Splits a command line the way a POSIX shell would: '…' is literal, "…"
// honours \" \\ \$ \`, $'…' honours C escapes, a backslash escapes the next
// character and a backslash-newline joins lines.
function tokenize(cmd: string): string[] {
  const out: string[] = [];
  let cur = "";
  let started = false;
  const C_ESCAPES: Record<string, string> = { n: "\n", t: "\t", r: "\r", "\\": "\\", "'": "'", '"': '"' };
  for (let i = 0; i < cmd.length; i++) {
    const c = cmd[i];
    if (c === "$" && cmd[i + 1] === "'") {
      started = true;
      for (i += 2; i < cmd.length && cmd[i] !== "'"; i++) {
        if (cmd[i] === "\\" && i + 1 < cmd.length) cur += C_ESCAPES[cmd[++i]] ?? "\\" + cmd[i];
        else cur += cmd[i];
      }
    } else if (c === "'") {
      started = true;
      const end = cmd.indexOf("'", i + 1);
      const stop = end < 0 ? cmd.length : end;
      cur += cmd.slice(i + 1, stop);
      i = stop;
    } else if (c === '"') {
      started = true;
      for (i++; i < cmd.length && cmd[i] !== '"'; i++) {
        if (cmd[i] === "\\" && '"\\$`\n'.includes(cmd[i + 1] ?? "x")) {
          i++;
          if (cmd[i] !== "\n") cur += cmd[i];
        } else cur += cmd[i];
      }
    } else if (c === "\\") {
      i++;
      if (i < cmd.length && cmd[i] !== "\n") {
        cur += cmd[i];
        started = true;
      }
    } else if (/\s/.test(c)) {
      if (started) out.push(cur);
      cur = "";
      started = false;
    } else {
      cur += c;
      started = true;
    }
  }
  if (started) out.push(cur);
  return out;
}

const DATA_FLAGS = new Set(["-d", "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode"]);
const HEADER_FLAGS: Record<string, string> = {
  "-b": "Cookie", "--cookie": "Cookie",
  "-A": "User-Agent", "--user-agent": "User-Agent",
  "-e": "Referer", "--referer": "Referer",
};
// Flags whose argument must be skipped so it is not taken for the URL.
const IGNORED_WITH_ARG = new Set([
  "-o", "--output", "-m", "--max-time", "--connect-timeout", "-w", "--write-out", "-x", "--proxy",
  "--retry", "-T", "--upload-file", "-r", "--range", "--cacert", "--cert", "-E", "--key", "--resolve",
  "--limit-rate", "-c", "--cookie-jar", "-K", "--config", "--max-redirs",
]);

export function base64Utf8(s: string): string {
  let binary = "";
  for (const b of new TextEncoder().encode(s)) binary += String.fromCharCode(b);
  return btoa(binary);
}

function safeDecode(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

function paramsFromUrl(url: string): Record<string, string> {
  const query = url.split("#")[0].split("?").slice(1).join("?");
  const params: Record<string, string> = {};
  for (const pair of query.split("&")) {
    if (!pair) continue;
    const [k, ...v] = pair.split("=");
    params[safeDecode(k)] = safeDecode(v.join("="));
  }
  return params;
}

// A JSON body keeps its types; key=value&… form data becomes a JSON object
// (the body editor is JSON); anything else is kept as raw text.
function bodyFromData(data: string): string {
  if (formatJson(data).error === null) return formatJson(data).output;
  if (/^[^=&\s{}[\]]+=[^&]*(&[^=&\s]+=[^&]*)*$/.test(data)) return formatJson(paramsFromUrl("?" + data)).output;
  return data;
}

export function curlConverter(curlString: string): THittableCurlJson {
  const empty = { method: "GET", url: "", params: "{}", headers: "{}", body: "{}" };
  if (!curlString) return empty;

  const tokens = tokenize(curlString.replace(/\r\n?/g, "\n"));
  if (tokens[0] === "curl") tokens.shift();

  let method = "";
  let url = "";
  const headers: Record<string, string> = {};
  const data: string[] = [];
  const form: Record<string, string> = {};

  for (let i = 0; i < tokens.length; i++) {
    let flag = tokens[i];
    let arg: string | undefined;
    const next = () => (arg !== undefined ? arg : tokens[++i] ?? "");

    if (flag.startsWith("--") && flag.includes("=")) {
      arg = flag.slice(flag.indexOf("=") + 1);
      flag = flag.slice(0, flag.indexOf("="));
    } else if (/^-[XHdubAeF]./.test(flag)) {
      arg = flag.slice(2);
      flag = flag.slice(0, 2);
    }

    if (flag === "-X" || flag === "--request") method = next().toUpperCase();
    else if (flag === "-H" || flag === "--header") {
      const h = next();
      const colon = h.indexOf(":");
      if (colon > 0) headers[h.slice(0, colon).trim()] = h.slice(colon + 1).trim();
    } else if (DATA_FLAGS.has(flag)) data.push(next());
    else if (flag === "--json") {
      data.push(next());
      headers["Content-Type"] ??= "application/json";
      headers["Accept"] ??= "application/json";
    } else if (flag === "-F" || flag === "--form") {
      const [k, ...v] = next().split("=");
      if (k) form[k] = v.join("=");
    } else if (flag === "-u" || flag === "--user") headers["Authorization"] = `Basic ${base64Utf8(next())}`;
    else if (flag in HEADER_FLAGS) headers[HEADER_FLAGS[flag]] = next();
    else if (flag === "-I" || flag === "--head") method = "HEAD";
    else if (flag === "--url") url = next();
    else if (IGNORED_WITH_ARG.has(flag)) next();
    else if (!flag.startsWith("-") && !url) url = flag;
  }

  const rawBody = data.join("&");
  const hasBody = data.length > 0 || Object.keys(form).length > 0;
  return {
    method: method || (hasBody ? "POST" : "GET"),
    url,
    params: formatJson(paramsFromUrl(url)).output,
    headers: formatJson(headers).output,
    body: data.length ? bodyFromData(rawBody) : Object.keys(form).length ? formatJson(form).output : "{}",
  };
}

// Single-quotes a word for a POSIX shell.
const shellQuote = (s: string) => `'${s.replace(/'/g, `'\\''`)}'`;

export function jsonToCurl(json: THittableCurlJson): string {
  const method = (json.method || "GET").toUpperCase();
  // `-X HEAD` makes curl wait for a body that never comes; -I is the HEAD flag.
  let curl = `curl ${method === "HEAD" ? "-I" : `-X ${method}`} ${shellQuote(json.url)}`;

  if (json.headers) {
    try {
      const headersObj = JSON.parse(json.headers);
      for (const [key, value] of Object.entries(headersObj ?? {})) {
        curl += ` -H ${shellQuote(`${key}: ${value}`)}`;
      }
    } catch {
      // invalid header JSON: the form blocks sending it, so nothing to add
    }
  }

  if (json.body && !["GET", "HEAD", "DELETE"].includes(method)) {
    curl += ` -d ${shellQuote(json.body)}`;
  }

  return curl;
}
