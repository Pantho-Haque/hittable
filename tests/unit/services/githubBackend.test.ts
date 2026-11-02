import { describe, it, expect, beforeEach } from "vitest";
import {
  buildCommit,
  branchSlug,
  createGitHubBackend,
  EXAMPLE_HIT,
  gitBlobSha,
  nextBranchName,
  prefillMessage,
  utf8ToBase64,
} from "@/services/github/githubBackend";
import { copyKey, hasWorkingCopies, memoryStore, type KVStore, type WorkingCopy } from "@/services/github/workingCopy";
import { findNode } from "@/services/workspace/backend";
import { fakeGitHub, META } from "../helpers/fakeGitHubClient";

const GET_HIT = '{\n  "method": "GET",\n  "url": "{{baseUrl}}/users"\n}\n';
const SEED = {
  "README.md": "# api-tests\n",
  "hittable/env.json": "{}\n",
  "hittable/users/get.hit": GET_HIT,
  ".github/workflows/ci.yml": "on: push\n",
};
const text = (b: Uint8Array) => new TextDecoder().decode(b);
const path = (p: string) => p.split("/");

let gh: ReturnType<typeof fakeGitHub>;
let kv: KVStore;
const open = (o: { branch?: string; meta?: Partial<typeof META> } = {}) =>
  createGitHubBackend({ owner: "octo", repo: "api-tests", branch: o.branch ?? "main", meta: { ...META, ...o.meta }, client: gh.client, kv });

beforeEach(() => {
  gh = fakeGitHub(SEED, { protectedBranches: ["protected"] });
  kv = memoryStore();
});

describe("gitBlobSha", () => {
  it("matches git's blob hash", async () => {
    // `printf 'hello\n' | git hash-object --stdin`
    expect(await gitBlobSha("hello\n")).toBe("ce013625030ba8dba906f756967f9e9ca394464a");
  });
});

describe("status (FR-G4)", () => {
  it("M for an edit, A for a new file, D for a delete; editing back to the base clears M", async () => {
    const b = await open();
    await b.writeFile(path("hittable/users/get.hit"), GET_HIT.replace("users", "people"));
    await b.createFile(path("hittable/users/new.hit"), "{}");
    await b.remove(path("README.md"));
    expect(b.status()).toEqual({ "hittable/users/get.hit": "M", "hittable/users/new.hit": "A", "README.md": "D" });

    await b.writeFile(path("hittable/users/get.hit"), GET_HIT);
    expect(b.status()).toEqual({ "hittable/users/new.hit": "A", "README.md": "D" });
  });

  it("keeps deleted files in the tree, struck through, read-only until discarded", async () => {
    const b = await open();
    await b.remove(path("README.md"));
    expect(findNode(await b.listTree(), ["README.md"])).not.toBeNull();
    expect(b.readOnlyReason(["README.md"])).toMatch(/deleted/);
    await b.discard(["README.md"]);
    expect(b.status()).toEqual({});
    expect(b.readOnlyReason(["README.md"])).toBeNull();
  });

  it("never counts env.local.json as a change, but keeps it on this device", async () => {
    const b = await open();
    await b.createFile(path("hittable/env.local.json"), '{"token":"secret"}');
    expect(b.status()).toEqual({});
    expect(b.changes()).toEqual([]);
    expect(b.hasLocalEnv()).toBe(true);
    expect(text(await b.readFile(path("hittable/env.local.json")))).toContain("secret");
  });

  it("refuses to edit GitHub workflows", async () => {
    const b = await open();
    await expect(b.writeFile(path(".github/workflows/ci.yml"), "x")).rejects.toThrow(/workflows/);
    await expect(b.createFile(path(".github/workflows/new.yml"), "x")).rejects.toThrow(/workflows/);
    await expect(b.remove([".github"])).rejects.toThrow(/workflows/);
    expect(b.readOnlyReason(path(".github/workflows/ci.yml"))).toMatch(/workflows/);
    expect(b.status()).toEqual({});
  });

  it("a read-only repository refuses every write", async () => {
    const b = await open({ meta: { canPush: false } });
    expect(b.readOnly).toBe(true);
    await expect(b.writeFile(path("README.md"), "x")).rejects.toThrow(/read-only/);
    expect((await open({ meta: { archived: true } })).readOnly).toBe(true);
  });

  it("rename is a delete plus an add; an empty new folder still shows", async () => {
    const b = await open();
    await b.rename(path("hittable/users"), path("hittable/people"));
    expect(b.status()).toEqual({ "hittable/users/get.hit": "D", "hittable/people/get.hit": "A" });
    expect(text(await b.readFile(path("hittable/people/get.hit")))).toBe(GET_HIT);

    await b.createDirectory(path("hittable/empty"));
    expect(findNode(await b.listTree(), path("hittable/empty"))?.kind).toBe("directory");
    expect(b.changes()).toHaveLength(2);
    await expect(b.createFile(path("hittable/env.json"))).rejects.toThrow(/already exists/);
  });

  it("downloads a blob once (cached by SHA)", async () => {
    const b = await open();
    await b.readFile(path("README.md"));
    await b.readFile(path("README.md"));
    expect(gh.calls.blob).toBe(1);
  });

  it("lists a truncated tree lazily, hittable/ first", async () => {
    gh = fakeGitHub(SEED, { truncated: true });
    const b = await open();
    const tree = await b.listTree();
    expect(findNode(tree, ["hittable", "env.json"])).not.toBeNull();
    expect(findNode(tree, [".github"])?.children).toBeUndefined();
    await b.loadDirectory([".github"]);
    expect(findNode(await b.listTree(), [".github"])?.children?.map((n) => n.name)).toEqual(["workflows"]);
  });
});

