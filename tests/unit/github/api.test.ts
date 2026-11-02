import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SESSION_COOKIE } from "@/utils/server/github/session";
import { mapCommitErrors } from "@/utils/server/github/commit";
import { GET as reposGET } from "@/app/api/github/repos/route";
import { GET as branchesGET, POST as branchesPOST } from "@/app/api/github/repos/[owner]/[repo]/branches/route";
import { GET as headGET } from "@/app/api/github/repos/[owner]/[repo]/head/route";
import { GET as treeGET } from "@/app/api/github/repos/[owner]/[repo]/tree/route";
import { GET as blobGET } from "@/app/api/github/repos/[owner]/[repo]/blob/[sha]/route";
import { POST as commitPOST } from "@/app/api/github/repos/[owner]/[repo]/commit/route";
import { POST as pullsPOST } from "@/app/api/github/repos/[owner]/[repo]/pulls/route";
import { GET as compareGET } from "@/app/api/github/repos/[owner]/[repo]/compare/route";
import { POST as logoutPOST } from "@/app/api/github/logout/route";
import { API, ghJson, mockFetch, params, req, setCookie, stubEnv } from "./helpers";

beforeEach(stubEnv);
afterEach(() => vi.unstubAllEnvs());

const SHA = "a".repeat(40);
const SHA2 = "b".repeat(40);
const OR = params({ owner: "octo", repo: "api-tests" });

describe("every route", () => {
  it("answers 401 signed_out without a session", async () => {
    const res = await reposGET(req("/api/github/repos", { session: null }), params({}));
    expect(res.status).toBe(401);
    expect(await res.json()).toEqual({ error: "signed_out" });
  });

  it("validates owner and repo", async () => {
    mockFetch(() => ghJson([]));
    for (const [owner, repo] of [["..", "r"], ["o", "."], ["o", "a b"], ["o%2F..", "r"]]) {
      const res = await branchesGET(req("/x"), params({ owner, repo }));
      expect(res.status).toBe(400);
    }
  });

  it("maps a GitHub 401 to signed_out and clears the cookie", async () => {
    mockFetch(() => ghJson({ message: "Bad credentials" }, 401));
    const res = await reposGET(req("/api/github/repos"), params({}));
    expect(res.status).toBe(401);
    expect(await res.json()).toEqual({ error: "signed_out" });
    expect(setCookie(res, SESSION_COOKIE)).toMatch(/Max-Age=0/);
  });

  it.each([
    [{ "retry-after": "30" }, 403, 30],
    [{ "x-ratelimit-remaining": "0", "x-ratelimit-reset": String(Math.floor(Date.now() / 1000) + 120) }, 403, 120],
    [{}, 429, 60],
  ])("maps a rate limit (%o, %i) to 429 rate_limited", async (headers, status, retryAfter) => {
    mockFetch(() => ghJson({ message: "You have exceeded a secondary rate limit" }, status, headers));
    const res = await reposGET(req("/api/github/repos"), params({}));
    expect(res.status).toBe(429);
    const body = await res.json();
    expect(body.error).toBe("rate_limited");
    expect(body.retryAfter).toBeGreaterThanOrEqual(retryAfter - 1);
    expect(body.retryAfter).toBeLessThanOrEqual(retryAfter);
    expect(res.headers.get("retry-after")).toBe(String(body.retryAfter));
  });
});

describe("guard", () => {
  const body = JSON.stringify({ name: "x", fromSha: SHA });

  it("refuses a mutating call without X-Hittable", async () => {
    const calls = mockFetch(() => ghJson({}));
    const res = await branchesPOST(req("/x", { method: "POST", body, headers: { origin: "https://app.test" } }), OR);
    expect(res.status).toBe(403);
    expect(await res.json()).toMatchObject({ error: "bad_origin" });
    expect(calls).toHaveLength(0);
  });

  it("refuses a foreign or missing Origin", async () => {
    mockFetch(() => ghJson({}));
    for (const origin of ["https://evil.test", "null", ""]) {
      const res = await branchesPOST(req("/x", { method: "POST", body, headers: { origin, "x-hittable": "1" } }), OR);
      expect(res.status).toBe(403);
    }
    const res = await logoutPOST(req("/api/github/logout", { method: "POST", headers: { origin: "https://evil.test", "x-hittable": "1" } }));
    expect(res.status).toBe(403);
  });
});

