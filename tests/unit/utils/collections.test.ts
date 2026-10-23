import { describe, it, expect } from "vitest";
import {
  createCollectionName, createCurlName, createFolderName, renameCollectionName, renameCurlName, renameFolderName,
  isAlreadyExists, isAlreadyExistsInPath, updateCurl, deleteCollectionName, deleteItem,
  updateEnv, updateSecrets, resolveEnv,
} from "@/utils/hittableCollectionModifier";
import {
  migrateCollections, findRoute, findFolder, getItemsAtPath, findRoutePath, getAllRouteNames, collectAllRouteNames,
  insertItem, removeItem, renameItem, updateRoute, nameExistsInPath, countItemsInFolder,
} from "@/utils/treeHelpers";
import type { THittableCollections, THittableItem, THittableCurlJson } from "@/types";

const r = (name: string, curl = `curl https://x.dev/${name}`): THittableItem => ({ type: "route", name, curl, response: "" });
const f = (name: string, items: THittableItem[] = []): THittableItem => ({ type: "folder", name, items });

const base = (): THittableCollections => [
  {
    collectionName: "Shop", env: { BASE: "https://shop.dev" }, secrets: { TOKEN: "t" },
    items: [r("Health"), f("Users", [r("List"), f("Admin", [r("Ban")])]), f("Empty")],
  },
  { collectionName: "Blog", env: {}, secrets: {}, items: [r("Posts")] },
];
const shop = (cols: THittableCollections) => cols[0].items;

describe("creating collections, routes and folders", () => {
  it("adds an empty collection with env and secrets", () => {
    const next = createCollectionName(base(), "New");
    expect(next[2]).toEqual({ collectionName: "New", items: [], env: {}, secrets: {} });
  });

  it("adds a route at the top level or inside a nested folder, leaving other collections alone", () => {
    const prev = base();
    const top = createCurlName(prev, "Shop", "Ping", "curl https://x.dev/ping");
    expect(shop(top).at(-1)).toEqual({ type: "route", name: "Ping", curl: "curl https://x.dev/ping", response: "" });
    const nested = createCurlName(prev, "Shop", "Unban", "curl x", ["Users", "Admin"]);
    expect(getItemsAtPath(shop(nested), ["Users", "Admin"]).map((i) => i.name)).toEqual(["Ban", "Unban"]);
    expect(nested[1]).toBe(prev[1]);
    // the previous state is never mutated
    expect(getItemsAtPath(shop(prev), ["Users", "Admin"]).map((i) => i.name)).toEqual(["Ban"]);
  });

  it("adds a folder inside another folder", () => {
    const next = createFolderName(base(), "Shop", "Roles", ["Users"]);
    expect(getItemsAtPath(shop(next), ["Users"]).map((i) => [i.type, i.name])).toEqual([["route", "List"], ["folder", "Admin"], ["folder", "Roles"]]);
  });

  it("inserting into a folder path that does not exist changes nothing", () => {
    expect(shop(createCurlName(base(), "Shop", "X", "curl", ["Nope"]))).toEqual(shop(base()));
  });
});

describe("renaming", () => {
  it("renames a collection", () => {
    expect(renameCollectionName(base(), "Blog", "Journal").map((c) => c.collectionName)).toEqual(["Shop", "Journal"]);
  });
  it("renames a route inside a nested folder and keeps its curl", () => {
    const next = renameCurlName(base(), "Ban", "Shop", "Block", ["Users", "Admin"]);
    expect(findRoute(shop(next), ["Users", "Admin"], "Block")?.curl).toBe("curl https://x.dev/Ban");
    expect(findRoute(shop(next), ["Users", "Admin"], "Ban")).toBeUndefined();
  });
  it("renames a folder and keeps its contents", () => {
    const next = renameFolderName(base(), "Users", "Shop", "People");
    expect(findRoute(shop(next), ["People", "Admin"], "Ban")).toBeDefined();
  });
  it("only renames inside the given collection", () => {
    const cols = base();
    cols[1].items.push(r("Health"));
    const next = renameCurlName(cols, "Health", "Shop", "Up");
    expect(next[1].items.map((i) => i.name)).toEqual(["Posts", "Health"]);
  });
});

