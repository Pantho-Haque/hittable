// Landing page copy. Specific and short: every line names something the app does.

export const GITHUB_REPO_URL = "https://github.com/Pantho-Haque/hittable";
export const AUTHOR_URL = "https://github.com/Pantho-Haque";
export const SIGN_IN_URL = "/api/github/login?next=/hittable";
export const INSTALL_COMMAND = "curl -fsSL https://raw.githubusercontent.com/Pantho-Haque/hittable/main/install.sh | sh";

// How it works: three commits on one branch. `artifact` is what you see at that step.
export const STEPS: { sha: string; title: string; desc: string; artifact: string }[] = [
  {
    sha: "a1c90e2",
    title: "Sign in with GitHub",
    desc: "One OAuth consent. No new account, no password to keep.",
    artifact: "scope: repo read:user",
  },
  {
    sha: "5d7b3f0",
    title: "Open a repository",
    desc: "Pick any repo you can see. Its hittable/ folder becomes the request tree.",
    artifact: "acme/payments-api · main",
  },
  {
    sha: "3f2a9c1",
    title: "Send, edit, commit",
    desc: "Run requests, change them, review the diff and commit to a branch.",
    artifact: "M hittable/users/create.hit",
  },
];

// `token` is a real fragment of the feature, shown in mono instead of an icon.
export const FEATURES: { token: string; title: string; desc: string }[] = [
  { token: "Ctrl/Cmd K", title: "Command palette", desc: "Every request, file and action from one search box." },
  { token: "none · json · text · form", title: "Request runner", desc: "Method, URL, params, headers and body, with the body type inferred from the file." },
  { token: "<<API_KEY>>", title: "Variables", desc: "Values come from hittable/env.json. Secrets go in env.local.json, which git ignores." },
  { token: "HITTABLE_PRODUCTION", title: "Production guard", desc: "Anything other than GET, HEAD or OPTIONS asks before it reaches a production environment." },
  { token: "curl · fetch · python · httpie", title: "Copy as code", desc: "Copy any request as cURL, fetch, Python requests or HTTPie." },
  { token: "curl -X POST …", title: "Paste curl", desc: "Paste a curl command into the URL bar and it becomes a request." },
  { token: "#share=…", title: "Share links", desc: "The request travels in the link's fragment, which no server ever receives." },
  { token: "$.data[*].id", title: "JSONPath filter", desc: "Cut a large response down to the field you need." },
  { token: "before ↔ after", title: "Compare responses", desc: "Diff a response with the previous one, key by key." },
  { token: "A  M  D", title: "Git changes", desc: "See which .hit files changed, with a diff, before you commit." },
];

export const SCOPES: { name: string; why: string }[] = [
  { name: "repo", why: "Read and write your repositories, to load .hit files and commit your changes back." },
  { name: "read:user", why: "Your name and avatar, for the account menu." },
];

export const NOT_REQUESTED = ["admin:org", "workflow", "delete_repo", "admin:repo_hook", "gist", "user:email"];

export const PRIVACY_FACTS: { term: string; detail: string }[] = [
  { term: "Token", detail: "Kept on the server in an encrypted, httpOnly cookie. Browser JavaScript never sees it." },
  { term: "Storage", detail: "No database. Your requests live in your repo; unsaved edits stay on this device." },
  { term: "Revoke", detail: "Sign out revokes the grant. You can also remove it any time in GitHub Settings → Applications." },
  { term: "Organisations", detail: "An organisation may need to approve the app before its repositories show up." },
];
