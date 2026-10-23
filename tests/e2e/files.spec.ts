import { test, expect, API, urlInput, send, statusBadge, editor } from "./fixtures/test";
import { seedPrivateFolder, readFile, readBytes, node, expand, openWorkspace } from "./fixtures/directory";

// What happens when a user opens real files from their project: big source
// files, images, binaries, and .hit files edited in Runner and Text. The
// terminal app follows the same rules (shellapp/ui/screens/filekinds_test.go).

const PNG_3x2 =
  "iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAEElEQVR4nGP4z8AAQQxwFgBB0gX7h/C5SAAAAABJRU5ErkJggg==";
const SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><rect width="40" height="20" fill="teal"/><script>window.__svgRan = true</script></svg>`;
const BIG_SOURCE = Array.from({ length: 5000 }, (_, i) => `export const value${i} = ${i}; // line ${i + 1}`).join("\n") + "\n";

const codeEditor = (page: import("@playwright/test").Page, name: string) => page.getByRole("textbox", { name: `Edit ${name}` });

async function open(page: import("@playwright/test").Page, files: Record<string, string>, path: string) {
  await seedPrivateFolder(page, files);
  await openWorkspace(page);
  const parts = path.split("/");
  for (let i = 1; i < parts.length; i++) await expand(page, parts.slice(0, i).join("/"));
  await node(page, path).click();
}

test.describe("opening real files", () => {
  test.skip(({ browserName }) => browserName !== "chromium", "Directory mode needs the File System Access API (Chromium only)");

  test("a 5,000-line source file opens quickly, scrolls to the end, and saves an edit", async ({ page }) => {
    const start = Date.now();
    await open(page, { "src/big.ts": BIG_SOURCE }, "src/big.ts");
    const ed = codeEditor(page, "big.ts");
    await expect(ed).toBeVisible();
    expect(Date.now() - start).toBeLessThan(8_000);

    await ed.click();
    await page.keyboard.press("ControlOrMeta+End");
    await expect(page.getByText("// line 5000")).toBeVisible();
    await page.keyboard.type("// added at the end");
    // Read until the last write lands (a file being swapped in can't be read for a moment).
    const saved = () => readFile(page, "src/big.ts").catch(() => "");
    await expect.poll(saved, { timeout: 5_000 }).toContain("// added at the end");
    const text = await saved();
    expect(text.indexOf("// added at the end")).toBeGreaterThan(text.indexOf("// line 5000"));
    expect(text.startsWith("export const value0 = 0; // line 1\n")).toBe(true); // the rest is untouched
  });

  test("an image is shown as a picture, and typing never touches its bytes", async ({ page }) => {
    await open(page, { "assets/logo.png": `base64:${PNG_3x2}` }, "assets/logo.png");
    const img = page.getByRole("img", { name: "logo.png" });
    await expect(img).toBeVisible();
    await expect.poll(() => img.evaluate((el: HTMLImageElement) => [el.naturalWidth, el.naturalHeight])).toEqual([3, 2]);
    await expect(page.getByText(/PNG image · 3 × 2/)).toBeVisible();
    await expect(page.getByRole("textbox", { name: /^Edit / })).toHaveCount(0);
    await page.keyboard.type("oops");
    await page.keyboard.press("ControlOrMeta+s");
    expect(await readBytes(page, "assets/logo.png")).toBe(PNG_3x2);
  });

  test("an SVG opens as code with a picture preview, and its scripts never run", async ({ page }) => {
    await open(page, { "assets/badge.svg": SVG }, "assets/badge.svg");
    await expect(codeEditor(page, "badge.svg")).toBeVisible();
    await page.getByRole("button", { name: "Preview" }).click();
    const img = page.getByRole("img", { name: "badge.svg" });
    await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.naturalWidth)).toBe(40);
    expect(await page.evaluate(() => (window as unknown as { __svgRan?: boolean }).__svgRan)).toBeUndefined();
    await page.getByRole("button", { name: "Code" }).click();
    await expect(codeEditor(page, "badge.svg")).toBeVisible();
  });

  test("other binary files say so, and are never opened as text or rewritten", async ({ page }) => {
    const bin = btoa(String.fromCharCode(0x7f, 0x45, 0x4c, 0x46, 0, 0, 0, 1, 2, 3, 0, 255));
    await open(page, { "bin/tool": `base64:${bin}` }, "bin/tool");
    await expect(page.getByText(/Binary file · 12 B/)).toBeVisible();
    await expect(page.getByRole("textbox", { name: /^Edit / })).toHaveCount(0);
    await page.keyboard.type("x");
    expect(await readBytes(page, "bin/tool")).toBe(bin);
  });

  test("a text file over 1 MiB is not loaded into the editor", async ({ page }) => {
    const huge = "x".repeat(1024 * 1024 + 10);
    await open(page, { "logs/huge.log": huge }, "logs/huge.log");
    await expect(page.getByText(/Too large to edit · 1\.0 MB/)).toBeVisible();
    await expect(page.getByRole("textbox", { name: /^Edit / })).toHaveCount(0);
    expect((await readFile(page, "logs/huge.log")).length).toBe(huge.length);
  });
});

