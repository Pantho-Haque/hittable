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
  sidebar,
  sidebarItem,
  collection,
  route,
  openRoute,
  dialog,
  clipboardText,
} from "./fixtures/test";
import type { Page } from "@playwright/test";

async function openEnv(page: Page, name: string) {
  await sidebar(page).getByRole("button", { name: "Environment Variables" }).click();
  const env = dialog(page, `Environment: ${name}`);
  await expect(env).toBeVisible();
  return env;
}

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
  test("<<KEY>> resolves in URL and headers when sending, and stays a template in storage", async ({ page, seed }) => {
    await seed({
      collections: [
        collection("Vars", [route("r", `curl -X GET <<BASE>>/echo?id=<<ID>> -H "Authorization: Bearer <<TOKEN>>"`)], {
          BASE: API,
          ID: "42",
          TOKEN: "t0k3n",
        }),
      ],
    });
    await openRoute(page, "Vars", "r");
    await expect(urlInput(page)).toHaveValue("<<BASE>>/echo?id=<<ID>>");
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Bearer t0k3n"')).toBeVisible();
    await expect(response(page).getByText('"42"')).toBeVisible();

    await urlInput(page).press("ControlOrMeta+s");
    const [{ items }] = await page.evaluate(() => JSON.parse(localStorage.getItem("hittable")!));
    expect(items[0].curl).toContain("<<BASE>>/echo?id=<<ID>>");
    expect(items[0].curl).toContain("Bearer <<TOKEN>>");
    expect(items[0].curl).not.toContain("t0k3n");
  });

  test("a secret wins over an env var of the same name", async ({ page, seed }) => {
    await seed({
      collections: [collection("Clash", [route("r", `curl -X GET ${API}/echo -H "X-Key: <<KEY>>"`)], { KEY: "from-env" }, { KEY: "from-secret" })],
    });
    await openRoute(page, "Clash", "r");
    await send(page);
    await expect(response(page).getByText('"from-secret"')).toBeVisible();
    await expect(response(page).getByText('"from-env"')).toHaveCount(0);
  });

  test("an unknown variable is left as-is so you notice it", async ({ page, seed }) => {
    await seed({ collections: [collection("Unknown", [route("r", `curl -X GET ${API}/echo?v=<<MISSING>>`)])] });
    await openRoute(page, "Unknown", "r");
    await send(page);
    await expect(response(page).getByText('"<<MISSING>>"')).toBeVisible();
  });

  test("add, use and delete variables in the Env dialog", async ({ page, seed }) => {
    await seed({ collections: [collection("UI", [route("r", `curl -X GET <<HOST>>/users/<<USER>>`)])] });
    await openRoute(page, "UI", "r");
    let env = await openEnv(page, "UI");
    await expect(env.getByText("No environment variables yet")).toBeVisible();
    await env.getByRole("button", { name: "Add" }).click();
    await env.getByPlaceholder("KEY").last().fill("HOST");
    await env.getByPlaceholder("value").last().fill(API);
    await env.getByRole("button", { name: "Add" }).click();
    await env.getByPlaceholder("KEY").last().fill("USER");
    await env.getByPlaceholder("value").last().fill("1");
    await env.getByRole("button", { name: "Secrets" }).click();
    await expect(env.getByText("No secrets yet")).toBeVisible();
    await expect(env.getByText(/Secrets are not resolved during export/)).toBeVisible();
    await env.getByRole("button", { name: "Add" }).click();
    await env.getByPlaceholder("KEY").fill("USER");
    await env.getByPlaceholder("value").fill("3");
    await env.getByRole("button", { name: "Save · Ctrl/Cmd+S" }).click();
    await expect(env).toBeHidden();

    await send(page);
    await expect(response(page).getByText('"Grace Hopper"')).toBeVisible();

    await page.reload();
    env = await openEnv(page, "UI");
    await expect(env.getByPlaceholder("KEY")).toHaveCount(2);
    await env.getByRole("button", { name: "Secrets" }).click();
    await env.locator("div.grid", { has: page.getByPlaceholder("KEY") }).getByRole("button").click();
    await expect(env.getByText("No secrets yet")).toBeVisible();
    await page.keyboard.press("ControlOrMeta+s");
    await expect(env).toBeHidden();
    await send(page);
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();
  });

  test("Escape and the backdrop close the Env dialog without saving", async ({ page, seed }) => {
    await seed({ collections: [collection("Esc", [route("r", `curl -X GET ${API}/echo`)], { A: "1" })] });
    await openRoute(page, "Esc", "r");
    let env = await openEnv(page, "Esc");
    await env.getByPlaceholder("value").fill("changed");
    await page.keyboard.press("Escape");
    await expect(env).toBeHidden();
    env = await openEnv(page, "Esc");
    await expect(env.getByPlaceholder("value")).toHaveValue("1");
    await page.mouse.click(5, 5);
    await expect(env).toBeHidden();
  });
});

