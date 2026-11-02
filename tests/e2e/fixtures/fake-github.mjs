// A fake GitHub for the e2e suite: just the endpoints /api/github/* calls.
// Plain node:http, no dependencies. Usage: node fake-github.mjs <port>
// The app points at it with GITHUB_OAUTH_URL=http://localhost:<port>,
// GITHUB_API_URL=http://localhost:<port>/api, GITHUB_GRAPHQL_URL=.../api/graphql.
//
// OAuth (client_id "test-client", secret "test-secret"):
//   GET  /login/oauth/authorize      auto-approves: 302 to redirect_uri?code&state
//                                    (or ?error=access_denied when config.deny)
//   POST /login/oauth/access_token   code + code_verifier (checked against the S256
//                                    challenge) or grant_type=refresh_token
// API (Bearer token from the exchange; page size capped at 3 so lists paginate):
//   GET  /api/user                                 the user "octo"
//   GET  /api/user/repos                           5 repos, 2 pages, newest push first
//   GET  /api/repos/:o/:r/branches[/:branch]       branch head has an ETag, 304 on If-None-Match
//   POST /api/repos/:o/:r/git/refs                 create a branch
//   GET  /api/repos/:o/:r/git/trees/:sha[?recursive=1]   commit or tree sha
//   GET  /api/repos/:o/:r/git/blobs/:sha           raw with Accept *raw*, else JSON base64
//   POST /api/repos/:o/:r/pulls
//   GET  /api/repos/:o/:r/compare/:base...:head
//   DELETE /api/applications/:client_id/grant      Basic auth; revokes that token
//   POST /api/graphql                              createCommitOnBranch only
//
// Seeded data (restored by POST /__fake/reset):
//   octo/api-tests   private, push. main: README.md, hittable/env.json,
//                    hittable/users/get.hit, assets/logo.png (binary).
//                    Branches: main, dev, protected (protected: commits are refused).
//   octo/website     public, push, no hittable/ folder: README.md, index.html
//   octo/old-archive archived (push true, as GitHub reports it)
//   acme/shared      org repo, permissions.push false: hittable/env.json, hittable/ping.hit
//   octo/empty       an empty repository: no branches, trees answer 409
//
// Control (for tests):
//   POST /__fake/reset                        restore the seed, forget tokens and counters
//   POST /__fake/config {deny?, expiring?, tokenTtl?, truncate?: "o/r" | null}
//        deny: authorize answers access_denied; expiring: tokens come with
//        expires_in=tokenTtl (default 28800) + a refresh_token; truncate: recursive
//        trees of that repo come back truncated (top level only).
//   POST /__fake/advance {repo, branch?, path, contents?, delete?, message?}
//        someone else pushes a commit (contents is UTF-8 text) → {sha}
//   GET  /__fake/state?repo=o/r&branch=main   {head, files:{path: utf8}, branches,
//        pulls, revocations, refreshes, tokens}
import http from "node:http";
import { createHash, randomBytes } from "node:crypto";

const port = Number(process.argv[2] || 4200);
const CLIENT_ID = "test-client";
const CLIENT_SECRET = "test-secret";
const PAGE_SIZE = 3;

const sha1 = (buf) => createHash("sha1").update(buf).digest("hex");
const b64url = (buf) => Buffer.from(buf).toString("base64url");

// 1x1 transparent PNG, the "binary file" and the avatar.
const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
  "base64",
);

let blobs, trees, commits, repos, tokens, refreshTokens, codes, counters, config;

function putBlob(buf) {
  const sha = sha1(Buffer.concat([Buffer.from(`blob ${buf.length}\0`), buf]));
  blobs.set(sha, buf);
  return sha;
}

// files: Map path → blob sha. Returns the root tree sha; every subtree is stored.
function writeTree(files) {
  const root = new Map();
  for (const [path, sha] of files) {
    const parts = path.split("/");
    let dir = root;
    for (const p of parts.slice(0, -1)) {
      if (!dir.has(p)) dir.set(p, new Map());
      dir = dir.get(p);
    }
    dir.set(parts.at(-1), sha);
  }
  const write = (dir) => {
    const entries = [...dir.entries()]
      .sort(([a], [b]) => (a < b ? -1 : 1))
      .map(([name, v]) =>
        v instanceof Map
          ? { path: name, mode: "040000", type: "tree", sha: write(v) }
          : { path: name, mode: "100644", type: "blob", sha: v, size: blobs.get(v).length },
      );
    const sha = sha1("tree\0" + JSON.stringify(entries));
    trees.set(sha, entries);
    return sha;
  };
  return write(root);
}

