"use client";

import { useCallback, useState, useEffect, useRef, useMemo } from "react";
import { useWorkspace } from "@/context/workspaceContext";
import { THitFileContent, THittableCurlJson, TResponseJson, THittableEnv } from "@/types";
import { useExtension } from "@/hooks/useExtension";
import ExtensionStatus from "@/components/modals/ExtensionStatus";
import UrlBar from "@/components/RequestForm/UrlBar";
import TabEditor from "@/components/RequestForm/TabEditor";
import ResponsePanel from "@/components/RequestForm/ResponsePanel";
import { DataSource } from "@/types/workspace";
import type { WsNode } from "@/services/workspace/backend";
import { parseHitFile, serializeHitFile } from "@/utils/workspace/hitFileParser";
import SourceEditor from "./SourceEditor";
import Split from "@/components/ui/Split";
import useMediaQuery from "@/hooks/useMediaQuery";
import { useStoredFlag } from "@/utils/prefs";
import { Columns2, Rows2 } from "lucide-react";

export default function HitFileEditor() {
  const { rawTextContent, updateRawTextContent, saveRawTextContent, isFileLoaded, activeFile, envContent, readOnly: workspaceReadOnly, fileNotice, backendKind, tree, readWorkspaceFile, addUploadFile } = useWorkspace();
  // In a GitHub repository a send does not write its response into the file:
  // that would mark the request changed after every run and put response data
  // (tokens, personal data) into the next commit. The response still shows;
  // the file keeps the response it was committed with. A local folder keeps
  // today's behaviour (and the terminal app's): the last response is saved.
  const keepResponseOutOfFile = backendKind === "github";
  const storedResponseRef = useRef<TResponseJson>(null);
  const readOnly = workspaceReadOnly || !!fileNotice;
  const { available: extensionAvailable, checked: extensionChecked, capabilities } = useExtension();
  const workspaceFiles = useMemo(() => filePaths(tree), [tree]);
  const [viewMode, setViewMode] = useState<"runner" | "text">("text");
  // Response below the request (default) or beside it; a phone always stacks.
  const [responseBeside, setResponseBeside] = useStoredFlag("hittable:response-beside", false);
  const narrow = useMediaQuery("(max-width: 767px)");
  const beside = responseBeside && !narrow;

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
  // A JSON error in Params, Headers or a JSON body blocks Send, as it says.
  const [jsonError, setJsonError] = useState<string | null>(null);


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
    storedResponseRef.current = content.response ?? null;
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
    const response = keepResponseOutOfFile ? storedResponseRef.current : proxyResponse;
    const serialized = serializeHitFile({ method: formInput.method, url: formInput.url, headers, params, body: formInput.body, response });
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

  // Headers and params are compared as data: the form holds them pretty
  // printed, the file as objects, so comparing text always differed.
  const isUnsaved = useCallback(() => {
    if (!parsedContent) return false;
    return (
      parsedContent.method !== formInput.method ||
      parsedContent.url !== formInput.url ||
      !sameJson(parsedContent.headers, formInput.headers) ||
      parsedContent.body !== formInput.body ||
      !sameJson(parsedContent.params, formInput.params)
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
      response: keepResponseOutOfFile ? storedResponseRef.current : proxyResponse,
    };
    const serialized = serializeHitFile(content);
    updateRawTextContent(serialized);
    saveRawTextContent(serialized);
  }, [formInput, proxyResponse, keepResponseOutOfFile, updateRawTextContent, saveRawTextContent]);


  const dataSource: DataSource = {
    formInput,
    setFormInput,
    proxyResponse,
    setProxyResponse,
    env: envContent as THittableEnv,
    requestKey: activeFile?.path.join("/") ?? "",
    extensionAvailable,
    extensionBinary: capabilities.includes("binary"),
    hitPath: activeFile?.path,
    readFile: readWorkspaceFile,
    addUploadFile,
    workspaceFiles,
    isUnsaved,
    handleSaveCollection,
    readOnly,
  };

  return (
    <div className="flex flex-col h-full">
      <div className="flex shrink-0 items-center gap-1 px-3 py-2 border-b border-white/10 bg-(--surface-3)">
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
        <div className="ml-auto flex items-center gap-2">
          {viewMode === "runner" && !narrow && (
            <button
              type="button"
              onClick={() => setResponseBeside(!responseBeside)}
              aria-label={beside ? "Show the response below the request" : "Show the response beside the request"}
              title={beside ? "Response below" : "Response beside"}
              className="workspace-button"
            >
              {beside ? <Rows2 size={15} aria-hidden="true" /> : <Columns2 size={15} aria-hidden="true" />}
            </button>
          )}
          <ExtensionStatus checked={extensionChecked} available={extensionAvailable} />
        </div>
      </div>

      {viewMode === "text" ? (
        <SourceEditor key={activeFile?.path.join("/")} path={activeFile?.path ?? []}
          value={rawTextContent} onChange={updateRawTextContent}
          onSave={saveRawTextContent} disabled={!isFileLoaded || readOnly} />
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
              <UrlBar dataSource={dataSource} error={jsonError} />
              <Split
                id="runner"
                orientation={beside ? "horizontal" : "vertical"}
                defaultSize={0.4}
                minFirst={beside ? 240 : 96}
                minSecond={beside ? 280 : 120}
                collapsible={["first", "second"]}
                firstLabel="Request"
                secondLabel="Response"
                className="split-gap flex-1"
                first={<TabEditor dataSource={dataSource} setError={setJsonError} />}
                second={<ResponsePanel dataSource={dataSource} />}
              />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/** Whether a file's object and the form's JSON text hold the same data. */
function sameJson(stored: unknown, formText: string): boolean {
  try {
    return JSON.stringify(stored ?? {}) === JSON.stringify(JSON.parse(formText || "{}"));
  } catch {
    return false; // half-typed JSON differs from anything saved
  }
}

/** Every file path in a tree (directories the backend left unloaded are skipped). */
function filePaths(nodes: WsNode[]): string[][] {
  return nodes.flatMap((n) => (n.kind === "file" ? [n.path] : filePaths(n.children ?? [])));
}
