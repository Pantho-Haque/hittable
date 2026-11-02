// File uploads from the web app: the body that is built, how it is sent, and
// what Copy as / curl import make of it. The parse rules are in fileBody.test.ts.
import { describe, it, expect, vi, afterEach } from "vitest";
import { buildUploadBody } from "@/utils/uploadBody";
import { hittableProxy } from "@/utils/hittableProxy";
import { generateCode } from "@/utils/codegen";
import { curlConverter } from "@/utils/curlConverter";
import type { THittableCurlJson } from "@/types";

const PNG = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0, 1, 2, 255]);
const HIT = ["hittable", "users", "upload.hit"];
const files: Record<string, Uint8Array> = {
  "hittable/users/files/avatar.png": PNG,
  "hittable/files/report.pdf": new TextEncoder().encode("%PDF-1.7"),
};
const readFile = vi.fn(async (p: string[]) => {
  const f = files[p.join("/")];
  if (!f) throw new DOMException("gone", "NotFoundError");
  return f;
});
const ctx = { hitPath: HIT, readFile };

async function parts(u: Awaited<ReturnType<typeof buildUploadBody>>) {
  if (u?.kind !== "bytes") throw new Error("expected bytes");
  return new Response(u.bytes, { headers: { "content-type": u.headers["Content-Type"] ?? u.headers["content-type"] } }).formData();
}

describe("buildUploadBody", () => {
  it("plain bodies are left alone", async () => {
    expect(await buildUploadBody('{"a":1}', { "Content-Type": "application/json" }, ctx)).toBeNull();
    expect(await buildUploadBody("", {}, ctx)).toBeNull();
  });

  it("@@ sends a literal @ body", async () => {
    expect(await buildUploadBody("@@handle", {}, ctx)).toEqual({ kind: "text", body: "@handle" });
  });

  it("a binary body is the file's bytes; Content-Type: the header, else the type option, else a guess", async () => {
    const own = await buildUploadBody("@files/avatar.png", { "content-type": "image/x-custom" }, ctx);
    expect(own).toEqual({ kind: "bytes", bytes: PNG, headers: { "content-type": "image/x-custom" } });
    expect((await buildUploadBody("@files/avatar.png;type=image/webp", {}, ctx)) as { headers: object }).toMatchObject({ headers: { "Content-Type": "image/webp" } });
    expect((await buildUploadBody("@../files/report.pdf", { Accept: "*/*" }, ctx)) as { headers: object }).toMatchObject({ headers: { Accept: "*/*", "Content-Type": "application/pdf" } });
  });

  it("multipart: text and file parts in order, a real boundary, filename and type", async () => {
    const u = await buildUploadBody(
      JSON.stringify({ name: "Tanvir", avatar: "@files/avatar.png;filename=me.png", handle: "@@t" }),
      { "Content-Type": "multipart/form-data", "X-Trace": "1" },
      ctx,
    );
    expect(u?.kind === "bytes" && u.headers["Content-Type"]).toMatch(/^multipart\/form-data; boundary=/);
    expect(u?.kind === "bytes" && u.headers["X-Trace"]).toBe("1");
    const form = await parts(u);
    expect([...form.keys()]).toEqual(["name", "avatar", "handle"]);
    expect(form.get("name")).toBe("Tanvir");
    expect(form.get("handle")).toBe("@t");
    const file = form.get("avatar") as File;
    expect(file.name).toBe("me.png");
    expect(file.type).toBe("image/png");
    expect(new Uint8Array(await file.arrayBuffer())).toEqual(PNG);
  });

  it("a missing file, a path outside the workspace, or no workspace: an error naming the path", async () => {
    await expect(buildUploadBody("@files/nope.png", {}, ctx)).rejects.toThrow("File not found: files/nope.png");
    await expect(buildUploadBody("@../../../etc/passwd", {}, ctx)).rejects.toThrow(/leaves the workspace/);
    await expect(buildUploadBody("@/etc/passwd", {}, ctx)).rejects.toThrow(/terminal app/);
    await expect(buildUploadBody("@files/avatar.png", {}, {})).rejects.toThrow(/in a workspace/);
  });
});

