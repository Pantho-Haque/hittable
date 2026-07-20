import {
  THittableCollections,
  THittableCurlJson,
  THittableEnv,
  THittableItem,
  THittableCollection,
} from "@/types";
import {
  insertItem,
  removeItem,
  renameItem,
  updateRoute,
  nameExistsInPath,
} from "@/utils/treeHelpers";

export const createCollectionName = (
  prev: THittableCollections,
  currentName: string,
): THittableCollections => {
  return [...prev, { collectionName: currentName, items: [], env: {}, secrets: {} }];
};

export const createCurlName = (
  prev: THittableCollections,
  selectedCollection: string,
  currentName: string,
  curlString: string,
  folderPath: string[] = [],
): THittableCollections => {
  const newItem: THittableItem = {
    type: "route",
    name: currentName,
    curl: curlString,
    response: "",
  };
  return prev.map((collection) => {
    if (collection.collectionName === selectedCollection) {
      return {
        ...collection,
        items: insertItem(collection.items, folderPath, newItem),
      };
    }
    return collection;
  });
};

export const createFolderName = (
  prev: THittableCollections,
  selectedCollection: string,
  folderName: string,
  folderPath: string[] = [],
): THittableCollections => {
  const newFolder: THittableItem = {
    type: "folder",
    name: folderName,
    items: [],
  };
  return prev.map((collection) => {
    if (collection.collectionName === selectedCollection) {
      return {
        ...collection,
        items: insertItem(collection.items, folderPath, newFolder),
      };
    }
    return collection;
  });
};

export const renameCollectionName = (
  prev: THittableCollections,
  currentName: string,
  newName: string,
): THittableCollections => {
  return prev.map((collection) => {
    if (collection.collectionName === currentName) {
      return { ...collection, collectionName: newName };
    }
    return collection;
  });
};

export const renameCurlName = (
  prev: THittableCollections,
  currentName: string,
  collectionName: string,
  newName: string,
  folderPath: string[] = [],
): THittableCollections => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return {
        ...collection,
        items: renameItem(collection.items, folderPath, currentName, newName),
      };
    }
    return collection;
  });
};

export const renameFolderName = (
  prev: THittableCollections,
  currentName: string,
  collectionName: string,
  newName: string,
  folderPath: string[] = [],
): THittableCollections => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return {
        ...collection,
        items: renameItem(collection.items, folderPath, currentName, newName),
      };
    }
    return collection;
  });
};

export const isAlreadyExists = (
  collectionCurlList: { [key: string]: string[] },
  type: string,
  newName: string,
  collectionName?: string,
): boolean => {
  if (type === "collection") {
    return Object.keys(collectionCurlList).includes(newName);
  } else if (collectionCurlList && !!collectionName) {
    return collectionCurlList[collectionName]?.includes(newName) ?? false;
  }
  return false;
};

export const isAlreadyExistsInPath = (
  collection: THittableCollection | undefined,
  folderPath: string[],
  name: string,
): boolean => {
  if (!collection) return false;
  return nameExistsInPath(collection.items, folderPath, name);
};

export const updateCurl = (
  prev: THittableCollections,
  collectionName: string,
  curlName: string,
  curl: string,
  response: string,
  folderPath: string[] = [],
): THittableCollections => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return {
        ...collection,
        items: updateRoute(collection.items, folderPath, curlName, curl, response),
      };
    }
    return collection;
  });
};

export const deleteCollectionName = (
  prev: THittableCollections,
  currentName: string,
): THittableCollections => {
  return prev.filter((collection) => collection.collectionName !== currentName);
};

export const deleteItem = (
  prev: THittableCollections,
  collectionName: string,
  itemName: string,
  folderPath: string[] = [],
): THittableCollections => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return {
        ...collection,
        items: removeItem(collection.items, folderPath, itemName),
      };
    }
    return collection;
  });
};

export const deleteCurlName = deleteItem;
export const deleteFolderName = deleteItem;

export const updateEnv = (
  prev: THittableCollections,
  collectionName: string,
  env: Record<string, string>,
): THittableCollections => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return { ...collection, env };
    }
    return collection;
  });
};

export function resolveEnv(formInput: THittableCurlJson, env?: THittableEnv): THittableCurlJson {
  if (!env) return formInput;
  const newFormInput: THittableCurlJson = { ...formInput };
  (Object.entries(newFormInput) as [string, unknown][]).forEach(([key, val]) => {
    if (typeof val === 'string') {
      (newFormInput as Record<string, unknown>)[key] = val.replace(
        /<<(\w+)>>/g,
        (_, envKey: string) => env[envKey] ?? `<<${envKey}>>`
      );
    }
  });

  return newFormInput;
}

export const updateSecrets = (
  prev: THittableCollections,
  collectionName: string,
  secrets: Record<string, string>,
): THittableCollections => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return { ...collection, secrets };
    }
    return collection;
  });
};
