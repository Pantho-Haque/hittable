import { ghPages, json } from "@/utils/server/github/client";
import { githubRoute } from "@/utils/server/github/handler";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

type GhRepo = {
  full_name: string;
  name: string;
  owner: { login: string };
  private: boolean;
  fork: boolean;
  archived: boolean;
  permissions?: { push?: boolean };
  default_branch: string;
  pushed_at: string | null;
  description: string | null;
};

export const GET = githubRoute(async ({ gh }) => {
  const repos = await ghPages<GhRepo>(
    gh,
    "/user/repos?per_page=100&sort=pushed&affiliation=owner,collaborator,organization_member",
    1000,
  );
  return json(
    repos.map((r) => ({
      fullName: r.full_name,
      owner: r.owner.login,
      name: r.name,
      private: r.private,
      fork: r.fork,
      archived: r.archived,
      canPush: !!r.permissions?.push,
      defaultBranch: r.default_branch,
      pushedAt: r.pushed_at,
      description: r.description,
    })),
  );
});
