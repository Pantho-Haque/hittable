"use client";

import type { JsonValue, TResponseJson } from "@/types";
import { reasonPhrase, statusKind, formatSize, type StatusKind } from "@/utils/httpStatus";
import { EXTENSION_URL } from "@/config";
import { CheckCircle2, AlertCircle, Send, Search, Copy, Check, Braces, FileText, Globe, Crosshair, ListFilter, GitCompareArrows, Zap, ZapOff, Columns2, Rows2, ExternalLink } from "lucide-react";
import { useState, useCallback, useMemo, useRef, useEffect } from "react";
import { MatchCtx } from "./ResponsePanelComponents/MatchContext";
import FloatingSearch from "./ResponsePanelComponents/FloatingSearch";
import CopyButton from "./ResponsePanelComponents/CopyButton";
import JsonNode from "./ResponsePanelComponents/JsonNode";
import { Highlight } from "./ResponsePanelComponents/Highlight";
import { countMatches } from "@/utils/responsePanelUtils";
import useKeypress from "@/hooks/useKeypress";
import HtmlPreview from "./ResponsePanelComponents/HtmlPreview";
import type { ElementInfo } from "./ResponsePanelComponents/HtmlPreview";
import HtmlSourceViewer from "./ResponsePanelComponents/HtmlSourceViewer";
import ElementInspector from "./ResponsePanelComponents/ElementInspector";
import { findElementSourceLine, formatHtml } from "@/utils/htmlFormatter";
import type { DataSource } from "@/types/workspace";
import JsonPathFilter from "./ResponsePanelComponents/JsonPathFilter";
import ResponseCompare from "./ResponsePanelComponents/ResponseCompare";
import { queryJsonPath } from "@/utils/jsonPath";
import Split from "@/components/ui/Split";
import useMediaQuery from "@/hooks/useMediaQuery";
import { useStoredFlag } from "@/utils/prefs";

type ResponseViewMode = "json" | "text" | "html";

function HeadersTable({ headers, searchQuery }: { headers: Record<string, string>; searchQuery?: string }) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const copyHeader = (key: string, value: string) => {
    navigator.clipboard.writeText(`${key}: ${value}`);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 1500);
  };

  const entries = Object.entries(headers);
  if (entries.length === 0) {
    return (
      <div className="flex items-center justify-center py-8 text-white/50 text-[11px]">
        No response headers
      </div>
    );
  }

  const filtered = searchQuery
    ? entries.filter(([k, v]) =>
        k.toLowerCase().includes(searchQuery.toLowerCase()) ||
        v.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : entries;

  return (
    <div className="flex flex-col">
      {filtered.map(([key, value]) => (
        <div
          key={key}
          className="flex items-start gap-3 px-3 py-1.5 border-b border-white/5 hover:bg-white/2 transition-colors group"
        >
          <span className="text-[11px] md:text-[11px] text-cyan-400/70 font-mono shrink-0 min-w-[140px]">
            {key}
          </span>
          <span className="text-[11px] md:text-[11px] text-white/60 font-mono break-all flex-1">
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
              <Copy className="h-2.5 w-2.5 text-white/50 hover:text-white/60" />
            )}
          </button>
        </div>
      ))}
      {searchQuery && filtered.length === 0 && (
        <div className="flex items-center justify-center py-6 text-white/50 text-[11px]">
          No headers matching &quot;{searchQuery}&quot;
        </div>
      )}
    </div>
  );
}

function detectViewMode(headers: Record<string, string>): ResponseViewMode {
  const contentType = (headers["content-type"] ?? headers["Content-Type"] ?? "").toLowerCase();
  if (contentType.includes("application/json") || contentType.includes("text/json")) {
    return "json";
  }
  if (contentType.includes("text/html") || contentType.includes("application/xhtml")) {
    return "html";
  }
  return "text";
}