describe("working copy persistence", () => {
  it("survives a reload: same overlay, same status", async () => {
    const b = await open();
    await b.writeFile(path("hittable/users/get.hit"), "changed");
    const saved = await kv.get<WorkingCopy>("copies", copyKey("octo", "api-tests", "main"));
    expect(saved?.overlay["hittable/users/get.hit"]).toEqual({ content: "changed" });
    expect(await hasWorkingCopies(kv)).toBe(true);

    const again = await open();
    expect(again.status()).toEqual({ "hittable/users/get.hit": "M" });
    expect(text(await again.readFile(path("hittable/users/get.hit")))).toBe("changed");
  });

  it("forgets the copy once nothing is changed", async () => {
    const b = await open();
    await b.writeFile(path("README.md"), "x");
    await b.writeFile(path("README.md"), SEED["README.md"]);
    expect(await hasWorkingCopies(kv)).toBe(false);
  });
});

describe("commit payload (FR-G5)", () => {
  it("base64 UTF-8 additions, deletions only for base files, never env.local.json", () => {
    const input = buildCommit(
      {
        "a.hit": { content: "héllo ✓" },
        "gone.txt": { deleted: true },
        "never-existed.txt": { deleted: true },
        "hittable/env.local.json": { content: "{}" },
        "empty": { dir: true },
      },
      (p) => p === "gone.txt",
      { branch: "main", expectedHeadOid: "a".repeat(40), message: "m" },
    );
    expect(input.additions).toEqual([{ path: "a.hit", contents: utf8ToBase64("héllo ✓") }]);
    expect(Buffer.from(input.additions[0].contents, "base64").toString("utf8")).toBe("héllo ✓");
    expect(input.deletions).toEqual([{ path: "gone.txt" }]);
  });

  it("two edits and a deletion make one commit with exactly those changes", async () => {
    const b = await open();
    await b.writeFile(path("hittable/users/get.hit"), "one");
    await b.writeFile(path("hittable/env.json"), '{"a":"1"}\n');
    await b.remove(path("README.md"));
    await b.createFile(path("hittable/env.local.json"), "{}");
    const res = await b.commit("Update 3 files");
    expect(gh.committed).toHaveLength(1);
    expect(gh.committed[0].additions.map((a) => a.path)).toEqual(["hittable/env.json", "hittable/users/get.hit"]);
    expect(gh.committed[0].deletions).toEqual([{ path: "README.md" }]);
    expect(gh.files()["hittable/users/get.hit"]).toBe("one");
    expect(gh.files()["hittable/env.local.json"]).toBeUndefined();
    // The working copy is clear and based on the new commit; env.local.json stays.
    expect(b.baseSha()).toBe(res.sha);
    expect(b.changes()).toEqual([]);
    expect(b.hasLocalEnv()).toBe(true);
  });

  it("prefills the message from the changes", () => {
    expect(prefillMessage([{ path: "hittable/users/get.hit", status: "M" }])).toBe("Update hittable/users/get.hit");
    expect(prefillMessage([{ path: "a.hit", status: "A" }])).toBe("Add a.hit");
    expect(prefillMessage([{ path: "a.hit", status: "D" }])).toBe("Delete a.hit");
    expect(prefillMessage([
      { path: "a.hit", status: "M" },
      { path: "b.hit", status: "A" },
    ])).toBe("Update 2 files\n\n- Update a.hit\n- Add b.hit");
    expect(branchSlug("Update hittable/users/get.hit")).toBe("hittable/update-hittable-users-get-hit");
    expect(nextBranchName("hittable/x")).toBe("hittable/x-2");
    expect(nextBranchName("hittable/x-2")).toBe("hittable/x-3");
  });

  it("a protected branch is refused; the PR path creates a branch, commits and opens a PR", async () => {
    gh.branches.set("protected", gh.branches.get("main")!);
    const b2 = await open({ branch: "protected" });
    await b2.writeFile(path("README.md"), "new readme");
    await expect(b2.commit("Update README.md")).rejects.toMatchObject({ code: "protected" });
    expect(b2.status()).toEqual({ "README.md": "M" });

    const { pr } = await b2.commitToNewBranch({ branch: "hittable/update-readme", message: "Update README.md", title: "Update README.md", body: "" });
    expect(pr.number).toBe(1);
    expect(gh.pulls[0]).toMatchObject({ head: "hittable/update-readme", base: "protected", title: "Update README.md" });
    expect(gh.files("hittable/update-readme")["README.md"]).toBe("new readme");
    expect(b2.changes()).toEqual([]);
  });

  it("Initialize Hittable stages three added files", async () => {
    gh = fakeGitHub({ "README.md": "# site\n" });
    const b = await open();
    await b.initialize();
    expect(b.status()).toEqual({ "hittable/env.json": "A", "hittable/requests/example.hit": "A", "hittable/.gitignore": "A" });
    expect(JSON.parse(text(await b.readFile(path("hittable/requests/example.hit"))))).toMatchObject({ method: "GET", response: null });
    expect(EXAMPLE_HIT).toContain("jsonplaceholder.typicode.com/todos/1");
  });
});

