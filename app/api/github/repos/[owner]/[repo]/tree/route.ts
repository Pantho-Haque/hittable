import { ApiError, ghJson, json } from "@/utils/server/github/client";
import { githubRoute, isRepoPath, isSha, repoPath } from "@/utils/server/github/handler";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

type GhTree = { sha: string; truncated: boolean; tree: { path: string; type: string; sha: string; size?: number }[] };

// ?sha=<commit or tree> : the whole tree, recursive (GitHub dereferences a commit to its tree).
// ?sha=<tree>&path=dir  : one directory, not recursive, paths prefixed with dir/ (lazy
//                         loading when the recursive answer came back truncated).
// Submodules (type "commit") are left out.
export const GET = githubRoute<{ owner: string; repo: string }>(async ({ gh, req, params }) => {
  const q = new URL(req.url).searchParams;
  const sha = q.get("sha");
  const dir = q.get("path");
  if (!isSha(sha)) throw new ApiError(400, "invalid", "sha must be a commit or tree SHA");
  if (dir !== null && !isRepoPath(dir)) throw new ApiError(400, "invalid", "path must be a repository-relative directory");
  const t = await ghJson<GhTree>(gh, `${repoPath(params)}/git/trees/${sha}${dir === null ? "?recursive=1" : ""}`);
  return json({
    sha: t.sha,
    truncated: !!t.truncated,
    entries: t.tree
      .filter((e) => e.type === "blob" || e.type === "tree")
      .map((e) => ({ path: dir ? `${dir}/${e.path}` : e.path, type: e.type, sha: e.sha, size: e.size ?? 0 })),
  });
});
