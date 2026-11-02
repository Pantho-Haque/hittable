import { ApiError, graphql, type Gh, type GraphqlError } from "./client";
import { isBranchName, isRepoPath, isSha } from "./handler";

export const MAX_COMMIT_BYTES = 4 * 1024 * 1024; // Vercel's request body limit is 4.5 MB
export const MAX_CHANGES = 500;

export type CommitInput = {
  branch: string;
  expectedHeadOid: string;
  message: string;
  additions: { path: string; contents: string }[];
  deletions: { path: string }[];
};

const BASE64 = /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/;
const bad = (message: string) => new ApiError(400, "invalid", message);

export function validateCommit(body: Record<string, unknown>): CommitInput {
  const { branch, expectedHeadOid, message } = body;
  const additions = body.additions ?? [];
  const deletions = body.deletions ?? [];
  if (!isBranchName(branch)) throw bad("branch is not a valid branch name");
  if (!isSha(expectedHeadOid)) throw bad("expectedHeadOid must be a commit SHA");
  if (typeof message !== "string" || !message.trim()) throw bad("message is required");
  if (!Array.isArray(additions) || !Array.isArray(deletions)) throw bad("additions and deletions must be arrays");
  const total = additions.length + deletions.length;
  if (total === 0) throw bad("Nothing to commit");
  if (total > MAX_CHANGES) throw bad(`A commit can hold at most ${MAX_CHANGES} changes`);
  const seen = new Set<string>();
  const checkPath = (p: unknown) => {
    if (!isRepoPath(p)) throw bad(`Invalid path: ${String(p).slice(0, 200)}`);
    if (seen.has(p)) throw bad(`Path listed twice: ${p}`);
    seen.add(p);
    return p;
  };
  return {
    branch,
    expectedHeadOid,
    message,
    additions: additions.map((a: { path?: unknown; contents?: unknown }) => {
      const path = checkPath(a?.path);
      if (typeof a.contents !== "string" || !BASE64.test(a.contents)) throw bad(`contents of ${path} must be base64`);
      return { path, contents: a.contents };
    }),
    deletions: deletions.map((d: { path?: unknown }) => ({ path: checkPath(d?.path) })),
  };
}

// GitHub reports createCommitOnBranch failures as GraphQL errors on a 200.
// The messages are not a documented API, so this matches on type where GitHub
// sets one and on the wording seen in practice otherwise:
//   head moved:  type STALE_DATA, or `Expected branch to point to "<oid>" but it did not`
//   protected:   "Protected branch update failed", "Repository rule violations",
//                "push declined", "Changes must be made through a pull request"
//   workflow:    "refusing to allow an OAuth App to create or update workflow ... without `workflow` scope"
//   forbidden:   type FORBIDDEN, "does not have the correct permissions", "Resource not accessible", "must have push access"
//   not found:   type NOT_FOUND, "Could not resolve to", "does not exist"
export function mapCommitErrors(errors: GraphqlError[]): ApiError {
  const msg = errors.map((e) => e.message).join("\n");
  const types = errors.map((e) => e.type ?? "");
  if (/workflow.*scope/i.test(msg)) return new ApiError(403, "workflow_scope", msg);
  if (types.includes("STALE_DATA") || /expected branch to point to/i.test(msg)) return new ApiError(409, "head_moved", msg);
  if (/protected branch|rule violation|push declined|through a pull request/i.test(msg)) return new ApiError(403, "protected", msg);
  if (types.includes("FORBIDDEN") || /correct permissions|not accessible|push access|permission/i.test(msg)) {
    return new ApiError(403, "forbidden", msg);
  }
  if (types.includes("NOT_FOUND") || /could not resolve to|does not exist/i.test(msg)) return new ApiError(404, "not_found", msg);
  return new ApiError(422, "commit_failed", msg);
}

const MUTATION = `mutation ($input: CreateCommitOnBranchInput!) {
  createCommitOnBranch(input: $input) { commit { oid url } }
}`;

export async function createCommit(gh: Gh, owner: string, repo: string, c: CommitInput): Promise<{ sha: string; url: string }> {
  const nl = c.message.indexOf("\n");
  const headline = (nl < 0 ? c.message : c.message.slice(0, nl)).trim();
  const bodyText = nl < 0 ? "" : c.message.slice(nl + 1).trim();
  if (!headline) throw bad("The first line of the message is empty");
  const res = await graphql<{ createCommitOnBranch: { commit: { oid: string; url: string } } | null }>(gh, MUTATION, {
    input: {
      branch: { repositoryNameWithOwner: `${owner}/${repo}`, branchName: c.branch },
      message: bodyText ? { headline, body: bodyText } : { headline },
      expectedHeadOid: c.expectedHeadOid,
      fileChanges: { additions: c.additions, deletions: c.deletions },
    },
  });
  if (res.errors?.length) throw mapCommitErrors(res.errors);
  const commit = res.data?.createCommitOnBranch?.commit;
  if (!commit) throw new ApiError(502, "upstream", "GitHub returned no commit");
  return { sha: commit.oid, url: commit.url };
}
