import { NextResponse, NextRequest } from "next/server";

export async function POST(req: NextRequest) {
  let url: string;
  let method: string;
  let clientHeaders: string | Record<string, string> | undefined;
  let body: unknown;

  try {
    const parsed = await req.json();
    url = parsed.url;
    method = parsed.method;
    clientHeaders = parsed.headers;
    body = parsed.body;
  } catch {
    return NextResponse.json(
      { error: "Invalid request body" },
      { status: 400 }
    );
  }

  let parsedHeaders: Record<string, string> = {};
  try {
    parsedHeaders =
      typeof clientHeaders === "string"
        ? JSON.parse(clientHeaders || "{}")
        : (clientHeaders ?? {});
  } catch {
    parsedHeaders = {};
  }

  const blocked = ["host", "content-length", "transfer-encoding", "connection"];
  blocked.forEach((h) => {
    delete parsedHeaders[h];
    delete parsedHeaders[h.toLowerCase()];
  });

  const noBody = ["GET", "HEAD"].includes(method.toUpperCase());
  const bodyToSend = noBody
    ? undefined
    : typeof body === "string"
      ? body
      : JSON.stringify(body);

  let upstream: Response;
  try {
    upstream = await fetch(url, {
      method: method.toUpperCase(),
      headers: parsedHeaders,
      body: bodyToSend,
    });
  } catch (err) {
    return NextResponse.json(
      {
        data: null,
        status: 0,
        statusText: "Network Error",
        ok: false,
        headers: {},
        cookies: {},
        error: err instanceof Error ? err.message : "Failed to connect to target URL",
      },
      { status: 200 }
    );
  }

  const isHead = method.toUpperCase() === "HEAD";
  const isNoBody =
    method.toUpperCase() === "HEAD" ||
    [100, 101, 102, 103, 204, 205, 304].includes(upstream.status);

  let data = null;

  if (!isNoBody) {
    const contentType = upstream.headers.get("content-type") ?? "";
    const raw = await upstream.text();

    if (!raw) {
      data = null;
    } else if (contentType.includes("application/json")) {
      try {
        data = JSON.parse(raw);
      } catch {
        data = raw;
      }
    } else {
      data = raw;
    }
  }

  const responseHeaders: Record<string, string> = {};
  upstream.headers.forEach((value, key) => {
    responseHeaders[key] = value;
  });

  const cookies: Record<string, string> = {};
  const setCookie = upstream.headers.get("set-cookie");
  if (setCookie) {
    setCookie.split(",").forEach((part) => {
      const [pair] = part.trim().split(";");
      const eqIdx = pair.indexOf("=");
      if (eqIdx !== -1) {
        const k = pair.slice(0, eqIdx).trim();
        const v = pair.slice(eqIdx + 1).trim();
        cookies[k] = v;
      }
    });
  }

  return NextResponse.json({
    data: isHead ? responseHeaders : data,
    status: upstream.status,
    statusText: upstream.statusText,
    ok: upstream.ok,
    headers: responseHeaders,
    cookies,
  });
}
