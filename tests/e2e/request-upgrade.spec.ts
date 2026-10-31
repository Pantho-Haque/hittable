import {
  test,
  expect,
  API,
  blankRequest,
  collection,
  editor,
  fillTab,
  methodSelect,
  openRoute,
  response,
  route,
  send,
  statusBadge,
  urlInput,
} from "./fixtures/test";

// The Round 1 request-building requirements (docs/web-ux-requirements.md),
// end to end against the fixture API.

test.describe("body types (FR-4)", () => {
  test("a Form body arrives URL-encoded with its Content-Type", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await methodSelect(page).selectOption("POST");
    await urlInput(page).fill(`${API}/echo`);
    await page.getByRole("tab", { name: "Body", exact: true }).click();
    await page.getByRole("radio", { name: "Form" }).click();
    await page.getByPlaceholder("Field").first().fill("a");
    await page.getByPlaceholder("Value").first().fill("1");
    await page.getByRole("button", { name: "Add row" }).click();
    await page.getByPlaceholder("Field").nth(1).fill("b");
    await page.getByPlaceholder("Value").nth(1).fill("x y");
    await send(page);
    await expect(statusBadge(page)).toContainText("200");
    await expect(response(page).getByText('"a=1&b=x+y"')).toBeVisible();
    await expect(response(page).getByText('"application/x-www-form-urlencoded"')).toBeVisible();
  });

  test("a Text body arrives byte for byte, even when it is not JSON", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await methodSelect(page).selectOption("POST");
    await urlInput(page).fill(`${API}/echo`);
    await page.getByRole("tab", { name: "Body", exact: true }).click();
    await page.getByRole("radio", { name: "Text" }).click();
    await page.getByLabel("body text").fill("plain <not> json");
    await send(page);
    await expect(response(page).getByText('"plain <not> json"')).toBeVisible();
    await expect(response(page).getByText('"text/plain"')).toBeVisible();
  });

  test("the type survives a save and reload", async ({ page, seed }) => {
    await seed({ collections: [collection("Bodies", [route("form", `curl -X POST ${API}/echo`)])] });
    await openRoute(page, "Bodies", "form");
    await page.getByRole("tab", { name: "Body", exact: true }).click();
    await page.getByRole("radio", { name: "Form" }).click();
    await page.getByPlaceholder("Field").first().fill("k");
    await page.getByPlaceholder("Value").first().fill("v");
    await page.keyboard.press("ControlOrMeta+s");
    await expect(page.getByText(/Unsaved/)).toBeHidden();
    await page.reload();
    await page.getByRole("tab", { name: "Body", exact: true }).click();
    await expect(page.getByRole("radio", { name: "Form" })).toHaveAttribute("aria-checked", "true");
    await expect(page.getByPlaceholder("Field").first()).toHaveValue("k");
  });

  test("invalid JSON blocks Send only for a JSON body", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await fillTab(page, "body", "{oops");
    await expect(page.getByRole("button", { name: "Send" })).toBeDisabled();
    await page.getByRole("radio", { name: "Text" }).click();
    await expect(page.getByRole("button", { name: "Send" })).toBeEnabled();
    void editor;
  });
});

// The fixture API answers every OPTIONS with 204 (and /echo with 200 for
// anything else), so a 204 shows the method really went out as OPTIONS.
test("OPTIONS can be sent (FR-5)", async ({ page, seed }) => {
  await blankRequest(page, seed);
  await methodSelect(page).selectOption("OPTIONS");
  await urlInput(page).fill(`${API}/echo`);
  await send(page);
  await expect(statusBadge(page)).toHaveText("204 No Content");
});

test.describe("cancel (FR-6)", () => {
  test("Cancel returns to idle at once, and the next Send works", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/slow?ms=5000`);
    await send(page);
    // A click within 400 ms of sending is taken as the tail of a double-click.
    await page.waitForTimeout(500);
    const started = Date.now();
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(page.getByRole("button", { name: "Send" })).toBeEnabled();
    expect(Date.now() - started).toBeLessThan(1500);
    await expect(response(page).getByText("Request cancelled.")).toBeVisible();
    await urlInput(page).fill(`${API}/users/1`);
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
  });

  test("Esc cancels while the URL bar has focus", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/slow?ms=5000`);
    await urlInput(page).press("ControlOrMeta+Enter");
    await expect(page.getByRole("button", { name: "Cancel" })).toBeVisible();
    await urlInput(page).press("Escape");
    await expect(response(page).getByText("Request cancelled.")).toBeVisible();
  });
});

test.describe("status (FR-7)", () => {
  for (const [code, text, kind] of [
    [201, "201 Created", "success"],
    [302, "302 Found", "info"],
    [404, "404 Not Found", "warning"],
    [503, "503 Service Unavailable", "danger"],
  ] as const) {
    test(`${code} reads "${text}" and is shown as ${kind}`, async ({ page, seed }) => {
      await blankRequest(page, seed);
      await urlInput(page).fill(`${API}/status/${code}`);
      await send(page);
      await expect(statusBadge(page)).toHaveText(text);
      await expect(statusBadge(page)).toHaveAttribute("data-status-kind", kind);
    });
  }

  test("size is the bytes received", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/text`);
    await send(page);
    // "plain text line one\n  indented line two\nhello world" is 51 bytes.
    await expect(response(page).getByText("51 B")).toBeVisible();
  });
});

test.describe("variables and curl (FR-3, FR-9)", () => {
  test("unknown variables are marked and named before sending", async ({ page, seed }) => {
    await seed({ collections: [collection("Vars", [route("r", `curl -X GET <<BASE_URL>>/x/<<MISSING>>`)], { BASE_URL: API })] });
    await openRoute(page, "Vars", "r");
    const bar = urlInput(page).locator("..");
    await expect(bar.locator('[data-var="BASE_URL"]')).toHaveAttribute("data-resolved", "true");
    await expect(bar.locator('[data-var="MISSING"]')).toHaveAttribute("data-resolved", "false");
    await expect(page.getByRole("note")).toContainText("Unknown variable <<MISSING>>");
  });

  test("a pasted curl fills the form at once, and Undo puts the old request back", async ({ page, seed }) => {
    await blankRequest(page, seed, `${API}/users/1`);
    await urlInput(page).focus();
    await page.evaluate((cmd) => {
      const el = document.activeElement as HTMLTextAreaElement;
      const dt = new DataTransfer();
      dt.setData("text/plain", cmd);
      el.dispatchEvent(new ClipboardEvent("paste", { clipboardData: dt, bubbles: true, cancelable: true }));
    }, `curl -X DELETE ${API}/users/9 -H "X-Trace: abc"`);
    await expect(urlInput(page)).toHaveValue(`${API}/users/9`, { timeout: 300 });
    await expect(methodSelect(page)).toHaveValue("DELETE");
    await page.getByRole("button", { name: "Undo" }).click();
    await expect(urlInput(page)).toHaveValue(`${API}/users/1`);
    await expect(methodSelect(page)).toHaveValue("GET");
  });
});
