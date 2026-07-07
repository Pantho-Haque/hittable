import { THittableCollections, THittableCurlJson, THittableEnv } from "@/types";

export const createCollectionName = (
  prev: THittableCollections,
  currentName: string,
) => {
  return [...prev, { collectionName: currentName, curls: [], env: {} }];
};

export const createCurlName = (
  prev: THittableCollections,
  selectedCollection: string,
  currentName: string,
  curlString: string,
) => {
  return prev.map((collection) => {
    if (collection.collectionName === selectedCollection) {
      return {
        ...collection,
        curls: [...collection.curls, { name: currentName, curl: curlString, response: "" }],
      };
    }
    
    return collection;
  });
};

export const renameCollectionName = (
  prev: THittableCollections,
  currentName: string,
  newName: string,
) => {
  return prev.map((collection) => {
    if (collection.collectionName === currentName) {
      return {
        ...collection,
        collectionName: newName,
      };
    }
    return collection;
  });
};

export const renameCurlName = (
  prev: THittableCollections,
  currentName: string,
  collectionName: string,
  newName: string,
) => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return {
        ...collection,
        curls: collection.curls.map((curl) =>
          curl.name === currentName ? { ...curl, name: newName } : curl,
        ),
      };
    } else return collection;
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

export const updateCurl = (
  prev: THittableCollections,
  collectionName: string,
  curlName: string,
  curl: string,
  response: string,
) => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return {
        ...collection,
        curls: collection.curls.map((c) =>
          c.name === curlName ? { ...c, curl, response } : c,
        ),

      };
    }
    return collection;
  });
};

export const deleteCollectionName = (
  prev: THittableCollections,
  currentName: string,
) => {
  return prev.filter((collection) => collection.collectionName !== currentName);
};

export const deleteCurlName = (
  prev: THittableCollections,
  collectionName: string,
  currentName: string,
) => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return {
        ...collection,
        curls: collection.curls.filter((curl) => curl.name !== currentName),
      };
    }else return collection;
  });
};

export const updateEnv = (
  prev: THittableCollections,
  collectionName: string,
  env: Record<string, string>,
) => {
  return prev.map((collection) => {
    if (collection.collectionName === collectionName) {
      return {
        ...collection,
        env,
      };
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