export default function ResponsePanel({ dataSource }: { dataSource: DataSource }) {
  const { proxyResponse, formInput, env, requestKey } = dataSource;
  const [searchQuery, setSearchQuery] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const [activeTab, setActiveTab] = useState<"body" | "headers">("body");
  const [viewMode, setViewMode] = useState<ResponseViewMode>("text");
  const [headersCopied, setHeadersCopied] = useState(false);
  const [userOverrodeMode, setUserOverrodeMode] = useState(false);

  const [inspectMode, setInspectMode] = useState(false);
  const [selectedElement, setSelectedElement] = useState<ElementInfo | null>(null);
  const [highlightedSourceLine, setHighlightedSourceLine] = useState<number | null>(null);
  const [previewScripts, setPreviewScripts] = useStoredFlag("hittable:preview-scripts", true);
  const [sourceBeside, setSourceBeside] = useStoredFlag("hittable:source-beside", false);
  const [prettySource, setPrettySource] = useStoredFlag("hittable:source-pretty", true);
  const narrow = useMediaQuery("(max-width: 767px)");

  const [filterOpen, setFilterOpen] = useState(false);
  const [filterText, setFilterText] = useState("");
  const [compareOpen, setCompareOpen] = useState(false);
  const [ignoreText, setIgnoreText] = useState("");
  const compareButtonRef = useRef<HTMLButtonElement>(null);

  // FR-14: the response this panel showed before the current one, for the
  // same request. Opening a .hit file shows its saved response and a send
  // replaces it, so the outgoing response is the "previous" one. The request's
  // identity is its file path (requestKey), not its URL, so editing the URL
  // and sending again still compares. The key is read when a response
  // arrives: switching files changes it a render before the new file's saved
  // response lands, and that response must not become the new file's
  // "previous".
  const [seen, setSeen] = useState({
    response: proxyResponse,
    key: requestKey,
    method: formInput.method,
    url: formInput.url,
    previous: null as TResponseJson,
  });
  if (seen.response !== proxyResponse) {
    // Errors and empty states are not worth comparing against: keep the last real response.
    const usable = seen.response && !seen.response.error ? seen.response : seen.previous;
    setSeen({
      response: proxyResponse,
      key: requestKey,
      method: formInput.method,
      url: formInput.url,
      previous: seen.key === requestKey ? usable : null,
    });
    setFilterText("");
    setCompareOpen(false);
  }

  const previous = seen.previous;

  const closeCompare = useCallback(() => {
    setCompareOpen(false);
    compareButtonRef.current?.focus();
  }, []);

  const matchEls = useRef<HTMLElement[]>([]);
  const prevResponseRef = useRef(proxyResponse);

  // Clear user override when a new response arrives
  useEffect(() => {
    if (proxyResponse !== prevResponseRef.current) {
      prevResponseRef.current = proxyResponse;
      // eslint-disable-next-line react-hooks/set-state-in-effect -- guard check prevents cascading renders
      if (userOverrodeMode) setUserOverrodeMode(false);
    }
  }, [proxyResponse, userOverrodeMode]);

  const responseHeaders = useMemo(() => {
    if (!proxyResponse?.headers) return {};
    if (typeof proxyResponse.headers === "object" && proxyResponse.headers !== null) {
      return proxyResponse.headers as Record<string, string>;
    }
    return {};
  }, [proxyResponse?.headers]);

  // Compute effective view mode: auto-detect when user hasn't overridden
  const effectiveViewMode: ResponseViewMode = useMemo(() => {
    if (userOverrodeMode) return viewMode;
    if (activeTab !== "body") return viewMode;
    return detectViewMode(responseHeaders);
  }, [userOverrodeMode, viewMode, activeTab, responseHeaders]);

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

  // Request URL for the HTML preview's base href, with <<KEY>> resolved the
  // way the request was sent. The URL in the bar, not the
  // last saved one: that is what was sent.
  const requestUrl = formInput.url?.replace(
    /<<(\w+)>>/g,
    (m, k: string) => (Object.hasOwn(env, k) ? env[k] : m),
  );

  // Relative links in an HTML page resolve against where it came from: the
  // final URL after redirects when the proxy reports it, else the request's.
  const baseUrl = proxyResponse?.url || requestUrl;

  const showCopyButton = activeTab === "body" && effectiveViewMode !== "html";

  const kind = proxyResponse?.status != null ? statusKind(proxyResponse.status) : null;

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

  // FR-13: one match shows as itself, several as an array of the matches.
  const filterResult = useMemo(
    () => (filterText.trim() && parsedData !== null ? queryJsonPath(parsedData, filterText) : null),
    [filterText, parsedData],
  );
  const filterActive = !!filterResult?.ok;
  const shownData: JsonValue | null = filterResult?.ok
    ? ((filterResult.matches.length === 1 ? filterResult.matches[0] : filterResult.matches) as JsonValue)
    : parsedData;
  const canFilter = parsedData !== null && !proxyResponse?.error;
  const clearFilter = useCallback(() => {
    setFilterText("");
    setFilterOpen(false);
  }, []);

  const rawText = useMemo(() => {
    if (!proxyResponse?.data) return "";
    return typeof proxyResponse.data === "string"
      ? proxyResponse.data
      : JSON.stringify(proxyResponse.data, null, 2);
  }, [proxyResponse?.data]);

  const shownText = filterActive ? JSON.stringify(shownData, null, 2) : rawText;

  // The source pane shows this, and an inspected element is looked up in it,
  // so the line marked is the line shown.
  // A very large page is shown as received: formatting it would stall the tab.
  const tooLargeToFormat = rawText.length > 2_000_000;
  const htmlSource = useMemo(() => {
    if (effectiveViewMode !== "html") return "";
    if (!prettySource || tooLargeToFormat) return rawText;
    try {
      return formatHtml(rawText, { indentSize: 2, maxLineLength: 120 });
    } catch {
      return rawText;
    }
  }, [effectiveViewMode, rawText, prettySource, tooLargeToFormat]);

  const headerCount = Object.keys(responseHeaders).length;

  const totalMatches = useMemo(() => {
    if (activeTab === "headers") {
      if (!searchQuery) return 0;
      const q = searchQuery.toLowerCase();
      return Object.entries(responseHeaders).filter(
        ([k, v]) => k.toLowerCase().includes(q) || v.toLowerCase().includes(q)
      ).length;
    }
    return shownData && searchQuery ? countMatches(shownData, searchQuery) : 0;
  }, [shownData, searchQuery, activeTab, responseHeaders]);

  const canSearch = (parsedData !== null && !proxyResponse?.error) || activeTab === "headers";

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };


  const handleCopyHeaders = () => {
    navigator.clipboard.writeText(JSON.stringify(responseHeaders, null, 2));
    setHeadersCopied(true);
    setTimeout(() => setHeadersCopied(false), 1500);
  };

  const handleViewModeChange = (mode: ResponseViewMode) => {
    setViewMode(mode);
    setUserOverrodeMode(true);
  };

  const matchCtxValue = useMemo(() => ({ register, activeIndex }), [register, activeIndex]);

  return (
    <MatchCtx.Provider value={matchCtxValue}>
      <section aria-label="Response" className="relative flex flex-col rounded-lg border border-white/8 bg-(--surface-1)/60 overflow-hidden h-full">
        {/* ── Header ── */}
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-white/5 bg-(--surface-2)/50 px-2 md:px-4 py-1 md:py-2 shrink-0">
          <span className="text-xs font-semibold text-(--text)">
            Response
          </span>

          {proxyResponse?.status != null && kind && (
            <span
              aria-live="polite"
              data-status-kind={kind}
              className={`flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-bold border ${STATUS_CLASS[kind]}`}
            >
              {kind === "success" ? (
                <CheckCircle2 className="h-3 w-3" aria-hidden="true" />
              ) : (
                <AlertCircle className="h-3 w-3" aria-hidden="true" />
              )}
              {proxyResponse.status} {reasonPhrase(proxyResponse.status, proxyResponse.statusText)}
            </span>
          )}

          {proxyResponse?.durationMs != null && (
            <span className="text-xs text-white/60 font-mono" title="Round trip, including the proxy hop">
              {formatDuration(proxyResponse.durationMs)}
            </span>
          )}

          {proxyResponse?.sizeBytes != null && proxyResponse.sizeBytes > 0 && (
            <span className="text-xs text-white/60 font-mono" title="Response body size as received">
              {formatSize(proxyResponse.sizeBytes)}
            </span>
          )}

          <div className="ml-auto flex items-center gap-1">
            {proxyResponse && !proxyResponse.error && (
              <button
                ref={compareButtonRef}
                type="button"
                onClick={() => previous && setCompareOpen((o) => !o)}
                aria-label="Compare with previous response"
                aria-pressed={compareOpen}
                aria-disabled={!previous}
                title={previous ? "Compare with previous response" : "No earlier response for this request yet"}
                className={`p-0.5 md:p-1 rounded transition-colors ${
                  !previous
                    ? "text-white/30 cursor-not-allowed"
                    : compareOpen
                      ? "text-(--brand) bg-white/5 ring-1 ring-white/15"
                      : "text-white/60 hover:text-white/80 hover:bg-white/5"
                }`}
              >
                <GitCompareArrows className="h-2.5 w-2.5 md:h-3 md:w-3" aria-hidden="true" />
              </button>
            )}
            {canFilter && activeTab === "body" && !compareOpen && (
              <button
                type="button"
                onClick={() => setFilterOpen((o) => !o)}
                aria-label={filterActive ? "Filter with JSONPath (active)" : "Filter with JSONPath"}
                aria-pressed={filterOpen}
                title="Filter with JSONPath"
                className={`relative p-0.5 md:p-1 rounded transition-colors ${
                  filterOpen
                    ? "text-(--brand) bg-white/5 ring-1 ring-white/15"
                    : "text-white/60 hover:text-white/80 hover:bg-white/5"
                }`}
              >
                <ListFilter className="h-2.5 w-2.5 md:h-3 md:w-3" aria-hidden="true" />
                {filterActive && (
                  <span data-testid="filter-dot" aria-hidden="true" className="absolute -top-0.5 -right-0.5 h-1.5 w-1.5 rounded-full bg-(--brand)" />
                )}
              </button>
            )}
            {canSearch && (
              <button
                onClick={() => setSearchOpen((o) => !o)}
                title="Search (⌘F)"
                className={`p-0.5 md:p-1 rounded transition-colors ${
                  searchOpen
                    ? "text-yellow-300/90 bg-yellow-400/10 ring-1 ring-yellow-400/20"
                    : "text-white/50 hover:text-white/60 hover:bg-white/5"
                }`}
              >
                <Search className="h-2.5 w-2.5 md:h-3 md:w-3" />
              </button>
            )}
            {showCopyButton && shownData && !compareOpen && <CopyButton data={shownData} />}
            {activeTab === "headers" && headerCount > 0 && (
              <button
                onClick={handleCopyHeaders}
                title="Copy all headers"
                className="p-0.5 md:p-1 rounded transition-colors text-white/50 hover:text-white/60 hover:bg-white/5"
              >
                {headersCopied ? (
                  <Check className="h-2.5 w-2.5 md:h-3 md:w-3 text-emerald-400" />
                ) : (
                  <Copy className="h-2.5 w-2.5 md:h-3 md:w-3" />
                )}
              </button>
            )}
            {activeTab === "body" && (
              <div className="flex items-center gap-0.5 ml-1 border-l border-white/8 pl-1">
                <button
                  onClick={() => handleViewModeChange("json")}
                  title="JSON tree view"
                  className={`p-0.5 md:p-1 rounded transition-colors ${
                    effectiveViewMode === "json"
                      ? "text-cyan-300/90 bg-cyan-400/10 ring-1 ring-cyan-400/20"
                      : "text-white/50 hover:text-white/60 hover:bg-white/5"
                  }`}
                >
                  <Braces className="h-2.5 w-2.5 md:h-3 md:w-3" />
                </button>
                <button
                  onClick={() => handleViewModeChange("text")}
                  title="Raw text view"
                  className={`p-0.5 md:p-1 rounded transition-colors ${
                    effectiveViewMode === "text"
                      ? "text-cyan-300/90 bg-cyan-400/10 ring-1 ring-cyan-400/20"
                      : "text-white/50 hover:text-white/60 hover:bg-white/5"
                  }`}
                >
                  <FileText className="h-2.5 w-2.5 md:h-3 md:w-3" />
                </button>
                <button
                  onClick={() => handleViewModeChange("html")}
                  title="HTML preview"
                  className={`p-0.5 md:p-1 rounded transition-colors ${
                    effectiveViewMode === "html"
                      ? "text-cyan-300/90 bg-cyan-400/10 ring-1 ring-cyan-400/20"
                      : "text-white/50 hover:text-white/60 hover:bg-white/5"
                  }`}
                >
                  <Globe className="h-2.5 w-2.5 md:h-3 md:w-3" />
                </button>
              </div>
            )}
          </div>
        </div>

        {/* ── Tabs ── */}
        {proxyResponse && !proxyResponse.error && !compareOpen && (
          <div role="tablist" aria-label="Response view" className="flex items-center border-b border-white/5 bg-(--surface-2)/30 shrink-0">
            {(["body", "headers"] as const).map((tab) => (
              <button
                key={tab}
                id={`res-tab-${tab}`}
                role="tab"
                type="button"
                aria-selected={activeTab === tab}
                tabIndex={activeTab === tab ? 0 : -1}
                onClick={() => setActiveTab(tab)}
                onKeyDown={(e) => {
                  if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
                    e.preventDefault();
                    const next = tab === "body" ? "headers" : "body";
                    setActiveTab(next);
                    document.getElementById(`res-tab-${next}`)?.focus();
                  }
                }}
                className={`relative px-3 py-2 md:py-1.5 text-xs font-semibold capitalize transition-colors cursor-pointer min-h-[44px] md:min-h-0 ${
                  activeTab === tab ? "text-(--brand)" : "text-white/60 hover:text-white/80"
                }`}
              >
                {tab}
                {tab === "headers" && headerCount > 0 && (
                  <span className="ml-1 text-xs text-white/60">({headerCount})</span>
                )}
                {activeTab === tab && (
                  <span className="absolute bottom-0 left-2 right-2 h-px bg-cyan-400" aria-hidden="true" />
                )}
              </button>
            ))}
          </div>
        )}

        {/* ── Floating Search ── */}
        {filterOpen && canFilter && activeTab === "body" && !compareOpen && (
          <JsonPathFilter value={filterText} onChange={setFilterText} onClear={clearFilter} result={filterResult} />
        )}

        {searchOpen && canSearch && !compareOpen && (
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
          <div className="flex-1 min-h-0 overflow-hidden flex flex-col">
            {compareOpen && previous && proxyResponse && !proxyResponse.error ? (
              <ResponseCompare
                previous={previous}
                current={proxyResponse}
                ignore={ignoreText}
                onIgnoreChange={setIgnoreText}
                onClose={closeCompare}
              />
            ) : activeTab === "headers" ? (
              <div className="flex-1 min-h-0 overflow-auto p-2 md:p-3">
                <HeadersTable headers={responseHeaders} searchQuery={searchQuery} />
              </div>
            ) : proxyResponse.error ? (
              <ResponseError error={proxyResponse.error} />
            ) : (effectiveViewMode === "json" || (filterActive && effectiveViewMode === "html")) && shownData !== null && parsedData ? (
              <div className="flex-1 min-h-0 overflow-auto p-2 md:p-3">
                <JsonNode
                  key={filterActive ? `filter:${filterText}` : "full"}
                  value={shownData}
                  depth={0}
                  searchQuery={searchQuery}
                  defaultOpen={true}
                />
              </div>
            ) : effectiveViewMode === "html" ? (
              <Split
                id="html-response"
                orientation={sourceBeside && !narrow ? "horizontal" : "vertical"}
                defaultSize={0.55}
                minFirst={80}
                minSecond={80}
                collapsible={["first", "second"]}
                firstLabel="Preview"
                secondLabel="Source"
                className="flex-1"
                first={
                <div className="flex min-h-0 flex-1 flex-col">
                  <div className="flex shrink-0 items-center gap-1 border-b border-(--border) bg-(--surface-2) px-2 py-1">
                    <Globe className="h-3 w-3 text-(--muted)" aria-hidden="true" />
                    <span className="text-xs font-semibold text-(--text)">Preview</span>
                    <div className="ml-auto flex items-center gap-0.5">
                      {!narrow && (
                        <button
                          type="button"
                          onClick={() => setSourceBeside(!sourceBeside)}
                          aria-label={sourceBeside ? "Show the source below the preview" : "Show the source beside the preview"}
                          title={sourceBeside ? "Source below" : "Source beside"}
                          className="rounded p-1 text-(--muted) transition-colors hover:bg-(--ink-600) hover:text-(--text)"
                        >
                          {sourceBeside ? <Rows2 className="h-3.5 w-3.5" aria-hidden="true" /> : <Columns2 className="h-3.5 w-3.5" aria-hidden="true" />}
                        </button>
                      )}
                      <button
                        type="button"
                        aria-pressed={previewScripts}
                        onClick={() => setPreviewScripts(!previewScripts)}
                        title={previewScripts ? "Page scripts run, isolated from Hittable. Click to block them." : "Page scripts are blocked. Click to run them, isolated from Hittable."}
                        className={`flex items-center gap-1 rounded px-1.5 py-1 text-[11px] transition-colors ${
                          previewScripts ? "text-(--text) hover:bg-(--ink-600)" : "text-(--muted) hover:bg-(--ink-600) hover:text-(--text)"
                        }`}
                      >
                        {previewScripts ? <Zap className="h-3 w-3" aria-hidden="true" /> : <ZapOff className="h-3 w-3" aria-hidden="true" />}
                        Scripts {previewScripts ? "on" : "off"}
                      </button>
                      {/^https?:\/\//i.test(baseUrl ?? "") && (
                        <a
                          href={baseUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          aria-label="Open the page in a browser tab"
                          title="Open in a browser tab (for pages that refuse to load inside a frame)"
                          className="rounded p-1 text-(--muted) transition-colors hover:bg-(--ink-600) hover:text-(--text)"
                        >
                          <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                        </a>
                      )}
                      <button
                        type="button"
                        onClick={() => setInspectMode(!inspectMode)}
                        aria-pressed={inspectMode}
                        aria-label={inspectMode ? "Exit inspect mode" : "Inspect elements"}
                        title={inspectMode ? "Exit inspect mode" : "Inspect elements: click one in the preview to find it in the source"}
                        className={`rounded p-1 transition-colors ${
                          inspectMode ? "bg-(--accent-soft) text-(--brand)" : "text-(--muted) hover:bg-(--ink-600) hover:text-(--text)"
                        }`}
                      >
                        <Crosshair className="h-3.5 w-3.5" aria-hidden="true" />
                      </button>
                    </div>
                  </div>
                  <div className="min-h-0 flex-1 bg-white">
                    <HtmlPreview
                      html={rawText}
                      baseUrl={baseUrl}
                      scripts={previewScripts}
                      inspectMode={inspectMode}
                      onElementSelect={(info) => {
                        setSelectedElement(info);
                        setHighlightedSourceLine(findElementSourceLine(info.tagName, info.attributes, htmlSource.split("\n")));
                      }}
                    />
                  </div>
                  {selectedElement && (
                    <ElementInspector
                      info={selectedElement}
                      onClose={() => {
                        setSelectedElement(null);
                        setHighlightedSourceLine(null);
                      }}
                    />
                  )}
                </div>
                }
                second={
                  <HtmlSourceViewer
                    source={htmlSource}
                    highlightedLine={highlightedSourceLine}
                    pretty={prettySource && !tooLargeToFormat}
                    onPrettyChange={tooLargeToFormat ? undefined : (p) => {
                      setPrettySource(p);
                      setHighlightedSourceLine(null);
                    }}
                  />
                }
              />
            ) : (
              <div className="flex-1 min-h-0 overflow-auto p-2 md:p-3">
                <pre className="font-mono text-[11px] md:text-[11px] text-white/60 leading-relaxed whitespace-pre-wrap break-all">
                  {searchQuery ? (
                    <Highlight text={shownText} query={searchQuery} />
                  ) : (
                    shownText
                  )}
                </pre>
              </div>
            )}
          </div>
        ) : (
          <div className="flex flex-1 items-center justify-center py-8 gap-2 text-white/50">
            <Send size={14} />
            <span className="text-xs text-(--muted)">
              Send a request to see the response
            </span>
          </div>
        )}
      </section>
    </MatchCtx.Provider>
  );
}