describe("duplicate-name checks", () => {
  it("isAlreadyExists checks collections, and routes within a collection", () => {
    const list = { Shop: ["Health", "List"], Blog: ["Posts"] };
    expect(isAlreadyExists(list, "collection", "Shop")).toBe(true);
    expect(isAlreadyExists(list, "collection", "shop")).toBe(false);
    expect(isAlreadyExists(list, "route", "Posts", "Blog")).toBe(true);
    expect(isAlreadyExists(list, "route", "Posts", "Shop")).toBe(false);
    expect(isAlreadyExists(list, "route", "Posts")).toBe(false);
    expect(isAlreadyExists(list, "route", "x", "Missing")).toBe(false);
  });
  it("isAlreadyExistsInPath sees routes and folders at that exact level only", () => {
    const [col] = base();
    expect(isAlreadyExistsInPath(col, [], "Users")).toBe(true);
    expect(isAlreadyExistsInPath(col, [], "List")).toBe(false);
    expect(isAlreadyExistsInPath(col, ["Users"], "List")).toBe(true);
    expect(isAlreadyExistsInPath(col, ["Users", "Admin"], "Ban")).toBe(true);
    expect(isAlreadyExistsInPath(undefined, [], "x")).toBe(false);
  });
});

describe("saving and deleting", () => {
  it("updateCurl replaces the curl and response of a nested route", () => {
    const next = updateCurl(base(), "Shop", "Ban", "curl -X POST x", '{"status":200}', ["Users", "Admin"]);
    expect(findRoute(shop(next), ["Users", "Admin"], "Ban")).toMatchObject({ curl: "curl -X POST x", response: '{"status":200}' });
  });
  it("updateCurl never turns a folder with the route's name into a route", () => {
    const next = updateCurl(base(), "Shop", "Users", "curl", "");
    expect(shop(next)[1]).toEqual(shop(base())[1]);
  });
  it("deletes a collection, a nested route and a folder with everything in it", () => {
    expect(deleteCollectionName(base(), "Shop").map((c) => c.collectionName)).toEqual(["Blog"]);
    expect(getItemsAtPath(shop(deleteItem(base(), "Shop", "Ban", ["Users", "Admin"])), ["Users", "Admin"])).toEqual([]);
    expect(shop(deleteItem(base(), "Shop", "Users")).map((i) => i.name)).toEqual(["Health", "Empty"]);
  });
  it("updateEnv and updateSecrets replace only that collection's maps", () => {
    const env = updateEnv(base(), "Shop", { A: "1" });
    expect(env[0]).toMatchObject({ env: { A: "1" }, secrets: { TOKEN: "t" } });
    const sec = updateSecrets(base(), "Blog", { S: "x" });
    expect(sec[1].secrets).toEqual({ S: "x" });
    expect(sec[0].secrets).toEqual({ TOKEN: "t" });
  });
});

describe("resolveEnv: <<KEY>> templates at send time", () => {
  const form: THittableCurlJson = {
    method: "POST", url: "<<BASE>>/users/<<ID>>", headers: '{"Authorization":"Bearer <<TOKEN>>"}',
    body: '{"name":"<<NAME>>","again":"<<NAME>>"}', params: '{"q":"<<Q>>"}',
  };

  it("replaces every occurrence in every field", () => {
    expect(resolveEnv(form, { BASE: "https://x.dev", ID: "7", TOKEN: "abc", NAME: "Ada", Q: "x" })).toEqual({
      method: "POST", url: "https://x.dev/users/7", headers: '{"Authorization":"Bearer abc"}',
      body: '{"name":"Ada","again":"Ada"}', params: '{"q":"x"}',
    });
  });

  it("leaves unknown keys as-is so the user notices", () => {
    expect(resolveEnv(form, { BASE: "https://x.dev" }).url).toBe("https://x.dev/users/<<ID>>");
  });

  it("an empty value resolves to empty, not to the template", () => {
    expect(resolveEnv(form, { ID: "" }).url).toBe("<<BASE>>/users/");
  });

  it("without an env returns the input untouched, and never mutates it", () => {
    expect(resolveEnv(form, undefined)).toBe(form);
    const copy = { ...form };
    resolveEnv(form, { BASE: "x" });
    expect(form).toEqual(copy);
  });

  it("is a single pass: values containing templates or $ patterns are inserted literally", () => {
    const out = resolveEnv({ ...form, url: "<<A>>|<<B>>" }, { A: "<<B>>", B: "$&$1" });
    expect(out.url).toBe("<<B>>|$&$1");
  });

  it("does not resolve keys from Object.prototype", () => {
    expect(resolveEnv({ ...form, url: "<<constructor>>/<<toString>>" }, { A: "1" }).url).toBe("<<constructor>>/<<toString>>");
  });

  it("only matches word-character names", () => {
    expect(resolveEnv({ ...form, url: "<<a-b>> << A >>" }, { "a-b": "x", A: "y" }).url).toBe("<<a-b>> << A >>");
  });
});

