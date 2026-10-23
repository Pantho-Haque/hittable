import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { configure } from "@testing-library/dom";

// findBy / waitFor default to 1 s, which a busy CI machine can exceed.
configure({ asyncUtilTimeout: 5000 });

// Vitest runs without globals, so Testing Library cannot register its own
// cleanup; unmount rendered trees between tests here (DOM tests only).
afterEach(async () => {
  if (typeof document === "undefined") return;
  const { cleanup } = await import("@testing-library/react");
  cleanup();
});
