import {
  test,
  expect,
  sidebar,
  sidebarItem,
  rowAction,
  dialog,
  urlInput,
  collection,
  route,
  folder,
  API,
} from "./fixtures/test";

test.describe("first run", () => {
  test("a new visitor gets the sample collection and an empty runner", async ({ page }) => {
    await page.goto("/hittable");
    await expect(sidebar(page).getByText("Collections", { exact: true })).toBeVisible();
    await expect(sidebarItem(page, "JSONPlaceholder Lab")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Select a route" })).toBeVisible();

    await sidebarItem(page, "JSONPlaceholder Lab").click();
    // Opening a collection opens its first route.
    await expect(page).toHaveURL(/c=JSONPlaceholder\+Lab|c=JSONPlaceholder%20Lab/);
    await expect(urlInput(page)).toHaveValue("https://jsonplaceholder.typicode.com/posts?_limit=5");
    for (const m of ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"]) {
      await expect(sidebar(page).getByText(m, { exact: true })).toBeVisible();
    }
  });

  test("with nothing stored it says there are no collections yet", async ({ page, seed }) => {
    await seed({ collections: [] });
    await page.goto("/hittable");
    await expect(page.getByRole("heading", { name: "No collections yet" })).toBeVisible();
    await expect(sidebar(page).getByText("No collections")).toBeVisible();
  });
});

test.describe("collections", () => {
  test.beforeEach(async ({ seed }) => {
    await seed({ collections: [] });
  });

  test("create a collection, a route and a folder, and keep them after reload", async ({ page }) => {
    await page.goto("/hittable");
    await sidebar(page).getByRole("button", { name: "New Collection" }).click();
    const create = dialog(page, "Create collection");
    await expect(create).toBeVisible();
    await create.getByRole("textbox").fill("Users API");
    await create.getByRole("textbox").press("Enter");
    await expect(create).toBeHidden();
    await expect(sidebar(page).getByText("No routes yet")).toBeVisible();

    await sidebar(page).getByRole("button", { name: "New Route" }).click();
    const createRoute = dialog(page, "Create route");
    await expect(createRoute.getByText("Adding to Users API")).toBeVisible();
    await createRoute.getByRole("textbox", { name: "get-users" }).fill("list users");
    await createRoute.getByPlaceholder("Paste you curl here").fill(`curl -X GET ${API}/users -H "Accept: application/json"`);
    await createRoute.getByRole("button", { name: "Create" }).click();
    await expect(urlInput(page)).toHaveValue(`${API}/users`);
    await expect(sidebarItem(page, "list users")).toContainText("GET");

    await sidebar(page).getByRole("button", { name: "New Folder" }).click();
    await expect(sidebarItem(page, "New Folder")).toBeVisible();

    await page.reload();
    await expect(sidebarItem(page, "list users")).toBeVisible();
    await expect(sidebarItem(page, "New Folder")).toBeVisible();
    await expect(urlInput(page)).toHaveValue(`${API}/users`);
  });

  test("an empty name does not create anything and Escape cancels", async ({ page }) => {
    await page.goto("/hittable");
    await sidebar(page).getByRole("button", { name: "New Collection" }).click();
    await dialog(page, "Create collection").getByRole("button", { name: "Create" }).click();
    await expect(dialog(page, "Create collection")).toBeHidden();
    await expect(sidebar(page).getByText("No collections")).toBeVisible();

    await sidebar(page).getByRole("button", { name: "New Collection" }).click();
    await dialog(page, "Create collection").getByRole("textbox").fill("never");
    await page.keyboard.press("Escape");
    await expect(dialog(page, "Create collection")).toBeHidden();
    await expect(sidebarItem(page, "never")).toHaveCount(0);
  });

  test("clicking the backdrop closes a dialog", async ({ page }) => {
    await page.goto("/hittable");
    await sidebar(page).getByRole("button", { name: "New Collection" }).click();
    await expect(dialog(page, "Create collection")).toBeVisible();
    await page.mouse.click(5, 300);
    await expect(dialog(page, "Create collection")).toBeHidden();
  });

  test("a dialog takes focus, keeps Tab inside, and gives focus back on close", async ({ page }) => {
    await page.goto("/hittable");
    const opener = sidebar(page).getByRole("button", { name: "New Collection" });
    // A keyboard user: focus the button and press Enter. (Focus return matters
    // for them; macOS Safari doesn't focus a button on mouse click at all.)
    await opener.focus();
    await page.keyboard.press("Enter");
    const create = dialog(page, "Create collection");
    // Typing straight away goes into the name field.
    await expect(create.getByRole("textbox")).toBeFocused();
    await page.keyboard.type("typed");
    await expect(create.getByRole("textbox")).toHaveValue("typed");
    await page.keyboard.press("Tab");
    await expect(create.getByRole("button", { name: "Cancel" })).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(create.getByRole("button", { name: "Create" })).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(create.getByRole("textbox")).toBeFocused();
    await page.keyboard.press("Shift+Tab");
    await expect(create.getByRole("button", { name: "Create" })).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(create).toBeHidden();
    await expect(opener).toBeFocused();
  });

  test("a duplicate collection name gets a suffix instead of overwriting", async ({ page, seed }) => {
    await seed({ collections: [collection("Shop", [route("one", `curl -X GET ${API}/users`)])] });
    await page.goto("/hittable");
    await sidebar(page).getByRole("button", { name: "New Collection" }).click();
    await dialog(page, "Create collection").getByRole("textbox").fill("Shop");
    await page.keyboard.press("Enter");
    await expect(sidebar(page).getByRole("button", { name: /Shop - New/ })).toBeVisible();
    await sidebar(page).getByRole("button", { name: /Shop - New/ }).click();
    await expect(sidebarItem(page, "Shop")).toBeVisible();
    await expect(sidebarItem(page, "Shop - New")).toBeVisible();
  });
});

test.describe("routes and folders", () => {
  test.beforeEach(async ({ seed }) => {
    await seed({
      collections: [
        collection("Store", [
          folder("orders", [route("list orders", `curl -X GET ${API}/users`), route("cancel order", `curl -X DELETE ${API}/users/1`)]),
          route("health", `curl -X GET ${API}/echo`),
          route("create user", `curl -X POST ${API}/users -H "Content-Type: application/json" -d '{"name":"x"}'`),
        ]),
        collection("Other", [route("ping", `curl -X GET ${API}/echo`)]),
      ],
    });
  });

  test("drill into a folder, open a route and come back with the breadcrumb", async ({ page }) => {
    await page.goto("/hittable?c=Store");
    await expect(sidebarItem(page, "orders")).toContainText("2");
    await sidebarItem(page, "orders").click();
    await expect(page).toHaveURL(/p=orders/);
    await sidebarItem(page, "cancel order").click();
    await expect(page.getByRole("combobox", { name: "HTTP method" })).toHaveValue("DELETE");
    await expect(urlInput(page)).toHaveValue(`${API}/users/1`);

    // The sidebar breadcrumb jumps back to the collection root.
    // [0] is the back arrow, [1] the breadcrumb root.
    await sidebar(page).getByRole("button", { name: "Store", exact: true }).nth(1).click();
    await expect(sidebarItem(page, "health")).toBeVisible();
    // The back arrow leaves the collection.
    await sidebar(page).getByRole("button", { name: "Store", exact: true }).click();
    await expect(sidebarItem(page, "Other")).toBeVisible();
  });

  test("a deep link opens the route inside its folder", async ({ page }) => {
    await page.goto("/hittable?c=Store&r=list%20orders&p=orders");
    await expect(urlInput(page)).toHaveValue(`${API}/users`);
    await expect(page.getByRole("button", { name: "list orders" })).toBeVisible();
  });

  test("Shift+T adds an untitled route, but not while typing", async ({ page }) => {
    await page.goto("/hittable?c=Store&r=health");
    await urlInput(page).click();
    await page.keyboard.press("Shift+T");
    await expect(urlInput(page)).toHaveValue(`${API}/echoT`);
    await expect(sidebarItem(page, "Untitled")).toHaveCount(0);

    await page.locator("body").click({ position: { x: 600, y: 10 } });
    await page.keyboard.press("Shift+T");
    await expect(sidebarItem(page, "Untitled")).toBeVisible();
    await expect(page).toHaveURL(/r=Untitled/);
    await page.keyboard.press("Shift+T");
    await expect(sidebarItem(page, "Untitled - New")).toBeVisible();
  });

  test("rename a route, a folder and a collection", async ({ page }) => {
    await page.goto("/hittable?c=Store&r=health");
    await rowAction(page, "health", "Rename");
    const rename = dialog(page, "Rename route");
    await expect(rename.getByText("Currently: health")).toBeVisible();
    await expect(rename.getByRole("textbox")).toHaveValue("health");
    await rename.getByRole("textbox").fill("status check");
    await rename.getByRole("textbox").press("Enter");
    await expect(sidebarItem(page, "status check")).toBeVisible();
    await expect(sidebarItem(page, "health")).toHaveCount(0);
    await expect(page).toHaveURL(/r=status(\+|%20)check/);

    await rowAction(page, "orders", "Rename");
    await dialog(page, "Rename folder").getByRole("textbox").fill("purchases");
    await dialog(page, "Rename folder").getByRole("button", { name: "Rename" }).click();
    await sidebarItem(page, "purchases").click();
    await expect(sidebarItem(page, "list orders")).toBeVisible();

    await page.goto("/hittable");
    await rowAction(page, "Other", "Rename");
    await dialog(page, "Rename collection").getByRole("textbox").fill("Renamed");
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/c=Renamed/); // the rename settles before the reload
    await page.goto("/hittable");
    await expect(sidebarItem(page, "Renamed")).toBeVisible();
    await expect(sidebarItem(page, "Other")).toHaveCount(0);
  });

  test("renaming onto an existing name is refused with a message", async ({ page }) => {
    await page.goto("/hittable?c=Store&r=health");
    await rowAction(page, "health", "Rename");
    const rename = dialog(page, "Rename route");
    await rename.getByRole("textbox").fill("create user");
    await rename.getByRole("button", { name: "Rename" }).click();
    await expect(rename.getByText('"create user" already exists')).toBeVisible();
    await rename.getByRole("button", { name: "Cancel" }).click();
    await expect(sidebarItem(page, "health")).toBeVisible();

    await page.goto("/hittable");
    await rowAction(page, "Other", "Rename");
    await dialog(page, "Rename collection").getByRole("textbox").fill("Store");
    await page.keyboard.press("Enter");
    await expect(dialog(page, "Rename collection").getByText('"Store" already exists')).toBeVisible();
  });

  test("delete a route after confirming", async ({ page }) => {
    await page.goto("/hittable?c=Store&r=health");
    await rowAction(page, "health", "Delete");
    const del = dialog(page, "Delete route");
    await expect(del.getByText('This will permanently remove "health".')).toBeVisible();
    await del.getByRole("button", { name: "Cancel" }).click();
    await expect(sidebarItem(page, "health")).toBeVisible();

    await rowAction(page, "health", "Delete");
    await dialog(page, "Delete route").getByRole("button", { name: "Delete" }).click();
    await expect(sidebarItem(page, "health")).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "Select a route" })).toBeVisible();
    await page.reload();
    await expect(sidebarItem(page, "create user")).toBeVisible();
    await expect(sidebarItem(page, "health")).toHaveCount(0);
  });

  test("deleting a folder warns how much goes with it", async ({ page }) => {
    await page.goto("/hittable?c=Store");
    await rowAction(page, "orders", "Delete");
    const del = dialog(page, "Delete folder");
    await expect(del.getByText(/remove "orders" and its 2 route\(s\), 0 sub-folder\(s\)/)).toBeVisible();
    await del.getByRole("button", { name: "Delete" }).click();
    await expect(sidebarItem(page, "orders")).toHaveCount(0);
    await expect(sidebarItem(page, "health")).toBeVisible();
  });

  test("delete a whole collection", async ({ page }) => {
    await page.goto("/hittable");
    await rowAction(page, "Store", "Delete");
    const del = dialog(page, "Delete collection");
    await expect(del.getByText('This will permanently remove "Store" and all its routes.')).toBeVisible();
    await del.getByRole("button", { name: "Delete" }).click();
    await expect(sidebarItem(page, "Store")).toHaveCount(0);
    await expect(sidebarItem(page, "Other")).toBeVisible();
    await page.reload();
    await expect(sidebarItem(page, "Store")).toHaveCount(0);
  });

  test("the filter box narrows a long list", async ({ page, seed }) => {
    await seed({
      collections: [collection("Many", ["alpha", "beta", "gamma", "delta", "epsilon"].map((n) => route(n, `curl -X GET ${API}/echo`)))],
    });
    await page.goto("/hittable?c=Many");
    const filter = sidebar(page).getByPlaceholder("Filter...");
    await filter.fill("ta");
    await expect(sidebarItem(page, "beta")).toBeVisible();
    await expect(sidebarItem(page, "delta")).toBeVisible();
    await expect(sidebarItem(page, "alpha")).toHaveCount(0);
    await filter.fill("zzz");
    await expect(sidebar(page).getByText("No matching items")).toBeVisible();
  });

  test("the breadcrumb above the URL bar switches routes and collections", async ({ page }) => {
    await page.goto("/hittable?c=Store&r=health");
    const crumbs = page.getByRole("navigation", { name: "Breadcrumb" });
    await crumbs.getByRole("button", { name: "health", exact: true }).click();
    await crumbs.getByRole("button", { name: /^POST\s*create user$/ }).click();
    await expect(page.getByRole("combobox", { name: "HTTP method" })).toHaveValue("POST");
    await crumbs.getByRole("button", { name: "Store", exact: true }).click();
    await crumbs.getByRole("button", { name: "Other", exact: true }).click();
    await expect(page).toHaveURL(/c=Other&r=ping/);
    await expect(urlInput(page)).toHaveValue(`${API}/echo`);
  });

  test("Ctrl/Cmd+B collapses and expands the sidebar, and it is remembered", async ({ page }) => {
    // With "reduce motion" on, the sidebar resizes instantly, so a quick second
    // click can't land on a button that is still sliding.
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/hittable?c=Store");
    await expect(sidebarItem(page, "health")).toBeVisible();
    await page.keyboard.press("ControlOrMeta+b");
    await expect(sidebarItem(page, "health")).toBeHidden();
    await expect(sidebar(page).getByRole("button", { name: "Expand sidebar" })).toBeVisible();
    await page.reload();
    await expect(sidebar(page).getByRole("button", { name: "Expand sidebar" })).toBeVisible();
    await sidebar(page).getByRole("button", { name: "Expand sidebar" }).click();
    await expect(sidebarItem(page, "health")).toBeVisible();
    await sidebar(page).getByRole("button", { name: "Collapse sidebar" }).click();
    await expect(sidebarItem(page, "health")).toBeHidden();
  });
});
