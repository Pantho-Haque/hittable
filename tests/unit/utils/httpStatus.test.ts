import { describe, it, expect } from "vitest";
import { formatSize, reasonPhrase, statusKind } from "@/utils/httpStatus";

describe("status presentation", () => {
  it("classes: 2xx success, 1xx/3xx info, 4xx warning, 5xx danger", () => {
    expect([201, 302, 101, 404, 503, 0].map(statusKind)).toEqual(["success", "info", "info", "warning", "danger", "danger"]);
  });

  it("uses the server's reason phrase, and fills one in when HTTP/2 sends none", () => {
    expect(reasonPhrase(201, "")).toBe("Created");
    expect(reasonPhrase(302)).toBe("Found");
    expect(reasonPhrase(404, "  ")).toBe("Not Found");
    expect(reasonPhrase(200, "Fine Thanks")).toBe("Fine Thanks");
    expect(reasonPhrase(599)).toBe("");
  });

  it("sizes in B, KB and MB", () => {
    expect(formatSize(1000)).toBe("1000 B");
    expect(formatSize(1536)).toBe("1.5 KB");
    expect(formatSize(3 * 1024 * 1024)).toBe("3.0 MB");
  });
});
