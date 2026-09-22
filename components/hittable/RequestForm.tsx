"use client";
import { formatJson } from "@/utils/formatJson";
import { MessageCircleWarning, ChevronDown, RotateCcw, Folder, Route } from "lucide-react";
import { useEffect, useState, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  NoExtensionModal,
  ResponsePanel,
  TabEditor,
  UrlBar,
} from "@/components";
import { useDataContext } from "@/context/dataContext";
import { THittableItem } from "@/types";
import { getItemsAtPath } from "@/utils/treeHelpers";
import { METHOD_COLORS } from "@/constants";

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
  const { collectionName, folderPath, curlName, curlJson, responseJson } =
    selectorResponse!;

  const [error, setError] = useState<string | null>(null);
  const [openDropdown, setOpenDropdown] = useState<
    "collection" | "route" | number | null
  >(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const lastSyncedRouteRef = useRef<string | null>(null);
  const curlJsonRef = useRef(curlJson);

  useEffect(() => {
    curlJsonRef.current = curlJson;
  });

  useEffect(() => {
    const routeKey = `${collectionName}::${folderPath.join("/")}::${curlName}`;
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
  }, [collectionName, folderPath, curlName, responseJson, setFormInput, setProxyResponse]);

  // Close dropdown on outside click
  useEffect(() => {
    if (!openDropdown) return;
    const handler = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node)
      ) {
        setOpenDropdown(null);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [openDropdown]);

  const handleCollectionSelect = useCallback(
    (name: string) => {
      const col = collections.find((c) => c.collectionName === name);
      const firstItem = col?.items?.[0];
      const firstRoute =
        firstItem?.type === "route" ? firstItem.name : "";
      const url = `/hittable?c=${encodeURIComponent(name)}${firstRoute ? `&r=${encodeURIComponent(firstRoute)}` : ""}`;
      router.push(url);
      setOpenDropdown(null);
    },
    [collections, router],
  );

  const handleRouteSelect = useCallback(
    (name: string) => {
      const pathParam =
        folderPath.length > 0
          ? `&p=${encodeURIComponent(folderPath.join("/"))}`
          : "";
      router.push(
        `/hittable?c=${encodeURIComponent(collectionName)}&r=${encodeURIComponent(name)}${pathParam}`,
      );
      setOpenDropdown(null);
    },
    [collectionName, folderPath, router],
  );

  const currentCollection = collections.find(
    (c) => c.collectionName === collectionName,
  );
  const currentItems = currentCollection
    ? getItemsAtPath(currentCollection.items, folderPath)
    : [];
  const currentRoutes = currentItems
    .filter((i): i is THittableItem & { type: "route" } => i.type === "route");
  const collectionNames = collections.map((c) => c.collectionName);

  // Get siblings at the PARENT level of a breadcrumb segment (one level up from the clicked segment)
  const getItemsAtParentLevel = (levelIndex: number): THittableItem[] => {
    if (!currentCollection) return [];
    const parentPath = folderPath.slice(0, levelIndex);
    return getItemsAtPath(currentCollection.items, parentPath);
  };

  const handleFolderItemSelect = useCallback(
    (levelIndex: number, item: THittableItem) => {
      const parentPath = folderPath.slice(0, levelIndex);
      if (item.type === "folder") {
        // Drill into the selected folder
        router.push(
          `/hittable?c=${encodeURIComponent(collectionName)}&p=${encodeURIComponent([...parentPath, item.name].join("/"))}`,
        );
      } else {
        // Select a route at this parent level
        const pathParam = parentPath.length > 0
          ? `&p=${encodeURIComponent(parentPath.join("/"))}`
          : "";
        router.push(
          `/hittable?c=${encodeURIComponent(collectionName)}&r=${encodeURIComponent(item.name)}${pathParam}`,
        );
      }
      setOpenDropdown(null);
    },
    [collectionName, folderPath, router],
  );

  function extractMethod(curlStr: string): string {
    const methodMatch = curlStr.match(
      /-X\s+(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)/i,
    );
    if (methodMatch) return methodMatch[1].toUpperCase();
    if (curlStr.match(/curl\s+-I\b/)) return "HEAD";
    return "GET";
  }

  return (
    <div className="h-full w-full flex flex-col gap-4 font-mono">
      <NoExtensionModal
        checked={extensionChecked}
        available={extensionAvailable}
      />

      <div className="flex flex-col md:flex-row justify-between items-center">
        <div
          className="flex justify-start items-center gap-2 h-3 w-full relative"
          ref={dropdownRef}
        >
          {/* Collection breadcrumb */}
          <div className="relative">
            <button
              onClick={() =>
                setOpenDropdown(
                  openDropdown === "collection" ? null : "collection",
                )
              }
              className="flex items-center gap-1 text-[8px] md:text-[9px] uppercase text-cyan-500/40 hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {collectionName || "No collection"}
              <ChevronDown
                size={8}
                className={`transition-transform ${openDropdown === "collection" ? "rotate-180" : ""}`}
              />
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

          {/* Folder path segments */}
          {folderPath.map((segment, i) => {
            const siblingItems = getItemsAtParentLevel(i);
            const isOpen = openDropdown === i;
            return (
              <span key={i} className="flex items-center gap-1 relative">
                <span className="text-white/10 text-xs">/</span>
                <button
                  onClick={() => setOpenDropdown(isOpen ? null : i)}
                  className="flex items-center gap-1 text-[8px] md:text-[9px] uppercase text-cyan-500/40 hover:text-cyan-400 transition-colors cursor-pointer"
                >
                  {segment}
                  <ChevronDown
                    size={7}
                    className={`transition-transform ${isOpen ? "rotate-180" : ""}`}
                  />
                </button>
                {isOpen && siblingItems.length > 0 && (
                  <div className="absolute top-full left-2 mt-1 z-50 bg-[#0e1f35] border border-white/10 rounded-lg shadow-xl shadow-black/50 py-1 min-w-[180px] max-h-[220px] overflow-y-auto">
                    {siblingItems.map((item) => {
                      const isCurrentFolder = item.type === "folder" && item.name === segment;
                      const isCurrentRoute = item.type === "route" && item.name === curlName;
                      const isActive = isCurrentFolder || isCurrentRoute;
                      const method = item.type === "route" ? extractMethod((item as THittableItem & { type: "route" }).curl) : null;
                      const mc = method ? METHOD_COLORS[method] ?? "#94a3b8" : null;
                      return (
                        <button
                          key={item.name}
                          onClick={() => handleFolderItemSelect(i, item)}
                          className={`w-full text-left px-3 py-1.5 text-[10px] transition-colors cursor-pointer flex items-center gap-2 ${
                            isActive
                              ? "text-cyan-400 bg-cyan-500/10"
                              : "text-white/50 hover:text-white/80 hover:bg-white/5"
                          }`}
                        >
                          {isActive && (
                            <span className="w-0.5 h-3 bg-cyan-400 rounded-r-full shrink-0" />
                          )}
                          {item.type === "folder" ? (
                            <Folder size={10} className="text-cyan-500/40 shrink-0" />
                          ) : mc ? (
                            <span
                              className="shrink-0 px-1 py-0.5 rounded text-[8px] font-bold leading-none border"
                              style={{
                                color: mc,
                                borderColor: `${mc}40`,
                                backgroundColor: `${mc}12`,
                              }}
                            >
                              {method}
                            </span>
                          ) : (
                            <Route size={10} className="text-white/30 shrink-0" />
                          )}
                          <span className="truncate">{item.name}</span>
                        </button>
                      );
                    })}
                  </div>
                )}
              </span>
            );
          })}

          <span className="text-white/10 text-xs">/</span>

          {/* Route breadcrumb */}
          <div className="relative">
            <button
              onClick={() =>
                setOpenDropdown(
                  openDropdown === "route" ? null : "route",
                )
              }
              className="flex items-center gap-1 text-[8px] md:text-[9px] uppercase text-cyan-500/70 hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {curlName || "No route"}
              <ChevronDown
                size={8}
                className={`transition-transform ${openDropdown === "route" ? "rotate-180" : ""}`}
              />
            </button>
            {openDropdown === "route" && currentRoutes.length > 0 && (
              <div className="absolute top-full left-0 mt-1 z-50 bg-[#0e1f35] border border-white/10 rounded-lg shadow-xl shadow-black/50 py-1 min-w-[180px] max-h-[220px] overflow-y-auto">
                {currentRoutes.map((item) => {
                  const isActive = item.name === curlName;
                  const method = extractMethod(item.curl);
                  const mc = METHOD_COLORS[method] ?? "#94a3b8";
                  return (
                    <button
                      key={item.name}
                      onClick={() => handleRouteSelect(item.name)}
                      className={`w-full text-left px-3 py-1.5 text-[10px] transition-colors cursor-pointer flex items-center gap-2 ${
                        isActive
                          ? "text-cyan-400 bg-cyan-500/10"
                          : "text-white/50 hover:text-white/80 hover:bg-white/5"
                      }`}
                    >
                      {isActive && (
                        <span className="w-0.5 h-3 bg-cyan-400 rounded-r-full shrink-0" />
                      )}
                      <span
                        className="shrink-0 px-1 py-0.5 rounded text-[8px] font-bold leading-none border"
                        style={{
                          color: mc,
                          borderColor: `${mc}40`,
                          backgroundColor: `${mc}12`,
                        }}
                      >
                        {method}
                      </span>
                      <span className="truncate">{item.name}</span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </div>
        {isUnsaved() && (
          <div className="flex items-center w-full justify-start md:justify-end gap-2">
            <button
              onClick={handleRevert}
              title="Discard changes"
              className="flex items-center gap-1.5 rounded-md border border-white/10 bg-white/5 px-2 py-1 text-[8px] md:text-[10px] font-semibold text-white/40 transition-all cursor-pointer hover:border-amber-500/30 hover:text-amber-400 min-h-[28px]"
            >
              <RotateCcw className="h-3 w-3" />
            </button>
            <span className="flex items-center gap-1.5 rounded-md bg-amber-500/10 px-2.5 py-1 text-[8px] md:text-[10px] font-semibold text-amber-400 border border-amber-500/20">
              <MessageCircleWarning
                className="h-3 w-3"
                strokeWidth={2.5}
              />
              Unsaved · Ctrl/Cmd+S
            </span>
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
          <p className="text-xs tracking-widest uppercase text-cyan-300 mb-2">
            Hittable
          </p>
          <h2 className="text-2xl font-semibold tracking-tight text-slate-100 mb-3">
            {hasCollections ? "Select a route" : "No collections yet"}
          </h2>
          <p className="text-sm text-slate-400 max-w-[300px] leading-relaxed">
            {hasCollections
              ? "Choose a collection and route from the sidebar to start making requests."
              : "Create a collection in the sidebar, then add routes to start testing your APIs."}
          </p>
        </div>
      </div>
    </div>
  );
}
