import { describe, it, expect } from "vitest";
import { fuzzyScore } from "@/utils/fuzzy";

describe("fuzzyScore", () => {
  it("returns null when the letters are not there in order", () => {
    expect(fuzzyScore("xyz", "Environment")).toBeNull();
    expect(fuzzyScore("tnx", "Environment")).toBeNull();
  });

  it("matches case-insensitively and an empty query matches everything", () => {
    expect(fuzzyScore("ENV", "environment")).not.toBeNull();
    expect(fuzzyScore("", "anything")).toBe(0);
  });

  it("ranks a prefix above a word start above a mid-word substring above a scattered match", () => {
    const prefix = fuzzyScore("env", "Environment variables")!;
    const wordStart = fuzzyScore("env", "Edit env file")!;
    const midWord = fuzzyScore("env", "Reenvision")!;
    const scattered = fuzzyScore("env", "Event view")!;
    expect(prefix).toBeGreaterThan(wordStart);
    expect(wordStart).toBeGreaterThan(midWord);
    expect(midWord).toBeGreaterThan(scattered);
  });

  it("prefers a later word-start occurrence over an earlier mid-word one", () => {
    expect(fuzzyScore("user", "superuser / user list")!).toBeGreaterThan(fuzzyScore("user", "superuser")!);
  });

  it("needs every space-separated word to match", () => {
    expect(fuzzyScore("users admin", "Create user Users / Admin")).not.toBeNull();
    expect(fuzzyScore("users billing", "Create user Users / Admin")).toBeNull();
  });

  it("rewards consecutive letters in a scattered match", () => {
    expect(fuzzyScore("crus", "create user")!).toBeLessThan(fuzzyScore("crus", "crusty")!);
    expect(fuzzyScore("cu", "create user")).not.toBeNull();
  });
});