describe("repos", () => {
  const repo = (n: number, push = true) => ({
    full_name: `octo/r${n}`,
    name: `r${n}`,
    owner: { login: "octo" },
    private: n % 2 === 0,
    fork: false,
    archived: false,
    permissions: { push },
    default_branch: "main",
    pushed_at: "2026-10-01T00:00:00Z",
    description: null,
  });

  it("follows Link pages and maps the fields", async () => {
    const calls = mockFetch((url) =>
      url.searchParams.get("page") === "2"
        ? ghJson([repo(3, false)])
        : ghJson([repo(1), repo(2)], 200, { link: `<${API}/user/repos?page=2>; rel="next", <${API}/user/repos?page=2>; rel="last"` }),
    );
    const res = await reposGET(req("/api/github/repos"), params({}));
    const list = await res.json();
    expect(list).toHaveLength(3);
    expect(list[2]).toEqual({
      fullName: "octo/r3",
      owner: "octo",
      name: "r3",
      private: false,
      fork: false,
      archived: false,
      canPush: false,
      defaultBranch: "main",
      pushedAt: "2026-10-01T00:00:00Z",
      description: null,
    });
    const first = new URL(calls[0].url);
    expect(first.pathname).toBe("/user/repos");
    expect(first.searchParams.get("per_page")).toBe("100");
    expect(first.searchParams.get("sort")).toBe("pushed");
    expect(first.searchParams.get("affiliation")).toBe("owner,collaborator,organization_member");
    expect(calls[0].headers.get("x-github-api-version")).toBe("2022-11-28");
    expect(calls[0].headers.get("user-agent")).toBe("hittable");
    expect(calls[0].headers.get("accept")).toBe("application/vnd.github+json");
  });

  it("does not follow a Link to another host", async () => {
    mockFetch(() => ghJson([repo(1)], 200, { link: '<https://evil.test/steal>; rel="next"' }));
    const res = await reposGET(req("/api/github/repos"), params({}));
    expect(res.status).toBe(502);
  });
});

describe("branches", () => {
  it("lists name, sha and protected", async () => {
    mockFetch(() => ghJson([{ name: "main", commit: { sha: SHA }, protected: true }]));
    expect(await (await branchesGET(req("/x"), OR)).json()).toEqual([{ name: "main", sha: SHA, protected: true }]);
  });

  it("creates a branch ref", async () => {
    const calls = mockFetch(() => ghJson({ ref: "refs/heads/hittable/x", object: { sha: SHA } }, 201));
    const res = await branchesPOST(req("/x", { method: "POST", mutating: true, body: JSON.stringify({ name: "hittable/x", fromSha: SHA }) }), OR);
    expect(res.status).toBe(201);
    expect(await res.json()).toEqual({ name: "hittable/x", sha: SHA });
    expect(calls[0].url).toBe(`${API}/repos/octo/api-tests/git/refs`);
    expect(JSON.parse(calls[0].body)).toEqual({ ref: "refs/heads/hittable/x", sha: SHA });
  });

  it("maps an existing branch to 409 and refuses bad names", async () => {
    mockFetch(() => ghJson({ message: "Reference already exists" }, 422));
    const res = await branchesPOST(req("/x", { method: "POST", mutating: true, body: JSON.stringify({ name: "main", fromSha: SHA }) }), OR);
    expect(res.status).toBe(409);
    expect((await res.json()).error).toBe("branch_exists");
    for (const name of ["a..b", "a b", "x.lock", "/a", "a/", "@", "a~b"]) {
      const bad = await branchesPOST(req("/x", { method: "POST", mutating: true, body: JSON.stringify({ name, fromSha: SHA }) }), OR);
      expect(bad.status, name).toBe(400);
    }
  });
});

