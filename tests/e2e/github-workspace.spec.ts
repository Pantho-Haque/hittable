import { test, expect, urlInput } from "./fixtures/test";
import type { APIRequestContext, Page } from "@playwright/test";

// FR-G2..G8 through the UI, against the fake GitHub (tests/e2e/fixtures/fake-github.mjs).
// The fake is one process with one state shared by every worker and project,
// and each test resets it, so this file runs serially on one project only.
const FAKE = "http://localhost:4200";
const REPO = "octo/api-tests";
const GET_HIT = "hittable/users/get.hit";

test.describe.configure({ mode: "serial" });
test.skip(({ browserName, isMobile }) => browserName !== "chromium" || isMobile, "One shared fake GitHub: runs on one project");

type FakeState = { head: string; message: string; files: Record<string, string>; branches: Record<string, string>; pulls: { head: string; base: string; title: string }[]; revocations: number };
const fakeState = async (request: APIRequestContext, repo = REPO, branch = "main"): Promise<FakeState> =>
  (await request.get(`${FAKE}/__fake/state?repo=${repo}&branch=${encodeURIComponent(branch)}`)).json();

const tree = (page: Page) => page.getByRole("tree", { name: "Workspace files" });
const item = (page: Page, name: string) => tree(page).getByRole("treeitem", { name, exact: true }).first();
const panel = (page: Page) => page.getByRole("complementary", { name: "Source control" });
const changesButton = (page: Page) => page.getByRole("button", { name: /^Source control, \d+ changes?$/ });

async function expandDir(page: Page, name: string) {
  const it = item(page, name);
  if ((await it.getAttribute("aria-expanded")) !== "true") await it.locator("> div").first().click();
  await expect(it).toHaveAttribute("aria-expanded", "true");
}

/** Open a file by its path, expanding the folders above it. */
async function openFile(page: Page, path: string) {
  const parts = path.split("/");
  for (const dir of parts.slice(0, -1)) await expandDir(page, dir);
  await item(page, parts.at(-1)!).locator("> div").first().click();
}

async function openRepo(page: Page, repo = REPO, ref?: string) {
  await page.goto(`/hittable/gh/${repo}${ref ? `?ref=${ref}` : ""}`);
  await expect(tree(page)).toBeVisible();
  await expect(page.getByRole("button", { name: /^Branch: / })).toBeVisible();
}

async function editUrl(page: Page, url: string) {
  await openFile(page, GET_HIT);
  await expect(page.getByRole("button", { name: "Runner" })).toHaveAttribute("aria-pressed", "true");
  await urlInput(page).fill(url);
  await expect(item(page, "get.hit")).toHaveAccessibleDescription("Modified");
}

async function openChanges(page: Page) {
  if (!(await panel(page).isVisible())) await changesButton(page).click();
  await expect(panel(page)).toBeVisible();
}

test.beforeEach(async ({ page, request }) => {
  await request.post(`${FAKE}/__fake/reset`);
  // The fake authorizes at once; the callback lands back on the launcher.
  await page.goto("/api/github/login?next=/hittable");
  await expect(page).toHaveURL(/\/hittable$/);
});

