"use client";

import {
  createContext,
  useContext,
  useState,
  ReactNode,
  useEffect,
  useCallback,
  useRef,
} from "react";
import type { TEnvFile } from "@/types";
import {
  ENV_PATH,
  findNode,
  isReservedPath,
  readEnv,
  type FileStatus,
  type WorkspaceBackend,
  type WsNode,
} from "@/services/workspace/backend";
import { createFolderBackend } from "@/services/workspace/folderBackend";
import { pickFolder, scaffoldHittable } from "@/services/workspace/DirectoryService";
import {
  saveDirectoryHandle,
  loadDirectoryHandle,
  removeDirectoryHandle,
  verifyPermission,
} from "@/services/workspace/HandleStore";
import { serializeHitFile } from "@/utils/workspace/hitFileParser";
import { NotificationContext } from "@/context/notifyContext";
import { classifyFile, editorKind, type EditorKind, type FileView } from "@/utils/workspace/fileKind";
import { useToolListener } from "@/utils/uiEvents";

export type ActiveFile = { path: string[]; kind: EditorKind } | null;

type WorkspaceContextType = {
  /** Which kind of workspace is open, or null on the launcher. */
  backendKind: WorkspaceBackend["kind"] | null;
  /** Folder name, or owner/repo. */
  label: string;
  readOnly: boolean;
  /** path.join("/") → git status; empty for folders. */
  status: Record<string, FileStatus>;
  tree: WsNode[];
  activeFile: ActiveFile;
  rawTextContent: string;
  /** env.json with env.local.json layered on top. */
  envContent: TEnvFile;
  isRefreshing: boolean;
  isFileLoaded: boolean;
  /** What the open file is: editable text, an image, a binary or too large to edit. */
  fileView: FileView;
  /** Why the open file can't be edited although the workspace can (e.g. a GitHub workflow), or null. */
  fileNotice: string | null;
  fileSize: number;
  /** An object URL for the open image, to show it. */
  fileUrl: string | null;
  connectingStage: TConnectingStage | null;
  /** Show any backend (e.g. a GitHub repository). */
  openBackend: (backend: WorkspaceBackend) => Promise<void>;
  /** Pick a folder on disk and open it. False when cancelled or failed. */
  openFolder: () => Promise<boolean>;
  /** Reopen the last folder. `prompt` asks for permission (needs a user gesture). */
  reconnectFolder: (prompt?: boolean) => Promise<boolean>;
  closeWorkspace: () => void;
  refreshTree: () => Promise<void>;
  /** List a directory the backend left unloaded (large GitHub trees). */
  loadDirectory: (path: string[]) => Promise<void>;
  refreshEnv: () => Promise<void>;
  setActiveFile: (path: string[] | null) => void;
  updateRawTextContent: (content: string) => void;
  saveRawTextContent: (content?: string) => Promise<void> | void;
  /** A .hit file without `text` gets a blank request. Fails when the name is taken or reserved. */
  createEntry: (path: string[], kind: "file" | "directory", text?: string) => Promise<void>;
  renameEntry: (path: string[], newName: string) => Promise<void>;
  deleteEntry: (path: string[]) => Promise<void>;
  /** Bytes of any file in the workspace (a request's upload). */
  readWorkspaceFile: (path: string[]) => Promise<Uint8Array>;
  /** Copy a file from the computer into hittable/files/ under a free name; returns its path. */
  addUploadFile: (file: File) => Promise<string[]>;
} | null;

/**
 * Opening a workspace is a multi-second job (scaffolding, then a full recursive
 * walk of the folder). The stage is surfaced so the UI can say which part is
 * running rather than showing an unexplained pause.
 */
export type TConnectingStage = "preparing" | "scanning" | "reconnecting";

const CONNECTING_LABELS: Record<TConnectingStage, string> = {
  preparing: "Preparing workspace…",
  scanning: "Reading folder…",
  reconnecting: "Reconnecting to your workspace…",
};

export function connectingLabel(stage: TConnectingStage): string {
  return CONNECTING_LABELS[stage];
}

const BLANK_HIT = serializeHitFile({ method: "GET", url: "", headers: {}, params: {}, body: "", response: null });

const key = (path: string[]) => path.join("/");
/** Where files picked from the computer for an upload are copied. */
export const UPLOADS_DIR = ["hittable", "files"];
const within = (path: string[], root: string[]) => key(path) === key(root) || key(path).startsWith(key(root) + "/");

function writable(b: WorkspaceBackend | null) {
  if (!b) throw new Error("No workspace is open");
  if (b.readOnly) throw new Error("This workspace is read-only");
  return b;
}