describe("head", () => {
  it("returns the sha and ETag, and passes a 304 through", async () => {
    const calls = mockFetch((_url, call) =>
      call.headers.get("if-none-match") === '"e1"'
        ? new Response(null, { status: 304, headers: { etag: '"e1"' } })
        : ghJson({ name: "feature/x", commit: { sha: SHA } }, 200, { etag: '"e1"' }),
    );
    const res = await headGET(req("/x?branch=feature/x"), OR);
    expect(await res.json()).toEqual({ sha: SHA });
    expect(res.headers.get("etag")).toBe('"e1"');
    expect(calls[0].url).toBe(`${API}/repos/octo/api-tests/branches/feature/x`);

    const again = await headGET(req("/x?branch=feature/x", { headers: { "if-none-match": '"e1"' } }), OR);
    expect(again.status).toBe(304);
    expect(again.headers.get("etag")).toBe('"e1"');
    expect(calls[1].headers.get("if-none-match")).toBe('"e1"');
  });

  it("requires a branch", async () => {
    expect((await headGET(req("/x"), OR)).status).toBe(400);
  });
});

describe("tree", () => {
  it("returns blobs and trees, recursively, without submodules", async () => {
    const calls = mockFetch(() =>
      ghJson({
        sha: SHA2,
        truncated: false,
        tree: [
          { path: "hittable", type: "tree", sha: SHA },
          { path: "hittable/env.json", type: "blob", sha: SHA, size: 3 },
          { path: "vendor/lib", type: "commit", sha: SHA },
        ],
      }),
    );
    const res = await treeGET(req(`/x?sha=${SHA}`), OR);
    expect(await res.json()).toEqual({
      sha: SHA2,
      truncated: false,
      entries: [
        { path: "hittable", type: "tree", sha: SHA, size: 0 },
        { path: "hittable/env.json", type: "blob", sha: SHA, size: 3 },
      ],
    });
    expect(calls[0].url).toBe(`${API}/repos/octo/api-tests/git/trees/${SHA}?recursive=1`);
  });

  it("loads one directory non-recursively with ?path", async () => {
    const calls = mockFetch(() => ghJson({ sha: SHA, truncated: false, tree: [{ path: "get.hit", type: "blob", sha: SHA2, size: 9 }] }));
    const res = await treeGET(req(`/x?sha=${SHA}&path=hittable/users`), OR);
    expect((await res.json()).entries).toEqual([{ path: "hittable/users/get.hit", type: "blob", sha: SHA2, size: 9 }]);
    expect(calls[0].url).toBe(`${API}/repos/octo/api-tests/git/trees/${SHA}`);
  });

  it("refuses a bad sha or path", async () => {
    expect((await treeGET(req("/x?sha=main"), OR)).status).toBe(400);
    expect((await treeGET(req(`/x?sha=${SHA}&path=../x`), OR)).status).toBe(400);
  });

  it("reports an empty repository", async () => {
    mockFetch(() => ghJson({ message: "Git Repository is empty." }, 409));
    const res = await treeGET(req(`/x?sha=${SHA}`), OR);
    expect(res.status).toBe(409);
    expect((await res.json()).error).toBe("empty_repo");
  });
});

