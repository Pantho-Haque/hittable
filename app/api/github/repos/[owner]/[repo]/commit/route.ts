import { json } from "@/utils/server/github/client";
import { MAX_COMMIT_BYTES, createCommit, validateCommit } from "@/utils/server/github/commit";
import { githubRoute, readJson, repoPath } from "@/utils/server/github/handler";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export const POST = githubRoute<{ owner: string; repo: string }>(
  async ({ gh, req, params }) => {
    repoPath(params);
    const input = validateCommit(await readJson<Record<string, unknown>>(req, MAX_COMMIT_BYTES));
    return json(await createCommit(gh, params.owner, params.repo, input));
  },
  { mutating: true },
);
