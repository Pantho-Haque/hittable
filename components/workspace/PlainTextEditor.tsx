"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { useWorkspace } from "@/context/workspaceContext";

export default function PlainTextEditor() {
  const { rawTextContent, updateRawTextContent, saveRawTextContent, isFileLoaded, activeFile } = useWorkspace();
  const [localContent, setLocalContent] = useState(rawTextContent);
  const hasLoadedRef = useRef(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const lineNumbersRef = useRef<HTMLDivElement>(null);

  /* eslint-disable react-hooks/set-state-in-effect -- syncing local state from loaded file content */
  useEffect(() => {
    if (isFileLoaded) {
      setLocalContent(rawTextContent);
      hasLoadedRef.current = true;
    }
  }, [isFileLoaded, rawTextContent]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const lineCount = Math.max(localContent.split("\n").length, 1);

  const syncScroll = useCallback(() => {
    if (textareaRef.current && lineNumbersRef.current) {
      lineNumbersRef.current.scrollTop = textareaRef.current.scrollTop;
    }
  }, []);

  const handleChange = useCallback((value: string) => {
    setLocalContent(value);
    updateRawTextContent(value);
    if (hasLoadedRef.current) {
      saveRawTextContent();
    }
  }, [updateRawTextContent, saveRawTextContent]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === "s") {
      e.preventDefault();
      saveRawTextContent();
    }
  }, [saveRawTextContent]);

  const fileName = activeFile?.path[activeFile.path.length - 1] ?? "file";

  return (
    <div className="flex flex-col h-full bg-[#0a1628]">
      <div className="flex items-center justify-between px-4 py-2 border-b border-white/5">
        <span className="text-xs font-mono text-white/50 truncate">{fileName}</span>
      </div>

      <div className="flex-1 min-h-0 overflow-hidden flex">
        <div
          ref={lineNumbersRef}
          className="w-12 bg-[#0e1f35] border-r border-white/5 overflow-hidden select-none py-2 text-right shrink-0"
        >
          {Array.from({ length: lineCount }, (_, i) => (
            <div key={i} className="px-2 text-[11px] text-white/20 leading-5">
              {i + 1}
            </div>
          ))}
        </div>

        <textarea
          ref={textareaRef}
          value={localContent}
          onChange={(e) => handleChange(e.target.value)}
          onScroll={syncScroll}
          onKeyDown={handleKeyDown}
          className="flex-1 bg-transparent px-4 py-2 text-[13px] font-mono text-white/60 leading-5 resize-none focus:outline-none placeholder-white/20"
          placeholder="Start typing..."
          spellCheck={false}
        />
      </div>
    </div>
  );
}
