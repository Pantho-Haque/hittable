import {
  test,
  expect,
  API,
  blankRequest,
  urlInput,
  methodSelect,
  sendButton,
  send,
  editor,
  fillTab,
  response,
  statusBadge,
  sidebarItem,
  collection,
  route,
  openRoute,
} from "./fixtures/test";

test.describe("building and sending requests", () => {
  test("GET with query params and headers: status, time, size and a JSON tree", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/echo`);
    await fillTab(page, "params", '{"page": "2", "q": "hello world"}');
    // Params and the query string are one value.
    await expect(urlInput(page)).toHaveValue(`${API}/echo?page=2&q=hello%20world`);
    await fillTab(page, "headers", '{"X-Trace": "abc-123", "Accept": "application/json"}');
    await send(page);

    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText(/^\d+(ms|\.\d+s)$/)).toBeVisible();
    await expect(response(page).getByText(/^\d+(\.\d)? (B|KB)$/)).toBeVisible();
    const body = response(page);
    await expect(body.getByText('"x-trace"')).toBeVisible();
    await expect(body.getByText('"abc-123"')).toBeVisible();
    await expect(body.getByText('"hello world"')).toBeVisible();
    await expect(body.getByText('"GET"')).toBeVisible();
  });

  test("editing the query string fills the Params tab", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/users?limit=1&sort=name%20asc`);
    await page.getByRole("button", { name: "Params", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "params").inputValue())).toEqual({ limit: "1", sort: "name asc" });
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();
    await expect(response(page).getByText('"Alan Turing"')).toHaveCount(0);
  });

  test("table mode edits the same params and headers", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/echo`);
    await page.getByRole("button", { name: "Headers", exact: true }).click();
    await page.getByRole("button", { name: "Table mode" }).click();
    await page.getByPlaceholder("Header name").fill("X-From-Table");
    await page.getByPlaceholder("Header value").fill("yes");
    await page.getByRole("button", { name: "Params", exact: true }).click();
    await page.getByPlaceholder("Param name").fill("color");
    await page.getByPlaceholder("Value").fill("blue");
    await expect(urlInput(page)).toHaveValue(`${API}/echo?color=blue`);
    await page.getByRole("button", { name: "JSON mode" }).click();
    await page.getByRole("button", { name: "Headers", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "headers").inputValue())).toEqual({ "X-From-Table": "yes" });
    await send(page);
    await expect(response(page).getByText('"x-from-table"')).toBeVisible();
    await expect(response(page).getByText('"blue"')).toBeVisible();
  });

  for (const { method, path, body, status, expectText } of [
    { method: "POST", path: "/users", body: '{"name": "Linus", "email": "linus@example.com"}', status: "201 Created", expectText: '"Linus"' },
    { method: "PUT", path: "/users/2", body: '{"name": "Replaced"}', status: "200 OK", expectText: '"Replaced"' },
    { method: "PATCH", path: "/users/3", body: '{"active": false}', status: "200 OK", expectText: '"Grace Hopper"' },
  ]) {
    test(`${method} sends a JSON body`, async ({ page, seed }) => {
      await blankRequest(page, seed);
      await methodSelect(page).selectOption(method);
      await urlInput(page).fill(`${API}${path}`);
      await fillTab(page, "headers", '{"Content-Type": "application/json"}');
      await fillTab(page, "body", body);
      await send(page);
      await expect(statusBadge(page)).toHaveText(status);
      await expect(response(page).getByText(expectText)).toBeVisible();
    });
  }

  test("the body reaches the server unchanged", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await methodSelect(page).selectOption("POST");
    await urlInput(page).fill(`${API}/echo`);
    await fillTab(page, "headers", '{"Content-Type": "application/json"}');
    await fillTab(page, "body", '{"nested": {"list": [1, 2, 3]}, "unicode": "héllo ✓"}');
    await send(page);
    await expect(response(page).getByText('"héllo ✓"')).toBeVisible();
    await expect(response(page).getByText('"POST"')).toBeVisible();
    await expect(response(page).getByText('"application/json"')).toBeVisible();
  });

  test("DELETE with an empty 204 reply", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await methodSelect(page).selectOption("DELETE");
    await urlInput(page).fill(`${API}/users/1`);
    await send(page);
    await expect(statusBadge(page)).toHaveText("204 No Content");
    await expect(response(page).getByRole("button", { name: /^headers/ })).toBeVisible();
  });

  test("HEAD shows the response headers as the body", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await methodSelect(page).selectOption("HEAD");
    await urlInput(page).fill(`${API}/users/1`);
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"content-type"')).toBeVisible();
  });

  test("redirects are followed", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/redirect`);
    await send(page);
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Ada Lovelace"')).toBeVisible();
  });

  for (const [code, text] of [
    ["404", "404 Not Found"],
    ["500", "500 Internal Server Error"],
    ["301", "301 Moved Permanently"],
  ] as const) {
    test(`a ${code} reply is shown with its status`, async ({ page, seed }) => {
      await blankRequest(page, seed);
      await urlInput(page).fill(`${API}/status/${code}`);
      await send(page);
      await expect(statusBadge(page)).toHaveText(text);
      await expect(response(page).getByText(`"status ${code}"`)).toBeVisible();
    });
  }

  test("a missing user is a JSON 404, not an app error", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/users/999`);
    await send(page);
    await expect(statusBadge(page)).toHaveText("404 Not Found");
    await expect(response(page).getByText('"User not found"')).toBeVisible();
  });
});

test.describe("failures are explained", () => {
  for (const { name, url, message, status } of [
    { name: "an unreachable host", url: "http://[::1]:4101/nothing", message: /ECONNREFUSED/, status: "0 Network Error" },
    { name: "an invalid URL", url: "not a url", message: "Invalid URL: not a url", status: "0 Bad Request" },
    { name: "a non-http scheme", url: "ftp://example.com/file", message: "Only http and https URLs can be proxied", status: "0 Bad Request" },
    { name: "an empty URL", url: "", message: "Missing required fields: url, method", status: "0 Bad Request" },
    { name: "an upstream that never answers", url: `${API}/hang`, message: "Upstream did not answer within 4000 ms", status: "0 Timeout" },
    { name: "a response over the size cap", url: `${API}/huge`, message: /larger than the proxy limit/, status: "0 Response Too Large" },
  ]) {
    test(`${name}`, async ({ page, seed }) => {
      await blankRequest(page, seed);
      await urlInput(page).fill(url);
      await send(page);
      await expect(response(page).getByText(message)).toBeVisible({ timeout: 10_000 });
      await expect(statusBadge(page)).toHaveText(status);
      await expect(sendButton(page)).toBeEnabled();
    });
  }

  test("invalid JSON in a tab blocks sending and says why", async ({ page, seed, request }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/echo`);
    await fillTab(page, "body", '{"broken": ');
    await expect(page.getByText(/JSON|Unexpected end/i).first()).toBeVisible();
    await expect(sendButton(page)).toBeDisabled();
    let proxied = false;
    page.on("request", (r) => r.url().endsWith("/api/proxy") && (proxied = true));
    await editor(page, "body").press("ControlOrMeta+Enter");
    await expect(page.getByText("Cannot send request")).toBeVisible();
    expect(proxied).toBe(false);

    await editor(page, "body").fill('{"broken": false}');
    await expect(sendButton(page)).toBeEnabled();
    void request;
  });

  test("a slow request shows a loading state and cannot be sent twice", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/slow?ms=1500`);
    await send(page);
    await expect(page.getByRole("button", { name: "Sending…" })).toBeDisabled();
    await expect(statusBadge(page)).toHaveText("200 OK", { timeout: 10_000 });
    await expect(page.getByRole("button", { name: "Send", exact: true })).toBeEnabled();
    await expect(response(page).getByText(/^1\.\d\ds$/)).toBeVisible();
  });
});

test.describe("keyboard and saving", () => {
  test("Ctrl/Cmd+Enter sends from anywhere, even while typing", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await urlInput(page).fill(`${API}/users/2`);
    await urlInput(page).press("ControlOrMeta+Enter");
    await expect(statusBadge(page)).toHaveText("200 OK");
    await expect(response(page).getByText('"Alan Turing"')).toBeVisible();
  });

  test("Ctrl/Cmd+J beautifies the JSON in the active tab", async ({ page, seed }) => {
    await blankRequest(page, seed);
    await fillTab(page, "body", '{"a":1,"b":[1,2]}');
    await editor(page, "body").press("ControlOrMeta+j");
    await expect(editor(page, "body")).toHaveValue('{\n\t"a": 1,\n\t"b": [\n\t\t1,\n\t\t2\n\t]\n}');
    await page.getByRole("button", { name: "Beautify JSON (Ctrl/Cmd+J)" }).click();
    await expect(editor(page, "body")).toHaveValue('{\n\t"a": 1,\n\t"b": [\n\t\t1,\n\t\t2\n\t]\n}');
  });

  test("unsaved changes show a pill, Ctrl/Cmd+S saves them with the response", async ({ page, seed }) => {
    await seed({ collections: [collection("Saving", [route("me", `curl -X GET ${API}/users/1`)])] });
    await openRoute(page, "Saving", "me");
    const pill = page.getByText("Unsaved · Ctrl/Cmd+S");
    await expect(pill).toBeHidden();
    await expect(page.getByRole("button", { name: "Save", exact: true })).toBeDisabled();

    await urlInput(page).fill(`${API}/users/3`);
    await expect(pill).toBeVisible();
    await send(page);
    await expect(response(page).getByText('"Grace Hopper"')).toBeVisible();
    await urlInput(page).press("ControlOrMeta+s");
    await expect(pill).toBeHidden();

    await page.reload();
    await expect(urlInput(page)).toHaveValue(`${API}/users/3`);
    // The last response travels with the saved route.
    await expect(response(page).getByText('"Grace Hopper"')).toBeVisible();
    await expect(sidebarItem(page, "me")).toContainText("GET");
  });

  test("a saved JSON body keeps its numbers, booleans and objects", async ({ page, seed }) => {
    await seed({ collections: [collection("Types", [route("r", `curl -X POST ${API}/echo -H "Content-Type: application/json" -d '{"n":1,"b":true,"z":null,"o":{"k":[2]}}'`)])] });
    await openRoute(page, "Types", "r");
    await page.getByRole("button", { name: "Body", exact: true }).click();
    const expected = { n: 1, b: true, z: null, o: { k: [2] } };
    await expect.poll(async () => JSON.parse(await editor(page, "body").inputValue())).toEqual(expected);
    await send(page);
    await expect(response(page).getByText("true", { exact: true })).toBeVisible();
    await expect(page.getByText("Unsaved · Ctrl/Cmd+S")).toBeHidden();
    await urlInput(page).press("ControlOrMeta+s");
    await page.reload();
    await page.getByRole("button", { name: "Body", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "body").inputValue())).toEqual(expected);
  });

  test("reformatting alone is not an unsaved change", async ({ page, seed }) => {
    await seed({ collections: [collection("Fmt", [route("r", `curl -X POST ${API}/echo -H "A: 1" -d '{"x":1}'`)])] });
    await openRoute(page, "Fmt", "r");
    await fillTab(page, "body", '{   "x" :   1   }');
    await expect(page.getByText("Unsaved · Ctrl/Cmd+S")).toBeHidden();
    await fillTab(page, "body", '{"x": 2}');
    await expect(page.getByText("Unsaved · Ctrl/Cmd+S")).toBeVisible();
  });

  test("Discard changes restores the saved request", async ({ page, seed }) => {
    await seed({ collections: [collection("Undo", [route("r", `curl -X GET ${API}/users/1`)])] });
    await openRoute(page, "Undo", "r");
    await methodSelect(page).selectOption("DELETE");
    await urlInput(page).fill(`${API}/users/2`);
    await page.getByRole("button", { name: "Discard changes" }).click();
    await expect(methodSelect(page)).toHaveValue("GET");
    await expect(urlInput(page)).toHaveValue(`${API}/users/1`);
    await expect(page.getByText("Unsaved · Ctrl/Cmd+S")).toBeHidden();
  });

  test("the save button saves too, and the method colour follows the method", async ({ page, seed }) => {
    await seed({ collections: [collection("Btn", [route("r", `curl -X GET ${API}/users/1`)])] });
    await openRoute(page, "Btn", "r");
    await methodSelect(page).selectOption("PATCH");
    await expect(methodSelect(page)).toHaveCSS("color", "rgb(167, 139, 250)");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(sidebarItem(page, "r")).toContainText("PATCH");
    await page.reload();
    await expect(methodSelect(page)).toHaveValue("PATCH");
  });

  test("closing the tab with unsaved edits asks first", async ({ page, seed }) => {
    await seed({ collections: [collection("Leave", [route("r", `curl -X GET ${API}/users/1`)])] });
    await openRoute(page, "Leave", "r");
    await urlInput(page).fill(`${API}/users/2`);
    const prompted = new Promise<string>((resolve) => page.once("dialog", (d) => resolve(d.type())));
    await page.close({ runBeforeUnload: true });
    expect(await prompted).toBe("beforeunload");
  });

  test("switching routes loads each one's own request", async ({ page, seed }) => {
    await seed({
      collections: [
        collection("Two", [
          route("first", `curl -X GET ${API}/users/1 -H "X-One: 1"`),
          route("second", `curl -X POST ${API}/users -H "Content-Type: application/json" -d '{"name":"Two"}'`),
        ]),
      ],
    });
    await openRoute(page, "Two", "first");
    await sidebarItem(page, "second").click();
    await expect(methodSelect(page)).toHaveValue("POST");
    await page.getByRole("button", { name: "Body", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "body").inputValue())).toEqual({ name: "Two" });
    await sidebarItem(page, "first").click();
    await expect(methodSelect(page)).toHaveValue("GET");
    await page.getByRole("button", { name: "Headers", exact: true }).click();
    await expect.poll(async () => JSON.parse(await editor(page, "headers").inputValue())).toEqual({ "X-One": "1" });
  });
});
