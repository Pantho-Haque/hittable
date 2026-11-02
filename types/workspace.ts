import { THittableCurlJson, TResponseJson, THittableEnv } from "./hittable";

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
    url?: string;
  } | null;
};

export type TEnvFile = Record<string, string>;

/**
 * What the request Runner (URL bar, tabs, response panel) works on: one open
 * .hit file, from a local folder or a GitHub repository.
 */
export type DataSource = {
  formInput: THittableCurlJson;
  setFormInput: React.Dispatch<React.SetStateAction<THittableCurlJson>>;
  proxyResponse: TResponseJson;
  setProxyResponse: React.Dispatch<React.SetStateAction<TResponseJson>>;
  /** Variables for <<KEY>>: env.json with env.local.json layered on top. */
  env: THittableEnv;
  /** Identifies the open request (its path), e.g. to compare responses per request. */
  requestKey: string;
  extensionAvailable: boolean;
  /** The extension can carry file uploads to localhost. */
  extensionBinary?: boolean;
  /** The open .hit file's path: upload paths are relative to its folder. */
  hitPath?: string[];
  /** Bytes of a workspace file, for uploads. */
  readFile?: (path: string[]) => Promise<Uint8Array>;
  /** Copy a file from the computer into the workspace (hittable/files/); returns its path. */
  addUploadFile?: (file: File) => Promise<string[]>;
  /** Every file in the workspace, for picking an upload. */
  workspaceFiles?: string[][];
  isUnsaved: () => boolean;
  handleSaveCollection: () => void;
  /** No editing: a repository you cannot push to, or an archived one. */
  readOnly?: boolean;
};
