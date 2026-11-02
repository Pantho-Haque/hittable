
export type THittableEnv = {
  [key: string]: string;
};

// Legacy type for migration detection
export type THittableCurlJson = {
  method: string;
  url: string;
  headers: string;
  body: string;
  params: string;
};

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
  /** The URL the response came from, after redirects. */
  url?: string;
} | null;

export type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [k: string]: JsonValue };
