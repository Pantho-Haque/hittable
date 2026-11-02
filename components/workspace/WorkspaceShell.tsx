"use client";
import type { ReactNode } from "react";
import { Lock, PanelLeft } from "lucide-react";
import { useShortcuts } from "@/context/ShortcutKeypressProvider";
import { useWorkspace } from "@/context/workspaceContext";
import DirectoryTree from "./DirectoryTree";
import HitFileEditor from "./HitFileEditor";
import PlainTextEditor from "./PlainTextEditor";
import FileViewer from "./FileViewer";
import MarkdownEditor from "./MarkdownEditor";
import WorkspaceConnecting from "./WorkspaceConnecting";
import Split from "@/components/ui/Split";
import useMediaQuery from "@/hooks/useMediaQuery";

/**
 * The workspace chrome for any backend: a header bar (explorer toggle, then
 * `header`), the explorer, an optional `sidePanel` (e.g. Source Control) and
 * the editor for the open file. `emptyState` replaces the hint shown when no file is open.
 */
export default function WorkspaceShell({ header, sidePanel, emptyState }: { header: ReactNode; sidePanel?: ReactNode; emptyState?: ReactNode }) {
  const {
    shortcuts: { toggleSidebar },
    toggle,
  } = useShortcuts();
  const { activeFile, connectingStage, isFileLoaded, fileView, fileNotice, readOnly } = useWorkspace();
  // On a phone the source-control panel is an overlay, not a column.
  const narrow = useMediaQuery("(max-width: 767px)");

  const editor = (
    <main className="flex-1 min-w-0 h-full overflow-hidden" aria-label="File editor">
      <div className="h-full w-full flex flex-col">
        {activeFile && isFileLoaded && fileNotice && (
          <p role="note" className="flex shrink-0 items-start gap-2 border-b border-(--border) bg-(--surface-3) px-3 py-2 text-xs leading-5 text-(--muted)">
            <Lock size={14} className="mt-0.5 shrink-0 text-(--warning)" aria-hidden="true" /> {fileNotice}
          </p>
        )}
        {activeFile ? (
          !isFileLoaded ? null : fileView.kind !== "text" ? (
            <FileViewer />
          ) : activeFile.kind === "hit" ? (
            <HitFileEditor />
          ) : activeFile.kind === "markdown" ? (
            <MarkdownEditor />
          ) : (
            <PlainTextEditor />
          )
        ) : (
          emptyState ?? (
            <p className="m-auto max-w-sm p-8 text-center text-sm leading-6 text-white/60">
              Select a file in the explorer to edit a request, environment, or note.
            </p>
          )
        )}
      </div>
    </main>
  );

  return (
    <div className="workspace relative h-[calc(100dvh-44px)] w-full flex flex-col bg-(--ink-700) overflow-hidden">
      {connectingStage && <WorkspaceConnecting stage={connectingStage} />}
      <div className="flex min-h-11 shrink-0 items-center gap-3 border-b border-white/10 px-3">
        <button className="workspace-button" onClick={() => toggle("toggleSidebar")}
          aria-expanded={!toggleSidebar} aria-controls="directory-explorer" title="Toggle explorer (Ctrl/Cmd+B)">
          <PanelLeft size={16} aria-hidden="true" /> Explorer
        </button>
        {header}
        {readOnly && (
          <span className="rounded-full border border-white/15 px-2 py-0.5 text-[11px] text-white/70">Read-only</span>
        )}
      </div>
      <div className="flex min-h-0 flex-1 w-full relative">
        <Split
          id="explorer"
          orientation="horizontal"
          unit="px"
          defaultSize={256}
          minFirst={160}
          minSecond={200}
          collapsible={["first"]}
          collapsed={toggleSidebar ? "first" : null}
          // Split reports each change once, so a flip stays in step.
          onCollapsedChange={() => toggle("toggleSidebar")}
          restoreBar={false}
          firstLabel="Explorer"
          secondLabel="Editor"
          className="h-full w-full"
          first={
            <aside id="directory-explorer" aria-label="File explorer" className="h-full w-full overflow-hidden">
              <DirectoryTree />
            </aside>
          }
          second={
            sidePanel && !narrow ? (
              <Split
                id="source-control"
                orientation="horizontal"
                unit="px"
                defaultSize={288}
                minFirst={220}
                minSecond={240}
                firstLabel="Source control"
                secondLabel="Editor"
                className="h-full w-full"
                first={sidePanel}
                second={editor}
              />
            ) : (
              <>
                {sidePanel}
                {editor}
              </>
            )
          }
        />
      </div>
    </div>
  );
}
