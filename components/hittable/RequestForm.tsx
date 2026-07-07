"use client";
import { formatJson } from "@/utils/formatJson";
import { AlertCircle, MessageCircleWarning, ChevronDown, RotateCcw } from "lucide-react";
import { useEffect, useState, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  NoExtensionModal,
  ResponsePanel,
  TabEditor,
  UrlBar,
} from "@/components";
import { useDataContext } from "@/context/dataContext";

export default function RequestForm() {
  const { selectorResponse } = useDataContext();

  if (!selectorResponse) {
    return <EmptyState />;
  }
  return <InputForm />;
}

function InputForm() {
  const {
    selectorResponse,
    collections,
    extensionAvailable,
    extensionChecked,
    setFormInput,
    setProxyResponse,
    isUnsaved,
    handleRevert,
  } = useDataContext();

  const router = useRouter();
  const { collectionName, curlName, curlJson, responseJson } =
    selectorResponse!;

  const [error, setError] = useState<string | null>(null);
  const [openDropdown, setOpenDropdown] = useState<"collection" | "route" | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const lastSyncedRouteRef = useRef<string | null>(null);
  const curlJsonRef = useRef(curlJson);

  useEffect(() => {
    curlJsonRef.current = curlJson;
  });

  useEffect(() => {
    const routeKey = `${collectionName}::${curlName}`;
    // Only sync form inputs when the route ACTUALLY changes.
    // Avoids overwriting in-progress user edits when collections/responseJson update.
    if (lastSyncedRouteRef.current === routeKey) return;
    lastSyncedRouteRef.current = routeKey;

    const currentCurlJson = curlJsonRef.current;

    setFormInput({
      ...currentCurlJson,
      body: formatJson(currentCurlJson.body).output,
      headers: formatJson(currentCurlJson.headers).output,
    });
    // eslint-disable-next-line react-hooks/set-state-in-effect -- intentional: sync form state on route change
    setError(null);
     
    setProxyResponse(responseJson ?? null);
  }, [collectionName, curlName, responseJson, setFormInput, setProxyResponse]);

  // Close dropdown on outside click
  useEffect(() => {
    if (!openDropdown) return;
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpenDropdown(null);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [openDropdown]);

  const handleCollectionSelect = useCallback((name: string) => {
    const col = collections.find((c) => c.collectionName === name);
    const firstRoute = col?.curls[0]?.name ?? "";
    const url = `/hittable?c=${encodeURIComponent(name)}${firstRoute ? `&r=${encodeURIComponent(firstRoute)}` : ""}`;
    router.push(url);
    setOpenDropdown(null);
  }, [collections, router]);

  const handleRouteSelect = useCallback((name: string) => {
    router.push(`/hittable?c=${encodeURIComponent(collectionName)}&r=${encodeURIComponent(name)}`);
    setOpenDropdown(null);
  }, [collectionName, router]);

  const currentRoutes = collections.find((c) => c.collectionName === collectionName)?.curls.map((c) => c.name) ?? [];
  const collectionNames = collections.map((c) => c.collectionName);

  return (
    <div className="h-full w-full flex flex-col gap-4 font-mono">
      <NoExtensionModal
        checked={extensionChecked}
        available={extensionAvailable}
      />

      <div className="flex flex-col md:flex-row justify-between items-center">
        <div className="flex justify-start items-center gap-2 h-3 w-full relative" ref={dropdownRef}>
          {/* Collection breadcrumb */}
          <div className="relative">
            <button
              onClick={() => setOpenDropdown(openDropdown === "collection" ? null : "collection")}
              className="flex items-center gap-1 text-[8px] md:text-[9px] uppercase text-cyan-500/40 hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {collectionName || "No collection"}
              <ChevronDown size={8} className={`transition-transform ${openDropdown === "collection" ? "rotate-180" : ""}`} />
            </button>
            {openDropdown === "collection" && collectionNames.length > 0 && (
              <div className="absolute top-full left-0 mt-1 z-50 bg-[#0e1f35] border border-white/10 rounded-lg shadow-xl shadow-black/50 py-1 min-w-[180px] max-h-[200px] overflow-y-auto">
                {collectionNames.map((name) => (
                  <button
                    key={name}
                    onClick={() => handleCollectionSelect(name)}
                    className={`w-full text-left px-3 py-1.5 text-[10px] transition-colors cursor-pointer ${
                      name === collectionName
                        ? "text-cyan-400 bg-cyan-500/10"
                        : "text-white/50 hover:text-white/80 hover:bg-white/5"
                    }`}
                  >
                    {name}
                  </button>
                ))}
              </div>
            )}
          </div>

          <span className="text-white/10 text-xs">/</span>

          {/* Route breadcrumb */}
          <div className="relative">
            <button
              onClick={() => setOpenDropdown(openDropdown === "route" ? null : "route")}
              className="flex items-center gap-1 text-[8px] md:text-[9px] uppercase text-cyan-500/70 hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {curlName || "No route"}
              <ChevronDown size={8} className={`transition-transform ${openDropdown === "route" ? "rotate-180" : ""}`} />
            </button>
            {openDropdown === "route" && currentRoutes.length > 0 && (
              <div className="absolute top-full left-0 mt-1 z-50 bg-[#0e1f35] border border-white/10 rounded-lg shadow-xl shadow-black/50 py-1 min-w-[180px] max-h-[200px] overflow-y-auto">
                {currentRoutes.map((name) => (
                  <button
                    key={name}
                    onClick={() => handleRouteSelect(name)}
                    className={`w-full text-left px-3 py-1.5 text-[10px] transition-colors cursor-pointer ${
                      name === curlName
                        ? "text-cyan-400 bg-cyan-500/10"
                        : "text-white/50 hover:text-white/80 hover:bg-white/5"
                    }`}
                  >
                    {name}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
        {(isUnsaved() || error) && (
          <div className="flex items-center w-full justify-start md:justify-end gap-2">
            {isUnsaved() && !error && (
              <>
                <button
                  onClick={handleRevert}
                  title="Discard changes"
                  className="flex items-center gap-1.5 rounded-md border border-white/10 bg-white/5 px-2 py-1 text-[8px] md:text-[10px] font-semibold text-white/40 transition-all cursor-pointer hover:border-amber-500/30 hover:text-amber-400 min-h-[28px]"
                >
                  <RotateCcw className="h-3 w-3" />
                </button>
                <span className="flex items-center gap-1.5 rounded-md bg-amber-500/10 px-2.5 py-1 text-[8px] md:text-[10px] font-semibold text-amber-400 border border-amber-500/20">
                  <MessageCircleWarning className="h-3 w-3" strokeWidth={2.5} />
                  Unsaved · Ctrl/Cmd+S
                </span>
              </>
            )}
            {error && (
              <span className="flex items-center gap-1.5 rounded-md bg-red-500/10 px-2.5 py-1 text-[8px] md:text-[10px] font-semibold text-red-400 border border-red-500/20">
                <AlertCircle className="h-3 w-3" strokeWidth={2.5} />
                {error}
              </span>
            )}
          </div>
        )}
      </div>

      <UrlBar error={error} />

      <TabEditor setError={setError} />
      <ResponsePanel />
    </div>
  );
}

function EmptyState() {
  const { hasCollections } = useDataContext();
  return (
    <div className="h-full w-full flex items-center justify-center">
      <div className="relative flex flex-col items-center gap-6 text-center px-8">
        {/* Corner brackets */}
        <span className="absolute -top-6 -left-6 w-6 h-6 border-t-2 border-l-2 border-cyan-500/40" />
        <span className="absolute -top-6 -right-6 w-6 h-6 border-t-2 border-r-2 border-cyan-500/40" />
        <span className="absolute -bottom-6 -left-6 w-6 h-6 border-b-2 border-l-2 border-cyan-500/40" />
        <span className="absolute -bottom-6 -right-6 w-6 h-6 border-b-2 border-r-2 border-cyan-500/40" />

        <div className="w-16 h-16 rounded-full border border-cyan-500/20 bg-cyan-500/5 flex items-center justify-center">
          <svg
            width="28"
            height="28"
            viewBox="0 0 24 24"
            fill="none"
            stroke="#00e5cc"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            opacity="0.6"
          >
            <path d="M20 7H4a2 2 0 00-2 2v6a2 2 0 002 2h16a2 2 0 002-2V9a2 2 0 00-2-2z" />
            <path d="M12 12h.01" />
            <path d="M8 12h.01" />
            <path d="M16 12h.01" />
          </svg>
        </div>

        <div>
          <p className="text-[10px] tracking-[0.3em] uppercase text-cyan-500/60 mb-2">
            Hittable
          </p>
          <h2 className="text-xl font-bold text-white/80 mb-2">
            {hasCollections ? "Select a route" : "No collections yet"}
          </h2>
          <p className="text-xs text-white/30 max-w-[260px] leading-relaxed">
            {hasCollections
              ? "Choose a collection and route from the sidebar to start making requests."
              : "Create a collection in the sidebar, then add routes to start testing your APIs."}
          </p>
        </div>
      </div>
    </div>
  );
}
