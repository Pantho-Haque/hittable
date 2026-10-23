// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ImportModal from "@/components/modals/ImportModal";
import { compressString } from "@/utils/compressString";
import type { THittableCollection, THittableCollections } from "@/types";
import { renderWithData, stored } from "./renderWithData";

const existing: THittableCollections = [{ collectionName: "Shop", env: {}, secrets: {}, items: [] }];

const postman = {
  info: { name: "Shop", schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" },
  item: [
    { name: "List", request: { method: "GET", url: "{{base}}/items" } },
    { name: "Orders", item: [{ name: "Create", request: { method: "POST", url: "https://x.dev/o", body: { mode: "raw", raw: "{}" } } }] },
  ],
  variable: [{ key: "base", value: "https://x.dev" }],
};

const insomnia = {
  _type: "export",
  __export_format: 4,
  resources: [
    { _type: "workspace", _id: "wrk_1", name: "Ins" },
    { _type: "request", _id: "req_1", parentId: "wrk_1", name: "Ping", method: "GET", url: "{{ _.host }}/ping" },
    { _type: "environment", _id: "env_1", parentId: "wrk_1", data: { host: "https://ins.dev" } },
  ],
};

const native: THittableCollection = {
  collectionName: "Native",
  env: { base: "https://n.dev" },
  secrets: { token: "s3cr3t" },
  items: [{ type: "folder", name: "users", items: [{ type: "route", name: "get", curl: "curl -X GET <<base>>/u", response: "" }] }],
};

function setup() {
  renderWithData(<ImportModal />, { collections: existing });
  return userEvent.setup();
}

async function importText(user: ReturnType<typeof userEvent.setup>, text: string) {
  await user.click(screen.getByTitle("Import"));
  const box = screen.getByPlaceholderText(/Paste collection data here/);
  await user.click(box);
  await user.paste(text);
  await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Import" }));
}

const names = () => stored().map((c) => c.collectionName);

beforeEach(() => localStorage.clear());

describe("ImportModal", () => {
  it("detects a Postman collection, dedupes its name and reports what was skipped", async () => {
    const user = setup();
    await importText(user, JSON.stringify(postman));
    expect(names()).toEqual(["Shop", "Shop - 1"]);
    const imported = stored()[1];
    expect(imported.env).toMatchObject({ base: "https://x.dev" });
    expect(imported.items.map((i) => i.name)).toEqual(["List", "Orders"]);
    expect(screen.getByText("Imported from Postman")).toBeInTheDocument();
    expect(screen.getByText(/1 collection\(s\), 2 route\(s\).*scripts.*not supported/)).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("numbers repeated imports of the same collection Name - 1, Name - 2", async () => {
    const user = setup();
    await importText(user, JSON.stringify(postman));
    await importText(user, JSON.stringify(postman));
    expect(names()).toEqual(["Shop", "Shop - 1", "Shop - 2"]);
  });

  it("gives every collection from one paste its own name", async () => {
    const user = setup();
    // An Insomnia export with two workspaces becomes two collections.
    const twoFolders = {
      _type: "export",
      __export_format: 4,
      resources: [
        { _type: "workspace", _id: "w1", name: "Shop" },
        { _type: "workspace", _id: "w2", name: "Shop" },
        { _type: "request", _id: "r1", parentId: "w1", name: "a", method: "GET", url: "https://x.dev/a" },
        { _type: "request", _id: "r2", parentId: "w2", name: "b", method: "GET", url: "https://x.dev/b" },
      ],
    };
    await importText(user, JSON.stringify(twoFolders));
    expect(names()).toEqual(["Shop", "Shop - 1", "Shop - 2"]);
  });

  it("detects an Insomnia export and translates its variables", async () => {
    const user = setup();
    await importText(user, JSON.stringify(insomnia));
    expect(names()).toEqual(["Shop", "Ins"]);
    const imported = stored()[1];
    expect(imported.env).toEqual({ host: "https://ins.dev" });
    expect(imported.items[0]).toMatchObject({ type: "route", name: "Ping" });
    expect((imported.items[0] as { curl: string }).curl).toContain("<<host>>/ping");
    expect(screen.getByText("Imported from Insomnia")).toBeInTheDocument();
    expect(screen.getByText(/plugins, certificate configs, and cookie jars were not imported/)).toBeInTheDocument();
  });

  it("imports a compressed Hittable string with folders, env and secrets intact", async () => {
    const user = setup();
    await importText(user, compressString(JSON.stringify(native)));
    expect(names()).toEqual(["Shop", "Native"]);
    expect(stored()[1]).toEqual(native);
    expect(screen.getByText('Collection "Native" imported (Hittable native)')).toBeInTheDocument();
  });

  it("imports a plain Hittable collection object", async () => {
    const user = setup();
    await importText(user, JSON.stringify(native));
    expect(stored()[1]).toEqual(native);
  });

  it("upgrades the legacy flat `curls` form, turning slashed names into folders", async () => {
    const user = setup();
    const legacy = {
      collectionName: "Old",
      env: { a: "1" },
      curls: [
        { name: "ping", curl: "curl https://x.dev/ping", response: "" },
        { name: "users/list", curl: "curl https://x.dev/u", response: "" },
      ],
    };
    await importText(user, compressString(JSON.stringify(legacy)));
    const col = stored()[1];
    expect(col).not.toHaveProperty("curls");
    expect(col).toMatchObject({
      collectionName: "Old",
      env: { a: "1" },
      secrets: {},
      items: [
        { type: "route", name: "ping" },
        { type: "folder", name: "users", items: [{ type: "route", name: "list" }] },
      ],
    });
  });

  it("rejects a corrupt Hittable string and keeps the dialog open with the text", async () => {
    const user = setup();
    await importText(user, "definitely-not-a-valid-export");
    expect(screen.getByText("Invalid or corrupted import data")).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/Paste collection data here/)).toHaveValue("definitely-not-a-valid-export");
    expect(names()).toEqual(["Shop"]);
  });

  it("rejects a valid compressed string that is not a collection", async () => {
    const user = setup();
    await importText(user, compressString(JSON.stringify({ hello: "world" })));
    expect(screen.getByText("Invalid collection structure")).toBeInTheDocument();
    expect(names()).toEqual(["Shop"]);
  });

  it("rejects JSON it does not recognise", async () => {
    const user = setup();
    await importText(user, JSON.stringify({ foo: 1 }));
    expect(screen.getByText(/Unrecognized format/)).toBeInTheDocument();
    expect(names()).toEqual(["Shop"]);
  });

  it("says so when a Postman collection holds no requests", async () => {
    const user = setup();
    await importText(user, JSON.stringify({ info: postman.info, item: [] }));
    // An empty collection is still a collection; it imports with no routes.
    expect(names()).toEqual(["Shop", "Shop - 1"]);
  });

  it("fails cleanly on hostile Postman input instead of crashing", async () => {
    const user = setup();
    await importText(user, JSON.stringify({ info: postman.info, item: [null, 5, "x"] }));
    // No uncaught exception; whatever was stored is well-formed and the junk
    // entries did not become routes.
    expect(names()[0]).toBe("Shop");
    for (const col of stored()) expect(col.items).toEqual([]);
  });

  it("clears the error once the user edits the text", async () => {
    const user = setup();
    await importText(user, "garbage");
    expect(screen.getByText("Invalid or corrupted import data")).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText(/Paste collection data here/), "!");
    expect(screen.queryByText("Invalid or corrupted import data")).not.toBeInTheDocument();
  });

  it("closes on Escape and Cancel without importing", async () => {
    const user = setup();
    await user.click(screen.getByTitle("Import"));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(screen.getByTitle("Import"));
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(names()).toEqual(["Shop"]);
  });
});
