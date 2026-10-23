// A small stand-in for "the internet" that the e2e suite sends requests to.
// Plain node:http, no dependencies. Usage: node api-server.mjs <port>
import http from "node:http";

const port = Number(process.argv[2] || 4100);

const users = () => [
  { id: 1, name: "Ada Lovelace", email: "ada@example.com", active: true, tags: ["math", "poetry"] },
  { id: 2, name: "Alan Turing", email: "alan@example.com", active: false, tags: [] },
  { id: 3, name: "Grace Hopper", email: "grace@example.com", active: true, tags: ["cobol"] },
];

const XSS = '<img src=x onerror="window.__xss=1;alert(1)">';

const cors = {
  "access-control-allow-origin": "*",
  "access-control-allow-methods": "GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS",
  "access-control-allow-headers": "*",
  "access-control-expose-headers": "*",
};

function send(res, status, body, headers = {}) {
  const isJson = typeof body !== "string";
  const payload = isJson ? JSON.stringify(body) : body;
  res.writeHead(status, {
    ...cors,
    "content-type": isJson ? "application/json; charset=utf-8" : "text/plain; charset=utf-8",
    ...headers,
  });
  res.end(payload);
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, "http://fixture");
  const path = url.pathname;
  const method = req.method;
  let raw = "";
  for await (const chunk of req) raw += chunk;
  let body = raw;
  try {
    body = raw ? JSON.parse(raw) : null;
  } catch {
    // keep as text
  }

  if (method === "OPTIONS") return send(res, 204, "");

  // JSON CRUD on /users
  if (path === "/users" && method === "GET") {
    const limit = Number(url.searchParams.get("limit") || 0);
    const list = users();
    return send(res, 200, limit ? list.slice(0, limit) : list, { "x-total-count": String(list.length) });
  }
  if (path === "/users" && method === "POST") {
    return send(res, 201, { id: 4, ...(typeof body === "object" ? body : { raw: body }) }, { location: "/users/4" });
  }
  const userMatch = path.match(/^\/users\/(\d+)$/);
  if (userMatch) {
    const user = users().find((u) => u.id === Number(userMatch[1]));
    if (!user) return send(res, 404, { error: "User not found" });
    if (method === "GET" || method === "HEAD") return send(res, 200, user);
    if (method === "PUT") return send(res, 200, { id: user.id, ...body });
    if (method === "PATCH") return send(res, 200, { ...user, ...body });
    if (method === "DELETE") {
      res.writeHead(204, cors);
      return res.end();
    }
  }

  // Everything about the request, so tests can see what the proxy forwarded.
  if (path === "/echo") {
    return send(res, 200, {
      method,
      path,
      query: Object.fromEntries(url.searchParams),
      headers: req.headers,
      body,
    });
  }

  if (path === "/text") return send(res, 200, "plain text line one\n  indented line two\nhello world");

  if (path === "/html") {
    res.writeHead(200, { ...cors, "content-type": "text/html; charset=utf-8" });
    return res.end(`<!doctype html>
<html>
<head><title>Fixture Page</title></head>
<body>
<h1 id="greeting" class="hero">Hello from the fixture</h1>
<p>A paragraph with <a href="/users">a relative link</a>.</p>
<script>try { parent.__htmlScriptRan = true; } catch (e) {} window.__inner = true; alert("html script ran");</script>
<img src="x" onerror="try { parent.__htmlOnerror = true } catch (e) {}">
</body>
</html>`);
  }

  // An API that returns attacker-controlled strings: must render as text.
  if (path === "/xss") {
    return send(res, 200, { name: XSS, bio: "<script>window.__xss=2</script>", [XSS]: "key too" });
  }
  if (path === "/xss-text") return send(res, 200, `before ${XSS} after`);

  if (path === "/slow") {
    const ms = Number(url.searchParams.get("ms") || 1500);
    await new Promise((r) => setTimeout(r, ms));
    return send(res, 200, { slow: true, waitedMs: ms });
  }

  if (path === "/big") {
    const items = Array.from({ length: 2000 }, (_, i) => ({ id: i, label: `item-${i}`, value: i * 3 }));
    return send(res, 200, { count: items.length, items });
  }
  if (path === "/huge") {
    // Bigger than the proxy's 4 MB response cap.
    res.writeHead(200, { ...cors, "content-type": "text/plain" });
    return res.end("x".repeat(5 * 1024 * 1024));
  }

  if (path === "/cookie") {
    res.writeHead(200, {
      ...cors,
      "content-type": "application/json",
      "set-cookie": ["session=abc123; Path=/; HttpOnly", "theme=dark; Path=/"],
    });
    return res.end(JSON.stringify({ cookies: "set" }));
  }

  if (path === "/redirect") {
    res.writeHead(302, { ...cors, location: "/users/1" });
    return res.end();
  }

  const statusMatch = path.match(/^\/status\/(\d{3})$/);
  if (statusMatch) {
    const code = Number(statusMatch[1]);
    return send(res, code, { code, message: `status ${code}` });
  }

  if (path === "/hang") return; // never answers: the proxy timeout must fire

  send(res, 404, { error: `No fixture route for ${method} ${path}` });
});

// Dual-stack, so the app can reach it as [::1] (through the proxy) and as
// 127.0.0.1 (through the extension).
server.listen({ port, host: "::", ipv6Only: false }, () => {
  console.log(`fixture api listening on ${port}`);
});
