import fs from "node:fs";
import pako from "pako";
import type { Page } from "@playwright/test";
import {
  test,
  expect,
  API,
  sidebar,
  sidebarItem,
  rowAction,
  dialog,
  urlInput,
  methodSelect,
  send,
  response,
  statusBadge,
  collection,
  route,
  folder,
  clipboardText,
  editor,
} from "./fixtures/test";

const postman = {
  info: { name: "Petstore", schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" },
  variable: [{ key: "baseUrl", value: API }],
  auth: { type: "bearer", bearer: [{ key: "token", value: "pm-token", type: "string" }] },
  item: [
    {
      name: "Users",
      item: [
        {
          name: "List users",
          request: {
            method: "GET",
            header: [{ key: "Accept", value: "application/json" }],
            url: { raw: "{{baseUrl}}/users?limit=2", host: ["{{baseUrl}}"], path: ["users"], query: [{ key: "limit", value: "2" }] },
          },
        },
        {
          name: "Create user",
          request: {
            method: "POST",
            header: [{ key: "Content-Type", value: "application/json" }],
            body: { mode: "raw", raw: '{"name":"Pm"}' },
            url: "{{baseUrl}}/users",
          },
        },
      ],
    },
    {
      name: "Echo",
      event: [{ listen: "test", script: { exec: ["pm.test('x', () => {})"] } }],
      request: {
        method: "GET",
        header: [
          { key: "X-On", value: "1" },
          { key: "X-Off", value: "1", disabled: true },
        ],
        url: "{{baseUrl}}/echo",
      },
    },
  ],
};

const insomnia = {
  _type: "export",
  __export_format: 4,
  resources: [
    { _id: "wrk_1", _type: "workspace", name: "Insomnia Shop" },
    { _id: "env_1", _type: "environment", parentId: "wrk_1", name: "Base", data: { host: API } },
    { _id: "fld_1", _type: "request_group", parentId: "wrk_1", name: "Users" },
    { _id: "req_1", _type: "request", parentId: "fld_1", name: "Get user", method: "GET", url: "{{ _.host }}/users/3", headers: [{ name: "X-Ins", value: "1" }], parameters: [] },
    {
      _id: "req_2",
      _type: "request",
      parentId: "wrk_1",
      name: "Echo post",
      method: "POST",
      url: "{{ _.host }}/echo",
      body: { mimeType: "application/json", text: '{"from":"insomnia"}' },
      headers: [{ name: "Content-Type", value: "application/json" }],
    },
  ],
};

async function importText(page: Page, text: string) {
  await page.getByRole("button", { name: "Import" }).click();
  const d = dialog(page, "Import");
  await d.getByRole("textbox").fill(text);
  await d.getByRole("button", { name: "Import", exact: true }).click();
  return d;
}

async function openExport(page: Page, name: string) {
  await page.goto("/hittable");
  await rowAction(page, name, "Export");
  const d = dialog(page, "Export");
  await expect(d.getByText(`Export "${name}" in your preferred format`)).toBeVisible();
  return d;
}

const decompress = (s: string) =>
  JSON.parse(new TextDecoder().decode(pako.inflate(Buffer.from(s.replace(/-/g, "+").replace(/_/g, "/"), "base64"))));

test.describe("import", () => {
  test.beforeEach(async ({ seed }) => {
    await seed({ collections: [] });
  });

  test("a Postman v2.1 collection: folders, variables, inherited auth, disabled headers", async ({ page }) => {
    await page.goto("/hittable");
    const d = await importText(page, JSON.stringify(postman));
    await expect(d).toBeHidden();
    await expect(page.getByText("Imported from Postman")).toBeVisible();
    await expect(page.getByText(/scripts \(pre-request\/test\) are not supported/)).toBeVisible();

    await sidebarItem(page, "Petstore").click();
    await expect(sidebarItem(page, "Users")).toContainText("2");
    await sidebarItem(page, "Echo").click();
    await expect(urlInput(page)).toHaveValue("<<baseUrl>>/echo");
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Bearer pm-token"')).toBeVisible();
    await expect(response(page).getByText('"x-on"')).toBeVisible();
    await expect(response(page).getByText('"x-off"')).toHaveCount(0);

    await sidebarItem(page, "Users").click();
    await sidebarItem(page, "Create user").click();
    await expect(methodSelect(page)).toHaveValue("POST");
    await send(page);
    await expect(statusBadge(page)).toHaveText("201 Created");
    await expect(response(page).getByText('"Pm"')).toBeVisible();
  });

  test("an Insomnia v4 export: workspace, request groups and environment", async ({ page }) => {
    await page.goto("/hittable");
    await importText(page, JSON.stringify(insomnia));
    await expect(page.getByText("Imported from Insomnia")).toBeVisible();
    await sidebarItem(page, "Insomnia Shop").click();
    await sidebarItem(page, "Users").click();
    await sidebarItem(page, "Get user").click();
    await expect(urlInput(page)).toHaveValue("<<host>>/users/3");
    await send(page);
    await expect(response(page).getByText('"Grace Hopper"')).toBeVisible();

    await sidebar(page).getByRole("button", { name: "Insomnia Shop", exact: true }).nth(1).click();
    await sidebarItem(page, "Echo post").click();
    await send(page);
    await expect(response(page).getByText('"insomnia"')).toBeVisible();
  });

  test("a plain Hittable collection object imports as-is", async ({ page }) => {
    await page.goto("/hittable");
    const plain = collection("Plain JSON", [folder("f", [route("inner", `curl -X GET ${API}/users/2`)])], { A: "1" });
    const d = await importText(page, JSON.stringify(plain));
    await expect(d).toBeHidden();
    await sidebarItem(page, "Plain JSON").click();
    await sidebarItem(page, "f").click();
    await sidebarItem(page, "inner").click();
    await expect(urlInput(page)).toHaveValue(`${API}/users/2`);
  });

  test("the older flat `curls` format is upgraded on import", async ({ page }) => {
    await page.goto("/hittable");
    const legacy = { collectionName: "Legacy", env: {}, curls: [{ name: "old one", curl: `curl -X DELETE ${API}/users/1`, response: "" }] };
    const code = Buffer.from(pako.deflate(new TextEncoder().encode(JSON.stringify(legacy))))
      .toString("base64")
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=+$/, "");
    await importText(page, code);
    await expect(page.getByText(/Collection "Legacy" imported/)).toBeVisible();
    await sidebarItem(page, "Legacy").click();
    await expect(sidebarItem(page, "old one")).toContainText("DELETE");
  });

  test("importing the same collection twice keeps both", async ({ page }) => {
    await page.goto("/hittable");
    await importText(page, JSON.stringify(postman));
    await expect(sidebarItem(page, "Petstore")).toBeVisible();
    await importText(page, JSON.stringify(postman));
    await expect(sidebarItem(page, "Petstore - 1")).toBeVisible();
    await importText(page, JSON.stringify(postman));
    await expect(sidebarItem(page, "Petstore - 2")).toBeVisible();
  });

  test("bad input is refused with a reason and nothing is added", async ({ page }) => {
    await page.goto("/hittable");
    const d = await importText(page, "definitely-not-a-valid-export");
    await expect(d.getByText("Invalid or corrupted import data")).toBeVisible();
    await d.getByRole("textbox").fill('{"hello": "world"}');
    await d.getByRole("button", { name: "Import", exact: true }).click();
    await expect(d.getByText(/Unrecognized format/)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(d).toBeHidden();
    await expect(sidebar(page).getByText("No collections")).toBeVisible();
  });
});

test.describe("export", () => {
  test.beforeEach(async ({ seed }) => {
    await seed({
      collections: [
        collection(
          "Shop",
          [
            route("top", `curl -X GET <<BASE>>/users -H "Authorization: Bearer <<TOKEN>>"`, JSON.stringify({ data: { leaked: "top-response" }, status: 200 })),
            folder("nested", [route("deep", `curl -X POST <<BASE>>/users -H "Content-Type: application/json" -d '{"n":1}'`, JSON.stringify({ data: { leaked: "deep-response" }, status: 201 }))]),
          ],
          { BASE: API },
          { TOKEN: "super-secret-value" },
        ),
      ],
    });
  });

  test("Hittable native: a string that imports back, without responses or secret values", async ({ page }) => {
    const d = await openExport(page, "Shop");
    await d.getByRole("button", { name: "Copy" }).click();
    await expect(d).toBeHidden();
    const exported = await clipboardText(page);
    expect(exported).toMatch(/^[A-Za-z0-9_-]+$/);
    const decoded = decompress(exported);
    expect(decoded.collectionName).toBe("Shop");
    const text = JSON.stringify(decoded);
    expect(text).not.toContain("top-response");
    expect(text).not.toContain("deep-response");
    expect(text).not.toContain("super-secret-value");
    expect(decoded.env).toEqual({ BASE: API });

    await importText(page, exported);
    await expect(page.getByText(/Collection "Shop - 1" imported/)).toBeVisible();
    await sidebarItem(page, "Shop - 1").click();
    await sidebarItem(page, "nested").click();
    await sidebarItem(page, "deep").click();
    await expect(urlInput(page)).toHaveValue("<<BASE>>/users");
    await page.getByRole("button", { name: "Body", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "body").inputValue())).toEqual({ n: 1 });
  });

  test("Postman: a downloaded v2.1 file with {{variables}}", async ({ page }) => {
    const d = await openExport(page, "Shop");
    await d.getByText("Postman Collection v2.1").click();
    const [download] = await Promise.all([page.waitForEvent("download"), d.getByRole("button", { name: "Download .json" }).click()]);
    expect(download.suggestedFilename()).toBe("Shop-postman.json");
    const text = fs.readFileSync((await download.path())!, "utf8");
    const file = JSON.parse(text);
    expect(file.info.name).toBe("Shop");
    expect(file.info.schema).toContain("getpostman.com");
    const top = file.item.find((i: { name: string }) => i.name === "top");
    expect(top.request.method).toBe("GET");
    expect(JSON.stringify(top.request.url)).toContain("{{BASE}}/users");
    expect(top.request.header).toContainEqual(expect.objectContaining({ key: "Authorization", value: "Bearer {{TOKEN}}" }));
    const nested = file.item.find((i: { name: string }) => i.name === "nested");
    expect(nested.item[0].name).toBe("deep");
    expect(nested.item[0].request.method).toBe("POST");
    expect(text).not.toContain("super-secret-value");
    expect(text).not.toContain("deep-response");
  });

  test("Insomnia: a downloaded export with {{ _.variables }}", async ({ page }) => {
    const d = await openExport(page, "Shop");
    await d.getByText("Insomnia export").click();
    const [download] = await Promise.all([page.waitForEvent("download"), d.getByRole("button", { name: "Download .json" }).click()]);
    expect(download.suggestedFilename()).toBe("Shop-insomnia.json");
    const text = fs.readFileSync((await download.path())!, "utf8");
    const file = JSON.parse(text);
    expect(file._type).toBe("export");
    const requests = file.resources.filter((r: { _type: string }) => r._type === "request");
    expect(requests.map((r: { name: string }) => r.name).sort()).toEqual(["deep", "top"]);
    expect(requests.find((r: { name: string }) => r.name === "top").url).toBe("{{ _.BASE }}/users");
    expect(file.resources.some((r: { _type: string; name: string }) => r._type === "request_group" && r.name === "nested")).toBe(true);
    expect(text).not.toContain("super-secret-value");
    expect(text).not.toContain("top-response");
  });

  test("the Hittable native format has no download, and Cancel closes", async ({ page }) => {
    const d = await openExport(page, "Shop");
    await expect(d.getByRole("button", { name: "Download .json" })).toHaveCount(0);
    await d.getByText("Postman Collection v2.1").click();
    await expect(d.getByRole("button", { name: "Download .json" })).toBeVisible();
    await d.getByRole("button", { name: "Cancel" }).click();
    await expect(d).toBeHidden();
  });

  test("an export round-trips through Postman import", async ({ page }) => {
    const d = await openExport(page, "Shop");
    await d.getByText("Postman Collection v2.1").click();
    await d.getByRole("button", { name: "Copy" }).click();
    const exported = await clipboardText(page);
    await importText(page, exported);
    await expect(page.getByText("Imported from Postman")).toBeVisible();
    await sidebarItem(page, "Shop - 1").click();
    await sidebarItem(page, "top").click();
    await expect(urlInput(page)).toHaveValue("<<BASE>>/users");
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
  });
});