test("pick a repo, edit in the Runner, reload, review the diff and commit", async ({ page, request }) => {
  const picker = page.getByRole("listbox", { name: "Repositories" });
  await expect(picker.getByRole("option", { name: /octo\/api-tests/ })).toBeVisible();
  await expect(picker.getByRole("option", { name: /acme\/shared/ })).toContainText("Read-only");
  await expect(picker.getByRole("option", { name: /octo\/old-archive/ })).toContainText("Archived");
  await page.getByRole("combobox", { name: "Search repositories" }).fill("api");
  await picker.getByRole("option", { name: /octo\/api-tests/ }).click();
  await expect(page).toHaveURL(/\/hittable\/gh\/octo\/api-tests$/);

  await editUrl(page, "{{baseUrl}}/users/2");

  // The working copy is in IndexedDB: a reload keeps the change.
  await page.reload();
  await expandDir(page, "hittable");
  await expandDir(page, "users");
  await expect(item(page, "get.hit")).toHaveAccessibleDescription("Modified");
  await expect(changesButton(page)).toHaveAccessibleName("Source control, 1 change");

  await openChanges(page);
  await expect(panel(page).getByRole("listitem")).toHaveCount(1);
  await panel(page).getByRole("button", { name: /hittable\/users\/get\.hit, Modified/ }).click();
  const diff = page.getByRole("dialog", { name: GET_HIT });
  await expect(diff.getByText("users/2", { exact: false }).first()).toBeVisible();
  await diff.getByRole("button", { name: "Side by side" }).click();
  await expect(diff.getByRole("textbox", { name: `${GET_HIT}, before` })).toBeVisible();
  await diff.getByRole("button", { name: "Close" }).click();

  const before = await fakeState(request);
  await expect(panel(page).getByRole("textbox", { name: "Commit message" })).toHaveValue(`Update ${GET_HIT}`);
  await panel(page).getByRole("button", { name: /Commit to main/ }).click();
  await expect(page.getByText(/^Committed [0-9a-f]{7} to main$/)).toBeVisible();
  await expect(page.getByRole("button", { name: "View on GitHub" })).toBeVisible();

  const after = await fakeState(request);
  expect(after.head).not.toBe(before.head);
  expect(after.message).toBe(`Update ${GET_HIT}`);
  expect(JSON.parse(after.files[GET_HIT]).url).toBe("{{baseUrl}}/users/2");
  await expect(item(page, "get.hit")).not.toHaveAccessibleDescription("Modified");
  await expect(changesButton(page)).toHaveAccessibleName("Source control, 0 changes");
});

