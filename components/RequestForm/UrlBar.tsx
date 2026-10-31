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
import { addHistoryEntry } from "@/utils/historyModifier";
import { updateEnv } from "@/utils/hittableCollectionModifier";
import { ensureEnvVarsForText } from "@/utils/importers/variableSyntax";
import { Loader2, Save, Send, X, TriangleAlert } from "lucide-react";
import { useCallback, useState, useRef, useEffect, useMemo } from "react";
import { useDataContext } from "@/context/dataContext";
import { useNotification } from "@/hooks/useNotify";
import SaveFromHistoryModal from "@/components/modals/SaveFromHistoryModal";
import type { DataSource } from "@/types/workspace";

export default function UrlBar({ error, dataSource: propDataSource }: { error: string | null; dataSource?: DataSource }) {
  const contextData = useDataContext();
  const {
    formInput,
    setFormInput,
    extensionAvailable,
    setProxyResponse,
    selectorResponse,
    isUnsaved,
    handleSaveCollection,
    history,
    setHistory,
    collections,
    setCollections,
  } = propDataSource ?? contextData;
  const { error: notifyError, success: notifySuccess } = useNotification();

  const { env, secrets } = selectorResponse!;
  const mc = METHOD_COLORS[formInput.method] ?? "#94a3b8";

  const [proxyLoading, setProxyLoading] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const sentAtRef = useRef(0);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const highlightRef = useRef<HTMLDivElement>(null);
  const envRef = useRef(env);
  const secretsRef = useRef(secrets);

  useEffect(() => { envRef.current = env; }, [env]);
  useEffect(() => { secretsRef.current = secrets; }, [secrets]);

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


  // Variables the request uses that neither env nor secrets define. Shown as a
  // note under the bar so a typo is caught before the request goes out.
  const unknownVars = useMemo(
    () => findUnknownVars([formInput.url, formInput.headers, formInput.body, formInput.params], env, secrets),
    [formInput.url, formInput.headers, formInput.body, formInput.params, env, secrets],
  );

  // Render URL with <<KEY>> variables (teal when defined, red when not) and
  // :param path segments (amber) highlighted. Values are never shown here:
  // a secret must not leak into a screenshot.
  function renderHighlightedUrl(url: string): React.ReactNode {
    if (!url) return <span className="text-white/50">https://api.example.com/endpoint</span>;
    const parts = url.split(/(<<\w+>>|:\w+)/g);
    return parts.map((part, i) => {
      const v = /^<<(\w+)>>$/.exec(part);
      if (v) {
        const known = isDefined(v[1], env, secrets);
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
          <span key={i} className="text-amber-400 font-semibold">
            {part}
          </span>
        );
      }
      return (
        <span key={i} className="text-white/80">
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
      const res = await hittableProxy(formInput, envRef.current, secretsRef.current, extensionAvailable, controller.signal);
      const durationMs = Math.round(performance.now() - startTime);
      // The proxy reports the bytes it received; the extension path does not,
      // so fall back to measuring the body here.
      const sizeBytes = typeof res.sizeBytes === "number" ? res.sizeBytes : bodyBytes(res.data);
      setProxyResponse({ ...res, durationMs, sizeBytes });

      // Record in history via context
      setHistory(addHistoryEntry(history, {
        method: formInput.method,
        url: formInput.url,
        status: res.status,
        statusText: res.statusText,
        durationMs,
        sizeBytes,
        curlJson: formInput,
        responseJson: { ...res, durationMs, sizeBytes },
      }));
    } catch (err) {
      if (isAbortError(err)) {
        setProxyResponse({ error: "Request cancelled" });
        return;
      }
      setProxyResponse({ error: String(err) });

      // Record failed request in history
      setHistory(addHistoryEntry(history, {
        method: formInput.method,
        url: formInput.url,
        curlJson: formInput,
      }));
    } finally {
      abortRef.current = null;
      setProxyLoading(false);
    }
  }, [setProxyResponse, formInput, extensionAvailable, history, setHistory]);

  // FR-16: against a production environment, anything that changes state is
  // confirmed first. Every way of sending goes through here.
  const [confirmProd, setConfirmProd] = useState(false);
  const production = isProduction(env, secrets);
  const requestSend = useCallback(() => {
    if (abortRef.current) return;
    if (needsConfirmation(formInput.method, envRef.current, secretsRef.current)) {
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
      copyAs("curl", formInput, { ...(env ?? {}), ...(secrets ?? {}) })
        .then(() => notifySuccess({ title: "Copied as cURL", desc: "Variables are filled in." }))
        .catch(() => notifyError({ title: "Could not copy", desc: "The clipboard is not available here." }));
    };
    window.addEventListener(COPY_CURL_EVENT, onCopy);
    return () => window.removeEventListener(COPY_CURL_EVENT, onCopy);
  }, [formInput, env, secrets, notifySuccess, notifyError]);

  // Leaving the request (or the page) must not leave a request running whose
  // answer would land in whatever is open next.
  useEffect(() => () => abortRef.current?.abort(), []);

  function handleUrlPaste(e: React.ClipboardEvent<HTMLTextAreaElement>) {
    const pasted = e.clipboardData.getData("text").trim();

    if (!pasted.startsWith("curl ")) return;

    e.preventDefault(); // stop it from being typed into the input
    // Postman {{var}} and Insomnia {{ _.var }} become Hittable <<var>>, so the
    // variables created below are the ones resolved at send time.
    const parsed = curlConverter(pasted.replace(/\{\{\s*(?:_\.)?(\w+)\s*\}\}/g, "<<$1>>"));
    const before = formInput;
    setFormInput(parsed);
    notifySuccess({
      title: "Imported from curl",
      desc: `${parsed.method} ${parsed.url}`,
      timeout: 6000,
      action: { label: "Undo", onClick: () => setFormInput(before) },
    });

    // Create empty env entries for the referenced variables, keeping existing values
    if (selectorResponse?.collectionName) {
      const col = collections.find((c) => c.collectionName === selectorResponse.collectionName);
      if (col) {
        const updatedEnv = ensureEnvVarsForText(pasted, col.env);
        if (Object.keys(updatedEnv).length !== Object.keys(col.env).length) {
          setCollections((prev) => updateEnv(prev, selectorResponse.collectionName, updatedEnv));
        }
      }
    }
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
      className="relative flex items-center gap-2 rounded-lg flex-wrap border bg-(--surface-1) px-2 py-1.5 transition-all"
      style={{ borderColor: `${mc}33`, boxShadow: `0 0 20px ${mc}0d` }}
      onKeyDown={(e) => {
        if (e.key === "Escape" && abortRef.current) {
          e.preventDefault();
          cancelRequest();
        }
      }}
    >
      {/* Corner brackets */}
      <span
        className="absolute top-0 left-0 w-3 h-3 border-t border-l rounded-tl-lg"
        style={{ borderColor: `${mc}44` }}
      />
      <span
        className="absolute top-0 right-0 w-3 h-3 border-t border-r rounded-tr-lg"
        style={{ borderColor: `${mc}44` }}
      />
      <span
        className="absolute bottom-0 left-0 w-3 h-3 border-b border-l rounded-bl-lg"
        style={{ borderColor: `${mc}44` }}
      />
      <span
        className="absolute bottom-0 right-0 w-3 h-3 border-b border-r rounded-br-lg"
        style={{ borderColor: `${mc}44` }}
      />

      <select
        aria-label="HTTP method"
        className="w-full md:w-auto shrink-0 rounded-md border-0 bg-(--surface-2) px-2 py-2 md:py-1.5 text-xs font-bold tracking-widest outline-none cursor-pointer min-h-[44px] md:min-h-0"
        style={{ color: mc }}
        value={formInput.method}
        onChange={(e) => setFormInput({ ...formInput, method: e.target.value })}
      >
        {HITTABLE_METHODS.map((m) => (
          <option key={m} value={m}>
            {m}
          </option>
        ))}
      </select>

      {production && (
        <span
          title="This environment is marked production: changes are confirmed before sending"
          className="shrink-0 rounded border border-(--danger)/50 bg-(--danger)/15 px-1.5 py-0.5 text-xs font-bold tracking-wide text-(--danger)"
        >
          PRODUCTION
        </span>
      )}

      <div className="h-4 w-px bg-white/10" />

      {/* URL input with path parameter highlighting */}
      <div className="relative w-full flex-1 min-h-[28px]">
        {/* Highlight layer behind textarea */}
        <div
          ref={highlightRef}
          className="absolute inset-0 pointer-events-none whitespace-pre-wrap break-all py-1 text-[11px] md:text-xs leading-normal overflow-hidden"
          aria-hidden="true"
        >
          {renderHighlightedUrl(formInput.url)}
        </div>
        {/* Transparent textarea on top */}
        <textarea
          ref={textareaRef}
          aria-label="Request URL"
          className="relative w-full bg-transparent py-1 text-[11px] md:text-xs text-transparent caret-white/80 outline-none resize-none overflow-hidden z-10"
          style={{ caretColor: "rgba(255,255,255,0.8)" }}
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
          onPaste={handleUrlPaste}
          onScroll={syncScroll}
          spellCheck={false}
          rows={1}
        />
      </div>

      <div className="w-full flex justify-end items-center gap-1.5">
        <SaveFromHistoryModal />
        <button
          title="Save (Ctrl/Cmd+S)"
          disabled={!isUnsaved()}
          onClick={handleSaveCollection}
          className="flex items-center gap-1.5 rounded-md border border-white/10 bg-white/5 px-3 py-2 md:py-1.5 text-[11px] md:text-xs font-semibold text-white/60 transition-all cursor-pointer hover:border-cyan-500/30 hover:text-cyan-400 disabled:cursor-not-allowed disabled:opacity-20 min-h-[44px] md:min-h-0"
        >
          <Save className="h-3 w-3 md:h-3 md:w-3" />
          <span className="hidden sm:inline">Save</span>
        </button>

        <button
          title={proxyLoading ? "Cancel (Esc)" : "Send (Ctrl/Cmd+Enter)"}
          aria-label={proxyLoading ? "Cancel" : "Send"}
          disabled={!proxyLoading && !!error}
          onClick={proxyLoading ? cancelByClick : requestSend}
          className="flex items-center gap-1.5 rounded-md px-3 py-2 md:py-1.5 text-[11px] md:text-xs font-bold text-black transition-all cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 active:scale-95 min-h-[44px] md:min-h-0"
          style={{ background: mc, boxShadow: `0 0 12px ${mc}44` }}
        >
          {proxyLoading ? (
            <span className="relative flex h-3 w-3 items-center justify-center">
              <Loader2 className="absolute h-3 w-3 animate-spin" aria-hidden="true" />
              <X className="h-2 w-2" aria-hidden="true" />
            </span>
          ) : (
            <Send className="h-3 w-3" aria-hidden="true" />
          )}
          <span className="hidden sm:inline">{proxyLoading ? "Cancel" : "Send"}</span>
        </button>

        <CopyAsMenu
          formInput={formInput}
          env={env}
          secrets={secrets}
          onCopied={(label) => notifySuccess({ title: `Copied as ${label}`, desc: "Variables are filled in." })}
        />
      </div>
    </div>
    {confirmProd && (
      <ModalShell
        title={`Send ${formInput.method} to production?`}
        subtitle={`This goes to ${targetHost(formInput.url, env, secrets)}, which this environment marks as production.`}
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
      <p role="note" className="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs text-white/70">
        <TriangleAlert className="h-3.5 w-3.5 text-(--warning)" aria-hidden="true" />
        <span>
          Unknown variable{unknownVars.length > 1 ? "s" : ""}{" "}
          {unknownVars.map((k, i) => (
            <span key={k}>
              {i > 0 && ", "}
              <code className="rounded bg-white/5 px-1 font-mono text-(--danger)">{`<<${k}>>`}</code>
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

function isDefined(key: string, env?: Record<string, string>, secrets?: Record<string, string>) {
  return Object.prototype.hasOwnProperty.call(env ?? {}, key) || Object.prototype.hasOwnProperty.call(secrets ?? {}, key);
}

/** Distinct <<KEY>> names in the given texts that env and secrets do not define. */
export function findUnknownVars(texts: (string | undefined)[], env?: Record<string, string>, secrets?: Record<string, string>): string[] {
  const seen = new Set<string>();
  for (const t of texts) {
    for (const m of (t ?? "").matchAll(/<<(\w+)>>/g)) {
      if (!isDefined(m[1], env, secrets)) seen.add(m[1]);
    }
  }
  return [...seen];
}

function bodyBytes(data: unknown): number {
  if (data === null || data === undefined) return 0;
  return new TextEncoder().encode(typeof data === "string" ? data : JSON.stringify(data)).byteLength;
}
