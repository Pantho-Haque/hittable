import { expect, test, type APIRequestContext } from "@playwright/test";

// HTTP-only smoke test of /api/github/* against the fake GitHub
// (tests/e2e/fixtures/fake-github.mjs). No UI.
const FAKE = "http://localhost:4200";
const REPO = "octo/api-tests";

// Engine-independent, and it mutates the shared fake: run it once.
test.describe.configure({ mode: "serial" }); // one shared fake GitHub
test.skip(({ browserName }) => browserName !== "chromium", "HTTP only; runs on one project");

async function signIn(request: APIRequestContext, baseURL: string) {
  const login = await request.get("/api/github/login?next=/hittable/gh/octo/api-tests", { maxRedirects: 0 });
  expect(login.status()).toBe(302);
  const authorize = new URL(login.headers().location);
  expect(authorize.origin).toBe(FAKE);
  expect(authorize.searchParams.get("redirect_uri")).toBe(`${baseURL}/api/github/callback`);
  expect(authorize.searchParams.get("code_challenge_method")).toBe("S256");

  const approved = await request.get(authorize.href, { maxRedirects: 0 });
  expect(approved.status()).toBe(302);
  const callback = await request.get(approved.headers().location, { maxRedirects: 0 });
  expect(callback.status()).toBe(302);
  expect(callback.headers().location).toBe(`${baseURL}/hittable/gh/octo/api-tests`);
}

const fakeState = async (request: APIRequestContext) => (await request.get(`${FAKE}/__fake/state?repo=${REPO}`)).json();

test("sign in, list repos, commit and sign out over HTTP", async ({ request, baseURL }) => {
  const mutating = { "X-Hittable": "1", Origin: baseURL! };

  expect(await (await request.get("/api/github/session")).json()).toEqual({ user: null });
  expect((await request.get("/api/github/repos")).status()).toBe(401);

  await signIn(request, baseURL!);

  const { cookies } = await request.storageState();
  const session = cookies.find((c) => c.name === "hittable_session")!;
  expect(session.httpOnly).toBe(true);
  expect(session.value).not.toMatch(/gho_/);
  expect(cookies.find((c) => c.name === "hittable_oauth")).toBeUndefined();

  expect(await (await request.get("/api/github/session")).json()).toEqual({
    user: { login: "octo", name: "Octo Cat", avatarUrl: `${FAKE}/avatar.png` },
  });

  // Five repos over two fake pages.
  const repos = await (await request.get("/api/github/repos")).json();
  expect(repos.map((r: { fullName: string }) => r.fullName)).toEqual(
    expect.arrayContaining(["octo/api-tests", "octo/website", "acme/shared", "octo/old-archive", "octo/empty"]),
  );
  expect(repos.find((r: { fullName: string }) => r.fullName === "acme/shared").canPush).toBe(false);
  expect(repos.find((r: { fullName: string }) => r.fullName === "octo/old-archive").archived).toBe(true);

  // Head with ETag, then a 304.
  const head = await request.get(`/api/github/repos/${REPO}/head?branch=main`);
  const { sha } = await head.json();
  const again = await request.get(`/api/github/repos/${REPO}/head?branch=main`, { headers: { "If-None-Match": head.headers().etag } });
  expect(again.status()).toBe(304);

  const tree = await (await request.get(`/api/github/repos/${REPO}/tree?sha=${sha}`)).json();
  expect(tree.entries.map((e: { path: string }) => e.path)).toContain("hittable/users/get.hit");

  // Commit a unique file, so concurrent runs do not collide.
  const path = `hittable/e2e/${Date.now()}-${Math.random().toString(36).slice(2)}.hit`;
  const commit = await request.post(`/api/github/repos/${REPO}/commit`, {
    headers: mutating,
    data: {
      branch: "main",
      expectedHeadOid: sha,
      message: "Add e2e request\n\nFrom the smoke test",
      additions: [{ path, contents: Buffer.from('{"method":"GET"}').toString("base64") }],
      deletions: [],
    },
  });
  expect(commit.status()).toBe(200);
  const committed = await commit.json();
  const state = await fakeState(request);
  expect(state.head).toBe(committed.sha);
  expect(state.files[path]).toBe('{"method":"GET"}');
  expect(state.message).toBe("Add e2e request\n\nFrom the smoke test");

  // The old head is stale now.
  const stale = await request.post(`/api/github/repos/${REPO}/commit`, {
    headers: mutating,
    data: { branch: "main", expectedHeadOid: sha, message: "x", additions: [{ path, contents: "" }] },
  });
  expect(stale.status()).toBe(409);
  expect(await stale.json()).toMatchObject({ error: "head_moved" });

  // No CSRF header, no write.
  const csrf = await request.post(`/api/github/repos/${REPO}/commit`, { data: {} });
  expect(csrf.status()).toBe(403);

  const revocations = (await fakeState(request)).revocations;
  const logout = await request.post("/api/github/logout", { headers: mutating });
  expect(await logout.json()).toEqual({ ok: true });
  expect(await (await request.get("/api/github/session")).json()).toEqual({ user: null });
  expect((await fakeState(request)).revocations).toBe(revocations + 1);
});

test("a forged state is rejected", async ({ request, baseURL }) => {
  const login = await request.get("/api/github/login", { maxRedirects: 0 });
  const approved = await request.get(login.headers().location, { maxRedirects: 0 });
  const back = new URL(approved.headers().location);
  back.searchParams.set("state", "forged");
  const callback = await request.get(back.href, { maxRedirects: 0 });
  expect(callback.headers().location).toBe(`${baseURL}/hittable?auth_error=state_mismatch`);
  expect(await (await request.get("/api/github/session")).json()).toEqual({ user: null });
});