describe("blob", () => {
  const P = params({ owner: "octo", repo: "api-tests", sha: SHA });

  it("returns raw bytes, cacheable forever", async () => {
    const calls = mockFetch(() => new Response(new Uint8Array([0, 1, 2, 255])));
    const res = await blobGET(req("/x"), P);
    expect(res.headers.get("content-type")).toBe("application/octet-stream");
    expect(res.headers.get("cache-control")).toBe("private, max-age=31536000, immutable");
    expect([...new Uint8Array(await res.arrayBuffer())]).toEqual([0, 1, 2, 255]);
    expect(calls[0].headers.get("accept")).toBe("application/vnd.github.raw+json");
  });

  it("refuses over 4 MB, by header or by counting", async () => {
    mockFetch(() => new Response("x", { headers: { "content-length": String(6 * 1024 * 1024) } }));
    const res = await blobGET(req("/x"), P);
    expect(res.status).toBe(413);
    expect((await res.json()).error).toBe("too_large");

    mockFetch(
      () =>
        new Response(
          new ReadableStream({
            start(c) {
              for (let i = 0; i < 6; i++) c.enqueue(new Uint8Array(1024 * 1024));
              c.close();
            },
          }),
        ),
    );
    expect((await blobGET(req("/x"), P)).status).toBe(413);
  });
});

describe("commit", () => {
  const good = {
    branch: "main",
    expectedHeadOid: SHA,
    message: "Update get.hit\n\nMore detail\nhere",
    additions: [{ path: "hittable/users/get.hit", contents: Buffer.from("{}").toString("base64") }],
    deletions: [{ path: "old.txt" }],
  };
  const post = (body: unknown, extra: RequestInit & { mutating?: boolean } = { mutating: true }) =>
    commitPOST(req("/x", { method: "POST", body: typeof body === "string" ? body : JSON.stringify(body), ...extra }), OR);

  it("sends createCommitOnBranch and returns sha and url", async () => {
    const calls = mockFetch(() =>
      ghJson({ data: { createCommitOnBranch: { commit: { oid: SHA2, url: "https://github.com/octo/api-tests/commit/b" } } } }),
    );
    const res = await post(good);
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ sha: SHA2, url: "https://github.com/octo/api-tests/commit/b" });
    expect(calls[0].url).toBe(`${API}/graphql`);
    const sent = JSON.parse(calls[0].body);
    expect(sent.query).toContain("createCommitOnBranch");
    expect(sent.variables.input).toEqual({
      branch: { repositoryNameWithOwner: "octo/api-tests", branchName: "main" },
      message: { headline: "Update get.hit", body: "More detail\nhere" },
      expectedHeadOid: SHA,
      fileChanges: { additions: good.additions, deletions: good.deletions },
    });
  });

  it.each([
    ["no X-Hittable", null, 403],
    ["bad JSON", "{", 400],
    ["no changes", { ...good, additions: [], deletions: [] }, 400],
    ["no message", { ...good, message: " " }, 400],
    ["empty headline", { ...good, message: "\nbody" }, 400],
    ["bad head", { ...good, expectedHeadOid: "main" }, 400],
    ["bad branch", { ...good, branch: "a..b" }, 400],
    ["parent path", { ...good, additions: [{ path: "../x", contents: "" }] }, 400],
    ["absolute path", { ...good, additions: [{ path: "/etc/x", contents: "" }] }, 400],
    ["empty segment", { ...good, deletions: [{ path: "a//b" }] }, 400],
    ["dot segment", { ...good, deletions: [{ path: "a/./b" }] }, 400],
    ["not base64", { ...good, additions: [{ path: "a", contents: "not base64!" }] }, 400],
    ["duplicate path", { ...good, deletions: [{ path: "hittable/users/get.hit" }] }, 400],
    ["too many", { ...good, additions: [], deletions: Array.from({ length: 501 }, (_, i) => ({ path: `f${i}` })) }, 400],
    ["too large", { ...good, additions: [{ path: "a", contents: "A".repeat(4 * 1024 * 1024) }] }, 413],
  ])("refuses %s", async (_name, body, status) => {
    const calls = mockFetch(() => ghJson({}));
    const res = body === null ? await post(good, { headers: { origin: "https://app.test" } }) : await post(body);
    expect(res.status).toBe(status);
    expect(calls).toHaveLength(0);
  });

  it.each([
    [[{ type: "STALE_DATA", message: "Expected branch to point to \"x\" but it did not.  Pull and try again." }], 409, "head_moved"],
    [[{ message: 'Expected branch to point to "x" but it did not' }], 409, "head_moved"],
    [[{ type: "UNPROCESSABLE", message: "Protected branch update failed for refs/heads/main." }], 403, "protected"],
    [[{ message: "Repository rule violations found\n\nChanges must be made through a pull request." }], 403, "protected"],
    [[{ message: "push declined due to repository rule violations" }], 403, "protected"],
    [[{ type: "FORBIDDEN", message: "octo does not have the correct permissions to execute `CreateCommitOnBranch`" }], 403, "forbidden"],
    [[{ message: "refusing to allow an OAuth App to create or update workflow `.github/workflows/ci.yml` without `workflow` scope" }], 403, "workflow_scope"],
    [[{ type: "NOT_FOUND", message: "Could not resolve to a Repository" }], 404, "not_found"],
    [[{ message: "Something else" }], 422, "commit_failed"],
  ])("maps GraphQL errors %j to %i %s", async (errors, status, code) => {
    mockFetch(() => ghJson({ data: { createCommitOnBranch: null }, errors }));
    const res = await post(good);
    expect(res.status).toBe(status);
    expect((await res.json()).error).toBe(code);
    expect(mapCommitErrors(errors).code).toBe(code);
  });
});

