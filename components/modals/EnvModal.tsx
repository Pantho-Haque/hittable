"use client";

import { Plus, Settings2, Trash2, Lock } from "lucide-react";
import {
  useCallback,
  useEffect,
  useState,
} from "react";
import {
  THittableEnv,
} from "@/types";
import { createPortal } from "react-dom";
import { updateEnv, updateSecrets } from "@/utils/hittableCollectionModifier";
import { useDataContext } from "@/context/dataContext";

type TabKey = "env" | "secrets";

export default function EnvModal({ collectionName: propCollectionName }: { collectionName?: string } = {}) {
  const [open, setOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<TabKey>("env");
  const [localEnv, setLocalEnv] = useState<[string, string][]>([]);
  const [localSecrets, setLocalSecrets] = useState<[string, string][]>([]);
  const {selectorResponse, setSelectorResponse, setCollections, collections} = useDataContext()
  const effectiveCollectionName = propCollectionName ?? selectorResponse?.collectionName ?? "";
  const collection = collections.find((c) => c.collectionName === effectiveCollectionName);
  const env = collection?.env ?? selectorResponse?.env ?? {};
  const secrets = collection?.secrets ?? selectorResponse?.secrets ?? {};

  const openModal = () => {
    setLocalEnv(Object.entries(env ?? {}) as [string, string][]);
    setLocalSecrets(Object.entries(secrets ?? {}) as [string, string][]);
    setActiveTab("env");
    setOpen(true);
  };

  const handleEnvChange = (index: number, field: "key" | "value", val: string) => {
    setLocalEnv((prev) =>
      prev.map((entry, i) =>
        i === index ? (field === "key" ? [val, entry[1]] : [entry[0], val]) : entry,
      ),
    );
  };

  const handleSecretChange = (index: number, field: "key" | "value", val: string) => {
    setLocalSecrets((prev) =>
      prev.map((entry, i) =>
        i === index ? (field === "key" ? [val, entry[1]] : [entry[0], val]) : entry,
      ),
    );
  };

  const handleSave = useCallback(() => {
    const updatedEnv = Object.fromEntries(localEnv.filter(([k]) => k.trim())) as THittableEnv;
    const updatedSecrets = Object.fromEntries(localSecrets.filter(([k]) => k.trim())) as THittableEnv;
    setCollections((prev) => {
      let result = updateEnv(prev, effectiveCollectionName, updatedEnv);
      result = updateSecrets(result, effectiveCollectionName, updatedSecrets);
      return result;
    });
    setSelectorResponse((prev) => {
      if (!prev || prev.collectionName !== effectiveCollectionName) return prev;
      return { ...prev, env: updatedEnv, secrets: updatedSecrets };
    });
    setOpen(false);
  }, [localEnv, localSecrets, effectiveCollectionName, setCollections, setSelectorResponse]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
      if ((e.metaKey || e.ctrlKey) && e.key === "s") { e.preventDefault(); handleSave(); }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, handleSave]);

  const currentEntries = activeTab === "env" ? localEnv : localSecrets;
  const setCurrentEntries = activeTab === "env" ? setLocalEnv : setLocalSecrets;
  const handleChange = activeTab === "env" ? handleEnvChange : handleSecretChange;

  return (
    <>
      <button
        onClick={openModal}
        title="Environment Variables"
        className="modal-button-mini"
      >
        <Settings2 size={14} />
      </button>

      {open &&
        createPortal(
          <div
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            onClick={() => setOpen(false)}
          >
            <div
              className="relative bg-[#0a1628] border border-white/10 rounded-xl shadow-2xl p-6 w-[520px] max-h-[80vh] flex flex-col gap-5 font-mono overflow-hidden"
              onClick={(e) => e.stopPropagation()}
              style={{ boxShadow: "0 0 0 1px rgba(0,229,204,0.08), 0 24px 80px rgba(0,0,0,0.8)" }}
            >
              {/* Corner brackets */}
              <span className="absolute top-0 left-0 w-4 h-4 border-t border-l border-cyan-500/30 rounded-tl-xl" />
              <span className="absolute top-0 right-0 w-4 h-4 border-t border-r border-cyan-500/30 rounded-tr-xl" />
              <span className="absolute bottom-0 left-0 w-4 h-4 border-b border-l border-cyan-500/30 rounded-bl-xl" />
              <span className="absolute bottom-0 right-0 w-4 h-4 border-b border-r border-cyan-500/30 rounded-br-xl" />

              <div className="flex items-start justify-between">
                <div>
                  <p className="text-[9px] tracking-[0.3em] uppercase text-cyan-500/60 mb-1">Environment</p>
                  <h2 className="text-sm font-bold text-white/90 capitalize">{effectiveCollectionName}</h2>
                  <p className="text-[10px] text-white/25 mt-1">
                    Reference with <span className="text-cyan-400/60 font-mono">&lt;&lt;KEY&gt;&gt;</span> in your requests
                  </p>
                </div>
                <button
                  onClick={() => setCurrentEntries((prev) => [...prev, ["", ""]])}
                  className="flex items-center gap-1 text-[10px] text-cyan-400 border border-cyan-500/25 px-2.5 py-1.5 rounded-md hover:bg-cyan-500/10 cursor-pointer transition-colors"
                >
                  <Plus size={11} /> Add
                </button>
              </div>

              {/* Tabs */}
              <div className="flex gap-1 border-b border-white/10">
                <button
                  onClick={() => setActiveTab("env")}
                  className={`flex items-center gap-1.5 px-3 py-2 text-[10px] font-semibold transition-colors cursor-pointer border-b-2 -mb-px ${
                    activeTab === "env"
                      ? "text-cyan-400 border-cyan-400"
                      : "text-white/30 border-transparent hover:text-white/50"
                  }`}
                >
                  <Settings2 size={11} />
                  Env Vars
                </button>
                <button
                  onClick={() => setActiveTab("secrets")}
                  className={`flex items-center gap-1.5 px-3 py-2 text-[10px] font-semibold transition-colors cursor-pointer border-b-2 -mb-px ${
                    activeTab === "secrets"
                      ? "text-amber-400 border-amber-400"
                      : "text-white/30 border-transparent hover:text-white/50"
                  }`}
                >
                  <Lock size={11} />
                  Secrets
                </button>
              </div>

              {/* Tab content */}
              <div className="flex-1 overflow-y-auto flex flex-col gap-2">
                {activeTab === "secrets" && (
                  <p className="text-[9px] text-amber-400/50 px-1">
                    Secrets are not resolved during export — they stay as variable references in the target format.
                  </p>
                )}
                {currentEntries.length > 0 && (
                  <div className="grid grid-cols-[1fr_1fr_auto] gap-2 text-[9px] text-white/20 font-semibold uppercase tracking-widest px-1 mb-1">
                    <span>Key</span>
                    <span>Value</span>
                    <span />
                  </div>
                )}
                {currentEntries.map(([key, value], i) => (
                  <div key={i} className="grid grid-cols-[1fr_1fr_auto] gap-2 items-center">
                    <input
                      className="border-b border-white/10 bg-transparent focus:border-cyan-500/50 outline-none py-1.5 px-1 text-xs text-white/70 placeholder-white/15 transition-colors"
                      placeholder="KEY"
                      value={key}
                      onChange={(e) => handleChange(i, "key", e.target.value)}
                    />
                    <input
                      className="border-b border-white/10 bg-transparent focus:border-cyan-500/50 outline-none py-1.5 px-1 text-xs text-white/70 placeholder-white/15 transition-colors"
                      placeholder="value"
                      value={value}
                      onChange={(e) => handleChange(i, "value", e.target.value)}
                    />
                    <button
                      onClick={() => setCurrentEntries((prev) => prev.filter((_, idx) => idx !== i))}
                      className="p-1 text-white/20 hover:text-red-400 transition-colors cursor-pointer"
                    >
                      <Trash2 size={13} />
                    </button>
                  </div>
                ))}
                {currentEntries.length === 0 && (
                  <div className="flex flex-col items-center justify-center py-8 gap-2 text-white/20">
                    {activeTab === "env" ? <Settings2 size={20} /> : <Lock size={20} />}
                    <p className="text-xs">
                      {activeTab === "env" ? "No environment variables yet" : "No secrets yet"}
                    </p>
                  </div>
                )}
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t border-white/5">
                <button
                  onClick={() => setOpen(false)}
                  className="px-4 py-1.5 text-xs rounded-md border border-white/10 text-white/40 hover:bg-white/5 hover:text-white/70 transition-colors cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  onClick={handleSave}
                  className="px-4 py-1.5 text-xs rounded-md font-bold cursor-pointer active:scale-95 transition-all"
                  style={{ background: "rgba(0,229,204,0.15)", border: "1px solid rgba(0,229,204,0.3)", color: "#00e5cc" }}
                >
                  Save · Ctrl/Cmd+S
                </button>
              </div>
            </div>
          </div>,
          document.body,
        )}
    </>
  );
}