test.describe("curl", () => {
  test("pasting a curl command into the URL bar fills the whole form", async ({ page, seed, browserName }) => {
    await seed({ collections: [collection("Paste", [route("r", "")])] });
    await openRoute(page, "Paste", "r");
    await pasteInto(
      page,
      `curl -X POST '${API}/echo?source=paste' -H 'Content-Type: application/json' -H 'X-Pasted: yes' -d '{"hello":"world","n":5}'`,
      browserName,
    );
    await expect(methodSelect(page)).toHaveValue("POST");
    await expect(urlInput(page)).toHaveValue(`${API}/echo?source=paste`);
    await page.getByRole("button", { name: "Headers", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "headers").inputValue())).toEqual({ "Content-Type": "application/json", "X-Pasted": "yes" });
    await page.getByRole("button", { name: "Params", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "params").inputValue())).toEqual({ source: "paste" });
    await send(page);
    await expect(response(page).getByText('"world"')).toBeVisible();
    await expect(response(page).getByText('"yes"')).toBeVisible();
  });

  test("Postman and Insomnia variables in a pasted curl become collection variables", async ({ page, seed, browserName }) => {
    await seed({ collections: [collection("PasteVars", [route("r", "")])] });
    await openRoute(page, "PasteVars", "r");
    await pasteInto(page, `curl -X GET ${API}/echo -H "Authorization: Bearer {{token}}" -H "X-Org: {{ _.orgId }}"`, browserName);
    await expect(urlInput(page)).toHaveValue(`${API}/echo`);
    const env = await openEnv(page, "PasteVars");
    await expect(env.getByPlaceholder("KEY").nth(0)).toHaveValue("token");
    await expect(env.getByPlaceholder("KEY").nth(1)).toHaveValue("orgId");
    await expect(env.getByPlaceholder("value").nth(0)).toHaveValue("");
  });

  test("pasting a plain URL is just a URL", async ({ page, seed, browserName }) => {
    await seed({ collections: [collection("Plain", [route("r", "")])] });
    await openRoute(page, "Plain", "r");
    await pasteInto(page, `${API}/users/1`, browserName);
    await expect(urlInput(page)).toHaveValue(`${API}/users/1`);
    await expect(methodSelect(page)).toHaveValue("GET");
  });

  test("Copy as CURL produces a command that pastes back to the same request", async ({ page, seed, browserName }) => {
    await seed({ collections: [collection("Copy", [route("src", ""), route("dst", "")])] });
    await openRoute(page, "Copy", "src");
    await methodSelect(page).selectOption("PUT");
    await urlInput(page).fill(`${API}/users/2`);
    await fillTab(page, "headers", '{"Content-Type": "application/json", "X-Quote": "say \\"hi\\""}');
    await fillTab(page, "body", `{"name": "O'Brien", "age": 30}`);
    await page.getByRole("button", { name: "Copy as CURL" }).click();
    const curl = await clipboardText(page);
    expect(curl).toMatch(/^curl -X PUT /);
    expect(curl).toContain(`${API}/users/2`);
    expect(curl).toContain("Content-Type: application/json");

    await sidebarItem(page, "dst").click();
    await pasteInto(page, curl, browserName);
    await expect(methodSelect(page)).toHaveValue("PUT");
    await page.getByRole("button", { name: "Body", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "body").inputValue())).toEqual({ name: "O'Brien", age: 30 });
    await send(page);
    await expect(response(page).getByText(`"O'Brien"`)).toBeVisible();
  });

  test("a new route can be created straight from a curl", async ({ page, seed }) => {
    await seed({ collections: [collection("FromCurl", [])] });
    await page.goto("/hittable?c=FromCurl");
    await sidebar(page).getByRole("button", { name: "New Route" }).click();
    const create = dialog(page, "Create route");
    await create.getByRole("textbox", { name: "get-users" }).fill("patch grace");
    await create.getByPlaceholder("Paste you curl here").fill(`curl -X PATCH ${API}/users/3 -H "Content-Type: application/json" -d '{"active":false}'`);
    await create.getByRole("button", { name: "Create" }).click();
    await expect(sidebarItem(page, "patch grace")).toContainText("PATCH");
    await expect(methodSelect(page)).toHaveValue("PATCH");
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText("false", { exact: true })).toBeVisible();
  });
});
