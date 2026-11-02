import {
  test,
  expect,
  API,
  urlInput,
  methodSelect,
  send,
  editor,
  fillTab,
  response,
  statusBadge,
  openRequest,
  clipboardText,
  FOLDER_ONLY,
} from "./fixtures/test";
import { openFile, readFile } from "./fixtures/directory";
import type { Page } from "@playwright/test";

// Variables come from hittable/env.json, with hittable/env.local.json layered on top.
test.skip(({ browserName }) => browserName !== "chromium", FOLDER_ONLY);

async function pasteInto(page: Page, text: string, browserName: string) {
  await urlInput(page).focus();
  await page.keyboard.press("ControlOrMeta+a");
  if (browserName === "chromium") {
    await page.evaluate((t) => navigator.clipboard.writeText(t), text);
    await page.keyboard.press("ControlOrMeta+v");
    return;
  }
  // Other engines block clipboard access from tests: deliver the same paste
  // event a real paste fires, and do the browser's default insert if the app
  // lets it through.
  const handled = await urlInput(page).evaluate((el, t) => {
    const data = new DataTransfer();
    data.setData("text/plain", t);
    return !el.dispatchEvent(new ClipboardEvent("paste", { clipboardData: data, bubbles: true, cancelable: true }));
  }, text);
  if (!handled) await page.keyboard.insertText(text);
}

test.describe("variables", () => {
  test("<<KEY>> resolves in URL and headers when sending, and stays a template in the file", async ({ page }) => {
    await openRequest(page, {
      url: "<<BASE>>/echo?id=<<ID>>",
      headers: { Authorization: "Bearer <<TOKEN>>" },
      env: { BASE: API, ID: "42", TOKEN: "t0k3n" },
    });
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Bearer t0k3n"')).toBeVisible();
    await expect(response(page).getByText('"42"')).toBeVisible();

    await urlInput(page).press("ControlOrMeta+s");
    const saved = JSON.parse(await readFile(page, "hittable/req.hit"));
    expect(saved.url).toBe("<<BASE>>/echo?id=<<ID>>");
    expect(saved.headers).toEqual({ Authorization: "Bearer <<TOKEN>>" });
    expect(JSON.stringify({ ...saved, response: null })).not.toContain("t0k3n");
  });

  test("env.local.json wins over env.json for the same name", async ({ page }) => {
    await openRequest(page, {
      url: `${API}/echo`,
      headers: { "X-Key": "<<KEY>>" },
      env: { KEY: "from-env" },
      envLocal: { KEY: "from-local" },
    });
    await send(page);
    await expect(response(page).getByText('"from-local"')).toBeVisible();
    await expect(response(page).getByText('"from-env"')).toHaveCount(0);
  });

  test("an unknown variable is left as-is so you notice it", async ({ page }) => {
    await openRequest(page, { url: `${API}/echo?v=<<MISSING>>` });
    await send(page);
    await expect(response(page).getByText('"<<MISSING>>"')).toBeVisible();
  });

  test("Open Environment from the unknown-variable note opens env.json, and a value added there is used", async ({ page }) => {
    await openRequest(page, { url: "<<HOST>>/users/<<USER>>", env: { HOST: API } });
    const note = page.getByRole("note");
    await expect(note).toContainText("Unknown variable <<USER>>");
    await note.getByRole("button", { name: "Open Environment" }).click();
    const env = page.getByRole("textbox", { name: "Edit env.json" });
    await expect(env).toBeVisible();
    await env.click();
    await page.keyboard.press("ControlOrMeta+a");
    await page.keyboard.insertText(JSON.stringify({ HOST: API, USER: "3" }));
    await expect.poll(async () => JSON.parse(await readFile(page, "hittable/env.json"))).toEqual({ HOST: API, USER: "3" });

    await openFile(page, "hittable/req.hit");
    await expect(page.getByRole("note")).toHaveCount(0);
    await send(page);
    await expect(response(page).getByText('"Grace Hopper"')).toBeVisible();
  });
});

test.describe("curl", () => {
  test("pasting a curl command into the URL bar fills the whole form", async ({ page, browserName }) => {
    await openRequest(page);
    await pasteInto(
      page,
      `curl -X POST '${API}/echo?source=paste' -H 'Content-Type: application/json' -H 'X-Pasted: yes' -d '{"hello":"world","n":5}'`,
      browserName,
    );
    await expect(methodSelect(page)).toHaveValue("POST");
    await expect(urlInput(page)).toHaveValue(`${API}/echo?source=paste`);
    await page.getByRole("tab", { name: "Headers", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "headers").inputValue())).toEqual({ "Content-Type": "application/json", "X-Pasted": "yes" });
    await page.getByRole("tab", { name: "Params", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "params").inputValue())).toEqual({ source: "paste" });
    await send(page);
    await expect(response(page).getByText('"world"')).toBeVisible();
    await expect(response(page).getByText('"yes"')).toBeVisible();
  });

  test("Postman and Insomnia variables in a pasted curl become <<KEY>> and are named as unknown", async ({ page, browserName }) => {
    await openRequest(page);
    await pasteInto(page, `curl -X GET ${API}/echo -H "Authorization: Bearer {{token}}" -H "X-Org: {{ _.orgId }}"`, browserName);
    await expect(urlInput(page)).toHaveValue(`${API}/echo`);
    await page.getByRole("tab", { name: "Headers", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "headers").inputValue())).toEqual({
      Authorization: "Bearer <<token>>",
      "X-Org": "<<orgId>>",
    });
    await expect(page.getByRole("note")).toContainText("Unknown variables <<token>>, <<orgId>>");
  });

  test("pasting a plain URL is just a URL", async ({ page, browserName }) => {
    await openRequest(page);
    await pasteInto(page, `${API}/users/1`, browserName);
    await expect(urlInput(page)).toHaveValue(`${API}/users/1`);
    await expect(methodSelect(page)).toHaveValue("GET");
  });

  // The cURL button became a "Copy as" menu (FR-8); cURL is its first entry.
  test("Copy as cURL produces a command that pastes back to the same request", async ({ page, browserName }) => {
    await openRequest(page, {
      files: { "hittable/dst.hit": JSON.stringify({ method: "GET", url: "", headers: {}, params: {}, body: "", response: null }) },
    });
    await methodSelect(page).selectOption("PUT");
    await urlInput(page).fill(`${API}/users/2`);
    await fillTab(page, "headers", '{"Content-Type": "application/json", "X-Quote": "say \\"hi\\""}');
    await fillTab(page, "body", `{"name": "O'Brien", "age": 30}`);
    await page.getByRole("button", { name: /Copy as/ }).click();
    await page.getByRole("menuitem", { name: "cURL" }).click();
    const curl = await clipboardText(page);
    expect(curl).toMatch(/^curl -X PUT\b/);
    expect(curl).toContain(`${API}/users/2`);
    expect(curl).toContain("Content-Type: application/json");

    await openFile(page, "hittable/dst.hit");
    await expect(methodSelect(page)).toHaveValue("GET");
    await pasteInto(page, curl, browserName);
    await expect(methodSelect(page)).toHaveValue("PUT");
    await page.getByRole("tab", { name: "Body", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "body").inputValue())).toEqual({ name: "O'Brien", age: 30 });
    await send(page);
    await expect(response(page).getByText(`"O'Brien"`)).toBeVisible();
  });
});
