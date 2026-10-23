import type { Page } from "@playwright/test";
import {
  test,
  expect,
  API,
  blankRequest,
  urlInput,
  methodSelect,
  send,
  response,
  statusBadge,
  sidebarItem,
  collection,
  route,
  folder,
  openRoute,
  dialog,
  editor,
} from "./fixtures/test";

const historyButton = (page: Page) => page.getByRole("button", { name: /^History/ });

async function openHistory(page: Page) {
  await historyButton(page).click();
  const d = dialog(page, "Request History");
  await expect(d).toBeVisible();
  return d;
}

const entry = (i: number, url = `${API}/users/${i}`) => ({
  id: `seed-${i}`,
  timestamp: Date.now() - i * 60_000,
  method: "GET",
  url,
  status: 200,
  statusText: "OK",
  durationMs: 10 + i,
  sizeBytes: 100,
  curlJson: { method: "GET", url, headers: "{}", body: "", params: "{}" },
  responseJson: { data: { seeded: i }, status: 200, statusText: "OK", ok: true, headers: {}, durationMs: 10 + i, sizeBytes: 100 },
});

test.describe("history", () => {
  test("every send is logged newest first, failures included, and survives a reload", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await expect(historyButton(page)).toHaveAccessibleName("History");
    await urlInput(page).fill(`${API}/users/1`);
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await methodSelect(page).selectOption("DELETE");
    await urlInput(page).fill(`${API}/users/2`);
    await send(page);
    await expect(statusBadge(page)).toHaveText("204 No Content");
    await urlInput(page).fill("http://[::1]:4101/down");
    await send(page);
    await expect(statusBadge(page)).toHaveText("0 Network Error");
    await expect(historyButton(page)).toHaveAccessibleName("History, 3 requests");

    let h = await openHistory(page);
    await expect(h.getByText("3 requests logged")).toBeVisible();
    const rows = h.getByRole("button", { name: /^(GET|DELETE)/ });
    await expect(rows).toHaveCount(3);
    await expect(rows.nth(0)).toContainText("http://[::1]:4101/down");
    await expect(rows.nth(1)).toContainText(`${API}/users/2`);
    await expect(rows.nth(1)).toContainText("204");
    await expect(rows.nth(2)).toContainText(`${API}/users/1`);
    await expect(rows.nth(2)).toContainText("200");
    await expect(rows.nth(2)).toContainText(/\d+ms/);
    await expect(rows.nth(2)).toContainText("just now");

    await page.reload();
    h = await openHistory(page);
    await expect(h.getByRole("button", { name: /^(GET|DELETE)/ })).toHaveCount(3);
  });

  test("an entry loads back into the runner with its response and can be saved to a collection", async ({ page, seed }) => {
    await seed({
      collections: [collection("Target", [folder("saved", [])]), collection("Other", [])],
      history: [entry(2)],
    });
    await page.goto("/hittable");
    const h = await openHistory(page);
    await h.getByRole("button", { name: new RegExp(`GET\\s*${API.replace(/[[\]]/g, "\\$&")}/users/2`) }).click();
    await expect(h).toBeHidden();
    await expect(urlInput(page)).toHaveValue(`${API}/users/2`);
    await expect(response(page).getByText('"seeded"')).toBeVisible();
    await expect(page.getByRole("navigation", { name: "Breadcrumb" })).toContainText("History: GET /users/2");

    await page.getByRole("button", { name: "Save to…" }).click();
    const save = dialog(page, "Save from History");
    await save.getByRole("button", { name: "Target", exact: true }).click();
    await save.getByRole("button", { name: "saved" }).click();
    await save.getByRole("button", { name: "Save here" }).click();
    await save.getByRole("textbox", { name: "e.g. get-users" }).fill("user two");
    await save.getByRole("button", { name: "Save", exact: true }).click();
    await expect(save).toBeHidden();
    await expect(page).toHaveURL(/c=Target&r=user(\+|%20)two&p=saved/);
    await expect(sidebarItem(page, "user two")).toBeVisible();
    await expect(page.getByRole("button", { name: "Save to…" })).toHaveCount(0);
    await send(page);
    await expect(response(page).getByText('"Alan Turing"')).toBeVisible();
  });

  test("saving from history can create the collection, and asks for a route name first", async ({ page, seed }) => {
    await seed({ collections: [], history: [entry(3)] });
    await page.goto("/hittable");
    const h = await openHistory(page);
    await h.getByRole("button", { name: /\/users\/3/ }).click();
    await page.getByRole("button", { name: "Save to…" }).click();
    const save = dialog(page, "Save from History");
    await save.getByRole("button", { name: "+ Create new collection" }).click();
    await save.getByRole("textbox", { name: "New collection name" }).fill("From History");
    await save.getByRole("button", { name: "Save", exact: true }).click();
    await expect(save.getByText("Route name is required")).toBeVisible();
    await save.getByRole("textbox", { name: "e.g. get-users" }).fill("grace");
    await save.getByRole("button", { name: "Save", exact: true }).click();
    await expect(save).toBeHidden();
    await expect(page).toHaveURL(/c=From(\+|%20)History&r=grace/);
    await page.goto("/hittable");
    // Exactly one collection: the failed first attempt must not leave an empty one behind.
    await expect(sidebarItem(page, "From History")).toBeVisible();
    await expect(sidebarItem(page, "From History - New")).toHaveCount(0);
    await sidebarItem(page, "From History").click();
    await expect(urlInput(page)).toHaveValue(`${API}/users/3`);
  });

  test("an entry with <<variables>> in its URL loads back too", async ({ page, seed }) => {
    await seed({
      collections: [collection("Tpl", [route("r", `curl -X GET <<BASE>>/users/1`)], { BASE: API })],
    });
    await openRoute(page, "Tpl", "r");
    await send(page);
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();
    const h = await openHistory(page);
    await h.getByRole("button", { name: /<<BASE>>\/users\/1/ }).click();
    await expect(h).toBeHidden();
    await expect(urlInput(page)).toHaveValue("<<BASE>>/users/1");
    await expect(page.getByRole("button", { name: "Save to…" })).toBeVisible();
  });

  test("delete one entry, then clear them all", async ({ page, seed }) => {
    await seed({ history: [entry(1), entry(2), entry(3)] });
    await page.goto("/hittable");
    const h = await openHistory(page);
    const row = h.getByRole("button", { name: /\/users\/2/ });
    await row.hover();
    await row.getByRole("button", { name: "Delete entry" }).click();
    await expect(h.getByText("2 requests logged")).toBeVisible();
    await expect(h.getByRole("button", { name: /\/users\/2/ })).toHaveCount(0);
    await expect(h).toBeVisible();

    await h.getByRole("button", { name: "Clear all" }).click();
    await expect(h.getByText("No request history yet")).toBeVisible();
    await expect(h.getByText("0 requests logged")).toBeVisible();
    await h.getByRole("button", { name: "Close" }).click();
    await page.reload();
    await expect(historyButton(page)).toHaveAccessibleName("History");
  });

  test("history keeps the newest 100 requests", async ({ page, seed }) => {
    await seed({ history: Array.from({ length: 100 }, (_, i) => entry(i + 1)), collections: [collection("Cap", [route("r", `curl -X GET ${API}/echo?newest=1`)])] });
    await openRoute(page, "Cap", "r");
    await expect(page.getByTitle("History").getByText("99+")).toBeVisible();
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    const h = await openHistory(page);
    await expect(h.getByText("100 requests logged")).toBeVisible();
    await expect(h.getByRole("button", { name: /^GET/ }).first()).toContainText("newest=1");
    await expect(h.getByRole("button", { name: /\/users\/100$/ })).toHaveCount(0);
    await expect(editor(page, "params")).toBeAttached();
  });
});
