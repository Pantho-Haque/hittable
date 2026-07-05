export type TabSize = 2 | 4 | "tab";

export type THittableEnv = {
  [key: string]: string;
};

export type THittableCurl = {
  name: string;
  curl: string;
  response: string;
};

export type THittableCollection = {
  collectionName: string;
  curls: THittableCurl[];
  env: THittableEnv;
};

export type THittableCollections = THittableCollection[];

export type THittableCurlJson = {
  method: string;
  url: string;
  headers: string;
  body: string;
  params: string;
};

export type THittableSelectorSelection = {
    collectionName: string;
    curlName: string;
}

export type TResponseJson = {
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

export type THittableSelectorResponse = {
    collectionName: string;
    curlName: string;
    env?: THittableEnv;
    curlJson: THittableCurlJson;
    responseJson?: TResponseJson;
};

export type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [k: string]: JsonValue };

export type THistoryEntry = {
  id: string;
  timestamp: number;
  method: string;
  url: string;
  status?: number;
  statusText?: string;
  durationMs?: number;
  sizeBytes?: number;
  curlJson: THittableCurlJson;
  responseJson?: TResponseJson;
};

export type THistory = THistoryEntry[];
