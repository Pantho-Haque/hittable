"use client";

import { METHOD_COLORS, HITTABLE_METHODS } from "@/constants";
import useKeypress from "@/hooks/useKeypress";
import { curlConverter } from "@/utils/curlConverter";
import CopyAsMenu from "./CopyAsMenu";
import { copyAs } from "@/utils/codegen";
import { COPY_CURL_EVENT, openTool } from "@/utils/uiEvents";
import { isProduction, needsConfirmation, targetHost } from "@/utils/productionGuard";
import { ModalActions, ModalShell } from "@/components/ui/SharedModal";
import { hittableProxy, isAbortError } from "@/utils/hittableProxy";
import { getParamsfromUrl } from "@/utils/responsePanelUtils";
import { Loader2, Save, Send, X, TriangleAlert } from "lucide-react";
import { useCallback, useState, useRef, useEffect, useMemo } from "react";
import { useNotification } from "@/hooks/useNotify";
import type { DataSource } from "@/types/workspace";

export default function UrlBar({ error, dataSource }: { error: string | null; dataSource: DataSource }) {
  const {
    formInput,
    setFormInput,
    extensionAvailable,
    setProxyResponse,
    env,
    isUnsaved,
    handleSaveCollection,
    readOnly,
  } = dataSource;
  const { error: notifyError, success: notifySuccess } = useNotification();

  const mc = METHOD_COLORS[formInput.method] ?? "#94a3b8";

  const [proxyLoading, setProxyLoading] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const sentAtRef = useRef(0);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const highlightRef = useRef<HTMLDivElement>(null);
  const envRef = useRef(env);

  useEffect(() => { envRef.current = env; }, [env]);

  const autoResize = () => {
    const el = textareaRef.current;
    const hl = highlightRef.current;
    if (el) {
      el.style.height = "auto";
      el.style.height = `${el.scrollHeight}px`;
      if (hl) hl.style.height = `${el.scrollHeight}px`;
    }
  };

  // Sync highlight scroll with textarea
  const syncScroll = () => {
    const el = textareaRef.current;
    const hl = highlightRef.current;
    if (el && hl) {
      hl.scrollTop = el.scrollTop;
      hl.scrollLeft = el.scrollLeft;
    }
  };


  // Variables the request uses that env does not define. Shown as a
  // note under the bar so a typo is caught before the request goes out.
  const unknownVars = useMemo(
    () => findUnknownVars([formInput.url, formInput.headers, formInput.body, formInput.params], env),
    [formInput.url, formInput.headers, formInput.body, formInput.params, env],
  );

  // Render URL with <<KEY>> variables (teal when defined, red when not) and
  // :param path segments (amber) highlighted. Values are never shown here:
  // a secret must not leak into a screenshot.
  function renderHighlightedUrl(url: string): React.ReactNode {
    if (!url) return null; // the textarea's own placeholder shows
    const parts = url.split(/(<<\w+>>|:\w+)/g);
    return parts.map((part, i) => {
      const v = /^<<(\w+)>>$/.exec(part);
      if (v) {
        const known = isDefined(v[1], env);
        return (
          <span
            key={i}
            data-var={v[1]}
            data-resolved={known}
            className={known ? "text-(--brand) font-semibold" : "text-(--danger) font-semibold underline decoration-wavy underline-offset-2"}
          >
            {part}
          </span>
        );
      }
      if (/^:\w+$/.test(part)) {
        return (
          <span key={i} className="text-(--warning) font-semibold">
            {part}
          </span>
        );
      }
      return (
        <span key={i} className="text-(--text)">
          {part}
        </span>
      );
    });
  }

  const sendProxyRequest = useCallback(async () => {
    if (abortRef.current) return; // one request at a time; the button is Cancel now
    const controller = new AbortController();
    abortRef.current = controller;
    sentAtRef.current = performance.now();
    setProxyLoading(true);
    const startTime = performance.now();
    try {
      const res = await hittableProxy(formInput, envRef.current, undefined, extensionAvailable, controller.signal);
      const durationMs = Math.round(performance.now() - startTime);
      // The proxy reports the bytes it received; the extension path does not,
      // so fall back to measuring the body here.
      const sizeBytes = typeof res.sizeBytes === "number" ? res.sizeBytes : bodyBytes(res.data);
      setProxyResponse({ ...res, durationMs, sizeBytes });
    } catch (err) {
      if (isAbortError(err)) {
        setProxyResponse({ error: "Request cancelled" });
        return;
      }
      setProxyResponse({ error: String(err) });
    } finally {
      abortRef.current = null;
      setProxyLoading(false);
    }
  }, [setProxyResponse, formInput, extensionAvailable]);

  // FR-16: against a production environment, anything that changes state is
  // confirmed first. Every way of sending goes through here.
  const [confirmProd, setConfirmProd] = useState(false);
  const production = isProduction(env);
  const requestSend = useCallback(() => {
    if (abortRef.current) return;
    if (needsConfirmation(formInput.method, envRef.current)) {
      setConfirmProd(true);
      return;
    }
    sendProxyRequest();
  }, [formInput.method, sendProxyRequest]);

  const cancelRequest = useCallback(() => abortRef.current?.abort(), []);
  // Send turns into Cancel under the pointer, so the second click of a
  // double-click would cancel what the first one sent. Clicks that soon after
  // sending are ignored; Esc still cancels at once.
  const cancelByClick = useCallback(() => {
    if (performance.now() - sentAtRef.current < 400) return;
    cancelRequest();
  }, [cancelRequest]);

  // "Copy request as cURL" from the command palette.
  useEffect(() => {
    const onCopy = () => {
      copyAs("curl", formInput, env)
        .then(() => notifySuccess({ title: "Copied as cURL", desc: "Variables are filled in." }))
        .catch(() => notifyError({ title: "Could not copy", desc: "The clipboard is not available here." }));
    };
    window.addEventListener(COPY_CURL_EVENT, onCopy);
    return () => window.removeEventListener(COPY_CURL_EVENT, onCopy);
  }, [formInput, env, notifySuccess, notifyError]);

  // Leaving the request (or the page) must not leave a request running whose
  // answer would land in whatever is open next.
  useEffect(() => () => abortRef.current?.abort(), []);

  function handleUrlPaste(e: React.ClipboardEvent<HTMLTextAreaElement>) {
    const pasted = e.clipboardData.getData("text").trim();

    if (!pasted.startsWith("curl ")) return;

    e.preventDefault(); // stop it from being typed into the input
    // Postman {{var}} and Insomnia {{ _.var }} become Hittable <<var>>; any
    // not yet in env.json are then named as unknown under the bar.
    const parsed = curlConverter(pasted.replace(/\{\{\s*(?:_\.)?(\w+)\s*\}\}/g, "<<$1>>"));
    const before = formInput;
    setFormInput(parsed);
    notifySuccess({
      title: "Imported from curl",
      desc: `${parsed.method} ${parsed.url}`,
      timeout: 6000,
      action: { label: "Undo", onClick: () => setFormInput(before) },
    });
  }

  useKeypress({
    key: "Enter",
    isMeta: true,
    func: () => {
      if (error) {
        notifyError({ title: "Cannot send request", desc: "Fix the JSON error in your request body before sending." });
        return;
      }
      requestSend();
    },
  });

  useKeypress({
    key: "s",
    isMeta: true,
    func: handleSaveCollection,
  });

  // Auto-resize when URL changes externally (e.g. route selection)
  useEffect(() => {
    autoResize();
  }, [formInput.url]);

  return (
    <>
    <div
      className="flex flex-wrap items-start gap-2 md:flex-nowrap"
      onKeyDown={(e) => {
        if (e.key === "Escape" && abortRef.current) {
          e.preventDefault();
          cancelRequest();
        }
      }}
    >
      {/* Method and URL are one field: one border, one focus ring. */}
      <div className="url-field flex min-w-0 basis-full items-start rounded-md border border-(--border-strong) bg-(--surface-1) transition-[border-color,box-shadow] md:basis-auto md:flex-1">
        <select
          aria-label="HTTP method"
          className="h-9 w-24 shrink-0 cursor-pointer rounded-l-md border-r border-(--border) bg-transparent pl-3 pr-1 font-mono text-xs font-bold tracking-wide outline-none hover:bg-(--surface-2) disabled:cursor-default"
          style={{ color: mc }}
          value={formInput.method}
          disabled={readOnly}
          onChange={(e) => setFormInput({ ...formInput, method: e.target.value })}
        >
          {HITTABLE_METHODS.map((m) => (
            <option key={m} value={m} className="bg-(--surface-1)" style={{ color: METHOD_COLORS[m] }}>
              {m}
            </option>
          ))}
        </select>

        {production && (
          <span
            title="This environment is marked production: changes are confirmed before sending"
            className="ml-2 mt-2 shrink-0 rounded border border-(--danger)/50 bg-(--danger)/15 px-1.5 py-0.5 text-[11px] font-bold tracking-wide text-(--danger)"
          >
            PRODUCTION
          </span>
        )}

        {/* URL input with variable and path parameter highlighting */}
        <div className="relative min-w-0 flex-1">
          <div
            ref={highlightRef}
            className="pointer-events-none absolute inset-0 overflow-hidden whitespace-pre-wrap break-all px-3 py-2 font-mono text-[13px] leading-5"
            aria-hidden="true"
          >
            {renderHighlightedUrl(formInput.url)}
          </div>
          <textarea
            ref={textareaRef}
            aria-label="Request URL"
            className="relative z-10 block max-h-40 w-full resize-none overflow-hidden bg-transparent px-3 py-2 font-mono text-[13px] leading-5 text-transparent outline-none placeholder:text-(--muted)/70"
            style={{ caretColor: "var(--text)" }}
            placeholder="https://api.example.com/endpoint"
            value={formInput.url}
            onChange={(e) => {
              setFormInput({
                ...formInput,
                url: e.target.value,
                params: getParamsfromUrl(e.target.value),
              });
              autoResize();
            }}
            onKeyDown={(e) => {
              // Enter sends, as in other API clients; a URL has no line breaks.
              // Ctrl/Cmd+Enter is the page-wide shortcut, handled above.
              if (e.key !== "Enter" || e.metaKey || e.ctrlKey || e.nativeEvent.isComposing) return;
              e.preventDefault();
              if (!e.shiftKey && !error) requestSend();
            }}
            onPaste={handleUrlPaste}
            readOnly={readOnly}
            onScroll={syncScroll}
            spellCheck={false}
            rows={1}
          />
        </div>
      </div>

      <div className="ml-auto flex shrink-0 items-center gap-1.5">
        <button
          type="button"
          title={proxyLoading ? "Cancel (Esc)" : "Send (Ctrl/Cmd+Enter)"}
          aria-label={proxyLoading ? "Cancel" : "Send"}
          disabled={!proxyLoading && !!error}
          onClick={proxyLoading ? cancelByClick : requestSend}
          className={`flex h-9 min-w-22 items-center justify-center gap-1.5 rounded-md px-4 text-xs font-semibold transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
            proxyLoading
              ? "border border-(--border-strong) bg-(--surface-2) text-(--text) hover:border-(--danger)/60 hover:text-(--danger)"
              : "bg-(--brand) text-(--accent-ink) hover:brightness-110 active:brightness-95"
          }`}
        >
          {proxyLoading ? (
            <span className="relative flex h-3.5 w-3.5 items-center justify-center">
              <Loader2 className="absolute h-3.5 w-3.5 animate-spin" aria-hidden="true" />
              <X className="h-2 w-2" aria-hidden="true" />
            </span>
          ) : (
            <Send className="h-3.5 w-3.5" aria-hidden="true" />
          )}
          <span>{proxyLoading ? "Cancel" : "Send"}</span>
        </button>

        <button
          type="button"
          title={isUnsaved() ? "Save (Ctrl/Cmd+S)" : "Saved"}
          disabled={!isUnsaved()}
          onClick={handleSaveCollection}
          className="relative flex h-9 items-center gap-1.5 rounded-md border border-(--border-strong) px-3 text-xs font-medium text-(--text) transition-colors hover:bg-(--surface-2) disabled:cursor-default disabled:text-(--muted) disabled:opacity-60 disabled:hover:bg-transparent"
        >
          <Save className="h-3.5 w-3.5" aria-hidden="true" />
          <span className="max-sm:sr-only">Save</span>
          {isUnsaved() && <span className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full bg-(--warning)" aria-hidden="true" />}
        </button>

        <CopyAsMenu
          formInput={formInput}
          env={env}
          onCopied={(label) => notifySuccess({ title: `Copied as ${label}`, desc: "Variables are filled in." })}
        />
      </div>
    </div>
    {confirmProd && (
      <ModalShell
        title={`Send ${formInput.method} to production?`}
        subtitle={`This goes to ${targetHost(formInput.url, env)}, which this environment marks as production.`}
        onClose={() => setConfirmProd(false)}
      >
        <ModalActions
          onCancel={() => setConfirmProd(false)}
          onConfirm={() => {
            setConfirmProd(false);
            sendProxyRequest();
          }}
          confirmLabel={`Send ${formInput.method}`}
          confirmDanger
        />
      </ModalShell>
    )}
    {unknownVars.length > 0 && (
      <p role="note" className="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs text-(--muted)">
        <TriangleAlert className="h-3.5 w-3.5 text-(--warning)" aria-hidden="true" />
        <span>
          Unknown variable{unknownVars.length > 1 ? "s" : ""}{" "}
          {unknownVars.map((k, i) => (
            <span key={k}>
              {i > 0 && ", "}
              <code className="rounded bg-(--surface-2) px-1 font-mono text-(--danger)">{`<<${k}>>`}</code>
            </span>
          ))}{" "}
          — {unknownVars.length > 1 ? "they will be" : "it will be"} sent as written.
        </span>
        <button
          type="button"
          onClick={() => openTool("env")}
          className="rounded px-1.5 py-0.5 text-xs font-semibold text-(--brand) underline-offset-2 hover:underline cursor-pointer"
        >
          Open Environment
        </button>
      </p>
    )}
    </>
  );
}

function isDefined(key: string, env?: Record<string, string>) {
  return Object.prototype.hasOwnProperty.call(env ?? {}, key);
}

/** Distinct <<KEY>> names in the given texts that env does not define. */
export function findUnknownVars(texts: (string | undefined)[], env?: Record<string, string>): string[] {
  const seen = new Set<string>();
  for (const t of texts) {
    for (const m of (t ?? "").matchAll(/<<(\w+)>>/g)) {
      if (!isDefined(m[1], env)) seen.add(m[1]);
    }
  }
  return [...seen];
}

function bodyBytes(data: unknown): number {
  if (data === null || data === undefined) return 0;
  return new TextEncoder().encode(typeof data === "string" ? data : JSON.stringify(data)).byteLength;
}
