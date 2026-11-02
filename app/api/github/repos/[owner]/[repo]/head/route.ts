import { ApiError, ghFetch, json } from "@/utils/server/github/client";
import { encodePath, githubRoute, isBranchName, repoPath } from "@/utils/server/github/handler";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// Conditional: pass the client's If-None-Match through, so an unchanged head
// is a 304 that does not count against the rate limit.
export const GET = githubRoute<{ owner: string; repo: string }>(async ({ gh, req, params }) => {
  const branch = new URL(req.url).searchParams.get("branch");
  if (!isBranchName(branch)) throw new ApiError(400, "invalid", "branch is required");
  const inm = req.headers.get("if-none-match");
  const res = await ghFetch(gh, `${repoPath(params)}/branches/${encodePath(branch)}`, {
    headers: inm ? { "If-None-Match": inm } : {},
  });
  const etag = res.headers.get("etag");
  const headers: Record<string, string> = { "Cache-Control": "private, no-cache" };
  if (etag) headers.ETag = etag;
  if (res.status === 304) return new Response(null, { status: 304, headers });
  const b = (await res.json()) as { commit: { sha: string } };
  return json({ sha: b.commit.sha }, 200, headers);
});
