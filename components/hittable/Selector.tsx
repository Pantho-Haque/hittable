"use client";

import {
  Dispatch,
  SetStateAction,
  useEffect,
  useCallback,
  useMemo,
  useState,
} from "react";
import {
  Package2,
  Route,
  ChevronLeft,
  ChevronRight,
  Search,
  Folder,
  ArrowLeft,
  Plus,
  Settings2,
} from "lucide-react";
import {
  CreateModal,
  EnvModal,
  ImportModal,
  InfoModal,
  Menu,
  NoteModal,
  AuthModal,
} from "@/components";
import HistoryPanel from "./HistoryPanel";
import { curlConverter } from "@/utils/curlConverter";
import {
  THittableSelectorSelection,
  THittableItem,
} from "@/types";
import { parseStringToJson } from "@/utils/JsonStringParsing";
import { useRouter, useSearchParams } from "next/navigation";
import { useShortcuts } from "@/context/ShortcutKeypressProvider";
import useKeypress from "@/hooks/useKeypress";
import {
  createCurlName,
  createFolderName,
  isAlreadyExistsInPath,
} from "@/utils/hittableCollectionModifier";
import { useDataContext } from "@/context/dataContext";
import { METHOD_COLORS } from "@/constants";
import {
  getItemsAtPath,
  findRoute,
  collectAllRouteNames,
} from "@/utils/treeHelpers";

function EmptyState({
  icon: Icon,
  label,
}: {
  icon: typeof Package2;
  label: string;
}) {
  return (
    <div className="px-3 py-6 text-center">
      <Icon size={18} className="text-white/10 mx-auto mb-2" />
      <p className="text-[10px] text-white/20 leading-relaxed">{label}</p>
    </div>
  );
}

function PanelItem({
  name,
  isActive,
  onClick,
  menuSlot,
}: {
  name: string;
  isActive: boolean;
  onClick: () => void;
  menuSlot: React.ReactNode;
}) {
  return (
    <div
      title={name}
      onClick={onClick}
      className={`group/menu relative flex items-center justify-between mx-2 mb-0.5 rounded-md overflow-visible transition-all cursor-pointer
        ${
          isActive
            ? "bg-cyan-500/10 border border-cyan-500/20"
            : "border border-transparent hover:bg-white/4 hover:border-white/5"
        }`}
    >
      {isActive && (
        <span className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-4 bg-cyan-400 rounded-r-full" />
      )}
      <div
        className="flex-1 pl-3 pr-1 py-2.5 truncate text-xs font-semibold capitalize"
        style={{ color: isActive ? "#00e5cc" : "rgba(255,255,255,0.5)" }}
      >
        {name}
      </div>
      <div onClick={(e) => e.stopPropagation()}>
        {menuSlot}
      </div>
    </div>
  );
}

function extractMethod(curlStr: string): string {
  const methodMatch = curlStr.match(
    /-X\s+(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)/i,
  );
  if (methodMatch) return methodMatch[1].toUpperCase();
  if (curlStr.match(/curl\s+-I\b/)) return "HEAD";
  return "GET";
}

function MethodBadge({ method }: { method: string }) {
  const color = METHOD_COLORS[method.toUpperCase()] ?? "#94a3b8";
  return (
    <span
      className="shrink-0 px-1.5 py-0.5 rounded text-[9px] font-bold leading-none border"
      style={{
        color,
        borderColor: `${color}40`,
        backgroundColor: `${color}12`,
      }}
    >
      {method.toUpperCase()}
    </span>
  );
}

function RouteItem({
  item,
  selection,
  collectionName,
  folderPath,
  handleSelect,
  collectionCurlList,
  setCollections,
  setSelection,
}: {
  item: THittableItem & { type: "route" };
  selection: THittableSelectorSelection;
  collectionName: string;
  folderPath: string[];
  handleSelect: (collectionName: string, folderPath: string[], curlName: string) => void;
  collectionCurlList: Record<string, string[]>;
  setCollections: Dispatch<SetStateAction<import("@/types").THittableCollections>>;
  setSelection: Dispatch<SetStateAction<THittableSelectorSelection>>;
}) {
  const method = extractMethod(item.curl);
  const isActive =
    selection.curlName === item.name &&
    selection.collectionName === collectionName &&
    JSON.stringify(selection.folderPath) === JSON.stringify(folderPath);

  return (
    <div
      title={item.name}
      onClick={() => handleSelect(collectionName, folderPath, item.name)}
      className={`group/menu relative flex items-center justify-between mx-2 mb-0.5 rounded-md overflow-visible transition-all cursor-pointer
        ${isActive ? "bg-cyan-500/10 border border-cyan-500/20" : "border border-transparent hover:bg-white/4 hover:border-white/5"}`}
    >
      {isActive && (
        <span className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-4 bg-cyan-400 rounded-r-full" />
      )}
      <div className="flex items-center gap-1.5 flex-1 min-w-0 pl-3 pr-1 py-2.5">
        <MethodBadge method={method} />
        <span
          className="flex-1 truncate text-xs font-semibold capitalize"
          style={{ color: isActive ? "#00e5cc" : "rgba(255,255,255,0.5)" }}
        >
          {item.name}
        </span>
      </div>
      <div onClick={(e) => e.stopPropagation()}>
        <Menu
          type="route"
          collectionCurlList={collectionCurlList}
          currentName={item.name}
          collectionName={collectionName}
          folderPath={folderPath}
          setCollections={setCollections}
          setSelection={setSelection}
        />
      </div>
    </div>
  );
}

