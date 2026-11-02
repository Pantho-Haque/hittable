// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { screen, fireEvent, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { mountRunner, type RunnerOptions } from "../../helpers/runnerHarness";
import { rowsFromBody, rowsToBody } from "@/components/RequestForm/UploadFields";
import { headersForBodyType, inferBodyType } from "@/utils/bodyType";

const PNG = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0, 1, 2, 255]);
const KEY = "hittable/users/upload.hit";
const multipartReq = (body: Record<string, string>) => ({
  method: "POST", url: "https://api.example.com/up", params: "{}",
  headers: '{"Content-Type":"multipart/form-data"}', body: JSON.stringify(body),
});

let files: Record<string, Uint8Array>;
let fetchMock: ReturnType<typeof vi.fn>;
beforeEach(() => {
  files = { "hittable/users/files/avatar.png": PNG, "hittable/files/report.pdf": new Uint8Array([37, 80]) };
  fetchMock = vi.fn(async () => new Response(JSON.stringify({ status: 201, ok: true, data: {} })));
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

async function openBody(request: RunnerOptions["request"], extra: Partial<RunnerOptions> = {}) {
  const user = userEvent.setup();
  const r = mountRunner({ request, requestKey: KEY, files, ...extra });
  await user.click(screen.getByRole("tab", { name: "Body" }));
  return { user, ...r };
}

describe("body types", () => {
  it("multipart and file are inferred from the header and the body", () => {
    expect(inferBodyType('{"a":"1"}', '{"Content-Type":"multipart/form-data; boundary=x"}')).toBe("multipart");
    expect(inferBodyType("@files/a.png", "{}")).toBe("file");
    expect(inferBodyType("@fixtures/user.json", '{"Content-Type":"application/json"}')).toBe("file");
    expect(inferBodyType("@", "{}")).not.toBe("file");
    expect(inferBodyType("@@handle", "{}")).toBe("text");
  });
  it("Multipart sets its Content-Type; File leaves the type to the file", () => {
    expect(JSON.parse(headersForBodyType("{}", "multipart"))).toEqual({ "Content-Type": "multipart/form-data" });
    expect(JSON.parse(headersForBodyType('{"Content-Type":"multipart/form-data"}', "file"))).toEqual({});
    expect(JSON.parse(headersForBodyType('{"Content-Type":"image/png"}', "file"))).toEqual({ "Content-Type": "image/png" });
  });
  it("multipart rows round-trip, escaping text that starts with @", () => {
    const rows = [{ name: "a", file: false, value: "@x" }, { name: "f", file: true, value: "files/a.png" }, { name: "", file: false, value: "dropped" }];
    expect(JSON.parse(rowsToBody(rows))).toEqual({ a: "@@x", f: "@files/a.png" });
    expect(rowsFromBody(rowsToBody(rows))).toEqual(rows.slice(0, 2));
    expect(rowsFromBody("[1]")).toBeNull();
  });
});

describe("Multipart editor", () => {
  it("shows text and file rows; a file row knows whether the file is there", async () => {
    await openBody(multipartReq({ name: "Tanvir", avatar: "@files/avatar.png", gone: "@files/nope.png" }));
    expect(screen.getByRole("radio", { name: "Multipart" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByLabelText("Field 1 value")).toHaveValue("Tanvir");
    expect(screen.getByLabelText("Field 2 file")).toHaveValue("files/avatar.png");
    expect(screen.getByLabelText("Field 2 type")).toHaveValue("file");
    expect(screen.getByText("✓")).toBeInTheDocument();
    expect(screen.getByText(/Not found: hittable\/users\/files\/nope.png/)).toBeInTheDocument();
  });

  it("Add file, then choose one from the computer: copied into hittable/files and stored relative to the request", async () => {
    const { user, ctx } = await openBody(multipartReq({ name: "x" }));
    await user.click(screen.getByRole("button", { name: "Add file" }));
    await user.type(screen.getByLabelText("Field 2"), "doc");
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(input, new File([PNG], "me.png", { type: "image/png" }));
    await waitFor(() => expect(JSON.parse(ctx().formInput.body)).toEqual({ name: "x", doc: "@../files/me.png" }));
    expect(files["hittable/files/me.png"]).toEqual(PNG);
  });

  it("choose from the workspace", async () => {
    const { user, ctx } = await openBody(multipartReq({ doc: "@" + "x" }));
    await user.click(screen.getByRole("button", { name: /Choose/ }));
    await user.click(screen.getByRole("menuitem", { name: /From the workspace/ }));
    const dialog = screen.getByRole("dialog");
    await user.type(within(dialog).getByLabelText("Search files"), "report");
    await user.keyboard("{Enter}");
    expect(JSON.parse(ctx().formInput.body)).toEqual({ doc: "@../files/report.pdf" });
  });

  it("a dropped file is copied in", async () => {
    const { ctx } = await openBody(multipartReq({ doc: "@x" }));
    const cell = screen.getByLabelText("Field 1 file").parentElement!;
    const file = new File([PNG], "drop.png", { type: "image/png" });
    fireEvent.drop(cell, { dataTransfer: { files: [file], types: ["Files"] } });
    await waitFor(() => expect(JSON.parse(ctx().formInput.body)).toEqual({ doc: "@../files/drop.png" }));
  });

  it("sends the files as multipart through the proxy", async () => {
    const { user } = await openBody(multipartReq({ name: "Tanvir", avatar: "@files/avatar.png" }));
    await user.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalled());
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.headers).toEqual({ "X-Hittable": "1" });
    // jsdom's FormData isn't encoded by Node's Response, so the boundary is checked in the browser (e2e uploads spec).
    expect(JSON.parse((init.body as FormData).get("request") as string).url).toBe("https://api.example.com/up");
    expect((init.body as FormData).get("body")).toBeInstanceOf(Blob);
  });

  it("a missing file stops the send and says which", async () => {
    const { user } = await openBody(multipartReq({ gone: "@files/nope.png" }));
    await user.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText(/File not found: files\/nope.png/)).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("switching Form → Multipart keeps the fields", async () => {
    const { user, ctx } = await openBody({ method: "POST", url: "https://x.io", params: "{}", headers: '{"Content-Type":"application/x-www-form-urlencoded"}', body: "a=1&b=two" });
    await user.click(screen.getByRole("radio", { name: "Multipart" }));
    expect(JSON.parse(ctx().formInput.body)).toEqual({ a: "1", b: "two" });
    expect(JSON.parse(ctx().formInput.headers)).toEqual({ "Content-Type": "multipart/form-data" });
  });
});

describe("File body", () => {
  it("picking File starts empty and stays File; a typed path is the body", async () => {
    const { user, ctx } = await openBody("Create user");
    await user.click(screen.getByRole("radio", { name: "File" }));
    expect(ctx().formInput.body).toBe("");
    expect(screen.getByRole("radio", { name: "File" })).toHaveAttribute("aria-checked", "true");
    await user.type(screen.getByLabelText("Body file"), "../files/report.pdf");
    expect(ctx().formInput.body).toBe("@../files/report.pdf");
    expect(screen.getByText("✓")).toBeInTheDocument();
    expect(screen.getByText("application/pdf")).toBeInTheDocument();
  });

  it("leaving File clears the reference", async () => {
    const { user, ctx } = await openBody({ method: "PUT", url: "https://x.io", params: "{}", headers: "{}", body: "@files/avatar.png" });
    expect(screen.getByRole("radio", { name: "File" })).toHaveAttribute("aria-checked", "true");
    await user.click(screen.getByRole("radio", { name: "Text" }));
    expect(ctx().formInput.body).toBe("");
  });

  it("a read-only workspace can't copy files in", async () => {
    await openBody({ method: "PUT", url: "https://x.io", params: "{}", headers: "{}", body: "@files/avatar.png" }, { readOnly: true });
    expect(screen.queryByRole("button", { name: /Choose/ })).toBeNull();
    expect(screen.getByLabelText("Body file")).toHaveAttribute("readonly");
  });
});
