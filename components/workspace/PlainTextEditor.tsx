"use client";

import { useEffect, useMemo, useState } from "react";
import { useWorkspace } from "@/context/workspaceContext";
import { isSvg } from "@/utils/workspace/fileKind";
import SourceEditor from "./SourceEditor";

export default function PlainTextEditor() {
  const { rawTextContent, updateRawTextContent, saveRawTextContent, isFileLoaded, activeFile } = useWorkspace();
  const name = activeFile?.path[activeFile.path.length - 1] ?? "";
  const svg = isSvg(name);
  const [preview, setPreview] = useState(false);

  // An SVG is text, so it edits as code; Preview draws it through <img>,
  // which never runs the scripts an SVG can carry.
  const svgUrl = useMemo(
    () => (svg && preview ? URL.createObjectURL(new Blob([rawTextContent], { type: "image/svg+xml" })) : null),
    [svg, preview, rawTextContent],
  );
  useEffect(() => () => {
    if (svgUrl) URL.revokeObjectURL(svgUrl);
  }, [svgUrl]);

  const editor = (
    <SourceEditor
      key={activeFile?.path.join("/")}
      path={activeFile?.path ?? []}
      value={rawTextContent}
      disabled={!isFileLoaded}
      onChange={(value) => {
        updateRawTextContent(value);
        saveRawTextContent(value);
      }}
      onSave={saveRawTextContent}
    />
  );
  if (!svg) return editor;

  return (
    <div className="flex h-full flex-col">
      <div className="flex shrink-0 items-center gap-1 px-3 py-2 border-b border-white/10 bg-[#101c2d]">
        <button aria-pressed={!preview} onClick={() => setPreview(false)} className="workspace-button">
          Code
        </button>
        <button aria-pressed={preview} onClick={() => setPreview(true)} className="workspace-button">
          Preview
        </button>
      </div>
      {preview && svgUrl ? (
        <div className="flex flex-1 min-h-0 items-center justify-center overflow-auto p-4">
          {/* eslint-disable-next-line @next/next/no-img-element -- a local object URL */}
          <img src={svgUrl} alt={name} className="max-h-full max-w-full rounded bg-white/90 p-2" />
        </div>
      ) : (
        editor
      )}
    </div>
  );
}
