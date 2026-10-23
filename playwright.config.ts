import { defineConfig, devices } from "@playwright/test";

// The suite runs against a production build. A local API stands in for the
// internet, so the proxy is allowed to reach private addresses here only.
const PORT = 3100;
export const API_PORT = 4100;

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
    { name: "desktop", use: { ...devices["Desktop Chrome"], permissions: ["clipboard-read", "clipboard-write"] } },
    { name: "mobile", use: { ...devices["Pixel 7"], permissions: ["clipboard-read", "clipboard-write"] }, grep: /@mobile/ },
    // Playwright's Firefox does not start on macOS 27 yet ("Could not find profile
    // folder"); CI runs it on Linux. Set E2E_FIREFOX=1 to run it locally.
    ...(process.env.CI || process.env.E2E_FIREFOX ? [{ name: "firefox", use: { ...devices["Desktop Firefox"] } }] : []),
    { name: "webkit", use: { ...devices["Desktop Safari"] } },
    { name: "mobile-safari", use: { ...devices["iPhone 14"] }, grep: /@mobile/ },
  ],
  webServer: [
    {
      command: `node tests/e2e/fixtures/api-server.mjs ${API_PORT}`,
      port: API_PORT,
      reuseExistingServer: !process.env.CI,
    },
    {
      command: `pnpm exec next start -p ${PORT}`,
      port: PORT,
      reuseExistingServer: !process.env.CI,
      // A short upstream timeout keeps the timeout test quick.
      env: { PROXY_ALLOW_PRIVATE_NETWORKS: "true", PROXY_RATE_LIMIT_PER_MINUTE: "0", PROXY_TIMEOUT_MS: "4000" },
    },
  ],
});
