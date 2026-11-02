import { ApiError, ghJson, ghPages, json } from "@/utils/server/github/client";
import { githubRoute, isBranchName, isSha, readJson, repoPath } from "@/utils/server/github/handler";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

type Params = { owner: string; repo: string };
type GhBranch = { name: string; commit: { sha: string }; protected: boolean };

export const GET = githubRoute<Params>(async ({ gh, params }) => {
  const branches = await ghPages<GhBranch>(gh, `${repoPath(params)}/branches?per_page=100`, 1000);
  return json(branches.map((b) => ({ name: b.name, sha: b.commit.sha, protected: !!b.protected })));
});

export const POST = githubRoute<Params>(
  async ({ gh, req, params }) => {
    const path = repoPath(params);
    const { name, fromSha } = await readJson<{ name?: unknown; fromSha?: unknown }>(req);
    if (!isBranchName(name)) throw new ApiError(400, "invalid", "name is not a valid branch name");
    if (!isSha(fromSha)) throw new ApiError(400, "invalid", "fromSha must be a commit SHA");
    try {
      const ref = await ghJson<{ object: { sha: string } }>(gh, `${path}/git/refs`, {
        method: "POST",
        body: { ref: `refs/heads/${name}`, sha: fromSha },
      });
      return json({ name, sha: ref.object.sha }, 201);
    } catch (err) {
      if (err instanceof ApiError && err.status === 422 && /already exists/i.test(err.message)) {
        throw new ApiError(409, "branch_exists", err.message);
      }
      throw err;
    }
  },
  { mutating: true },
);
