"use client";

import { JsonValue } from "@/types";
import { CheckCircle2, AlertCircle, Send, Search } from "lucide-react";
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


export default function ResponsePanel() {
  const {proxyResponse} = useDataContext();
  const [searchQuery, setSearchQuery] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);

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
    console.log(matchEls.current.length)
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

  const totalMatches = useMemo(
    () =>
      parsedData && searchQuery ? countMatches(parsedData, searchQuery) : 0,
    [parsedData, searchQuery],
  );

  const hasJson = parsedData !== null && !proxyResponse?.error;

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
          </div>
        </div>

        {/* ── Floating Search ── */}
        {searchOpen && hasJson && (
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
            {proxyResponse.error ? (
              <span className="font-mono text-[8px] md:text-[11px] text-red-400 whitespace-pre-wrap">
                {proxyResponse.error}
              </span>
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
