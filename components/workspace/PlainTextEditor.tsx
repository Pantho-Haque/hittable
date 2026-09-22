"use client";

import { useWorkspace } from "@/context/workspaceContext";
import SourceEditor from "./SourceEditor";

export default function PlainTextEditor() {
  const { rawTextContent, updateRawTextContent, saveRawTextContent, isFileLoaded, activeFile } = useWorkspace();

  return (
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
}