describe("sending an upload", () => {
  afterEach(() => vi.unstubAllGlobals());
  const req = (over: Partial<THittableCurlJson>): THittableCurlJson => ({ method: "POST", url: "https://api.example.com/up", headers: "{}", params: "{}", body: "", ...over });

  it("goes to the proxy as a multipart envelope with X-Hittable, never base64", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ status: 201, ok: true, data: {} }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await hittableProxy(req({ body: "@<<DIR>>/avatar.png", headers: '{"Content-Type":"image/png"}' }), { DIR: "files" }, undefined, false, undefined, ctx);
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/proxy");
    expect(init.headers).toEqual({ "X-Hittable": "1" });
    const form = init.body as FormData;
    expect(JSON.parse(form.get("request") as string)).toEqual({ url: "https://api.example.com/up", method: "POST", headers: '{"Content-Type":"image/png"}' });
    expect(new Uint8Array(await (form.get("body") as Blob).arrayBuffer())).toEqual(PNG);
  });

  it("a GET never reads files", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("{}")));
    readFile.mockClear();
    await hittableProxy(req({ method: "GET", body: "@files/avatar.png" }), {}, undefined, false, undefined, ctx);
    expect(readFile).not.toHaveBeenCalled();
  });

  it("a 413 (e.g. Vercel's own page) says how big the upload was", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("<html>Request Entity Too Large</html>", { status: 413 })));
    const r = await hittableProxy(req({ body: "@files/avatar.png" }), {}, undefined, false, undefined, ctx);
    expect(r).toMatchObject({ status: 413, ok: false, error: expect.stringMatching(/The upload is 8 B.*terminal app/) });
  });
});

describe("Copy as for uploads", () => {
  const multipart: THittableCurlJson = {
    method: "POST", url: "https://api.example.com/up", params: "{}",
    headers: '{"Content-Type":"multipart/form-data","Authorization":"Bearer t"}',
    body: JSON.stringify({ name: "Tanvir", avatar: "@files/a.png;type=image/png", note: "@@x" }),
  };
  const binary: THittableCurlJson = { method: "PUT", url: "https://s3.example.com/k", params: "{}", headers: "{}", body: "@files/report.pdf" };

  it("cURL: -F for files, --form-string for text, no multipart header; --data-binary for a file body", () => {
    const c = generateCode("curl", multipart);
    expect(c).toContain("-F 'avatar=@files/a.png;type=image/png'");
    expect(c).toContain("--form-string 'name=Tanvir'");
    expect(c).toContain("--form-string 'note=@x'");
    expect(c).toContain("-H 'Authorization: Bearer t'");
    expect(c).not.toContain("multipart/form-data");
    const b = generateCode("curl", binary);
    expect(b).toContain("--data-binary '@files/report.pdf'");
    expect(b).toContain("-H 'Content-Type: application/pdf'");
  });

  it("HTTPie, fetch and Python", () => {
    expect(generateCode("httpie", multipart)).toMatch(/http --form POST[\s\S]*'avatar@files\/a\.png;type=image\/png'/);
    expect(generateCode("httpie", binary)).toContain("< 'files/report.pdf'");
    const f = generateCode("fetch", multipart);
    expect(f).toContain('form.append("avatar", await openAsBlob("files/a.png", { type: "image/png" }), "a.png");');
    expect(f).toContain("body: form,");
    expect(generateCode("fetch", binary)).toContain('body: await openAsBlob("files/report.pdf"),');
    const p = generateCode("python", multipart);
    expect(p).toContain('"avatar": ("a.png", open("files/a.png", "rb"), "image/png"),');
    expect(p).toContain("files=files");
    expect(generateCode("python", binary)).toContain('payload = open("files/report.pdf", "rb")');
  });

  it("curl import reads them back", () => {
    const m = curlConverter(generateCode("curl", multipart));
    expect(JSON.parse(m.headers)).toMatchObject({ "Content-Type": "multipart/form-data" });
    expect(JSON.parse(m.body)).toEqual({ name: "Tanvir", avatar: "@files/a.png;type=image/png", note: "@@x" });
    expect(curlConverter("curl -X PUT https://s3.example.com/k --data-binary @files/report.pdf").body).toBe("@files/report.pdf");
    const t = curlConverter("curl -T files/report.pdf https://s3.example.com/k");
    expect(t).toMatchObject({ method: "PUT", body: "@files/report.pdf" });
  });
});
