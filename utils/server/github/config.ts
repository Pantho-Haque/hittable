import { ApiError } from "./client";

export type GithubConfig = {
  clientId: string;
  clientSecret: string;
  sessionSecret: string;
  apiUrl: string;
  oauthUrl: string;
  graphqlUrl: string;
  appUrl?: string;
};

const trimSlash = (s: string) => s.replace(/\/+$/, "");

// Read on every request (route handlers are dynamic), so a test or a redeploy
// can change it. Throws a 500 "config" error naming the variable, never its value.
export function githubConfig(env: NodeJS.ProcessEnv = process.env): GithubConfig {
  const missing = ["GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "SESSION_SECRET"].filter((k) => !env[k]);
  if (missing.length) {
    throw new ApiError(500, "config", `GitHub sign-in is not configured: set ${missing.join(", ")}`);
  }
  if (Buffer.byteLength(env.SESSION_SECRET!) < 32) {
    throw new ApiError(500, "config", "GitHub sign-in is not configured: SESSION_SECRET must be at least 32 bytes");
  }
  const apiUrl = trimSlash(env.GITHUB_API_URL || "https://api.github.com");
  return {
    clientId: env.GITHUB_CLIENT_ID!,
    clientSecret: env.GITHUB_CLIENT_SECRET!,
    sessionSecret: env.SESSION_SECRET!,
    apiUrl,
    oauthUrl: trimSlash(env.GITHUB_OAUTH_URL || "https://github.com"),
    graphqlUrl: trimSlash(env.GITHUB_GRAPHQL_URL || `${apiUrl}/graphql`),
    appUrl: env.APP_URL ? trimSlash(env.APP_URL) : undefined,
  };
}

// The public origin: APP_URL when set (behind a proxy that rewrites Host),
// else the request's own origin.
export function appOrigin(req: Request, cfg: GithubConfig): string {
  return cfg.appUrl ?? new URL(req.url).origin;
}
