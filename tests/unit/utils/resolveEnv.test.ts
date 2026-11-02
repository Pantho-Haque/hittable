import { describe, it, expect } from "vitest";
import { resolveEnv } from "@/utils/resolveEnv";
import type { THittableCurlJson } from "@/types";

describe("resolveEnv: <<KEY>> templates at send time", () => {
  const form: THittableCurlJson = {
    method: "POST", url: "<<BASE>>/users/<<ID>>", headers: '{"Authorization":"Bearer <<TOKEN>>"}',
    body: '{"name":"<<NAME>>","again":"<<NAME>>"}', params: '{"q":"<<Q>>"}',
  };

  it("replaces every occurrence in every field", () => {
    expect(resolveEnv(form, { BASE: "https://x.dev", ID: "7", TOKEN: "abc", NAME: "Ada", Q: "x" })).toEqual({
      method: "POST", url: "https://x.dev/users/7", headers: '{"Authorization":"Bearer abc"}',
      body: '{"name":"Ada","again":"Ada"}', params: '{"q":"x"}',
    });
  });

  it("leaves unknown keys as-is so the user notices", () => {
    expect(resolveEnv(form, { BASE: "https://x.dev" }).url).toBe("https://x.dev/users/<<ID>>");
  });

  it("an empty value resolves to empty, not to the template", () => {
    expect(resolveEnv(form, { ID: "" }).url).toBe("<<BASE>>/users/");
  });

  it("without an env returns the input untouched, and never mutates it", () => {
    expect(resolveEnv(form, undefined)).toBe(form);
    const copy = { ...form };
    resolveEnv(form, { BASE: "x" });
    expect(form).toEqual(copy);
  });

  it("is a single pass: values containing templates or $ patterns are inserted literally", () => {
    const out = resolveEnv({ ...form, url: "<<A>>|<<B>>" }, { A: "<<B>>", B: "$&$1" });
    expect(out.url).toBe("<<B>>|$&$1");
  });

  it("does not resolve keys from Object.prototype", () => {
    expect(resolveEnv({ ...form, url: "<<constructor>>/<<toString>>" }, { A: "1" }).url).toBe("<<constructor>>/<<toString>>");
  });

  it("only matches word-character names", () => {
    expect(resolveEnv({ ...form, url: "<<a-b>> << A >>" }, { "a-b": "x", A: "y" }).url).toBe("<<a-b>> << A >>");
  });
});
