import { THittableCurlJson } from "@/types";
import parse, { ResultJSON } from "@bany/curl-to-json";
import { formatJson } from "./formatJson";

export function curlConverter(curlString: string): THittableCurlJson {
  if (!curlString) return { method: "GET", url: "", params: "{}", headers: "{}", body: "{}" };

  // Normalize line endings (Windows \r\n → Unix \n)
  const normalizedCurl = curlString.replace(/\r\n/g, "\n").replace(/\r/g, "\n");

  const urlRegex = /(?:https?:\/\/|<<)[^\s"']+/;
  const rawUrlFromCurl = normalizedCurl.match(urlRegex)?.[0] || "";

  const shieldedCurl = normalizedCurl.replace(/<<(\w+)>>/g, 'PH_$1_PH');

  let parsed: ResultJSON;
  try {
    parsed = parse(shieldedCurl);
  } catch {
    parsed = { url: rawUrlFromCurl, origin: rawUrlFromCurl, method: "GET", header: {}, data: {} };
  }
  const unshield = (val: string): string => {
    if (!val) return "";
    return val.toString().replace(/PH_(\w+)_PH/g, '<<$1>>');
  };

  const extractParams = (url: string) => {
    const queryPart = url.split('?')[1];
    if (!queryPart) return {};

    return queryPart.split('&').reduce((acc: Record<string, string>, pair: string) => {
      const [k, v] = pair.split('=');
      if (k) acc[unshield(k)] = unshield(v || "");
      return acc;
    }, {});
  };

  const cleanObj = (obj: Record<string, unknown>) => {
    const newObj: Record<string, string> = {};
    Object.entries(obj || {}).forEach(([k, v]) => {
      const value = typeof v === "string" ? v : (typeof v === "object" && v !== null ? JSON.stringify(v) : String(v ?? ""));
      newObj[unshield(k)] = unshield(value);
    });
    return newObj;
  };

  return {
    method: parsed.method || "GET",
    url: rawUrlFromCurl,
    params: formatJson(extractParams(rawUrlFromCurl)).output,
    headers: formatJson(cleanObj(parsed.header as Record<string, unknown>)).output,
    body: formatJson(cleanObj(parsed.data as Record<string, unknown>)).output,
  };
}


export function jsonToCurl(json: THittableCurlJson): string {
  let methodPart = `-X ${json.method}`;

  // Use the standard curl shorthand for HEAD
  if (json.method === "HEAD") {
    methodPart = "-I";
  }

  let curl = `curl ${methodPart} ${json.url}`;

  if (json.headers) {
    try {
      const headersObj = JSON.parse(json.headers);
      for (const [key, value] of Object.entries(headersObj)) {
        const escapedValue = String(value).replace(/"/g, '\\"');
        curl += ` -H "${key}: ${escapedValue}"`;
      }
    } catch {
      // fallback
    }
  }

  // Only add body if it exists and isn't a GET/HEAD/DELETE
  const noBodyMethods = ["GET", "HEAD", "DELETE"];
  if (json.body && !noBodyMethods.includes(json.method.toUpperCase())) {
    const escapedBody = json.body.replace(/'/g, "'\\''");
    curl += ` -d '${escapedBody}'`;
  }

  return curl;
}
