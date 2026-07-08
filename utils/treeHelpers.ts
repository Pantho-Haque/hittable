import {
  THittableCollections,
  THittableCollectionLegacy,
  THittableCollection,
  THittableItem,
  THittableRoute,
  THittableFolder,
} from "@/types";

function isLegacyCollection(col: unknown): col is THittableCollectionLegacy {
  if (!col || typeof col !== "object") return false;
  const c = col as Record<string, unknown>;
  return "collectionName" in c && "curls" in c && Array.isArray(c.curls);
}

function buildFolderTree(
  routes: { name: string; curl: string; response: string }[],
): THittableItem[] {
  const topLevel: THittableItem[] = [];
  const folderMap = new Map<string, THittableFolder>();

  for (const route of routes) {
    const slashIdx = route.name.indexOf("/");
    if (slashIdx <= 0) {
      topLevel.push({
        type: "route",
        name: route.name,
        curl: route.curl,
        response: route.response,
      });
      continue;
    }

    const folderName = route.name.substring(0, slashIdx).trim();
    const remaining = route.name.substring(slashIdx + 1).trim();

    let folder = folderMap.get(folderName);
    if (!folder) {
      folder = { type: "folder", name: folderName, items: [] };
      folderMap.set(folderName, folder);
      topLevel.push(folder);
    }

    // Recursively build nested structure
    const subRoute = { name: remaining, curl: route.curl, response: route.response };
    const subItems = buildFolderTree([subRoute]);
    folder.items.push(...subItems);
  }

  return topLevel;
}

export function migrateCollections(data: unknown): THittableCollections {
  if (!Array.isArray(data)) return [];

  const needsMigration = data.some(isLegacyCollection);
  if (!needsMigration) return data as THittableCollections;

  return data.map((col) => {
    if (!isLegacyCollection(col)) return col as THittableCollection;

    return {
      collectionName: col.collectionName,
      items: buildFolderTree(col.curls),
      env: col.env ?? {},
      secrets: {},
    };
  });
}

// ─── Tree Traversal Helpers ───

export function findRoute(
  items: THittableItem[],
  path: string[],
  routeName: string,
): THittableRoute | undefined {
  if (path.length === 0) {
    return items.find(
      (i) => i.type === "route" && i.name === routeName,
    ) as THittableRoute | undefined;
  }

  const [head, ...rest] = path;
  const folder = items.find(
    (i) => i.type === "folder" && i.name === head,
  ) as THittableFolder | undefined;
  if (!folder) return undefined;
  return findRoute(folder.items, rest, routeName);
}

export function findFolder(
  items: THittableItem[],
  path: string[],
): THittableFolder | undefined {
  if (path.length === 0) {
    return undefined; // Caller should handle this
  }

  const [head, ...rest] = path;
  const folder = items.find(
    (i) => i.type === "folder" && i.name === head,
  ) as THittableFolder | undefined;
  if (!folder) return undefined;
  if (rest.length === 0) return folder;
  if (folder.type !== "folder") return undefined;
  return findFolder(folder.items, rest);
}

export function getItemsAtPath(
  items: THittableItem[],
  path: string[],
): THittableItem[] {
  if (path.length === 0) return items;
  const [head, ...rest] = path;
  const folder = items.find(
    (i) => i.type === "folder" && i.name === head,
  ) as THittableFolder | undefined;
  if (!folder) return [];
  return getItemsAtPath(folder.items, rest);
}

export function findRoutePath(
  items: THittableItem[],
  routeName: string,
  currentPath: string[] = [],
): string[] | undefined {
  for (const item of items) {
    if (item.type === "route" && item.name === routeName) {
      return currentPath;
    }
    if (item.type === "folder") {
      const found = findRoutePath(item.items, routeName, [
        ...currentPath,
        item.name,
      ]);
      if (found) return found;
    }
  }
  return undefined;
}

export function getAllRouteNames(
  items: THittableItem[],
  prefix: string[] = [],
): string[] {
  const result: string[] = [];
  for (const item of items) {
    if (item.type === "route") {
      result.push(item.name);
    } else if (item.type === "folder") {
      result.push(...getAllRouteNames(item.items, [...prefix, item.name]));
    }
  }
  return result;
}

export function collectAllRouteNames(
  collection: THittableCollection,
): string[] {
  return getAllRouteNames(collection.items);
}

// ─── Tree Modification Helpers ───

export function insertItem(
  items: THittableItem[],
  path: string[],
  newItem: THittableItem,
): THittableItem[] {
  if (path.length === 0) {
    return [...items, newItem];
  }

  const [head, ...rest] = path;
  return items.map((item) => {
    if (item.type === "folder" && item.name === head) {
      return {
        ...item,
        items: insertItem(item.items, rest, newItem),
      };
    }
    return item;
  });
}

export function removeItem(
  items: THittableItem[],
  path: string[],
  itemName: string,
): THittableItem[] {
  if (path.length === 0) {
    return items.filter((i) => i.name !== itemName);
  }

  const [head, ...rest] = path;
  return items.map((item) => {
    if (item.type === "folder" && item.name === head) {
      return {
        ...item,
        items: removeItem(item.items, rest, itemName),
      };
    }
    return item;
  });
}

export function renameItem(
  items: THittableItem[],
  path: string[],
  oldName: string,
  newName: string,
): THittableItem[] {
  if (path.length === 0) {
    return items.map((i) => (i.name === oldName ? { ...i, name: newName } : i));
  }

  const [head, ...rest] = path;
  return items.map((item) => {
    if (item.type === "folder" && item.name === head) {
      return {
        ...item,
        items: renameItem(item.items, rest, oldName, newName),
      };
    }
    return item;
  });
}

export function updateRoute(
  items: THittableItem[],
  path: string[],
  routeName: string,
  curl: string,
  response: string,
): THittableItem[] {
  if (path.length === 0) {
    return items.map((i) =>
      i.type === "route" && i.name === routeName
        ? { ...i, curl, response }
        : i,
    );
  }

  const [head, ...rest] = path;
  return items.map((item) => {
    if (item.type === "folder" && item.name === head) {
      return {
        ...item,
        items: updateRoute(item.items, rest, routeName, curl, response),
      };
    }
    return item;
  });
}

export function nameExistsInPath(
  items: THittableItem[],
  path: string[],
  name: string,
): boolean {
  const targetItems = getItemsAtPath(items, path);
  return targetItems.some((i) => i.name === name);
}
