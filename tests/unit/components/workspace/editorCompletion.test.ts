import { describe, it, expect } from "vitest";
import { CompletionContext } from "@codemirror/autocomplete";
import { EditorState } from "@codemirror/state";
import { ensureSyntaxTree } from "@codemirror/language";
import { json } from "@codemirror/lang-json";
import { documentWordCompletion, hitFileCompletion } from "@/components/workspace/editorCompletion";

// `|` marks the cursor.
function at(doc: string, { explicit = false, lang = true } = {}) {
  const pos = doc.indexOf("|");
  const text = doc.replace("|", "");
  const state = EditorState.create({ doc: text, extensions: lang ? [json()] : [] });
  if (lang) ensureSyntaxTree(state, text.length, 5000);
  return new CompletionContext(state, pos, explicit);
}
const labels = (r: ReturnType<typeof hitFileCompletion>) => r?.options.map((o) => o.label) ?? null;

describe(".hit completion", () => {
  it("suggests the six .hit keys at the top level, inserting quotes and a colon", () => {
    const r = hitFileCompletion(at('{\n  ur|\n}'));
    expect(labels(r)).toEqual(["method", "url", "headers", "params", "body", "response"]);
    expect(r!.options.find((o) => o.label === "url")!.apply).toBe('"url": ');
  });

  it("inside an open quote the key is inserted bare", () => {
    const r = hitFileCompletion(at('{\n  "me|\n}'));
    expect(r!.options[0].apply).toBe("method");
    expect(r!.from).toBe(5); // after the quote
  });

  it("suggests HTTP verbs after \"method\"", () => {
    const r = hitFileCompletion(at('{ "method": "P|" }'));
    expect(labels(r)).toEqual(["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"]);
    expect(r!.options[0].apply).toBeUndefined();
  });

  it("suggests real header names inside \"headers\"", () => {
    const r = hitFileCompletion(at('{ "headers": { "Auth|" } }'));
    expect(labels(r)).toContain("Authorization");
    expect(labels(r)).toContain("Content-Type");
  });

  it("suggests content types after Content-Type, any case", () => {
    const r = hitFileCompletion(at('{ "headers": { "content-type": "app|" } }'));
    expect(labels(r)).toContain("application/json");
  });

  it("offers nothing inside other values", () => {
    expect(hitFileCompletion(at('{ "body": { "x": "y|" } }'))).toBeNull();
  });

  it("stays quiet with nothing typed unless asked", () => {
    expect(hitFileCompletion(at("{ | }"))).toBeNull();
    expect(labels(hitFileCompletion(at("{ | }", { explicit: true })))).toContain("url");
  });
});

describe("buffer-word completion", () => {
  it("offers words of three or more letters already in the file, once each", () => {
    const r = documentWordCompletion(at("alpha beta alpha go alp|", { lang: false }));
    expect(r!.options.map((o) => o.label)).toEqual(["alpha", "beta"]);
  });

  it("never suggests the half-typed word itself", () => {
    const r = documentWordCompletion(at("zebra zeb|", { lang: false }));
    expect(r!.options.map((o) => o.label)).toEqual(["zebra"]);
  });

  it("returns nothing when there is nothing to offer", () => {
    expect(documentWordCompletion(at("ab|", { lang: false }))).toBeNull();
    expect(documentWordCompletion(at("alpha |", { lang: false }))).toBeNull();
  });
});
