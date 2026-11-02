import { defineConfig, devices } from "@playwright/test";

// The suite runs against a production build. A local API stands in for the
// internet, so the proxy is allowed to reach private addresses here only.
// E2E_PORT: when something else already holds 3100.
const PORT = Number(process.env.E2E_PORT) || 3100;
export const API_PORT = 4100;
export const GITHUB_PORT = 4200;
const GITHUB_SPECS = /github-(auth|workspace)\.spec\.ts/;

export default defineConfig({
  testDir: "tests/e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["list"]] : "list",
  use: {
    baseURL: `http://localhost:${PORT}`,
    trace: "retain-on-failure",
  },
  projects: [
    // Clipboard permissions are a Chromium feature; the other engines test copy via the page.
    { name: "desktop", testIgnore: GITHUB_SPECS, use: { ...devices["Desktop Chrome"], permissions: ["clipboard-read", "clipboard-write"] } },
    { name: "mobile", testIgnore: GITHUB_SPECS, use: { ...devices["Pixel 7"], permissions: ["clipboard-read", "clipboard-write"] }, grep: /@mobile/ },
    // Playwright's Firefox does not start on macOS 27 yet ("Could not find profile
    // folder"); CI runs it on Linux. Set E2E_FIREFOX=1 to run it locally.
    ...(process.env.CI || process.env.E2E_FIREFOX ? [{ name: "firefox", testIgnore: GITHUB_SPECS, use: { ...devices["Desktop Firefox"] } }] : []),
    { name: "webkit", testIgnore: GITHUB_SPECS, use: { ...devices["Desktop Safari"] } },
    { name: "mobile-safari", testIgnore: GITHUB_SPECS, use: { ...devices["iPhone 14"] }, grep: /@mobile/ },
    // Both GitHub specs drive the one fake GitHub process and reset it between
    // tests, so they cannot run side by side: auth first, then the workspace.
    { name: "github-auth", testMatch: /github-auth\.spec\.ts/, use: { ...devices["Desktop Chrome"] } },
    { name: "github", testMatch: /github-workspace\.spec\.ts/, dependencies: ["github-auth"], use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: [
    {
      command: `node tests/e2e/fixtures/api-server.mjs ${API_PORT}`,
      port: API_PORT,
      reuseExistingServer: !process.env.CI,
    },
    {
      command: `node tests/e2e/fixtures/fake-github.mjs ${GITHUB_PORT}`,
      port: GITHUB_PORT,
      reuseExistingServer: !process.env.CI,
    },
    {
      command: `pnpm exec next start -p ${PORT}`,
      port: PORT,
      reuseExistingServer: !process.env.CI,
      // A short upstream timeout keeps the timeout test quick.
      env: {
        PROXY_ALLOW_PRIVATE_NETWORKS: "true",
        PROXY_RATE_LIMIT_PER_MINUTE: "0",
        PROXY_TIMEOUT_MS: "4000",
        // Sign in with GitHub against the fake GitHub fixture.
        GITHUB_CLIENT_ID: "test-client",
        GITHUB_CLIENT_SECRET: "test-secret",
        SESSION_SECRET: "e2e-session-secret-at-least-32-bytes-long",
        GITHUB_API_URL: `http://localhost:${GITHUB_PORT}/api`,
        GITHUB_OAUTH_URL: `http://localhost:${GITHUB_PORT}`,
        GITHUB_GRAPHQL_URL: `http://localhost:${GITHUB_PORT}/api/graphql`,
      },
    },
  ],
});