describe("remote changes (FR-G6)", () => {
  it("no overlap: the base advances and the changes stay", async () => {
    const b = await open();
    await b.writeFile(path("hittable/users/get.hit"), "mine");
    gh.push({ "README.md": "theirs" });
    await b.checkRemote({ force: true });
    expect(b.sync().remote).toBeNull();
    expect(b.baseSha()).toBe(gh.branches.get("main"));
    expect(b.status()).toEqual({ "hittable/users/get.hit": "M" });
    expect(text(await b.readFile(["README.md"]))).toBe("theirs");
  });

  it("overlap: the file is marked C and a commit does not overwrite", async () => {
    const b = await open();
    await b.writeFile(path("hittable/users/get.hit"), "mine");
    gh.push({ "hittable/users/get.hit": "theirs" });
    await expect(b.commit("Update")).rejects.toMatchObject({ code: "head_moved" });
    expect(b.status()).toEqual({ "hittable/users/get.hit": "C" });
    expect(b.sync().remote?.conflicts).toEqual(["hittable/users/get.hit"]);
    expect(gh.files()["hittable/users/get.hit"]).toBe("theirs");
    await expect(b.commit("Update")).rejects.toThrow(/conflicts/);
    expect(await b.readTheirs("hittable/users/get.hit")).toBe("theirs");
    expect(await b.readWorking("hittable/users/get.hit")).toBe("mine");
  });

  it("Keep mine moves onto their commit with my text", async () => {
    const b = await open();
    await b.writeFile(path("hittable/users/get.hit"), "mine");
    gh.push({ "hittable/users/get.hit": "theirs" });
    await b.checkRemote({ force: true });
    await b.keepMine();
    expect(b.sync().remote).toBeNull();
    expect(b.status()).toEqual({ "hittable/users/get.hit": "M" });
    await b.commit("Mine wins");
    expect(gh.files()["hittable/users/get.hit"]).toBe("mine");
  });

  it("Take theirs drops my change to that file", async () => {
    const b = await open();
    await b.writeFile(path("hittable/users/get.hit"), "mine");
    await b.writeFile(path("README.md"), "also mine");
    gh.push({ "hittable/users/get.hit": "theirs" });
    await b.checkRemote({ force: true });
    await b.takeTheirs();
    expect(b.status()).toEqual({ "README.md": "M" });
    expect(text(await b.readFile(path("hittable/users/get.hit")))).toBe("theirs");
  });

  it("the same change on both sides is not a conflict", async () => {
    const b = await open();
    await b.writeFile(path("README.md"), "same");
    gh.push({ "README.md": "same" });
    await b.checkRemote({ force: true });
    expect(b.sync().remote).toBeNull();
    expect(b.status()).toEqual({});
  });

  it("an unchanged head is a 304 and nothing happens; checks are throttled", async () => {
    const b = await open();
    const sha = b.baseSha();
    await b.checkRemote();
    gh.push({ "README.md": "x" });
    await b.checkRemote(); // within a minute of opening: skipped
    expect(b.baseSha()).toBe(sha);
    await b.checkRemote({ force: true });
    expect(b.baseSha()).not.toBe(sha);
  });

  it("reopening an old working copy reconciles with the new head", async () => {
    const b = await open();
    await b.writeFile(path("README.md"), "mine");
    gh.push({ "README.md": "theirs" });
    const again = await open();
    expect(again.status()).toEqual({ "README.md": "C" });
  });
});

describe("branches", () => {
  it("create branch from current moves the working copy to it", async () => {
    const b = await open();
    await b.writeFile(path("README.md"), "wip");
    await b.createBranchFromCurrent("feature");
    expect(gh.branches.get("feature")).toBe(b.baseSha());
    const moved = await open({ branch: "feature" });
    expect(moved.status()).toEqual({ "README.md": "M" });
    expect((await open()).status()).toEqual({});
  });

  it("an empty repository and a missing branch are told apart", async () => {
    await expect(open({ branch: "nope" })).rejects.toMatchObject({ code: "branch_not_found" });
    gh.branches.clear();
    await expect(open()).rejects.toMatchObject({ code: "empty_repo" });
  });
});