test.describe(".hit files: Runner and Text stay in step", () => {
  test.skip(({ browserName }) => browserName !== "chromium", "Directory mode needs the File System Access API (Chromium only)");

  const hit = (over: Record<string, unknown> = {}) =>
    JSON.stringify({ method: "GET", url: `${API}/users/1`, headers: { "X-Keep": "1" }, params: {}, body: "", response: null, ...over }, null, 2);
  const onDisk = async (page: import("@playwright/test").Page) => JSON.parse(await readFile(page, "hittable/req.hit"));

  test("a valid .hit opens in the Runner, like the terminal app", async ({ page }) => {
    await open(page, { "hittable/req.hit": hit() }, "hittable/req.hit");
    await expect(page.getByRole("button", { name: "Runner", exact: true })).toHaveAttribute("aria-pressed", "true");
    await expect(urlInput(page)).toHaveValue(`${API}/users/1`);
  });

  test("opening a .hit and switching views never rewrites it", async ({ page }) => {
    const custom = '{"method":"GET","url":"' + API + '/users/1","headers":{},"params":{},"body":"","response":null}';
    await open(page, { "hittable/req.hit": custom }, "hittable/req.hit");
    await expect(urlInput(page)).toHaveValue(`${API}/users/1`);
    await page.getByRole("button", { name: "Text", exact: true }).click();
    await page.getByRole("button", { name: "Runner", exact: true }).click();
    await page.waitForTimeout(500);
    expect(await readFile(page, "hittable/req.hit")).toBe(custom); // byte for byte: no noise in git
  });

  test("a Runner edit reaches the disk at once and shows in Text", async ({ page }) => {
    await open(page, { "hittable/req.hit": hit() }, "hittable/req.hit");
    await urlInput(page).fill(`${API}/users/4`);
    await expect.poll(async () => (await onDisk(page)).url, { timeout: 3_000 }).toBe(`${API}/users/4`);
    await page.getByRole("button", { name: "Text", exact: true }).click();
    await expect(codeEditor(page, "req.hit")).toContainText(`"${API}/users/4"`);
  });

  test("a Text edit reaches the disk at once and shows in the Runner", async ({ page }) => {
    await open(page, { "hittable/req.hit": hit() }, "hittable/req.hit");
    await page.getByRole("button", { name: "Text", exact: true }).click();
    const ed = codeEditor(page, "req.hit");
    await ed.click();
    await page.keyboard.press("ControlOrMeta+a");
    await page.keyboard.insertText(hit({ method: "DELETE", url: `${API}/users/9` }));
    await expect.poll(async () => (await onDisk(page)).method, { timeout: 3_000 }).toBe("DELETE");
    await page.getByRole("button", { name: "Runner", exact: true }).click();
    await expect(urlInput(page)).toHaveValue(`${API}/users/9`);
    await expect(page.getByRole("combobox", { name: "HTTP method" })).toHaveValue("DELETE");
  });

  test("typing headers keeps exactly what you type, and half-typed JSON never wipes the saved headers", async ({ page }) => {
    await open(page, { "hittable/req.hit": hit() }, "hittable/req.hit");
    await page.getByRole("button", { name: "Headers", exact: true }).click();
    const headers = editor(page, "headers");
    await headers.click();
    await page.keyboard.press("ControlOrMeta+a");
    const typed = '{"X-Keep": "1", "X-New": "2"}';
    // Mid-way, the JSON is invalid: the file must still hold the last valid headers.
    await page.keyboard.type(typed.slice(0, 20), { delay: 15 });
    await page.waitForTimeout(500);
    expect((await onDisk(page)).headers).toEqual({ "X-Keep": "1" });
    await page.keyboard.type(typed.slice(20), { delay: 15 });
    await expect(headers).toHaveValue(typed); // not reformatted under the cursor
    await expect.poll(async () => (await onDisk(page)).headers, { timeout: 3_000 }).toEqual({ "X-Keep": "1", "X-New": "2" });
  });

  test("switching to Text with half-typed headers shows the last valid file, not empty headers", async ({ page }) => {
    await open(page, { "hittable/req.hit": hit() }, "hittable/req.hit");
    await page.getByRole("button", { name: "Headers", exact: true }).click();
    await editor(page, "headers").click();
    await page.keyboard.press("ControlOrMeta+a");
    await page.keyboard.type('{"X-Broken": ', { delay: 10 });
    await page.getByRole("button", { name: "Text", exact: true }).click();
    await expect(codeEditor(page, "req.hit")).toContainText('"X-Keep": "1"');
    expect((await onDisk(page)).headers).toEqual({ "X-Keep": "1" });
  });

  test("a sent response lands in the file and in Text, with the templates intact", async ({ page }) => {
    await open(page, { "hittable/req.hit": hit({ url: "<<BASE>>/users/1" }), "hittable/env.json": JSON.stringify({ BASE: API }) }, "hittable/req.hit");
    await send(page);
    await expect(statusBadge(page)).toContainText("200");
    await expect.poll(async () => (await onDisk(page)).response?.status, { timeout: 3_000 }).toBe(200);
    await page.getByRole("button", { name: "Text", exact: true }).click();
    await expect(codeEditor(page, "req.hit")).toContainText('"<<BASE>>/users/1"');
    await expect(codeEditor(page, "req.hit")).toContainText('"status": 200');
  });
});