function makeCommit(files, parent, message) {
  const tree = writeTree(files);
  const sha = sha1(`commit ${tree} ${parent} ${message} ${++counters.commits}`);
  commits.set(sha, { tree, parent, message, files: new Map(files) });
  return sha;
}

function seedRepo(fullName, meta, fileObj, branchNames = ["main"], protectedBranches = []) {
  const [owner, name] = fullName.split("/");
  const files = new Map(Object.entries(fileObj).map(([p, c]) => [p, putBlob(Buffer.isBuffer(c) ? c : Buffer.from(c))]));
  const branches = new Map();
  if (files.size) {
    const head = makeCommit(files, null, "Initial commit");
    for (const b of branchNames) branches.set(b, head);
  }
  repos.set(fullName, {
    meta: {
      id: repos.size + 1,
      name,
      full_name: fullName,
      owner: { login: owner, type: owner === "octo" ? "User" : "Organization" },
      private: false,
      fork: false,
      archived: false,
      description: null,
      default_branch: "main",
      permissions: { admin: owner === "octo", push: true, pull: true },
      ...meta,
    },
    branches,
    protected: new Set(protectedBranches),
    pulls: [],
  });
}

function reset() {
  blobs = new Map();
  trees = new Map();
  commits = new Map();
  repos = new Map();
  tokens = new Map(); // access token → { exp: epoch seconds | null }
  refreshTokens = new Set();
  codes = new Map(); // code → { challenge, redirectUri }
  counters = { commits: 0, revocations: 0, refreshes: 0, pulls: 0 };
  config = { deny: false, expiring: false, tokenTtl: 28800, truncate: null };
  const day = 86400_000;
  const ago = (d) => new Date(Date.UTC(2026, 9, 31) - d * day).toISOString();
  seedRepo(
    "octo/api-tests",
    { private: true, pushed_at: ago(0), description: "Requests for the users API" },
    {
      "README.md": "# api-tests\n\nHittable requests for the users API.\n",
      "hittable/env.json": JSON.stringify({ baseUrl: "http://localhost:4100" }, null, 2) + "\n",
      "hittable/users/get.hit":
        JSON.stringify({ method: "GET", url: "{{baseUrl}}/users", headers: {}, params: {}, body: "" }, null, 2) + "\n",
      "assets/logo.png": PNG,
    },
    ["main", "dev", "protected"],
    ["protected"],
  );
  seedRepo(
    "octo/website",
    { pushed_at: ago(3), description: "A site without Hittable yet" },
    { "README.md": "# website\n", "index.html": "<!doctype html><h1>Hello</h1>\n" },
  );
  seedRepo("acme/shared", { private: true, pushed_at: ago(10), permissions: { admin: false, push: false, pull: true } }, {
    "hittable/env.json": "{}\n",
    "hittable/ping.hit": JSON.stringify({ method: "GET", url: "http://localhost:4100/users/1" }, null, 2) + "\n",
  });
  seedRepo("octo/old-archive", { archived: true, pushed_at: ago(400) }, { "README.md": "# old\n" });
  seedRepo("octo/empty", { pushed_at: null }, {});
}
reset();

const user = {
  login: "octo",
  id: 583231,
  name: "Octo Cat",
  avatar_url: `http://localhost:${port}/avatar.png`,
  html_url: "https://github.com/octo",
};

function send(res, status, body, headers = {}) {
  if (body === null) {
    res.writeHead(status, headers);
    return res.end();
  }
  const payload = Buffer.isBuffer(body) ? body : Buffer.from(typeof body === "string" ? body : JSON.stringify(body));
  res.writeHead(status, {
    "content-type": Buffer.isBuffer(body) ? "application/octet-stream" : "application/json; charset=utf-8",
    "content-length": payload.length,
    ...headers,
  });
  res.end(payload);
}

const notFound = (res) => send(res, 404, { message: "Not Found", documentation_url: "https://docs.github.com/rest" });

// GitHub-style pagination with an absolute Link header.
function paginate(req, res, url, list) {
  const per = Math.min(Number(url.searchParams.get("per_page")) || 30, PAGE_SIZE);
  const page = Math.max(1, Number(url.searchParams.get("page")) || 1);
  const last = Math.max(1, Math.ceil(list.length / per));
  const headers = {};
  if (page < last) {
    const link = (n) => {
      const u = new URL(url);
      u.searchParams.set("page", String(n));
      return `<http://${req.headers.host}${u.pathname}${u.search}>`;
    };
    headers.link = `${link(page + 1)}; rel="next", ${link(last)}; rel="last"`;
  }
  send(res, 200, list.slice((page - 1) * per, page * per), headers);
}

