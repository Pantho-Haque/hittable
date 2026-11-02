import { ApiError, ghJson, json } from "@/utils/server/github/client";
import { githubRoute, isBranchName, readJson, repoPath } from "@/utils/server/github/handler";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export const POST = githubRoute<{ owner: string; repo: string }>(
  async ({ gh, req, params }) => {
    const path = repoPath(params);
    const { head, base, title, body } = await readJson<Record<string, unknown>>(req);
    if (!isBranchName(head) || !isBranchName(base)) throw new ApiError(400, "invalid", "head and base must be branch names");
    if (typeof title !== "string" || !title.trim()) throw new ApiError(400, "invalid", "title is required");
    if (body !== undefined && typeof body !== "string") throw new ApiError(400, "invalid", "body must be a string");
    const pr = await ghJson<{ number: number; html_url: string }>(gh, `${path}/pulls`, {
      method: "POST",
      body: { head, base, title, body: body ?? "" },
    });
    return json({ number: pr.number, url: pr.html_url }, 201);
  },
  { mutating: true },
);