test("delete a file: D, then commit the deletion", async ({ page, request }) => {
  await openRepo(page);
  await item(page, "README.md").locator("> div").first().click({ button: "right" });
  await page.getByRole("button", { name: "Delete", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
  await expect(item(page, "README.md")).toHaveAccessibleDescription("Deleted");

  await openChanges(page);
  await expect(panel(page).getByRole("textbox", { name: "Commit message" })).toHaveValue("Delete README.md");
  await panel(page).getByRole("button", { name: /Commit to main/ }).click();
  await expect(page.getByText(/^Committed [0-9a-f]{7} to main$/)).toBeVisible();
  expect((await fakeState(request)).files["README.md"]).toBeUndefined();
  await expect(item(page, "README.md")).toHaveCount(0);
});

test("someone else changed the same file: conflict, nothing overwritten", async ({ page, request }) => {
  await openRepo(page);
  await editUrl(page, "{{baseUrl}}/users/mine");
  const theirs = JSON.stringify({ method: "GET", url: "{{baseUrl}}/users/theirs" }, null, 2) + "\n";
  await request.post(`${FAKE}/__fake/advance`, { data: { repo: REPO, path: GET_HIT, contents: theirs } });

  await openChanges(page);
  await panel(page).getByRole("button", { name: /Commit to main/ }).click();
  await expect(panel(page).getByRole("alert")).toContainText("changed the same files");
  await expect(item(page, "get.hit")).toHaveAccessibleDescription("Conflict");
  expect((await fakeState(request)).files[GET_HIT]).toBe(theirs);

  // Compare shows theirs against mine; Keep mine then commits on top of theirs.
  await page.getByRole("button", { name: "1 conflict" }).click();
  await page.getByRole("button", { name: `Compare ${GET_HIT}` }).click();
  await expect(page.getByRole("dialog", { name: GET_HIT })).toContainText("Theirs");
  await page.getByRole("dialog", { name: GET_HIT }).getByRole("button", { name: "Close" }).click();
  await page.getByRole("button", { name: "1 conflict" }).click();
  await page.getByRole("button", { name: "Keep mine" }).click();
  await expect(item(page, "get.hit")).toHaveAccessibleDescription("Modified");
  await panel(page).getByRole("button", { name: /Commit to main/ }).click();
  await expect(page.getByText(/^Committed [0-9a-f]{7} to main$/)).toBeVisible();
  expect(JSON.parse((await fakeState(request)).files[GET_HIT]).url).toBe("{{baseUrl}}/users/mine");
});

test("a protected branch: commit to a new branch and open a pull request", async ({ page, request }) => {
  await openRepo(page, REPO, "protected");
  await editUrl(page, "{{baseUrl}}/users/via-pr");
  await openChanges(page);
  await panel(page).getByRole("button", { name: /Commit to protected/ }).click();

  const dialog = page.getByRole("dialog", { name: /new branch & open a pull request/ });
  await expect(dialog).toContainText("protected is protected");
  await expect(dialog.getByRole("textbox", { name: /New branch/ })).toHaveValue("hittable/update-hittable-users-get-hit");
  await dialog.getByRole("button", { name: /Commit & open pull request/ }).click();
  await expect(page.getByText("Opened pull request #1")).toBeVisible();

  const state = await fakeState(request, REPO, "hittable/update-hittable-users-get-hit");
  expect(state.pulls).toEqual([expect.objectContaining({ head: "hittable/update-hittable-users-get-hit", base: "protected" })]);
  expect(JSON.parse(state.files[GET_HIT]).url).toBe("{{baseUrl}}/users/via-pr");
  // Still on the protected branch, with the committed change cleared.
  await expect(page).toHaveURL(/\?ref=protected$/);
  await expect(changesButton(page)).toHaveAccessibleName("Source control, 0 changes");
});

// In a repository a send shows its response but does not write it into the
// .hit file: otherwise every run would be a change and the next commit would
// carry response data. (A local folder still saves the last response.)
test("sending a request does not change the file", async ({ page }) => {
  await openRepo(page);
  await openFile(page, GET_HIT);
  await expect(page.getByRole("button", { name: "Runner" })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Send", exact: true }).click();
  await expect(page.getByRole("region", { name: "Response" }).locator("[aria-live=polite], [role=alert]").first()).toBeVisible();
  await expect(item(page, "get.hit")).not.toHaveAccessibleDescription("Modified");
  await expect(page.getByRole("button", { name: /^Source control/ })).not.toHaveAccessibleName(/[1-9] changes?$/);
});

test("switch branch from the branch switcher", async ({ page }) => {
  await openRepo(page);
  await page.getByRole("button", { name: /^Branch: main/ }).click();
  await page.getByRole("combobox", { name: /Find or create a branch/ }).fill("dev");
  // Branches load when the switcher opens; Enter picks the first match.
  await expect(page.getByRole("option", { name: /^dev/ })).toBeVisible();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\?ref=dev$/);
  await expect(page.getByRole("button", { name: /^Branch: dev/ })).toBeVisible();
});

test("archived, no-push and empty repositories", async ({ page }) => {
  await openRepo(page, "acme/shared");
  await expect(page.getByText("Read-only", { exact: true })).toBeVisible();
  await openFile(page, "hittable/ping.hit");
  await expect(urlInput(page)).not.toBeEditable();
  await openChanges(page);
  await expect(panel(page).getByRole("button", { name: /Commit to/ })).toHaveCount(0);
  await expect(panel(page)).toContainText("can't push to this repository");

  await openRepo(page, "octo/old-archive");
  await expect(page.getByText("Read-only", { exact: true })).toBeVisible();

  await page.goto("/hittable/gh/octo/empty");
  await expect(page.getByRole("heading", { name: "octo/empty is empty" })).toBeVisible();
});

test("a repo without hittable/: Initialize stages three added files", async ({ page }) => {
  await openRepo(page, "octo/website");
  await page.getByRole("button", { name: "Initialize Hittable here" }).click();
  await expect(changesButton(page)).toHaveAccessibleName("Source control, 3 changes");
  await openChanges(page);
  for (const path of ["hittable/.gitignore", "hittable/env.json", "hittable/requests/example.hit"]) {
    await expect(panel(page).getByRole("button", { name: `${path}, Added` })).toBeVisible();
  }
  await expect(panel(page).getByRole("listitem")).toHaveCount(3);
});

test("sign out with uncommitted changes asks first", async ({ page, request }) => {
  await openRepo(page);
  await editUrl(page, "{{baseUrl}}/users/unsaved");
  await page.getByRole("button", { name: "Account: octo" }).click();
  await page.getByRole("menuitem", { name: "Sign out" }).click();
  const ask = page.getByRole("dialog", { name: "You have uncommitted changes" });
  await ask.getByRole("button", { name: "Discard" }).click();
  await expect(page).toHaveURL(/\/$/);
  expect(await (await page.request.get("/api/github/session")).json()).toEqual({ user: null });
  expect((await fakeState(request)).revocations).toBe(1);
});