describe("pulls and compare", () => {
  it("opens a pull request", async () => {
    const calls = mockFetch(() => ghJson({ number: 7, html_url: "https://github.com/octo/api-tests/pull/7" }, 201));
    const res = await pullsPOST(
      req("/x", { method: "POST", mutating: true, body: JSON.stringify({ head: "hittable/x", base: "main", title: "T", body: "B" }) }),
      OR,
    );
    expect(res.status).toBe(201);
    expect(await res.json()).toEqual({ number: 7, url: "https://github.com/octo/api-tests/pull/7" });
    expect(JSON.parse(calls[0].body)).toEqual({ head: "hittable/x", base: "main", title: "T", body: "B" });
  });

  it("lists changed paths, renames on both sides", async () => {
    const calls = mockFetch(() =>
      ghJson({ files: [{ filename: "a" }, { filename: "c", previous_filename: "b" }] }),
    );
    const res = await compareGET(req(`/x?base=${SHA}&head=${SHA2}`), OR);
    expect(await res.json()).toEqual({ files: ["a", "c", "b"], truncated: false });
    expect(calls[0].url).toBe(`${API}/repos/octo/api-tests/compare/${SHA}...${SHA2}`);
  });
});

describe("logout", () => {
  it("revokes the grant with Basic auth and clears the cookie", async () => {
    const calls = mockFetch(() => new Response(null, { status: 204 }));
    const res = await logoutPOST(req("/api/github/logout", { method: "POST", mutating: true }));
    expect(await res.json()).toEqual({ ok: true });
    expect(setCookie(res, SESSION_COOKIE)).toMatch(/Max-Age=0/);
    expect(calls[0].method).toBe("DELETE");
    expect(calls[0].url).toBe(`${API}/applications/cid/grant`);
    expect(calls[0].headers.get("authorization")).toBe(`Basic ${Buffer.from("cid:csecret").toString("base64")}`);
    expect(JSON.parse(calls[0].body)).toEqual({ access_token: "tok" });
  });

  it("still clears the cookie when the revoke fails", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("offline"));
    const res = await logoutPOST(req("/api/github/logout", { method: "POST", mutating: true }));
    expect(res.status).toBe(200);
    expect(setCookie(res, SESSION_COOKIE)).toMatch(/Max-Age=0/);
  });
});
