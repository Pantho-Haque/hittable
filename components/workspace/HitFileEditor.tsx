"use client";

import { useCallback, useState, useEffect, useRef, useMemo } from "react";
import { useWorkspace } from "@/context/workspaceContext";
import { THitFileContent, THittableCurlJson, TResponseJson, THittableEnv } from "@/types";
import { useExtension } from "@/hooks/useExtension";
import UrlBar from "@/components/RequestForm/UrlBar";
import TabEditor from "@/components/RequestForm/TabEditor";
import ResponsePanel from "@/components/RequestForm/ResponsePanel";
import { DataSource } from "@/types/workspace";
import { parseHitFile, serializeHitFile } from "@/utils/workspace/hitFileParser";
import SourceEditor from "./SourceEditor";

export default function HitFileEditor() {
  const { rawTextContent, updateRawTextContent, saveRawTextContent, isFileLoaded, activeFile, envContent } = useWorkspace();
  const { available: extensionAvailable, checked: extensionChecked } = useExtension();
  const [viewMode, setViewMode] = useState<"runner" | "text">("text");

  const parsedContent = useMemo(() => {
    try {
      JSON.parse(rawTextContent);
      return parseHitFile(rawTextContent);
    } catch {
      return null;
    }
  }, [rawTextContent]);

  const parseError = rawTextContent && !parsedContent
    ? "This file isn't valid Hittable JSON right now — fix it in Text mode first"
    : null;

  const [formInput, setFormInput] = useState<THittableCurlJson>({
    method: "GET",
    url: "",
    headers: "{}",
    body: "",
    params: "{}",
  });

  const [proxyResponse, setProxyResponse] = useState<TResponseJson>(null);

  const selectorResponse = {
    collectionName: "",
    folderPath: [] as string[],
    curlName: activeFile?.path[activeFile.path.length - 1] ?? "",
    env: envContent as THittableEnv,
    secrets: {} as THittableEnv,
    curlJson: formInput,
    responseJson: proxyResponse,
  };

  // What the file held when the Runner last matched it: saving the same thing
  // again is skipped, so opening a file or switching views never rewrites it.
  const lastSerializedRef = useRef("");

  // The Runner is filled from the file only when a file opens or the user
  // switches to it, never while typing in it: re-reading after each save would
  // pretty-print the tab under the cursor.
  const loadRunner = useCallback((content: THitFileContent) => {
    setFormInput({
      method: content.method,
      url: content.url,
      headers: JSON.stringify(content.headers, null, 2),
      body: content.body,
      params: JSON.stringify(content.params, null, 2),
    });
    setProxyResponse(content.response ?? null);
    lastSerializedRef.current = serializeHitFile({ ...content, response: content.response ?? null });
  }, []);

  // A file that parses opens in the Runner, like the terminal app; one that
  // doesn't opens in Text so it can be fixed.
  useEffect(() => {
    if (!isFileLoaded) return;
    if (parsedContent) loadRunner(parsedContent);
    setViewMode(parsedContent ? "runner" : "text");
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once per opened file
  }, [isFileLoaded, activeFile]);

  useEffect(() => {
    if (!isFileLoaded || viewMode !== "runner") return;
    // Half-typed JSON in Params or Headers is not saved: the file keeps its
    // last valid value instead of having them replaced with {}.
    let headers: Record<string, string>;
    let params: Record<string, string>;
    try {
      headers = JSON.parse(formInput.headers || "{}");
      params = JSON.parse(formInput.params || "{}");
    } catch {
      return;
    }
    const serialized = serializeHitFile({ method: formInput.method, url: formInput.url, headers, params, body: formInput.body, response: proxyResponse });
    if (serialized === lastSerializedRef.current) return;
    lastSerializedRef.current = serialized;
    updateRawTextContent(serialized);
    saveRawTextContent(serialized);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only save when form/response changes, not on every render
  }, [formInput, proxyResponse]);

  useEffect(() => {
    if (!isFileLoaded || viewMode !== "text") return;
    saveRawTextContent();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only save when content changes in text mode
  }, [rawTextContent]);

  const isUnsaved = useCallback(() => {
    if (!parsedContent) return false;
    return (
      parsedContent.method !== formInput.method ||
      parsedContent.url !== formInput.url ||
      JSON.stringify(parsedContent.headers) !== formInput.headers ||
      parsedContent.body !== formInput.body ||
      JSON.stringify(parsedContent.params) !== formInput.params
    );
  }, [formInput, parsedContent]);

  const handleSaveCollection = useCallback(() => {
    const content: THitFileContent = {
      method: formInput.method,
      url: formInput.url,
      headers: (() => {
        try { return JSON.parse(formInput.headers || "{}"); } catch { return {}; }
      })(),
      params: (() => {
        try { return JSON.parse(formInput.params || "{}"); } catch { return {}; }
      })(),
      body: formInput.body,
      response: proxyResponse,
    };
    const serialized = serializeHitFile(content);
    updateRawTextContent(serialized);
    saveRawTextContent(serialized);
  }, [formInput, proxyResponse, updateRawTextContent, saveRawTextContent]);

  const handleRevert = useCallback(() => {
    if (!parsedContent) return;
    setFormInput({
      method: parsedContent.method,
      url: parsedContent.url,
      headers: JSON.stringify(parsedContent.headers, null, 2),
      body: parsedContent.body,
      params: JSON.stringify(parsedContent.params, null, 2),
    });
    setProxyResponse(parsedContent.response ?? null);
  }, [parsedContent]);

  const dataSource: DataSource = {
    formInput,
    setFormInput,
    proxyResponse,
    setProxyResponse,
    selectorResponse,
    extensionAvailable,
    extensionChecked,
    isUnsaved,
    handleSaveCollection,
    handleRevert,
    collections: [],
    setCollections: () => {},
    history: [],
    setHistory: () => {},
    setSelectorResponse: () => {},
  };

  return (
    <div className="flex flex-col h-full">
      <div className="flex shrink-0 items-center gap-1 px-3 py-2 border-b border-white/10 bg-[#101c2d]">
        <button
          aria-pressed={viewMode === "text"}
          onClick={() => setViewMode("text")}
          className="workspace-button"
        >
          Text
        </button>
        <button
          aria-pressed={viewMode === "runner"}
          onClick={() => {
            if (parsedContent) loadRunner(parsedContent);
            setViewMode("runner");
          }}
          className="workspace-button"
        >
          Runner
        </button>
      </div>

      {viewMode === "text" ? (
        <SourceEditor key={activeFile?.path.join("/")} path={activeFile?.path ?? []}
          value={rawTextContent} onChange={updateRawTextContent}
          onSave={saveRawTextContent} disabled={!isFileLoaded} />
      ) : (
        <div className="flex-1 min-h-0 flex flex-col">
          {parseError ? (
            <div className="flex-1 flex items-center justify-center p-8">
              <div className="text-center space-y-2">
                <p className="text-sm text-amber-400/80">{parseError}</p>
                <button
                  onClick={() => setViewMode("text")}
                  className="text-xs text-cyan-400/70 hover:text-cyan-400 transition-colors"
                >
                  Switch to Text mode to fix
                </button>
              </div>
            </div>
          ) : (
            <div className="flex-1 min-h-0 flex flex-col gap-2 p-2">
              <UrlBar dataSource={dataSource} error={null} />
              <TabEditor dataSource={dataSource} setError={() => {}} />
              <div className="flex-1 min-h-0">
                <ResponsePanel dataSource={dataSource} />
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
