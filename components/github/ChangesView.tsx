"use client";
import { useEffect, useState } from "react";
import { ArrowLeft, FileText, GitCompareArrows } from "lucide-react";
import Split from "@/components/ui/Split";
import useMediaQuery from "@/hooks/useMediaQuery";
import { useWorkspace } from "@/context/workspaceContext";
import { describeError } from "@/utils/github/api";
import type { GitHubBackend } from "@/services/github/githubBackend";
import { DiffBody, type DiffSides } from "./DiffView";
import SourceControlPanel, { useBackend } from "./SourceControlPanel";

/** Both sides of one file: the last commit (or theirs, when comparing with the remote) and yours. */
export async function readSides(backend: GitHubBackend, path: string, theirs: boolean): Promise<DiffSides> {
  const [original, modified, binary] = await Promise.all([
    theirs ? backend.readTheirs(path) : backend.readBase(path),
    backend.readWorking(path),
    backend.binarySides(path, theirs),
  ]);
  return {
    original,
    modified,
    binary,
    originalLabel: theirs ? `Theirs (latest on ${backend.branch})` : `Last commit (${backend.baseSha().slice(0, 7)})`,
    modifiedLabel: "Yours",
  };
}

type Loaded = { path: string; sides: DiffSides | null; error?: string };

/**
 * The Changes page: every changed file and the commit box on the left, the
 * picked file's diff on the right. It replaces the explorer and the editor
 * while open. On a phone it shows the list, then one diff at a time.
 */
export default function ChangesView({ backend, onClose }: { backend: GitHubBackend; onClose: () => void }) {
  useBackend(backend);
  const { setActiveFile } = useWorkspace();
  const narrow = useMediaQuery("(max-width: 767px)");
  const changes = backend.changes();
  const version = backend.version();
  const [picked, setPicked] = useState<string | null>(null);
  // The picked file, or the first one while it is still listed (a discard or commit drops it).
  const live = changes.some((c) => c.path === picked) ? picked : null;
  const selected = live ?? (narrow ? null : (changes[0]?.path ?? null));
  const [loaded, setLoaded] = useState<Loaded | null>(null);

  useEffect(() => {
    if (!selected) return;
    let on = true;
    // Keeps the shown diff until the new one is read, so a refresh never flashes.
    readSides(backend, selected, false).then(
      (sides) => on && setLoaded({ path: selected, sides }),
      (err) => on && setLoaded({ path: selected, sides: null, error: describeError(err) }),
    );
    return () => {
      on = false;
    };
  }, [backend, selected, version]);

  const change = changes.find((c) => c.path === selected);
  const shown = loaded?.path === selected ? loaded : null;

  const diff = (
    <section aria-label={selected ?? "Diff"} className="flex h-full min-w-0 flex-col gap-2 bg-(--ink-700) p-3">
      {selected && change ? (
        <>
          <div className="flex min-h-8 items-center gap-2">
            {narrow && (
              <button type="button" className="workspace-button" onClick={() => setPicked(null)} aria-label="Back to all changes">
                <ArrowLeft size={14} aria-hidden="true" />
              </button>
            )}
            <h2 className="min-w-0 flex-1 truncate font-mono text-xs text-(--text)" title={selected}>
              {selected}
            </h2>
            {change.status !== "D" && (
              <button
                type="button"
                className="workspace-button border border-(--border-strong)"
                title="Open the file in the editor"
                onClick={() => {
                  setActiveFile(selected.split("/"));
                  onClose();
                }}
              >
                <FileText size={14} aria-hidden="true" /> Open file
              </button>
            )}
          </div>
          <DiffBody path={selected} sides={shown?.sides ?? null} error={shown?.error} />
        </>
      ) : (
        <div className="m-auto flex max-w-sm flex-col items-center gap-2 p-8 text-center text-sm leading-6 text-(--muted)">
          <GitCompareArrows size={24} aria-hidden="true" />
          {changes.length === 0 ? "No changes to review. Edit a file and it shows up here." : "Pick a file to see its changes."}
        </div>
      )}
    </section>
  );

  const list = <SourceControlPanel backend={backend} selected={selected} onOpenDiff={setPicked} onClose={onClose} />;

  if (narrow) return <div className="h-full w-full">{selected ? diff : list}</div>;
  return (
    <Split
      id="changes"
      orientation="horizontal"
      unit="px"
      defaultSize={320}
      minFirst={240}
      minSecond={320}
      firstLabel="Changed files"
      secondLabel="Diff"
      className="h-full w-full"
      first={list}
      second={diff}
    />
  );
}
