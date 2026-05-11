"use client";

import {
  Dispatch,
  SetStateAction,
  useEffect,
  useCallback,
  useMemo,
} from "react";
import { Package2, Route, ChevronLeft, ChevronRight } from "lucide-react";
import {
  CreateModal,
  EnvModal,
  ImportModal,
  InfoModal,
  Menu,
  NoteModal,
} from "@/components";
import { curlConverter } from "@/utils/curlConverter";
import { THittableSelectorSelection } from "@/types";
import { parseStringToJson } from "@/utils/JsonStringParsing";
import { useRouter, useSearchParams } from "next/navigation";
import { useShortcuts } from "@/context/ShortcutKeypressProvider";
import useKeypress from "@/hooks/useKeypress";
import {
  createCurlName,
  isAlreadyExists,
} from "@/utils/hittableCollectionModifier";
import { useDataContext } from "@/context/dataContext";

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
        onClick={onClick}
      >
        {name}
      </div>
      {menuSlot}
    </div>
  );
}

export default function Selector() {
  const { collections, setCollections, setSelectorResponse } = useDataContext();

  const router = useRouter();
  const searchParams = useSearchParams();

  const collectionCurlList = useMemo(
    () =>
      collections.reduce(
        (acc, c) => {
          acc[c.collectionName] = c.curls.map((curl) => curl.name);
          return acc;
        },
        {} as Record<string, string[]>,
      ),
    [collections],
  );

  const {
    shortcuts: { toggleSidebar },
    toggle,
  } = useShortcuts();

  const selection = useMemo<THittableSelectorSelection>(() => {
    const c = searchParams.get("c") ?? "";
    const defaultRoute = collectionCurlList[c]?.[0] ?? "";
    const r = searchParams.get("r") ?? defaultRoute;
    return { collectionName: c, curlName: r };
  }, [searchParams, collectionCurlList]);

  const setSelection: Dispatch<SetStateAction<THittableSelectorSelection>> =
    useCallback(
      (valueOrUpdater) => {
        const next =
          typeof valueOrUpdater === "function"
            ? valueOrUpdater(selection)
            : valueOrUpdater;

        const c = next.collectionName ?? "";
        const r = next.curlName ?? "";

        if (!c) {
          router.push("/hittable");
          return;
        }

        if (!r) {
          router.push(`/hittable?c=${encodeURIComponent(c)}`);
          return;
        }

        router.push(
          `/hittable?c=${encodeURIComponent(c)}&r=${encodeURIComponent(r)}`,
        );
      },
      [router, selection],
    );

  const handleSelect = (collectionName: string, curlName: string) => {
    setSelection({ collectionName, curlName });
  };

  const createNewRoute = useCallback(() => {
    if (!selection.collectionName) return;

    let newName = "Untitled";
    while (
      isAlreadyExists(
        collectionCurlList,
        "route",
        newName,
        selection.collectionName,
      )
    ) {
      newName += " - New";
    }

    setCollections((prev) =>
      createCurlName(prev, selection.collectionName, newName, ""),
    );
    setSelection({
      collectionName: selection.collectionName,
      curlName: newName,
    });
  }, [
    selection.collectionName,
    collectionCurlList,
    setCollections,
    setSelection,
  ]);

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
    const curl = collection?.curls.find((c) => c.name === selection.curlName);

    setSelectorResponse({
      collectionName: selection.collectionName,
      curlName: selection.curlName,
      env: collection?.env,
      curlJson: curlConverter(curl?.curl || ""),
      responseJson: parseStringToJson(curl?.response || ""),
    });
  }, [selection, collections, setSelectorResponse]);

  const collectionsPanel = (
    <div
      className={`min-w-[75px] h-full flex flex-col bg-[#0a1628]/80 border-r border-white/5 overflow-visible transition-all duration-200 ${toggleSidebar ? "w-[40px]" : "w-[160px]"}`}
    >
      {/* Header */}
      <div className={`w-full ${toggleSidebar ? "h-full" : ""} px-3 pt-4 pb-3 border-b border-white/5 flex items-center justify-between gap-2 transition-all duration-100`}>
        {!toggleSidebar ? (
          <>
          <p className="text-[9px] tracking-[0.25em] uppercase text-cyan-500/50">
            Collections
          </p>
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
          <NoteModal />
          <EnvModal />
          <InfoModal />
        </div>
      )}
      </div>

      {/* Body — hidden when collapsed */}
      {!toggleSidebar && (
        <>
          <div className="px-3 pt-3 pb-2 border-b border-white/5">
            <CreateModal
              type="collection"
              selection={selection}
              setSelection={setSelection}
              collectionCurlList={collectionCurlList}
            />
          </div>

          <div className="flex-1 py-2 overflow-visible">
            {Object.keys(collectionCurlList).length === 0 ? (
              <EmptyState icon={Package2} label="No collections" />
            ) : (
              Object.entries(collectionCurlList).map(
                ([collectionName, curlList], i) => (
                  <PanelItem
                    key={i}
                    name={collectionName}
                    isActive={selection.collectionName === collectionName}
                    onClick={() =>
                      handleSelect(collectionName, curlList[0] ?? "")
                    }
                    menuSlot={
                      <Menu
                        type="collection"
                        collectionCurlList={collectionCurlList}
                        currentName={collectionName}
                        exportString={JSON.stringify(
                          collections.find(
                            (el) => el.collectionName === collectionName,
                          ),
                        )}
                        setCollections={setCollections}
                        setSelection={setSelection}
                      />
                    }
                  />
                ),
              )
            )}
          </div>
        </>
      )}
    </div>
  );

  const routesPanel = selection.collectionName && !toggleSidebar && (
    <div className="h-full w-[160px] flex flex-col bg-[#0c1a2e]/60 overflow-visible">
      {/* Header */}
      <div className="px-3 pt-4 pb-3 border-b border-white/5">
        <p className="text-[9px] tracking-[0.25em] uppercase text-cyan-500/50 mb-2">
          Routes
        </p>
        <div className="flex flex-col gap-1.5">
          <EnvModal />
          <CreateModal
            type="route"
            selection={selection}
            setSelection={setSelection}
            collectionCurlList={collectionCurlList}
          />
        </div>
      </div>

      {/* Body */}
      <div className="flex-1 py-2 overflow-y-auto">
        {collectionCurlList[selection.collectionName]?.length === 0 ? (
          <EmptyState icon={Route} label="No routes yet" />
        ) : (
          collectionCurlList[selection.collectionName]?.map((curl, i) => (
            <PanelItem
              key={i}
              name={curl}
              isActive={selection.curlName === curl}
              onClick={() => handleSelect(selection.collectionName, curl)}
              menuSlot={
                <Menu
                  type="route"
                  collectionCurlList={collectionCurlList}
                  currentName={curl}
                  collectionName={selection.collectionName}
                  setCollections={setCollections}
                  setSelection={setSelection}
                />
              }
            />
          ))
        )}
      </div>
    </div>
  );

  return (
    <div className="h-full flex shrink-0 border-r border-white/5">
      {collectionsPanel}
      {routesPanel}
    </div>
  );
}
