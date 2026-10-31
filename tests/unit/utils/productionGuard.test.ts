import { describe, it, expect } from "vitest";
import { isProduction, needsConfirmation, targetHost } from "@/utils/productionGuard";

describe("production guard", () => {
  it("is on for true / 1 / yes in env or secrets, off otherwise", () => {
    expect(isProduction({ HITTABLE_PRODUCTION: "true" })).toBe(true);
    expect(isProduction({}, { HITTABLE_PRODUCTION: "YES" })).toBe(true);
    expect(isProduction({ HITTABLE_PRODUCTION: "1" })).toBe(true);
    expect(isProduction({ HITTABLE_PRODUCTION: "false" })).toBe(false);
    expect(isProduction({})).toBe(false);
    expect(isProduction(undefined, undefined)).toBe(false);
  });

  it("only state-changing methods are confirmed", () => {
    const prod = { HITTABLE_PRODUCTION: "true" };
    for (const m of ["GET", "head", "OPTIONS"]) expect(needsConfirmation(m, prod)).toBe(false);
    for (const m of ["POST", "PUT", "PATCH", "DELETE"]) expect(needsConfirmation(m, prod)).toBe(true);
    expect(needsConfirmation("DELETE", {})).toBe(false);
  });

  it("names the host with variables filled in", () => {
    expect(targetHost("https://<<HOST>>/users", { HOST: "api.prod.io" })).toBe("api.prod.io");
    expect(targetHost("api.prod.io:8443/x")).toBe("api.prod.io:8443");
    expect(targetHost("<<NOPE>>")).toBe("<<NOPE>>");
  });
});
