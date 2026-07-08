"use client";

import { METHOD_COLORS, HITTABLE_METHODS } from "@/constants";
import useKeypress from "@/hooks/useKeypress";
import { curlConverter, jsonToCurl } from "@/utils/curlConverter";
import { hittableProxy } from "@/utils/hittableProxy";
import { getParamsfromUrl } from "@/utils/responsePanelUtils";
import { addHistoryEntry } from "@/utils/historyModifier";
import { updateEnv } from "@/utils/hittableCollectionModifier";
import { CheckCircle2, Terminal, Loader2, Save, Send } from "lucide-react";
import { useCallback, useState, useRef, useEffect } from "react";
import { useDataContext } from "@/context/dataContext";
import { useNotification } from "@/hooks/useNotify";

export default function UrlBar({ error }: { error: string | null }) {
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
  } = useDataContext();
  const { error: notifyError } = useNotification();

  const { env, secrets } = selectorResponse!;
  const mc = METHOD_COLORS[formInput.method] ?? "#94a3b8";

  const [curlCopied, setCurlCopied] = useState(false);
  const [proxyLoading, setProxyLoading] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const highlightRef = useRef<HTMLDivElement>(null);

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

  const handleCopyCurl = () => {
    navigator.clipboard.writeText(jsonToCurl(formInput));
    setCurlCopied(true);
    setTimeout(() => setCurlCopied(false), 2000);
  };

  // Render URL with :param path segments highlighted in amber
  function renderHighlightedUrl(url: string): React.ReactNode {
    if (!url) return <span className="text-white/20">https://api.example.com/endpoint</span>;
    const parts = url.split(/(:\w+)/g);
    return parts.map((part, i) => {
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
    setProxyLoading(true);
    setProxyResponse(null);
    const startTime = performance.now();
    try {
      const res = await hittableProxy(formInput, env, secrets, extensionAvailable);
      const durationMs = Math.round(performance.now() - startTime);
      const sizeBytes = res.data ? new TextEncoder().encode(JSON.stringify(res.data)).byteLength : 0;
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
      setProxyResponse({ error: String(err) });

      // Record failed request in history
      setHistory(addHistoryEntry(history, {
        method: formInput.method,
        url: formInput.url,
        curlJson: formInput,
      }));
    } finally {
      setProxyLoading(false);
    }
  }, [setProxyResponse, formInput, env, secrets, extensionAvailable, history, setHistory]);

  function handleUrlPaste(e: React.ClipboardEvent<HTMLTextAreaElement>) {
    const pasted = e.clipboardData.getData("text").trim();

    if (!pasted.startsWith("curl ")) return;

    setFormInput({ ...formInput, url: pasted });

    e.preventDefault(); // stop it from being typed into the input
    const parsed = curlConverter(pasted);
    setTimeout(() => {
      setFormInput(parsed);
    }, 1000);

    // Create env var entries for any Postman/Insomnia-style {{var}} references found in the curl
    const postmanVars = [...pasted.matchAll(/\{\{(\w+)\}\}/g)].map((m) => m[1]);
    const insomniaVars = [...pasted.matchAll(/\{\{\s*_\.(\w+)\s*\}\}/g)].map((m) => m[1]);
    const allVarNames = [...new Set([...postmanVars, ...insomniaVars])];
    if (allVarNames.length > 0 && selectorResponse?.collectionName) {
      const col = collections.find((c) => c.collectionName === selectorResponse.collectionName);
      if (col) {
        const updatedEnv = { ...col.env };
        for (const name of allVarNames) {
          if (!(name in updatedEnv)) {
            updatedEnv[name] = "";
          }
        }
        setCollections((prev) => updateEnv(prev, selectorResponse.collectionName, updatedEnv));
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
      sendProxyRequest();
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
    <div
      className="relative flex items-center gap-2 rounded-lg flex-wrap border bg-[#0a1628] px-2 py-1.5 transition-all"
      style={{ borderColor: `${mc}33`, boxShadow: `0 0 20px ${mc}0d` }}
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
        className="w-full md:w-auto shrink-0 rounded-md border-0 bg-[#0e1f35] px-2 py-2 md:py-1.5 text-xs font-bold tracking-widest outline-none cursor-pointer min-h-[44px] md:min-h-0"
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

      <div className="h-4 w-px bg-white/10" />

      {/* URL input with path parameter highlighting */}
      <div className="relative w-full flex-1 min-h-[28px]">
        {/* Highlight layer behind textarea */}
        <div
          ref={highlightRef}
          className="absolute inset-0 pointer-events-none whitespace-pre-wrap break-all py-1 text-[10px] md:text-xs leading-normal overflow-hidden"
          aria-hidden="true"
        >
          {renderHighlightedUrl(formInput.url)}
        </div>
        {/* Transparent textarea on top */}
        <textarea
          ref={textareaRef}
          className="relative w-full bg-transparent py-1 text-[10px] md:text-xs text-transparent caret-white/80 outline-none resize-none overflow-hidden z-10"
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
        <button
          title="Save (Ctrl/Cmd+S)"
          disabled={!isUnsaved()}
          onClick={handleSaveCollection}
          className="flex items-center gap-1.5 rounded-md border border-white/10 bg-white/5 px-3 py-2 md:py-1.5 text-[8px] md:text-xs font-semibold text-white/40 transition-all cursor-pointer hover:border-cyan-500/30 hover:text-cyan-400 disabled:cursor-not-allowed disabled:opacity-20 min-h-[44px] md:min-h-0"
        >
          <Save className="h-3 w-3 md:h-3 md:w-3" />
          <span className="hidden sm:inline">Save</span>
        </button>

        <button
          title="Send (Ctrl/Cmd+Enter)"
          disabled={proxyLoading || !!error}
          onClick={sendProxyRequest}
          className="flex items-center gap-1.5 rounded-md px-3 py-2 md:py-1.5 text-[8px] md:text-xs font-bold text-black transition-all cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 active:scale-95 min-h-[44px] md:min-h-0"
          style={{ background: mc, boxShadow: `0 0 12px ${mc}44` }}
        >
          {proxyLoading ? (
            <Loader2 className="h-3 w-3 md:h-3 md:w-3 animate-spin" />
          ) : (
            <Send className="h-3 w-3" />
          )}
          <span className="hidden sm:inline">{proxyLoading ? "Sending…" : "Send"}</span>
        </button>

        <button
          title="Copy as CURL"
          disabled={curlCopied}
          onClick={handleCopyCurl}
          className="flex items-center gap-1.5 rounded-md px-3 py-2 md:py-1.5 text-[8px] md:text-xs font-bold text-black transition-all cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 active:scale-95 min-h-[44px] md:min-h-0"
          style={{ background: mc, boxShadow: `0 0 12px ${mc}44` }}
        >
          {curlCopied ? (
            <CheckCircle2 className="h-3 w-3 md:h-3 md:w-3" />
          ) : (
            <Terminal className="h-3 w-3" />
          )}
        </button>
      </div>
    </div>
  );
}