const STATUS_CLASS: Record<StatusKind, string> = {
  success: "text-(--success) border-(--success)/30 bg-(--success)/10",
  info: "text-(--info) border-(--info)/30 bg-(--info)/10",
  warning: "text-(--warning) border-(--warning)/30 bg-(--warning)/10",
  danger: "text-(--danger) border-(--danger)/30 bg-(--danger)/10",
};

/**
 * A failed send, said in a way the user can act on: a cancelled request is
 * not an error, and a localhost request without the extension says how to
 * get it instead of only that it failed.
 */
function ResponseError({ error }: { error: string }) {
  if (/Request cancelled/.test(error)) {
    return (
      <p role="status" className="flex-1 flex items-center justify-center p-6 text-sm text-white/70">
        Request cancelled.
      </p>
    );
  }
  if (/Hittable Extension/i.test(error)) {
    return (
      <div role="alert" className="flex-1 flex flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="text-sm font-semibold text-white/90">localhost needs the Hittable browser extension</p>
        <p className="max-w-md text-xs text-white/70">
          Browsers stop a web page from calling your own machine. The extension sends localhost requests for this tab;
          everything else goes through the proxy and works without it.
        </p>
        <a
          href={EXTENSION_URL}
          className="rounded-md bg-(--brand) px-3 py-1.5 text-xs font-bold text-black hover:opacity-90"
        >
          Install the extension
        </a>
      </div>
    );
  }
  return (
    <div role="alert" className="flex-1 min-h-0 overflow-auto p-2 md:p-3">
      <span className="font-mono text-xs text-red-400 whitespace-pre-wrap">{error}</span>
    </div>
  );
}
