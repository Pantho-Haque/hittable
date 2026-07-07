"use client";

import { Clock, Trash2, ArrowRight } from "lucide-react";
import { useState } from "react";
import { THistoryEntry } from "@/types";
import { clearHistory, formatTimestamp } from "@/utils/historyModifier";
import { METHOD_COLORS } from "@/constants";
import { useDataContext } from "@/context/dataContext";
import { ModalShell, ModalActions } from "@/components";

export default function HistoryPanel() {
  const { setFormInput, setProxyResponse, setSelectorResponse, history, setHistory } = useDataContext();
  const [isOpen, setIsOpen] = useState(false);

  const handleClear = () => {
    setHistory(clearHistory());
  };

  const handleReplay = (entry: THistoryEntry) => {
    setFormInput(entry.curlJson);
    setProxyResponse(entry.responseJson ?? null);
    setSelectorResponse({
      collectionName: "",
      curlName: `History: ${entry.method} ${new URL(entry.url).pathname}`,
      curlJson: entry.curlJson,
      responseJson: entry.responseJson,
    });
    setIsOpen(false);
  };

  return (
    <>
      <button
        onClick={(e) => {
          e.stopPropagation();
          setIsOpen(true);
        }}
        title="History"
        className="modal-button-mini relative"
      >
        <Clock size={14} />
        {history.length > 0 && (
          <span className="modal-button-badge">{history.length > 99 ? "99+" : history.length}</span>
        )}
      </button>

      {isOpen && (
        <ModalShell
          title="Request History"
          subtitle={`${history.length} request${history.length !== 1 ? "s" : ""} logged`}
          onClose={() => setIsOpen(false)}
          size="md"
        >
          <div className="flex flex-col gap-2 min-h-0 flex-1 overflow-hidden">
            {history.length > 0 && (
              <div className="flex justify-end shrink-0">
                <button
                  onClick={handleClear}
                  className="flex items-center gap-1.5 px-2 py-1 text-[10px] text-white/30 hover:text-red-400 transition-colors cursor-pointer rounded border border-transparent hover:border-red-500/20"
                >
                  <Trash2 size={10} />
                  Clear all
                </button>
              </div>
            )}

            <div className="flex flex-col gap-1 overflow-y-auto flex-1 min-h-0">
              {history.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-12 text-white/20">
                  <Clock size={24} className="mb-3 opacity-40" />
                  <p className="text-xs">No request history yet</p>
                  <p className="text-[10px] text-white/10 mt-1">Send a request to see it here</p>
                </div>
              ) : (
                history.map((entry) => {
                  const mc = METHOD_COLORS[entry.method] ?? "#94a3b8";
                  const statusOk = entry.status != null && entry.status < 300;
                  const statusWarn = entry.status != null && entry.status >= 300 && entry.status < 500;

                  return (
                    <button
                      key={entry.id}
                      onClick={() => handleReplay(entry)}
                      className="flex flex-col gap-1 p-2.5 rounded-lg border border-white/5 hover:border-white/10 hover:bg-white/3 transition-all cursor-pointer text-left group"
                    >
                      <div className="flex items-center gap-2">
                        <span
                          className="text-[10px] font-bold shrink-0 px-1.5 py-0.5 rounded"
                          style={{ color: mc, background: `${mc}15` }}
                        >
                          {entry.method}
                        </span>
                        <span className="text-[11px] text-white/60 truncate flex-1 font-mono">
                          {entry.url}
                        </span>
                        <ArrowRight
                          size={10}
                          className="text-white/0 group-hover:text-white/30 transition-colors shrink-0"
                        />
                      </div>
                      <div className="flex items-center gap-3 text-[9px] text-white/25 pl-1">
                        {entry.status != null && (
                          <span
                            className="font-mono"
                            style={{
                              color: statusOk ? "#4ade80" : statusWarn ? "#fb923c" : "#f87171",
                            }}
                          >
                            {entry.status}
                          </span>
                        )}
                        {entry.statusText && (
                          <span className="text-white/15">{entry.statusText}</span>
                        )}
                        {entry.durationMs != null && (
                          <span className="font-mono">{entry.durationMs}ms</span>
                        )}
                        {entry.sizeBytes != null && entry.sizeBytes > 0 && (
                          <span className="font-mono">
                            {entry.sizeBytes < 1024 ? `${entry.sizeBytes} B` : `${(entry.sizeBytes / 1024).toFixed(1)} KB`}
                          </span>
                        )}
                        <span className="ml-auto">{formatTimestamp(entry.timestamp)}</span>
                      </div>
                    </button>
                  );
                })
              )}
            </div>
          </div>

          <ModalActions
            onCancel={() => setIsOpen(false)}
            onConfirm={() => setIsOpen(false)}
            confirmLabel="Close"
          />
        </ModalShell>
      )}
    </>
  );
}
