"use client";
import { formatJson } from "@/utils/formatJson";
import { AlertCircle, MessageCircleWarning } from "lucide-react";
import { useEffect, useState } from "react";
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
    extensionAvailable,
    extensionChecked,
    setFormInput,
    setProxyResponse,
    isUnsaved,
  } = useDataContext();

  const { collectionName, curlName, curlJson, responseJson } =
    selectorResponse!;

  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const temp = () => {
      setFormInput({
        ...curlJson,
        body: formatJson(curlJson.body).output,
        headers: formatJson(curlJson.headers).output,
      });
      setError(null);
      setProxyResponse(responseJson ?? null);
    };
    temp();
  }, [curlJson, responseJson, setFormInput, setProxyResponse]);

  return (
    <div className="h-full w-full flex flex-col gap-4 font-mono">
      <NoExtensionModal
        checked={extensionChecked}
        available={extensionAvailable}
      />

      <div className="flex flex-col md:flex-row justify-between items-center">
        <div className="flex justify-start items-center gap-2 h-3 w-full">
          <span className="text-[8px] md:text-[9px]  uppercase text-cyan-500/40">
            {collectionName}
          </span>
          <span className="text-white/10 text-xs">/</span>
          <span className="text-[8px] md:text-[9px] uppercase text-cyan-500/70">
            {curlName}
          </span>
        </div>
        {(isUnsaved() || error) && (
          <div className="flex items-center w-full justify-start md:justify-end gap-2">
            {isUnsaved() && !error && (
              <span className="flex items-center gap-1.5 rounded-md bg-amber-500/10 px-2.5 py-1 text-[8px] md:text-[10px] font-semibold text-amber-400 border border-amber-500/20">
                <MessageCircleWarning className="h-3 w-3" strokeWidth={2.5} />
                Unsaved · Ctrl/Cmd+S
              </span>
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
