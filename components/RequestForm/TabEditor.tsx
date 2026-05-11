"use client";

import { formatJson } from "@/utils/formatJson";
import { Dispatch, SetStateAction, useState } from "react";
import { modifyUrlForNewParams } from "@/utils/responsePanelUtils";
import { useDataContext } from "@/context/dataContext";

export default function TabEditor({
  setError,
}: {
  setError: Dispatch<SetStateAction<string | null>>;
}) {

  const { formInput, setFormInput } = useDataContext();

  const [activeTab, setActiveTab] = useState<"params" | "body" | "headers">(
    "params",
  );

  return (
    <div
      className="flex flex-col rounded-lg border border-white/8 bg-[#0a1628]/60 overflow-hidden"
      style={{ minHeight: 240 }}
    >
      <div className="flex items-center border-b border-white/5 bg-[#0e1f35]/50 px-1 pt-1 shrink-0">
        {(["params", "body", "headers"] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className="relative px-2 py-1 md:px-4 md:py-2 text-[8px] md:text-[10px] font-semibold tracking-[0.2em] uppercase transition-colors cursor-pointer"
            style={{
              color: activeTab === tab ? "#00e5cc" : "rgba(255,255,255,0.25)",
            }}
          >
            {tab.charAt(0).toUpperCase() + tab.slice(1)}
            {activeTab === tab && (
              <span className="absolute bottom-0 left-3 right-3 h-px bg-cyan-400" />
            )}
          </button>
        ))}
      </div>

      <textarea
        key={activeTab}
        className="flex-1 w-full resize-none bg-transparent p-2 md:p-4 text-[10px] md:text-[12px] text-white/70 outline-none placeholder-white/15 leading-relaxed"
        style={{ minHeight: 200 }}
        spellCheck={false}
        value={formInput[activeTab]}
        placeholder={
          activeTab === "body"
            ? '{\n  "key": "value"\n}'
            : '{\n  "Authorization": "Bearer ..."\n}'
        }
        onChange={(e) => {
          const val = e.target.value;
          const { output, error: jsonErr } = formatJson(val);
          setError(jsonErr);
          if(activeTab == "params")
          {
            const newUrl = modifyUrlForNewParams(formInput.url , output);
            setFormInput((prev) => ({ ...prev, url:newUrl , [activeTab]: output }));
          }else{
            setFormInput((prev) => ({ ...prev, [activeTab]: output }));
          }
        }}
      />
    </div>
  );
}
