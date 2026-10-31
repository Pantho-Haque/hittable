// Status presentation shared by the response panel and history.

export type StatusKind = "success" | "info" | "warning" | "danger";

/** 2xx success, 1xx/3xx info, 4xx warning, 5xx (and anything else) danger. */
export function statusKind(status: number): StatusKind {
  if (status >= 200 && status < 300) return "success";
  if ((status >= 100 && status < 200) || (status >= 300 && status < 400)) return "info";
  if (status >= 400 && status < 500) return "warning";
  return "danger";
}

// HTTP/2 responses carry no reason phrase, so statusText is often empty.
const REASONS: Record<number, string> = {
  100: "Continue", 101: "Switching Protocols", 103: "Early Hints",
  200: "OK", 201: "Created", 202: "Accepted", 203: "Non-Authoritative Information",
  204: "No Content", 205: "Reset Content", 206: "Partial Content",
  300: "Multiple Choices", 301: "Moved Permanently", 302: "Found", 303: "See Other",
  304: "Not Modified", 307: "Temporary Redirect", 308: "Permanent Redirect",
  400: "Bad Request", 401: "Unauthorized", 402: "Payment Required", 403: "Forbidden",
  404: "Not Found", 405: "Method Not Allowed", 406: "Not Acceptable", 408: "Request Timeout",
  409: "Conflict", 410: "Gone", 411: "Length Required", 412: "Precondition Failed",
  413: "Content Too Large", 414: "URI Too Long", 415: "Unsupported Media Type",
  416: "Range Not Satisfiable", 418: "I'm a teapot", 422: "Unprocessable Content",
  425: "Too Early", 426: "Upgrade Required", 428: "Precondition Required",
  429: "Too Many Requests", 431: "Request Header Fields Too Large",
  451: "Unavailable For Legal Reasons",
  500: "Internal Server Error", 501: "Not Implemented", 502: "Bad Gateway",
  503: "Service Unavailable", 504: "Gateway Timeout", 505: "HTTP Version Not Supported",
};

export function reasonPhrase(status: number, statusText?: string): string {
  return statusText?.trim() || REASONS[status] || "";
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
