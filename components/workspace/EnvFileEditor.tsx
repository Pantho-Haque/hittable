"use client";

import { useState, useCallback, useEffect } from "react";
import { Plus, Trash2 } from "lucide-react";
import { TEnvFile } from "@/types";

type EnvFileEditorProps = {
  initialContent: TEnvFile;
  onSave: (content: TEnvFile) => void;
};

export default function EnvFileEditor({ initialContent, onSave }: EnvFileEditorProps) {
  const [entries, setEntries] = useState<[string, string][]>(
    Object.entries(initialContent)
  );
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    setEntries(Object.entries(initialContent));
  }, [initialContent]);

  const handleKeyChange = useCallback((index: number, value: string) => {
    setEntries((prev) =>
      prev.map((entry, i) => (i === index ? [value, entry[1]] : entry))
    );
  }, []);

  const handleValueChange = useCallback((index: number, value: string) => {
    setEntries((prev) =>
      prev.map((entry, i) => (i === index ? [entry[0], value] : entry))
    );
  }, []);

  const handleAdd = useCallback(() => {
    setEntries((prev) => [...prev, ["", ""]]);
  }, []);

  const handleRemove = useCallback((index: number) => {
    setEntries((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const handleSave = useCallback(() => {
    const env: TEnvFile = {};
    entries.forEach(([key, value]) => {
      if (key.trim()) {
        env[key] = value;
      }
    });
    setIsSaving(true);
    onSave(env);
    setTimeout(() => setIsSaving(false), 1000);
  }, [entries, onSave]);

  return (
    <div className="flex flex-col h-full bg-[#0a1628]">
      <div className="flex items-center justify-between px-4 py-2 border-b border-white/5">
        <span className="text-xs font-semibold text-white/70 tracking-wide uppercase">
          Environment Variables
        </span>
        <div className="flex items-center gap-2">
          <button
            onClick={handleAdd}
            className="p-1.5 rounded hover:bg-white/5 text-white/40 hover:text-white/70 transition-colors"
            title="Add variable"
          >
            <Plus className="w-4 h-4" />
          </button>
          <button
            onClick={handleSave}
            className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
              isSaving
                ? "bg-emerald-400/10 text-emerald-400"
                : "bg-cyan-400/10 text-cyan-400 hover:bg-cyan-400/20"
            }`}
          >
            {isSaving ? "Saved" : "Save"}
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-auto p-4">
        <div className="space-y-2">
          {entries.map(([key, value], index) => (
            <div key={index} className="flex items-center gap-2">
              <input
                type="text"
                value={key}
                onChange={(e) => handleKeyChange(index, e.target.value)}
                placeholder="KEY"
                className="flex-1 px-3 py-2 bg-[#0e1f35] border border-white/10 rounded text-xs font-mono text-white/80 placeholder-white/20 focus:outline-none focus:border-cyan-400/50"
              />
              <input
                type="text"
                value={value}
                onChange={(e) => handleValueChange(index, e.target.value)}
                placeholder="value"
                className="flex-1 px-3 py-2 bg-[#0e1f35] border border-white/10 rounded text-xs font-mono text-white/80 placeholder-white/20 focus:outline-none focus:border-cyan-400/50"
              />
              <button
                onClick={() => handleRemove(index)}
                className="p-1.5 rounded hover:bg-red-400/10 text-white/20 hover:text-red-400 transition-colors"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}
        </div>

        {entries.length === 0 && (
          <div className="text-center py-8 text-white/30 text-xs">
            No environment variables defined.
          </div>
        )}
      </div>

      <div className="px-4 py-2 border-t border-white/5 text-[10px] text-white/20">
        Use <code className="px-1 py-0.5 bg-white/5 rounded">{"<<KEY>>"}</code> in
        your requests to reference these values.
      </div>
    </div>
  );
}