function issueTokens() {
  const t = `gho_fake_${randomBytes(12).toString("hex")}`;
  const out = { access_token: t, token_type: "bearer", scope: "repo,read:user" };
  if (config.expiring) {
    tokens.set(t, { exp: Math.floor(Date.now() / 1000) + config.tokenTtl });
    const rt = `ghr_fake_${randomBytes(12).toString("hex")}`;
    refreshTokens.add(rt);
    Object.assign(out, { expires_in: config.tokenTtl, refresh_token: rt, refresh_token_expires_in: 15897600 });
  } else {
    tokens.set(t, { exp: null });
  }
  return out;
}

function authed(req) {
  const m = (req.headers.authorization || "").match(/^(?:Bearer|token) (.+)$/i);
  const tok = m && tokens.get(m[1]);
  return !!tok && (tok.exp === null || tok.exp > Date.now() / 1000);
}

const resolve = (repo, ref) => (commits.has(ref) ? ref : repo.branches.get(ref));
const textFiles = (files) =>
  Object.fromEntries([...files].map(([p, sha]) => [p, blobs.get(sha).toString("utf8")]));

function walk(treeSha, prefix, out) {
  for (const e of trees.get(treeSha)) {
    const path = prefix + e.path;
    out.push({ ...e, path });
    if (e.type === "tree") walk(e.sha, path + "/", out);
  }
  return out;
}

const gqlError = (res, type, message) =>
  send(res, 200, { data: { createCommitOnBranch: null }, errors: [{ ...(type ? { type } : {}), path: ["createCommitOnBranch"], message }] });

function createCommitOnBranch(res, input) {
  const repo = repos.get(input?.branch?.repositoryNameWithOwner);
  if (!repo) return gqlError(res, "NOT_FOUND", `Could not resolve to a Repository with the name '${input?.branch?.repositoryNameWithOwner}'.`);
  if (!repo.meta.permissions.push || repo.meta.archived) {
    return gqlError(res, "FORBIDDEN", "octo does not have the correct permissions to execute `CreateCommitOnBranch`");
  }
  const name = input.branch.branchName;
  const head = repo.branches.get(name);
  if (!head) return gqlError(res, "NOT_FOUND", `A ref named "refs/heads/${name}" does not exist.`);
  const adds = input.fileChanges?.additions ?? [];
  const dels = input.fileChanges?.deletions ?? [];
  const workflow = [...adds, ...dels].find((f) => f.path.startsWith(".github/workflows/"));
  if (workflow) {
    return gqlError(res, null, `refusing to allow an OAuth App to create or update workflow \`${workflow.path}\` without \`workflow\` scope`);
  }
  if (repo.protected.has(name)) return gqlError(res, null, `Protected branch update failed for refs/heads/${name}.`);
  if (input.expectedHeadOid !== head) {
    return gqlError(res, "STALE_DATA", `Expected branch to point to "${input.expectedHeadOid}" but it did not.  Pull and try again.`);
  }
  const files = new Map(commits.get(head).files);
  for (const d of dels) {
    if (!files.delete(d.path)) {
      return gqlError(res, "UNPROCESSABLE", `A path was requested for deletion which does not exist as of commit oid \`${head}\``);
    }
  }
  for (const a of adds) files.set(a.path, putBlob(Buffer.from(a.contents, "base64")));
  const msg = input.message?.body ? `${input.message.headline}\n\n${input.message.body}` : input.message?.headline;
  const sha = makeCommit(files, head, msg);
  repo.branches.set(name, sha);
  repo.meta.pushed_at = new Date().toISOString();
  send(res, 200, {
    data: { createCommitOnBranch: { commit: { oid: sha, url: `https://github.com/${repo.meta.full_name}/commit/${sha}` } } },
  });
}

