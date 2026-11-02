// An in-memory GitHub behind the Client interface of services/github/githubBackend.ts.
import { GitHubError, type CommitInput, type TreeEntry } from "@/utils/github/api";
import { gitBlobSha, type Client } from "@/services/github/githubBackend";

type Files = Record<string, string>;

export function fakeGitHub(seed: Files, opts: { protectedBranches?: string[]; truncated?: boolean } = {}) {
  const blobs = new Map<string, Uint8Array>();
  const commits = new Map<string, Files>();
  const branches = new Map<string, string>();
  const pulls: { head: string; base: string; title: string; body: string }[] = [];
  const committed: CommitInput[] = [];
  let n = 0;
  const calls = { blob: 0, tree: 0 };

  const makeCommit = (files: Files) => {
    const sha = (++n).toString(16).padStart(40, "c");
    commits.set(sha, { ...files });
    return sha;
  };
  branches.set("main", makeCommit(seed));

  async function entries(files: Files): Promise<TreeEntry[]> {
    const out: TreeEntry[] = [];
    const dirs = new Set<string>();
    for (const [path, text] of Object.entries(files)) {
      const sha = await gitBlobSha(text);
      blobs.set(sha, new TextEncoder().encode(text));
      out.push({ path, type: "blob", sha, size: text.length });
      const parts = path.split("/");
      for (let i = 1; i < parts.length; i++) dirs.add(parts.slice(0, i).join("/"));
    }
    for (const d of dirs) out.push({ path: d, type: "tree", sha: `tree:${d}`, size: 0 });
    return out;
  }
  const headOf = (branch: string) => {
    const sha = branches.get(branch);
    if (!sha) throw new GitHubError("not_found", 404);
    return sha;
  };

  const client: Client = {
    async head(_o, _r, branch, etag) {
      const sha = headOf(branch);
      return etag === `"${sha}"` ? null : { sha, etag: `"${sha}"` };
    },
    async tree(_o, _r, sha, path) {
      calls.tree++;
      // Lazy listing of one directory: sha is "tree:<dir>" of the head commit.
      if (path !== undefined) {
        const files = commits.get(headOf("main"))!;
        const all = await entries(files);
        return { sha, truncated: false, entries: all.filter((e) => e.path.startsWith(path + "/") && !e.path.slice(path.length + 1).includes("/")) };
      }
      const files = commits.get(sha);
      if (!files) throw new GitHubError("not_found", 404);
      const all = await entries(files);
      return { sha, truncated: !!opts.truncated, entries: all };
    },
    async blob(_o, _r, sha) {
      calls.blob++;
      const b = blobs.get(sha);
      if (!b) throw new GitHubError("not_found", 404);
      return b;
    },
    async compare(_o, _r, base, head) {
      const a = commits.get(base)!;
      const b = commits.get(head)!;
      return { files: [...new Set([...Object.keys(a), ...Object.keys(b)])].filter((p) => a[p] !== b[p]), truncated: false };
    },
    async commit(_o, _r, input) {
      if (opts.protectedBranches?.includes(input.branch)) throw new GitHubError("protected", 403);
      if (headOf(input.branch) !== input.expectedHeadOid) throw new GitHubError("head_moved", 409);
      committed.push(input);
      const files = { ...commits.get(input.expectedHeadOid)! };
      for (const d of input.deletions) delete files[d.path];
      for (const a of input.additions) files[a.path] = Buffer.from(a.contents, "base64").toString("utf8");
      const sha = makeCommit(files);
      branches.set(input.branch, sha);
      return { sha, url: `https://github.com/o/r/commit/${sha}` };
    },
    async createBranch(_o, _r, name, fromSha) {
      if (branches.has(name)) throw new GitHubError("branch_exists", 409);
      branches.set(name, fromSha);
      return { name, sha: fromSha };
    },
    async pull(_o, _r, input) {
      pulls.push(input);
      return { number: pulls.length, url: `https://github.com/o/r/pull/${pulls.length}` };
    },
    async branches() {
      return [...branches].map(([name, sha]) => ({ name, sha, protected: !!opts.protectedBranches?.includes(name) }));
    },
  };

  return {
    client,
    calls,
    pulls,
    committed,
    branches,
    files: (branch = "main") => commits.get(headOf(branch))!,
    /** Someone else pushes. */
    push(change: Files, del: string[] = [], branch = "main") {
      const files = { ...commits.get(headOf(branch))!, ...change };
      for (const p of del) delete files[p];
      branches.set(branch, makeCommit(files));
    },
  };
}

export const META = {
  fullName: "octo/api-tests",
  owner: "octo",
  name: "api-tests",
  private: true,
  fork: false,
  archived: false,
  canPush: true,
  defaultBranch: "main",
  pushedAt: "2026-10-31T00:00:00Z",
  description: null,
};
