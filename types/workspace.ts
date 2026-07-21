import { THittableCurlJson, TResponseJson, THittableEnv, THittableCollections, THistory } from "./hittable";

export type TWorkspaceMode = "local" | "directory";

export type TDirectoryNode = {
  name: string;
  kind: "file" | "directory";
  handle: FileSystemFileHandle | FileSystemDirectoryHandle;
  children?: TDirectoryNode[];
};

export type THitFileContent = {
  method: string;
  url: string;
  headers: Record<string, string>;
  params: Record<string, string>;
  body: string;
  response: {
    data?: unknown;
    status?: number;
    statusText?: string;
    ok?: boolean;
    headers?: unknown;
    error?: string;
    cookies?: unknown;
    durationMs?: number;
    sizeBytes?: number;
  } | null;
};

export type TEnvFile = Record<string, string>;

export type TWorkspaceState = {
  mode: TWorkspaceMode;
  directoryHandle: FileSystemDirectoryHandle | null;
  tree: TDirectoryNode[];
  activeFile: {
    path: string[];
    handle: FileSystemFileHandle;
  } | null;
  isRefreshing: boolean;
};

export type DataSource = {
  formInput: THittableCurlJson;
  setFormInput: React.Dispatch<React.SetStateAction<THittableCurlJson>>;
  proxyResponse: TResponseJson;
  setProxyResponse: React.Dispatch<React.SetStateAction<TResponseJson>>;
  selectorResponse: {
    collectionName: string;
    folderPath: string[];
    curlName: string;
    env?: THittableEnv;
    secrets?: THittableEnv;
    curlJson: THittableCurlJson;
    responseJson?: TResponseJson;
  } | null;
  setSelectorResponse: React.Dispatch<React.SetStateAction<{
    collectionName: string;
    folderPath: string[];
    curlName: string;
    env?: THittableEnv;
    secrets?: THittableEnv;
    curlJson: THittableCurlJson;
    responseJson?: TResponseJson;
  } | null>>;
  extensionAvailable: boolean;
  extensionChecked: boolean;
  isUnsaved: () => boolean;
  handleSaveCollection: () => void;
  handleRevert: () => void;
  collections: THittableCollections;
  setCollections: React.Dispatch<React.SetStateAction<THittableCollections>>;
  history: THistory;
  setHistory: React.Dispatch<React.SetStateAction<THistory>>;
};