async function api(req, res, url, path, body) {
  const method = req.method;

  if (method === "DELETE" && path === `/applications/${CLIENT_ID}/grant`) {
    const basic = Buffer.from(`${CLIENT_ID}:${CLIENT_SECRET}`).toString("base64");
    if (req.headers.authorization !== `Basic ${basic}`) return send(res, 401, { message: "Bad credentials" });
    if (!tokens.has(body?.access_token)) return send(res, 422, { message: "Invalid token" });
    // GitHub revokes every token of the grant; only this one here, so parallel
    // tests signed in as the same fake user do not sign each other out.
    tokens.delete(body.access_token);
    counters.revocations++;
    return send(res, 204, null);
  }

  if (!authed(req)) return send(res, 401, { message: "Bad credentials", documentation_url: "https://docs.github.com/rest" });

  if (method === "GET" && path === "/user") return send(res, 200, user);
  if (method === "GET" && path === "/user/repos") {
    const list = [...repos.values()].map((r) => r.meta).sort((a, b) => (b.pushed_at ?? "").localeCompare(a.pushed_at ?? ""));
    return paginate(req, res, url, list);
  }
  if (method === "POST" && path === "/graphql") {
    if (!/createCommitOnBranch/.test(body?.query ?? "")) return send(res, 200, { errors: [{ message: "fake-github: unsupported query" }] });
    return createCommitOnBranch(res, body.variables?.input);
  }

  const m = path.match(/^\/repos\/([^/]+)\/([^/]+)(\/.*)?$/);
  const repo = m && repos.get(`${m[1]}/${m[2]}`);
  if (!repo) return notFound(res);
  const rest = m[3] ?? "";
  let r;

  if (method === "GET" && rest === "/branches") {
    const list = [...repo.branches].map(([name, sha]) => ({ name, commit: { sha }, protected: repo.protected.has(name) }));
    return paginate(req, res, url, list);
  }
  if (method === "GET" && (r = rest.match(/^\/branches\/(.+)$/))) {
    const name = decodeURIComponent(r[1]);
    const sha = repo.branches.get(name);
    if (!sha) return send(res, 404, { message: "Branch not found" });
    const etag = `"${sha}"`;
    if ((req.headers["if-none-match"] || "").replace(/^W\//, "") === etag) return send(res, 304, null, { etag });
    return send(res, 200, { name, commit: { sha, commit: { message: commits.get(sha).message } }, protected: repo.protected.has(name) }, { etag });
  }
  if (method === "POST" && rest === "/git/refs") {
    if (!repo.meta.permissions.push) return send(res, 403, { message: "Resource not accessible by integration" });
    const name = String(body?.ref ?? "").replace(/^refs\/heads\//, "");
    if (!name || !commits.has(body?.sha)) return send(res, 422, { message: "Object does not exist" });
    if (repo.branches.has(name)) return send(res, 422, { message: "Reference already exists" });
    repo.branches.set(name, body.sha);
    return send(res, 201, { ref: `refs/heads/${name}`, object: { sha: body.sha, type: "commit" } });
  }
  if (method === "GET" && (r = rest.match(/^\/git\/trees\/([0-9a-f]+)$/))) {
    if (!repo.branches.size) return send(res, 409, { message: "Git Repository is empty." });
    const sha = commits.has(r[1]) ? commits.get(r[1]).tree : r[1];
    if (!trees.has(sha)) return notFound(res);
    const recursive = url.searchParams.has("recursive");
    const truncate = recursive && config.truncate === repo.meta.full_name;
    const tree = recursive && !truncate ? walk(sha, "", []) : trees.get(sha);
    return send(res, 200, { sha, truncated: truncate, tree });
  }
  if (method === "GET" && (r = rest.match(/^\/git\/blobs\/([0-9a-f]+)$/))) {
    const buf = blobs.get(r[1]);
    if (!buf) return notFound(res);
    if (/raw/.test(req.headers.accept || "")) return send(res, 200, buf);
    return send(res, 200, { sha: r[1], size: buf.length, encoding: "base64", content: buf.toString("base64") });
  }
  if (method === "POST" && rest === "/pulls") {
    const { head, base, title } = body ?? {};
    if (!repo.branches.has(head) || !repo.branches.has(base) || !title) {
      return send(res, 422, { message: "Validation Failed", errors: [{ resource: "PullRequest", code: "invalid" }] });
    }
    if (repo.pulls.some((p) => p.head === head && p.base === base)) {
      return send(res, 422, { message: `A pull request already exists for octo:${head}.` });
    }
    const pr = { number: ++counters.pulls, head, base, title, body: body.body ?? "" };
    repo.pulls.push(pr);
    return send(res, 201, { ...pr, html_url: `https://github.com/${repo.meta.full_name}/pull/${pr.number}` });
  }
  if (method === "GET" && (r = rest.match(/^\/compare\/(.+)\.\.\.(.+)$/))) {
    const base = resolve(repo, decodeURIComponent(r[1]));
    const head = resolve(repo, decodeURIComponent(r[2]));
    if (!base || !head) return notFound(res);
    const a = commits.get(base).files;
    const b = commits.get(head).files;
    const files = [...new Set([...a.keys(), ...b.keys()])]
      .filter((p) => a.get(p) !== b.get(p))
      .map((p) => ({ filename: p, status: !a.has(p) ? "added" : !b.has(p) ? "removed" : "modified" }));
    return send(res, 200, { status: "diverged", files });
  }
  return send(res, 404, { message: `fake-github: no route for ${method} ${path}` });
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://${req.headers.host}`);
  const path = url.pathname;
  let raw = "";
  for await (const chunk of req) raw += chunk;
  let body = null;
  if (raw) {
    try {
      body = JSON.parse(raw);
    } catch {
      body = Object.fromEntries(new URLSearchParams(raw));
    }
  }

  try {
    if (path === "/avatar.png") return send(res, 200, PNG, { "content-type": "image/png" });

    if (path === "/__fake/reset" && req.method === "POST") {
      reset();
      return send(res, 200, { ok: true });
    }
    if (path === "/__fake/config" && req.method === "POST") {
      Object.assign(config, body);
      return send(res, 200, config);
    }
    if (path === "/__fake/advance" && req.method === "POST") {
      const repo = repos.get(body?.repo);
      const branch = body?.branch || "main";
      const head = repo?.branches.get(branch);
      if (!head) return notFound(res);
      const files = new Map(commits.get(head).files);
      if (body.delete) files.delete(body.path);
      else files.set(body.path, putBlob(Buffer.from(body.contents ?? `changed by someone else ${Date.now()}\n`)));
      const sha = makeCommit(files, head, body.message || `Update ${body.path}`);
      repo.branches.set(branch, sha);
      return send(res, 200, { sha });
    }
    if (path === "/__fake/state" && req.method === "GET") {
      const repo = repos.get(url.searchParams.get("repo") || "octo/api-tests");
      const head = repo?.branches.get(url.searchParams.get("branch") || "main");
      return send(res, 200, {
        head: head ?? null,
        message: head ? commits.get(head).message : null,
        files: head ? textFiles(commits.get(head).files) : {},
        branches: repo ? Object.fromEntries(repo.branches) : {},
        pulls: repo?.pulls ?? [],
        revocations: counters.revocations,
        refreshes: counters.refreshes,
        tokens: tokens.size,
      });
    }

    if (path === "/login/oauth/authorize" && req.method === "GET") {
      const q = url.searchParams;
      const redirectUri = q.get("redirect_uri");
      if (q.get("client_id") !== CLIENT_ID || !redirectUri) return send(res, 400, "bad client_id or redirect_uri");
      const back = new URL(redirectUri);
      if (q.get("state")) back.searchParams.set("state", q.get("state"));
      if (config.deny) {
        back.searchParams.set("error", "access_denied");
        back.searchParams.set("error_description", "The user has denied your application access.");
      } else {
        if (q.get("code_challenge_method") !== "S256" || !q.get("code_challenge")) return send(res, 400, "PKCE S256 required");
        const code = randomBytes(10).toString("hex");
        codes.set(code, { challenge: q.get("code_challenge"), redirectUri });
        back.searchParams.set("code", code);
      }
      return send(res, 302, null, { location: back.toString() });
    }

    if (path === "/login/oauth/access_token" && req.method === "POST") {
      if (body?.client_id !== CLIENT_ID || body?.client_secret !== CLIENT_SECRET) {
        return send(res, 200, { error: "incorrect_client_credentials" });
      }
      if (body.grant_type === "refresh_token") {
        if (!refreshTokens.delete(body.refresh_token)) return send(res, 200, { error: "bad_refresh_token" });
        counters.refreshes++;
        return send(res, 200, issueTokens());
      }
      const pending = codes.get(body.code);
      codes.delete(body.code);
      if (!pending) return send(res, 200, { error: "bad_verification_code" });
      if (body.redirect_uri && body.redirect_uri !== pending.redirectUri) return send(res, 200, { error: "redirect_uri_mismatch" });
      if (b64url(createHash("sha256").update(String(body.code_verifier ?? "")).digest()) !== pending.challenge) {
        return send(res, 200, { error: "bad_verification_code", error_description: "code_verifier does not match" });
      }
      return send(res, 200, issueTokens());
    }

    if (path === "/api" || path.startsWith("/api/")) return await api(req, res, url, path.slice(4), body);
    return notFound(res);
  } catch (err) {
    send(res, 500, { message: `fake-github: ${err.message}` });
  }
});

server.listen({ port, host: "::", ipv6Only: false }, () => {
  console.log(`fake github listening on ${port}`);
});