describe("tree traversal", () => {
  const items = shop(base());
  it("finds routes and folders by path", () => {
    expect(findRoute(items, [], "Health")?.name).toBe("Health");
    expect(findRoute(items, [], "Users")).toBeUndefined();
    expect(findRoute(items, ["Nope"], "Ban")).toBeUndefined();
    expect(findFolder(items, ["Users", "Admin"])?.name).toBe("Admin");
    expect(findFolder(items, [])).toBeUndefined();
    expect(findFolder(items, ["Health"])).toBeUndefined();
    expect(getItemsAtPath(items, ["Nope"])).toEqual([]);
  });
  it("findRoutePath returns the folder path of the first matching route", () => {
    expect(findRoutePath(items, "Ban")).toEqual(["Users", "Admin"]);
    expect(findRoutePath(items, "Health")).toEqual([]);
    expect(findRoutePath(items, "Missing")).toBeUndefined();
  });
  it("lists every route name and counts nested routes and folders", () => {
    expect(getAllRouteNames(items)).toEqual(["Health", "List", "Ban"]);
    expect(collectAllRouteNames(base()[0])).toEqual(["Health", "List", "Ban"]);
    expect(countItemsInFolder(items)).toEqual({ routes: 3, folders: 3 });
  });
  it("tree edits are immutable", () => {
    const before = JSON.stringify(items);
    insertItem(items, ["Users"], r("X"));
    removeItem(items, ["Users", "Admin"], "Ban");
    renameItem(items, [], "Health", "H");
    updateRoute(items, [], "Health", "c", "r");
    expect(JSON.stringify(items)).toBe(before);
    expect(nameExistsInPath(items, ["Users"], "Admin")).toBe(true);
  });
});

describe("migrateCollections: upgrading old saved data", () => {
  it("returns [] for anything that is not an array", () => {
    for (const v of [null, undefined, {}, "x", 1]) expect(migrateCollections(v)).toEqual([]);
  });

  it("leaves current data untouched", () => {
    const cols = base();
    expect(migrateCollections(cols)).toBe(cols);
  });

  it("turns flat 'Folder/Sub/Name' routes into real folders, sharing folders between routes", () => {
    const [col] = migrateCollections([{
      collectionName: "Old", env: { A: "1" },
      curls: [
        { name: "Health", curl: "c0", response: "" },
        { name: "Users/Admin/List", curl: "c1", response: "r1" },
        { name: "Users/Admin/Create", curl: "c2", response: "" },
        { name: "Users / Get", curl: "c3", response: "" },
        { name: "/root", curl: "c4", response: "" },
      ],
    }]);
    expect(col).toMatchObject({ collectionName: "Old", env: { A: "1" }, secrets: {} });
    expect(col.items.map((i) => [i.type, i.name])).toEqual([["route", "Health"], ["folder", "Users"], ["route", "/root"]]);
    const users = getItemsAtPath(col.items, ["Users"]);
    expect(users.map((i) => [i.type, i.name])).toEqual([["folder", "Admin"], ["route", "Get"]]);
    expect(getItemsAtPath(col.items, ["Users", "Admin"]).map((i) => i.name)).toEqual(["List", "Create"]);
    expect(findRoute(col.items, ["Users", "Admin"], "List")).toMatchObject({ curl: "c1", response: "r1" });
  });

  it("upgrades legacy entries in a mixed list and keeps the new ones", () => {
    const out = migrateCollections([{ collectionName: "Old", curls: [] }, base()[1]]);
    expect(out).toEqual([{ collectionName: "Old", items: [], env: {}, secrets: {} }, base()[1]]);
  });
});