function FolderItem({
  item,
  onClick,
}: {
  item: THittableItem & { type: "folder" };
  onClick: () => void;
}) {
  return (
    <div
      title={item.name}
      onClick={onClick}
      className="group/menu relative flex items-center mx-2 mb-0.5 rounded-md border border-transparent hover:bg-white/4 hover:border-white/5 transition-all cursor-pointer"
    >
      <div className="flex items-center gap-1.5 flex-1 min-w-0 pl-3 pr-1 py-2.5">
        <Folder size={12} className="shrink-0 text-cyan-500/40" />
        <span className="flex-1 truncate text-xs font-semibold text-white/50">
          {item.name}
        </span>
        <span className="text-[9px] text-white/50">{item.items.length}</span>
      </div>
    </div>
  );
}

export default function Selector() {
  const { collections, setCollections, setSelectorResponse } = useDataContext();

  const router = useRouter();
  const searchParams = useSearchParams();

  const {
    shortcuts: { toggleSidebar },
    toggle,
  } = useShortcuts();

  // Navigation state for drill-down — derived from URL params
  const [filterQuery, setFilterQuery] = useState("");

  // Parse URL params to restore deep-linked selection
  const urlSelection = useMemo(() => {
    const c = searchParams.get("c") ?? "";
    const r = searchParams.get("r") ?? "";
    const p = searchParams.get("p") ?? "";
    const folderPath = p ? p.split("/") : [];
    return { collectionName: c, curlName: r, folderPath };
  }, [searchParams]);

  // Derive nav state from URL
  const navCollection = urlSelection.collectionName || null;
  const navPath = urlSelection.folderPath;

  const selection = useMemo<THittableSelectorSelection>(() => {
    return {
      collectionName: urlSelection.collectionName,
      curlName: urlSelection.curlName,
      folderPath: urlSelection.folderPath,
    };
  }, [urlSelection]);

  const setSelection: Dispatch<SetStateAction<THittableSelectorSelection>> =
    useCallback(
      (valueOrUpdater) => {
        const next =
          typeof valueOrUpdater === "function"
            ? valueOrUpdater(selection)
            : valueOrUpdater;

        const c = next.collectionName ?? "";
        const r = next.curlName ?? "";
        const p = next.folderPath ?? [];

        if (!c) {
          router.push("/hittable");
          return;
        }

        const pathParam = p.length > 0 ? `&p=${encodeURIComponent(p.join("/"))}` : "";
        if (!r) {
          router.push(`/hittable?c=${encodeURIComponent(c)}${pathParam}`);
          return;
        }

        router.push(
          `/hittable?c=${encodeURIComponent(c)}&r=${encodeURIComponent(r)}${pathParam}`,
        );
      },
      [router, selection],
    );

  const collectionCurlList = useMemo(
    () =>
      collections.reduce(
        (acc, c) => {
          acc[c.collectionName] = collectAllRouteNames(c);
          return acc;
        },
        {} as Record<string, string[]>,
      ),
    [collections],
  );

  const handleSelect = useCallback(
    (collectionName: string, folderPath: string[], curlName: string) => {
      setSelection({ collectionName, folderPath, curlName });
    },
    [setSelection],
  );

  const handleDrillIntoCollection = useCallback(
    (collectionName: string) => {
      const col = collections.find((c) => c.collectionName === collectionName);
      if (!col) return;
      // Auto-select first route if available
      const firstRoute = col.items.find((i) => i.type === "route");
      if (firstRoute && firstRoute.type === "route") {
        setSelection({ collectionName, folderPath: [], curlName: firstRoute.name });
      } else {
        setSelection({ collectionName, folderPath: [], curlName: "" });
      }
    },
    [collections, setSelection],
  );

  const handleDrillIntoFolder = useCallback(
    (folderName: string) => {
      setSelection((prev) => ({
        ...prev,
        folderPath: [...prev.folderPath, folderName],
        curlName: "",
      }));
    },
    [setSelection],
  );

  const handleBack = useCallback(() => {
    if (navPath.length > 0) {
      setSelection((prev) => ({
        ...prev,
        folderPath: prev.folderPath.slice(0, -1),
        curlName: "",
      }));
    } else {
      router.push("/hittable");
    }
  }, [navPath, router, setSelection]);

  const handleBreadcrumbJump = useCallback(
    (targetPath: string[]) => {
      setSelection((prev) => ({
        ...prev,
        folderPath: targetPath,
        curlName: "",
      }));
    },
    [setSelection],
  );

  const createNewRoute = useCallback(() => {
    if (!navCollection) return;

    let newName = "Untitled";
    const col = collections.find((c) => c.collectionName === navCollection);
    while (isAlreadyExistsInPath(col, navPath, newName)) {
      newName += " - New";
    }

    setCollections((prev) =>
      createCurlName(prev, navCollection, newName, "", navPath),
    );
    setSelection({
      collectionName: navCollection,
      folderPath: navPath,
      curlName: newName,
    });
  }, [navCollection, navPath, collections, setCollections, setSelection]);

  const createNewFolder = useCallback(() => {
    if (!navCollection) return;

    let newName = "New Folder";
    const col = collections.find((c) => c.collectionName === navCollection);
    while (isAlreadyExistsInPath(col, navPath, newName)) {
      newName += " - New";
    }

    setCollections((prev) =>
      createFolderName(prev, navCollection, newName, navPath),
    );
  }, [navCollection, navPath, collections, setCollections]);

  useKeypress({
    key: "t",
    isShift: true,
    func: createNewRoute,
  });

  useEffect(() => {
    if (!selection.curlName) return setSelectorResponse(null);

    const collection = collections.find(
      (c) => c.collectionName === selection.collectionName,
    );
    if (!collection) return;

    const curl = findRoute(
      collection.items,
      selection.folderPath,
      selection.curlName,
    );

    setSelectorResponse({
      collectionName: selection.collectionName,
      folderPath: selection.folderPath,
      curlName: selection.curlName,
      env: collection?.env,
      secrets: collection?.secrets,
      curlJson: curlConverter(curl?.curl || ""),
      responseJson: parseStringToJson(curl?.response || ""),
    });
  }, [selection, collections, setSelectorResponse]);

  // Auto-collapse sidebar on mobile
  useEffect(() => {
    if (window.innerWidth < 768 && !toggleSidebar) {
      toggle("toggleSidebar");
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Determine what to show in the panel
  const currentCollection = navCollection
    ? collections.find((c) => c.collectionName === navCollection)
    : null;
  const currentItems = currentCollection
    ? getItemsAtPath(currentCollection.items, navPath)
    : [];
  const hasFilter = filterQuery.trim().length > 0;
  const filteredItems = hasFilter
    ? currentItems.filter((i) =>
        i.name.toLowerCase().includes(filterQuery.toLowerCase()),
      )
    : currentItems;

  const collectionsPanel = (
    <div
      className={`min-w-[75px] h-full flex flex-col bg-[#0a1628]/80 border-r border-white/5 overflow-visible transition-all duration-200 ${toggleSidebar ? "w-[40px]" : "w-[180px]"}`}
    >
      {/* Header */}
      <div
        className={`w-full ${toggleSidebar ? "h-full" : ""} px-3 pt-4 pb-3 border-b border-white/5 flex items-center justify-between gap-2 transition-all duration-100`}
      >
        {!toggleSidebar ? (
          <>
            {navCollection ? (
              <button
                onClick={handleBack}
                className="flex items-center gap-1.5 text-[9px] tracking-[0.25em] uppercase text-cyan-500/50 hover:text-cyan-400 transition-colors cursor-pointer"
              >
                <ArrowLeft size={12} />
                <span className="truncate">{currentCollection?.collectionName}</span>
              </button>
            ) : (
              <p className="text-[9px] tracking-[0.25em] uppercase text-cyan-500/50">
                Collections
              </p>
            )}
            <button
              onClick={() => toggle("toggleSidebar")}
              className="text-white/30 hover:text-cyan-400 transition-colors cursor-pointer"
            >
              <ChevronLeft size={14} />
            </button>
          </>
        ) : (
          <div className="w-full h-full flex flex-col justify-start items-center gap-3">
            <button
              onClick={() => toggle("toggleSidebar")}
              className="text-white/30 hover:text-cyan-400 transition-colors cursor-pointer"
            >
              <ChevronRight size={14} />
            </button>
            <ImportModal />
            <AuthModal />
            <HistoryPanel />
            <NoteModal />
            <EnvModal />
            <InfoModal />
          </div>
        )}
      </div>

      {/* Breadcrumb trail (when inside a collection) */}
      {navCollection && !toggleSidebar && navPath.length > 0 && (
        <div className="px-3 py-1.5 border-b border-white/5 flex items-center gap-1 text-[9px] overflow-x-auto whitespace-nowrap">
          <button
            onClick={() => handleBreadcrumbJump([])}
            className="text-cyan-500/40 hover:text-cyan-400 transition-colors cursor-pointer shrink-0"
          >
            {navCollection}
          </button>
          {navPath.map((segment, i) => (
            <span key={i} className="flex items-center gap-1 shrink-0">
              <span className="text-white/15">/</span>
              <button
                onClick={() => handleBreadcrumbJump(navPath.slice(0, i + 1))}
                className="text-cyan-500/40 hover:text-cyan-400 transition-colors cursor-pointer"
              >
                {segment}
              </button>
            </span>
          ))}
        </div>
      )}

      {/* Body — hidden when collapsed */}
      {!toggleSidebar && (
        <>
          <div className="px-3 pt-3 pb-2 border-b border-white/5">
            {navCollection ? (
              <div className="flex flex-col gap-1.5">
                {currentItems.length > 3 && (
                  <div className="relative">
                    <Search
                      size={10}
                      className="absolute left-2 top-1/2 -translate-y-1/2 text-white/20"
                    />
                    <input
                      type="text"
                      value={filterQuery}
                      onChange={(e) => setFilterQuery(e.target.value)}
                      placeholder="Filter..."
                      className="w-full bg-white/3 border border-white/5 rounded px-2 py-1 pl-6 text-[10px] text-white/60 placeholder-white/15 outline-none focus:border-cyan-500/30 transition-colors"
                    />
                  </div>
                )}
                <div className="flex items-center gap-1.5">
                  <EnvModal collectionName={navCollection} />
                  <CreateModal
                    type="route"
                    selection={selection}
                    setSelection={setSelection}
                    collectionCurlList={collectionCurlList}
                    folderPath={navPath}
                  />
                  <button
                    onClick={createNewFolder}
                    title="New Folder"
                    className="modal-button-mini"
                  >
                    <Folder size={14} />
                  </button>
                </div>
              </div>
            ) : (
              <CreateModal
                type="collection"
                selection={selection}
                setSelection={setSelection}
                collectionCurlList={collectionCurlList}
              />
            )}
          </div>

          <div className="flex-1 py-2 overflow-visible">
            {navCollection ? (
              // Drill-down view: show items in current folder
              filteredItems.length === 0 ? (
                <EmptyState
                  icon={navPath.length > 0 ? Folder : Route}
                  label={
                    hasFilter
                      ? "No matching items"
                      : navPath.length > 0
                        ? "Empty folder"
                        : "No routes yet"
                  }
                />
              ) : (
                <>
                  {/* Folders first */}
                  {filteredItems
                    .filter((i) => i.type === "folder")
                    .map((item) => (
                      <FolderItem
                        key={item.name}
                        item={item as THittableItem & { type: "folder" }}
                        onClick={() =>
                          handleDrillIntoFolder((item as THittableItem & { type: "folder" }).name)
                        }
                      />
                    ))}
                  {/* Then routes */}
                  {filteredItems
                    .filter((i) => i.type === "route")
                    .map((item) => (
                      <RouteItem
                        key={item.name}
                        item={item as THittableItem & { type: "route" }}
                        selection={selection}
                        collectionName={navCollection}
                        folderPath={navPath}
                        handleSelect={handleSelect}
                        collectionCurlList={collectionCurlList}
                        setCollections={setCollections}
                        setSelection={setSelection}
                      />
                    ))}
                </>
              )
            ) : (
              // Collection list view
              collections.length === 0 ? (
                <EmptyState icon={Package2} label="No collections" />
              ) : (
                collections.map((col) => (
                  <PanelItem
                    key={col.collectionName}
                    name={col.collectionName}
                    isActive={selection.collectionName === col.collectionName}
                    onClick={() =>
                      handleDrillIntoCollection(col.collectionName)
                    }
                    menuSlot={
                      <Menu
                        type="collection"
                        collectionCurlList={collectionCurlList}
                        currentName={col.collectionName}
                        exportString={JSON.stringify(col)}
                        setCollections={setCollections}
                        setSelection={setSelection}
                      />
                    }
                  />
                ))
              )
            )}
          </div>
        </>
      )}
    </div>
  );

  return (
    <div
      className={`h-full flex shrink-0 border-r border-white/5 ${!toggleSidebar ? "max-md:absolute max-md:z-40 max-md:inset-y-0 max-md:left-0 max-md:shadow-2xl max-md:shadow-black/50" : ""}`}
    >
      {collectionsPanel}
    </div>
  );
}
