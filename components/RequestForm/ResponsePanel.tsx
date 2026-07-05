"use client";

import { JsonValue } from "@/types";
import { CheckCircle2, AlertCircle, Send, Search, Copy, Check, Code2, Braces } from "lucide-react";
import { useState, useCallback, useMemo, useRef } from "react";
import {
  MatchCtx,
  FloatingSearch,
  CopyButton,
  JsonNode,
  MatchRegistry,
} from "@/components";
import { countMatches } from "@/utils/responsePanelUtils";
import useKeypress from "@/hooks/useKeypress";
import { useDataContext } from "@/context/dataContext";


function HeadersTable({ headers }: { headers: Record<string, string> }) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const copyHeader = (key: string, value: string) => {
    navigator.clipboard.writeText(`${key}: ${value}`);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 1500);
  };

  const entries = Object.entries(headers);
  if (entries.length === 0) {
    return (
      <div className="flex items-center justify-center py-8 text-white/20 text-[10px]">
        No response headers
      </div>
    );
  }

  return (
    <div className="flex flex-col">
      {entries.map(([key, value]) => (
        <div
          key={key}
          className="flex items-start gap-3 px-3 py-1.5 border-b border-white/5 hover:bg-white/2 transition-colors group"
        >
          <span className="text-[9px] md:text-[11px] text-cyan-400/70 font-mono shrink-0 min-w-[140px]">
            {key}
          </span>
          <span className="text-[9px] md:text-[11px] text-white/60 font-mono break-all flex-1">
            {value}
          </span>
          <button
            onClick={() => copyHeader(key, value)}
            className="opacity-0 group-hover:opacity-100 transition-opacity p-0.5 shrink-0"
            title={`Copy ${key}`}
          >
            {copiedKey === key ? (
              <Check className="h-2.5 w-2.5 text-emerald-400" />
            ) : (
              <Copy className="h-2.5 w-2.5 text-white/30 hover:text-white/60" />
            )}
          </button>
        </div>
      ))}
    </div>
  );
}

