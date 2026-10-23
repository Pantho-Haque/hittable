import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

export default defineConfig({
  resolve: { alias: { "@": fileURLToPath(new URL(".", import.meta.url)) } },
  test: {
    include: ["tests/unit/**/*.test.{ts,tsx}"],
    environment: "node", // component tests opt in with: // @vitest-environment jsdom
    setupFiles: ["tests/unit/setup.ts"],
    restoreMocks: true,
    // jsdom + user-event tests are slow on a loaded machine; fail on hangs, not on load.
    testTimeout: 20_000,
  },
});