function refuseReserved(path: string[]) {
  if (isReservedPath(path)) throw new Error(`"${path[path.length - 1]}" is a reserved name`);
}

const WorkspaceContext = createContext<WorkspaceContextType>(null);

/** A picker or file error as a sentence the user can act on. */
function errorText(err: unknown): string {
  const e = err as Error;
  if (e?.name === "NotAllowedError" || e?.name === "SecurityError") return "The browser didn't allow access to that folder. Try again, or pick another folder.";
  if (e?.name === "NotFoundError") return "The folder is no longer there.";
  return e?.message || String(err);
}

export const WorkspaceProvider = ({ children }: { children: ReactNode }) => {
  // Optional: unit tests mount the workspace without the toast layer.
  const notify = useContext(NotificationContext)?.notify;
  const notifyRef = useRef(notify);
  useEffect(() => { notifyRef.current = notify; });
  const [backend, setBackend] = useState<WorkspaceBackend | null>(null);
  const [tree, setTree] = useState<WsNode[]>([]);
  const [status, setStatus] = useState<Record<string, FileStatus>>({});
  const [activeFile, setActiveFileState] = useState<ActiveFile>(null);
  const [rawTextContent, setRawTextContent] = useState("");
  const [envContent, setEnvContent] = useState<TEnvFile>({});
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isFileLoaded, setIsFileLoaded] = useState(false);
  const [fileView, setFileView] = useState<FileView>({ kind: "text" });
  const fileViewRef = useRef<FileView>({ kind: "text" });
  const [fileNotice, setFileNotice] = useState<string | null>(null);
  const fileNoticeRef = useRef<string | null>(null);
  const [fileSize, setFileSize] = useState(0);
  const [fileUrl, setFileUrl] = useState<string | null>(null);
  const [connectingStage, setConnectingStage] = useState<TConnectingStage | null>(null);

  const backendRef = useRef<WorkspaceBackend | null>(null);
  const unsubscribeRef = useRef<(() => void) | undefined>(undefined);
  const hasLoadedRef = useRef(false);
  const activeFileRef = useRef<ActiveFile>(null);
  const writeQueueRef = useRef<Promise<void>>(Promise.resolve());
  // The newest unwritten content per file. A burst of keystrokes becomes one
  // write of the latest text, not one full-file write per key.
  const pendingWritesRef = useRef(new Map<string, string>());

  activeFileRef.current = activeFile;

  const setActiveFile = useCallback((path: string[] | null) => {
    setIsFileLoaded(false);
    hasLoadedRef.current = false;
    setActiveFileState(path ? { path, kind: editorKind(path[path.length - 1]) } : null);
  }, []);

  const syncStatus = useCallback((b: WorkspaceBackend) => {
    if (backendRef.current === b) setStatus(b.status?.() ?? {});
  }, []);

  const refreshTree = useCallback(async () => {
    const b = backendRef.current;
    if (!b) return;
    setIsRefreshing(true);
    try {
      const next = await b.listTree();
      if (backendRef.current === b) setTree(next);
      syncStatus(b);
    } finally {
      setIsRefreshing(false);
    }
  }, [syncStatus]);

  const loadDirectory = useCallback(async (path: string[]) => {
    const b = backendRef.current;
    const node = findNode(tree, path);
    if (!b?.loadDirectory || node?.kind !== "directory" || node.children) return;
    try {
      await b.loadDirectory(path);
      await refreshTree();
    } catch (err) {
      console.error("Could not list the folder:", err);
    }
  }, [tree, refreshTree]);

  const refreshEnv = useCallback(async () => {
    const b = backendRef.current;
    if (!b) return;
    const env = await readEnv(b);
    if (backendRef.current === b) setEnvContent(env);
  }, []);

  const reset = useCallback((b: WorkspaceBackend | null) => {
    unsubscribeRef.current?.();
    unsubscribeRef.current = undefined;
    backendRef.current = b;
    pendingWritesRef.current.clear();
    setBackend(b);
    setTree([]);
    setStatus({});
    setEnvContent({});
    setActiveFile(null);
  }, [setActiveFile]);

  const openBackend = useCallback(async (b: WorkspaceBackend) => {
    reset(b);
    const [nextTree, env] = await Promise.all([b.listTree(), readEnv(b)]);
    if (backendRef.current !== b) return;
    setTree(nextTree);
    setEnvContent(env);
    syncStatus(b);
    unsubscribeRef.current = b.subscribe?.(() => {
      syncStatus(b);
      void refreshTree();
    });
  }, [reset, syncStatus, refreshTree]);

  useEffect(() => () => unsubscribeRef.current?.(), []);

  const openFolderHandle = useCallback(async (handle: FileSystemDirectoryHandle, firstStage: TConnectingStage) => {
    setConnectingStage(firstStage);
    try {
      await scaffoldHittable(handle);
      await saveDirectoryHandle(handle);
      setConnectingStage((s) => (s === "preparing" ? "scanning" : s));
      await openBackend(createFolderBackend(handle));
      return true;
    } catch (err) {
      reset(null);
      notifyRef.current?.({ mode: "error", title: "Couldn't open the folder", desc: errorText(err), timeout: 6000 });
      return false;
    } finally {
      setConnectingStage(null);
    }
  }, [openBackend, reset]);

  const pickingRef = useRef(false);
  const openFolder = useCallback(async () => {
    // The OS picker is modal, so the loader only covers the work that follows
    // it — otherwise a cancelled picker would leave a spinner on screen.
    // A second click while it is open would throw "File picker already active".
    if (pickingRef.current) return false;
    pickingRef.current = true;
    let handle: FileSystemDirectoryHandle;
    try {
      handle = await pickFolder();
    } catch (err) {
      const name = (err as Error).name;
      if (name !== "AbortError" && !/already active/i.test((err as Error).message ?? "")) {
        notifyRef.current?.({ mode: "error", title: "Couldn't open the folder", desc: errorText(err), timeout: 6000 });
      }
      return false;
    } finally {
      pickingRef.current = false;
    }
    return openFolderHandle(handle, "preparing");
  }, [openFolderHandle]);

  const reconnectFolder = useCallback(async (prompt = false) => {
    const handle = await loadDirectoryHandle();
    if (!handle) return false;
    if (!(await verifyPermission(handle, prompt))) {
      // Refused when asked: forget it. Not asked yet: keep it for the launcher's Reconnect.
      if (prompt) await removeDirectoryHandle();
      return false;
    }
    return openFolderHandle(handle, "reconnecting");
  }, [openFolderHandle]);

  const closeWorkspace = useCallback(() => {
    if (backendRef.current?.kind === "folder") void removeDirectoryHandle();
    reset(null);
  }, [reset]);

  useEffect(() => {
    if (!activeFile || !backend) {
      fileNoticeRef.current = null;
      setFileNotice(null);
      setRawTextContent("");
      setIsFileLoaded(false);
      hasLoadedRef.current = false;
      return;
    }

    let cancelled = false;
    hasLoadedRef.current = false;
    setIsFileLoaded(false);

    let url: string | null = null;
    const loadFile = async () => {
      const notice = () => {
        fileNoticeRef.current = backend.readOnlyReason?.(activeFile.path) ?? null;
        setFileNotice(fileNoticeRef.current);
      };
      fileNoticeRef.current = null;
      try {
        const blob = new Blob([(await backend.readFile(activeFile.path)) as Uint8Array<ArrayBuffer>]);
        const view = await classifyFile(activeFile.path[activeFile.path.length - 1], blob);
        if (cancelled) return;
        fileViewRef.current = view;
        setFileView(view);
        setFileSize(blob.size);
        if (view.kind === "image") url = URL.createObjectURL(blob);
        setFileUrl(url);
        // Images, binaries and huge files never enter the editor.
        const content = view.kind === "text" ? await blob.text() : "";
        if (cancelled) return;
        notice();
        setRawTextContent(content);
      } catch (err) {
        if (cancelled) return;
        console.error("Failed to load file:", err);
        // Never let an empty editor overwrite a file that failed to load.
        notice();
        fileNoticeRef.current ??= "This file could not be loaded.";
        setFileNotice(fileNoticeRef.current);
        setRawTextContent("");
      }
      setIsFileLoaded(true);
      hasLoadedRef.current = true;
    };
    loadFile();

    return () => {
      cancelled = true;
      if (url) URL.revokeObjectURL(url);
    };
  }, [activeFile, backend]);

  const updateRawTextContent = useCallback((content: string) => {
    setRawTextContent(content);
  }, []);

  const saveRawTextContent = useCallback((content = rawTextContent) => {
    const file = activeFileRef.current;
    const b = backendRef.current;
    if (!file || !b || b.readOnly || fileNoticeRef.current || !hasLoadedRef.current) return;
    // Never write back a file the editor did not load as text.
    if (fileViewRef.current.kind !== "text") return;
    // Capture the destination before awaiting, and serialize writes instead of
    // dropping keystrokes that arrive while a previous write is in flight.
    const pending = pendingWritesRef.current;
    const k = key(file.path);
    const queued = pending.has(k);
    pending.set(k, content);
    if (queued) return writeQueueRef.current; // the queued write will carry this text
    writeQueueRef.current = writeQueueRef.current.then(async () => {
      const latest = pending.get(k);
      pending.delete(k);
      if (latest === undefined || backendRef.current !== b) return;
      await b.writeFile(file.path, latest);
      syncStatus(b);
      // env.local.json layers over env.json, so saving either re-reads both.
      const name = file.path[file.path.length - 1];
      if (file.kind === "env" || name === "env.local.json") await refreshEnv();
    }).catch((err) => {
      console.error("Failed to save file:", err);
    });
    return writeQueueRef.current;
  }, [rawTextContent, syncStatus, refreshEnv]);

  const createEntry = useCallback(async (path: string[], kind: "file" | "directory", text?: string) => {
    const b = writable(backendRef.current);
    refuseReserved(path);
    if (kind === "directory") await b.createDirectory(path);
    else await b.createFile(path, text ?? (path[path.length - 1].endsWith(".hit") ? BLANK_HIT : ""));
    await refreshTree();
  }, [refreshTree]);

  const renameEntry = useCallback(async (path: string[], newName: string) => {
    const b = writable(backendRef.current);
    const to = [...path.slice(0, -1), newName];
    refuseReserved(path);
    refuseReserved(to);
    await b.rename(path, to);
    const open = activeFileRef.current;
    if (open && within(open.path, path)) setActiveFile(null);
    await refreshTree();
  }, [refreshTree, setActiveFile]);

  const deleteEntry = useCallback(async (path: string[]) => {
    const b = writable(backendRef.current);
    refuseReserved(path);
    await b.remove(path);
    const open = activeFileRef.current;
    if (open && within(open.path, path)) setActiveFile(null);
    await refreshTree();
  }, [refreshTree, setActiveFile]);

  const readWorkspaceFile = useCallback(async (path: string[]) => {
    const b = backendRef.current;
    if (!b) throw new Error("No workspace is open");
    return b.readFile(path);
  }, []);

  const addUploadFile = useCallback(async (file: File) => {
    const b = writable(backendRef.current);
    const tree = await b.listTree();
    const dot = file.name.lastIndexOf(".");
    const [stem, ext] = dot > 0 ? [file.name.slice(0, dot), file.name.slice(dot)] : [file.name, ""];
    let path = [...UPLOADS_DIR, file.name];
    for (let n = 1; findNode(tree, path); n++) path = [...UPLOADS_DIR, `${stem}-${n}${ext}`];
    await b.writeBytes(path, new Uint8Array(await file.arrayBuffer()));
    await refreshTree();
    syncStatus(b);
    return path;
  }, [refreshTree, syncStatus]);

  // The Runner's "set it in Environment" link: open hittable/env.json,
  // creating it when it is missing and the workspace can be written.
  const openEnv = useCallback(async () => {
    const b = backendRef.current;
    if (!b) return;
    try {
      if (!findNode(tree, ENV_PATH)) {
        if (b.readOnly) return;
        await b.createFile(ENV_PATH, "{}\n");
        await refreshTree();
      }
      setActiveFile(ENV_PATH);
    } catch (err) {
      console.error("Could not open env.json:", err);
    }
  }, [tree, refreshTree, setActiveFile]);
  useToolListener("env", () => void openEnv(), backend ? undefined : "");

  return (
    <WorkspaceContext.Provider
      value={{
        backendKind: backend?.kind ?? null,
        label: backend?.label ?? "",
        readOnly: backend?.readOnly ?? false,
        status,
        tree,
        activeFile,
        rawTextContent,
        envContent,
        isRefreshing,
        isFileLoaded,
        fileView,
        fileNotice,
        fileSize,
        fileUrl,
        connectingStage,
        openBackend,
        openFolder,
        reconnectFolder,
        closeWorkspace,
        refreshTree,
        loadDirectory,
        refreshEnv,
        setActiveFile,
        updateRawTextContent,
        saveRawTextContent,
        createEntry,
        renameEntry,
        deleteEntry,
        readWorkspaceFile,
        addUploadFile,
      }}
    >
      {children}
    </WorkspaceContext.Provider>
  );
};

export const useWorkspace = () => {
  const context = useContext(WorkspaceContext);
  if (!context) {
    throw new Error("useWorkspace must be used within a WorkspaceProvider");
  }
  return context;
};
