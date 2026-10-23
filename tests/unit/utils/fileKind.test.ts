import { describe, expect, it } from "vitest";
import { MAX_EDITABLE_BYTES, classifyFile, humanSize, isSvg } from "@/utils/workspace/fileKind";

// Same rules and wording as the terminal app (shellapp/ui/screens/helpers_open.go).
describe("classifyFile", () => {
  const blob = (parts: BlobPart[]) => new Blob(parts);

  it("treats raster images as images, by extension, whatever their size", async () => {
    expect(await classifyFile("logo.PNG", blob(["x"]))).toEqual({ kind: "image", label: "PNG image" });
    expect(await classifyFile("photo.jpeg", blob(["x"]))).toEqual({ kind: "image", label: "JPG image" });
    expect(await classifyFile("anim.gif", blob([new Uint8Array(MAX_EDITABLE_BYTES + 1)]))).toMatchObject({ kind: "image" });
  });

  it("opens SVG, source and dotfiles as text", async () => {
    expect(await classifyFile("icon.svg", blob(["<svg/>"]))).toEqual({ kind: "text" });
    expect(isSvg("icon.SVG")).toBe(true);
    expect(await classifyFile(".env", blob(["A=1"]))).toEqual({ kind: "text" });
    expect(await classifyFile("empty.ts", blob([]))).toEqual({ kind: "text" });
  });

  it("calls anything with a NUL byte in its first 8 KB binary", async () => {
    expect(await classifyFile("tool", blob([new Uint8Array([0x7f, 0x45, 0, 1])]))).toEqual({ kind: "binary" });
    const lateNul = new Uint8Array(9000).fill(65);
    lateNul[8500] = 0;
    expect(await classifyFile("data.txt", blob([lateNul]))).toEqual({ kind: "text" });
  });

  it("refuses to load text over 1 MiB into the editor", async () => {
    expect(await classifyFile("huge.log", blob(["x".repeat(MAX_EDITABLE_BYTES)]))).toEqual({ kind: "text" });
    expect(await classifyFile("huge.log", blob(["x".repeat(MAX_EDITABLE_BYTES + 1)]))).toEqual({ kind: "toolarge" });
  });

  it("always opens .hit and env.json, however large: a saved response can be big", async () => {
    const big = blob(["x".repeat(MAX_EDITABLE_BYTES + 1)]);
    expect(await classifyFile("orders.hit", big)).toEqual({ kind: "text" });
    expect(await classifyFile("env.json", big)).toEqual({ kind: "text" });
  });

  it("formats sizes like the terminal app", () => {
    expect([humanSize(12), humanSize(1536), humanSize(1024 * 1024 + 10)]).toEqual(["12 B", "1.5 KB", "1.0 MB"]);
  });
});
