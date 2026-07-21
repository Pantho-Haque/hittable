"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { useWorkspace } from "@/context/workspaceContext";
import NoteEditor from "@/components/notes/NoteEditor";

export default function MarkdownEditor() {
  const { rawTextContent, updateRawTextContent, saveRawTextContent, isFileLoaded, activeFile } = useWorkspace();
  const [localContent, setLocalContent] = useState("");
  const [isUnsaved, setIsUnsaved] = useState(false);
  const hasLoadedRef = useRef(false);

  /* eslint-disable react-hooks/set-state-in-effect -- syncing from loaded file */
  useEffect(() => {
    if (isFileLoaded) {
      setLocalContent(rawTextContent);
      hasLoadedRef.current = true;
      setIsUnsaved(false);
    }
  }, [isFileLoaded, rawTextContent]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleChange = useCallback((value: string) => {
    setLocalContent(value);
    setIsUnsaved(true);
    updateRawTextContent(value);
    if (hasLoadedRef.current) {
      saveRawTextContent();
    }
  }, [updateRawTextContent, saveRawTextContent]);

  const selectedTitle = activeFile?.path[activeFile.path.length - 1] ?? "Note";

  return (
    <div className="flex flex-col h-full bg-[#0a1628] p-3">
      <NoteEditor
        selectedId={activeFile?.path.join("/") ?? null}
        selectedTitle={selectedTitle}
        content={localContent}
        isUnsaved={isUnsaved}
        onChange={handleChange}
      />
    </div>
  );
}
