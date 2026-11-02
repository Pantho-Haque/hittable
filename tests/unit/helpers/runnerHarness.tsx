import { useEffect, useState } from "react";
import { render } from "@testing-library/react";
import { NotificationProvider } from "@/context/notifyContext";
import UrlBar from "@/components/RequestForm/UrlBar";
import TabEditor from "@/components/RequestForm/TabEditor";
import ResponsePanel from "@/components/RequestForm/ResponsePanel";
import { curlConverter } from "@/utils/curlConverter";
import type { DataSource, THittableCurlJson, THittableEnv, TResponseJson } from "@/types";

// Mounts the request Runner (URL bar, tabs, response panel) the way the
// workspace does, on a small in-memory DataSource. Replaces the old Local-mode
// DataProvider harness.

export const SAMPLE_ENV: THittableEnv = { HOST: "api.example.com", TOKEN: "secret-token" };

export const SAMPLE_REQUESTS = {
  "List users": `curl -X GET https://<<HOST>>/users -H "Authorization: Bearer <<TOKEN>>"`,
  "Create user": `curl -X POST https://<<HOST>>/admin/users -d '{"name":"Ada"}'`,
} as const;

export type RunnerOptions = {
  /** A curl command, a request object, or the name of a SAMPLE_REQUESTS entry. Default "List users". */
  request?: string | THittableCurlJson;
  env?: THittableEnv;
  response?: TResponseJson;
  extensionAvailable?: boolean;
  requestKey?: string;
  readOnly?: boolean;
};

export type RunnerCtx = {
  formInput: THittableCurlJson;
  proxyResponse: TResponseJson;
  /** How many times Save (Ctrl/Cmd+S or the button) ran. */
  saves: number;
};

function toRequest(r: RunnerOptions["request"]): THittableCurlJson {
  if (r === undefined) return curlConverter(SAMPLE_REQUESTS["List users"]);
  if (typeof r !== "string") return r;
  return curlConverter(r in SAMPLE_REQUESTS ? SAMPLE_REQUESTS[r as keyof typeof SAMPLE_REQUESTS] : r);
}

export function mountRunner(opts: RunnerOptions = {}) {
  const ctxRef: { current: RunnerCtx | null } = { current: null };
  const control: {
    respond?: (r: TResponseJson) => void;
    setKey?: (k: string) => void;
  } = {};

  function Runner() {
    const [formInput, setFormInput] = useState<THittableCurlJson>(() => toRequest(opts.request));
    const [proxyResponse, setProxyResponse] = useState<TResponseJson>(opts.response ?? null);
    const [requestKey, setRequestKey] = useState(opts.requestKey ?? "hittable/users/list.hit");
    const [saved, setSaved] = useState<THittableCurlJson>(formInput);
    const [saves, setSaves] = useState(0);
    const [error, setError] = useState<string | null>(null);
    useEffect(() => {
      control.respond = setProxyResponse;
      control.setKey = setRequestKey;
    }, []);
    useEffect(() => {
      ctxRef.current = { formInput, proxyResponse, saves };
    });
    const dataSource: DataSource = {
      formInput,
      setFormInput,
      proxyResponse,
      setProxyResponse,
      env: opts.env ?? SAMPLE_ENV,
      requestKey,
      extensionAvailable: opts.extensionAvailable ?? false,
      isUnsaved: () => JSON.stringify(saved) !== JSON.stringify(formInput),
      handleSaveCollection: () => {
        setSaved(formInput);
        setSaves((n) => n + 1);
      },
      readOnly: opts.readOnly,
    };
    return (
      <>
        <UrlBar dataSource={dataSource} error={error} />
        <TabEditor dataSource={dataSource} setError={setError} />
        <ResponsePanel dataSource={dataSource} />
      </>
    );
  }

  const utils = render(
    <NotificationProvider>
      <Runner />
    </NotificationProvider>,
  );
  return {
    ...utils,
    ctx: () => ctxRef.current!,
    respond: (r: TResponseJson) => control.respond!(r),
    setRequestKey: (k: string) => control.setKey!(k),
  };
}
