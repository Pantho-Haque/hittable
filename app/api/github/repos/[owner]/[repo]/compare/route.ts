import { ApiError, ghJson, json } from "@/utils/server/github/client";
import { githubRoute, isBranchName, repoPath } from "@/utils/server/github/handler";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// Paths changed between two commits; a rename lists both the old and new path.
// GitHub lists at most 300 files, so truncated: true means "assume anything changed".
export const GET = githubRoute<{ owner: string; repo: string }>(async ({ gh, req, params }) => {
  const q = new URL(req.url).searchParams;
  const base = q.get("base");
  const head = q.get("head");
  if (!isBranchName(base) || !isBranchName(head)) throw new ApiError(400, "invalid", "base and head are required");
  const c = await ghJson<{ files?: { filename: string; previous_filename?: string }[] }>(
    gh,
    `${repoPath(params)}/compare/${encodeURIComponent(base)}...${encodeURIComponent(head)}`,
  );
  const files = c.files ?? [];
  const paths = new Set(files.flatMap((f) => (f.previous_filename ? [f.filename, f.previous_filename] : [f.filename])));
  return json({ files: [...paths], truncated: files.length >= 300 });
});