export default function ResponsePanel() {
  const {proxyResponse} = useDataContext();
  const [searchQuery, setSearchQuery] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const [activeTab, setActiveTab] = useState<"body" | "headers">("body");
  const [rawView, setRawView] = useState(false);

  const matchEls = useRef<HTMLElement[]>([]);

  const register = useCallback((el: HTMLElement) => {
    matchEls.current.push(el);
    el.dataset.matchIndex = String(matchEls.current.length - 1);
    return () => {
      matchEls.current = matchEls.current.filter((e) => e !== el);
      matchEls.current.forEach((e, i) => (e.dataset.matchIndex = String(i)));
    };
  }, []);

  const scrollToMatch = useCallback((idx: number) => {
    setTimeout(() => {
      const el = matchEls.current[idx];
      if (!el) return;
      el.scrollIntoView({ block: "center", behavior: "smooth" });
      setActiveIndex(idx);
    }, 30);
  }, []);

  const goNext = useCallback(() => {
    const total = matchEls.current.length;
    if (!total) return;
    scrollToMatch((activeIndex + 1) % total);
  }, [activeIndex, scrollToMatch]);

  const goPrev = useCallback(() => {
    const total = matchEls.current.length;
    if (!total) return;
    scrollToMatch((activeIndex - 1 + total) % total);
  }, [activeIndex, scrollToMatch]);

  useKeypress({
    key: "f",
    isMeta: true,
    func: () => {
      setSearchOpen(o => !o);
    },
  });

  const closeSearch = useCallback(() => {
    setSearchOpen(false);
    setSearchQuery("");
  }, []);

  const matchCtxValue = useMemo<MatchRegistry>(
    () => ({ register, activeIndex }),
    [register, activeIndex],
  );

  const statusOk = proxyResponse?.status != null && proxyResponse.status < 300;
  const statusWarn =
    proxyResponse?.status != null &&
    proxyResponse.status >= 300 &&
    proxyResponse.status < 500;

  const parsedData = useMemo<JsonValue | null>(() => {
    if (!proxyResponse?.data) return null;
    try {
      return typeof proxyResponse.data === "string"
        ? (JSON.parse(proxyResponse.data) as JsonValue)
        : (proxyResponse.data as JsonValue);
    } catch {
      return null;
    }
  }, [proxyResponse?.data]);

  const responseHeaders = useMemo(() => {
    if (!proxyResponse?.headers) return {};
    if (typeof proxyResponse.headers === "object" && proxyResponse.headers !== null) {
      return proxyResponse.headers as Record<string, string>;
    }
    return {};
  }, [proxyResponse?.headers]);

  const headerCount = Object.keys(responseHeaders).length;

  const totalMatches = useMemo(
    () =>
      parsedData && searchQuery ? countMatches(parsedData, searchQuery) : 0,
    [parsedData, searchQuery],
  );

  const hasJson = parsedData !== null && !proxyResponse?.error;

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <MatchCtx.Provider value={matchCtxValue}>
      <div className="relative flex flex-col rounded-lg border border-white/8 bg-[#0a1628]/60 overflow-hidden h-full">
        {/* ── Header ── */}
        <div className="flex items-center gap-3 border-b border-white/5 bg-[#0e1f35]/50 px-2 md:px-4 py-1 md:py-2 shrink-0">
          <span className="text-[7px] md:text-[9px] tracking-[0.25em] uppercase text-white/25">
            Response
          </span>

          {proxyResponse?.status != null && (
            <span
              aria-live="polite"
              className="flex items-center gap-1 rounded-md px-2 py-0.5 text-[8px] md:text-[10px] font-bold border"
              style={{
                background: statusOk
                  ? "rgba(74,222,128,0.08)"
                  : statusWarn
                    ? "rgba(251,146,60,0.08)"
                    : "rgba(248,113,113,0.08)",
                borderColor: statusOk
                  ? "rgba(74,222,128,0.2)"
                  : statusWarn
                    ? "rgba(251,146,60,0.2)"
                    : "rgba(248,113,113,0.2)",
                color: statusOk
                  ? "#4ade80"
                  : statusWarn
                    ? "#fb923c"
                    : "#f87171",
              }}
            >
              {statusOk ? (
                <CheckCircle2 className="h-2.5 w-2.5 md:h-3 md:w-3" />
              ) : (
                <AlertCircle className="h-2.5 w-2.5 md:h-3 md:w-3" />
              )}
              {proxyResponse.status} {proxyResponse.statusText}
            </span>
          )}

          {proxyResponse?.durationMs != null && (
            <span className="text-[8px] md:text-[10px] text-white/30 font-mono">
              {formatDuration(proxyResponse.durationMs)}
            </span>
          )}

          {proxyResponse?.sizeBytes != null && proxyResponse.sizeBytes > 0 && (
            <span className="text-[8px] md:text-[10px] text-white/30 font-mono">
              {formatSize(proxyResponse.sizeBytes)}
            </span>
          )}

          <div className="ml-auto flex items-center gap-1">
            {hasJson && (
              <button
                onClick={() => setSearchOpen((o) => !o)}
                title="Search (⌘F)"
                className={`p-0.5 md:p-1 rounded transition-colors ${
                  searchOpen
                    ? "text-yellow-300/90 bg-yellow-400/10 ring-1 ring-yellow-400/20"
                    : "text-white/25 hover:text-white/60 hover:bg-white/5"
                }`}
              >
                <Search className="h-2.5 w-2.5 md:h-3 md:w-3" />
              </button>
            )}
            {parsedData && <CopyButton data={parsedData} />}
            {hasJson && (
              <button
                onClick={() => setRawView((o) => !o)}
                title={rawView ? "Tree view" : "Raw view"}
                className={`p-0.5 md:p-1 rounded transition-colors ${
                  rawView
                    ? "text-cyan-300/90 bg-cyan-400/10 ring-1 ring-cyan-400/20"
                    : "text-white/25 hover:text-white/60 hover:bg-white/5"
                }`}
              >
                {rawView ? (
                  <Braces className="h-2.5 w-2.5 md:h-3 md:w-3" />
                ) : (
                  <Code2 className="h-2.5 w-2.5 md:h-3 md:w-3" />
                )}
              </button>
            )}
          </div>
        </div>

        {/* ── Tabs ── */}
        {proxyResponse && !proxyResponse.error && (
          <div className="flex items-center border-b border-white/5 bg-[#0e1f35]/30 shrink-0">
            {(["body", "headers"] as const).map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className="relative px-3 py-2 md:py-1.5 text-[9px] md:text-[10px] font-semibold tracking-[0.15em] uppercase transition-colors cursor-pointer min-h-[44px] md:min-h-0"
                style={{
                  color: activeTab === tab ? "#00e5cc" : "rgba(255,255,255,0.25)",
                }}
              >
                {tab}
                {tab === "headers" && headerCount > 0 && (
                  <span className="ml-1 text-[7px] text-white/20">({headerCount})</span>
                )}
                {activeTab === tab && (
                  <span className="absolute bottom-0 left-2 right-2 h-px bg-cyan-400" />
                )}
              </button>
            ))}
          </div>
        )}

        {/* ── Floating Search ── */}
        {searchOpen && hasJson && activeTab === "body" && (
          <FloatingSearch
            value={searchQuery}
            onChange={setSearchQuery}
            total={totalMatches}
            activeIndex={activeIndex}
            onNext={goNext}
            onPrev={goPrev}
            onClose={closeSearch}
          />
        )}

        {/* ── Body ── */}
        {proxyResponse ? (
          <div className="flex-1 overflow-auto p-2 md:p-3">
            {activeTab === "headers" ? (
              <HeadersTable headers={responseHeaders} />
            ) : proxyResponse.error ? (
              <span className="font-mono text-[8px] md:text-[11px] text-red-400 whitespace-pre-wrap">
                {proxyResponse.error}
              </span>
            ) : parsedData && rawView ? (
              <pre className="font-mono text-[9px] md:text-[11px] text-white/60 leading-relaxed whitespace-pre-wrap break-all">
                {typeof proxyResponse.data === "string"
                  ? proxyResponse.data
                  : JSON.stringify(proxyResponse.data, null, 2)}
              </pre>
            ) : parsedData ? (
              <JsonNode
                value={parsedData}
                depth={0}
                searchQuery={searchQuery}
                defaultOpen={true}
              />
            ) : (
              <pre className="text-[8px] md:text-[11px] text-white/60 leading-relaxed whitespace-pre-wrap wrap-words">
                {String(proxyResponse.data || '')}
              </pre>
            )}
          </div>
        ) : (
          <div className="flex flex-1 items-center justify-center py-8 gap-2 text-white/15">
            <Send size={14} />
            <span className="text-[8px] md:text-[10px] tracking-[0.2em] uppercase">
              Send a request to see the response
            </span>
          </div>
        )}
      </div>
    </MatchCtx.Provider>
  );
}
