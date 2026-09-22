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
import { TDirectoryNode, TWorkspaceMode, TEnvFile } from "@/types";
import {
  pickFolder,
  scaffoldHittable,
  validateHittable,
  readEnvFile,
  readHitFile,
} from "@/services/workspace/DirectoryService";
import {
  saveDirectoryHandle,
  loadDirectoryHandle,
  removeDirectoryHandle,
  verifyPermission,
} from "@/services/workspace/HandleStore";
import { serializeHitFile } from "@/utils/workspace/hitFileParser";
import { buildTree } from "@/utils/workspace/directoryTreeHelpers";

type ActiveFileState = {
  path: string[];
  handle: FileSystemFileHandle;
  kind: "hit" | "env" | "markdown" | "text";
} | null;

type WorkspaceContextType = {
  mode: TWorkspaceMode;
  setMode: (mode: TWorkspaceMode) => void;
  directoryHandle: FileSystemDirectoryHandle | null;
  hittableDir: FileSystemDirectoryHandle | null;
  tree: TDirectoryNode[];
  activeFile: ActiveFileState;
  rawTextContent: string;
  envContent: TEnvFile;
  isRefreshing: boolean;
  isFileLoaded: boolean;
  connectingStage: TConnectingStage | null;
  openFolder: () => Promise<void>;
  disconnectFolder: () => void;
  refreshTree: () => Promise<void>;
  refreshEnv: () => Promise<void>;
  setActiveFile: (file: { path: string[]; handle: FileSystemFileHandle } | null) => void;
  updateRawTextContent: (content: string) => void;
  saveRawTextContent: (content?: string) => void;
  deleteEntry: (parentHandle: FileSystemDirectoryHandle, name: string) => Promise<void>;
  renameEntry: (parentHandle: FileSystemDirectoryHandle, oldName: string, newName: string, kind: "file" | "directory") => Promise<void>;
  createEntry: (parentHandle: FileSystemDirectoryHandle, name: string, kind: "file" | "directory") => Promise<void>;
} | null;

/**
 * Opening a workspace is a multi-second job (scaffolding, then a full recursive
 * walk of the folder). The stage is surfaced so the UI can say which part is
 * running rather than showing an unexplained pause in local mode.
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

const WorkspaceContext = createContext<WorkspaceContextType>(null);

const MODE_STORAGE_KEY = "hittable-workspace-mode";

export const WorkspaceProvider = ({ children }: { children: ReactNode }) => {
  const [mode, setModeState] = useState<TWorkspaceMode>("local");
  const [directoryHandle, setDirectoryHandle] = useState<FileSystemDirectoryHandle | null>(null);
  const [hittableDir, setHittableDir] = useState<FileSystemDirectoryHandle | null>(null);
  const [tree, setTree] = useState<TDirectoryNode[]>([]);
  const [activeFile, setActiveFileState] = useState<ActiveFileState>(null);
  const [rawTextContent, setRawTextContent] = useState("");
  const [envContent, setEnvContent] = useState<TEnvFile>({});
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isFileLoaded, setIsFileLoaded] = useState(false);
  const [connectingStage, setConnectingStage] = useState<TConnectingStage | null>(null);

  const hasLoadedRef = useRef(false);
  const activeFileRef = useRef<ActiveFileState>(null);
  const writeQueueRef = useRef<Promise<void>>(Promise.resolve());
  const hittableDirRef = useRef<FileSystemDirectoryHandle | null>(null);

  activeFileRef.current = activeFile;
  hittableDirRef.current = hittableDir;

  const setMode = useCallback((newMode: TWorkspaceMode) => {
    setModeState(newMode);
    try {
      localStorage.setItem(MODE_STORAGE_KEY, newMode);
    } catch { /* ignore */ }
  }, []);

  const getFileKind = (name: string): "hit" | "env" | "markdown" | "text" => {
    if (name.endsWith(".hit")) return "hit";
    if (name === "env" || name === "env.json") return "env";
    if (name.endsWith(".md")) return "markdown";
    return "text";
  };

  const setActiveFileWithKind = useCallback((file: { path: string[]; handle: FileSystemFileHandle } | null) => {
    if (!file) {
      setActiveFileState(null);
      setIsFileLoaded(false);
      hasLoadedRef.current = false;
      return;
    }
    const name = file.path[file.path.length - 1];
    setIsFileLoaded(false);
    hasLoadedRef.current = false;
    setActiveFileState({ ...file, kind: getFileKind(name) });
  }, []);

  const refreshTree = useCallback(async () => {
    const handle = directoryHandle || hittableDir;
    if (!handle) return;
    setIsRefreshing(true);
    try {
      const newTree = await buildTree(handle);
      setTree(newTree);
    } finally {
      setIsRefreshing(false);
    }
  }, [directoryHandle, hittableDir]);

  const refreshEnv = useCallback(async () => {
    if (!hittableDirRef.current) return;
    const env = await readEnvFile(hittableDirRef.current);
    setEnvContent(env);
  }, []);

  const openFolder = useCallback(async () => {
    // The OS picker is modal, so the loader only covers the work that follows
    // it — otherwise a cancelled picker would leave a spinner on screen.
    let handle: FileSystemDirectoryHandle;
    try {
      handle = await pickFolder();
    } catch (err) {
      if ((err as Error).name !== "AbortError") {
        console.error("Failed to open folder:", err);
      }
      return;
    }

    setConnectingStage("preparing");
    try {
      const hittable = await scaffoldHittable(handle);
      await saveDirectoryHandle(handle);
      setDirectoryHandle(handle);
      setHittableDir(hittable);
      setTree([]);
      setActiveFileState(null);
      setRawTextContent("");
      setIsFileLoaded(false);
      hasLoadedRef.current = false;
      setConnectingStage("scanning");
      const newTree = await buildTree(handle);
      setTree(newTree);
      const env = await readEnvFile(hittable);
      setEnvContent(env);
      setMode("directory");
    } catch (err) {
      console.error("Failed to open folder:", err);
    } finally {
      setConnectingStage(null);
    }
  }, [setMode]);

  const disconnectFolder = useCallback(() => {
    removeDirectoryHandle();
    setDirectoryHandle(null);
    setHittableDir(null);
    setTree([]);
    setActiveFileState(null);
    setRawTextContent("");
    setIsFileLoaded(false);
    hasLoadedRef.current = false;
    setMode("local");
  }, [setMode]);

  useEffect(() => {
    const tryReconnect = async () => {
      let savedMode: TWorkspaceMode = "local";
      try {
        savedMode = (localStorage.getItem(MODE_STORAGE_KEY) as TWorkspaceMode) || "local";
      } catch { /* ignore */ }

      if (savedMode !== "directory") return;

      const savedHandle = await loadDirectoryHandle();
      if (!savedHandle) return;
      const hasPermission = await verifyPermission(savedHandle);
      if (!hasPermission) {
        removeDirectoryHandle();
        setMode("local");
        return;
      }
      setConnectingStage("reconnecting");
      try {
        const hittable = await validateHittable(savedHandle);
        setDirectoryHandle(savedHandle);
        setHittableDir(hittable);
        const newTree = await buildTree(savedHandle);
        setTree(newTree);
        const env = await readEnvFile(hittable);
        setEnvContent(env);
        setMode("directory");
      } catch {
        removeDirectoryHandle();
        setMode("local");
      } finally {
        setConnectingStage(null);
      }
    };
    tryReconnect();
  }, [setMode]);

  useEffect(() => {
    if (!activeFile) {
      setRawTextContent("");
      setIsFileLoaded(false);
      hasLoadedRef.current = false;
      return;
    }

    let cancelled = false;
    hasLoadedRef.current = false;
    setIsFileLoaded(false);

    const loadFile = async () => {
      try {
        const content = await readHitFile(activeFile.handle);
        if (cancelled) return;
        setRawTextContent(content);
        setIsFileLoaded(true);
        hasLoadedRef.current = true;
      } catch (err) {
        if (!cancelled) {
          console.error("Failed to load file:", err);
          setRawTextContent("");
          setIsFileLoaded(true);
          hasLoadedRef.current = true;
        }
      }
    };
    loadFile();

    return () => {
      cancelled = true;
    };
  }, [activeFile]);

  const updateRawTextContent = useCallback((content: string) => {
    setRawTextContent(content);
  }, []);

  const saveRawTextContent = useCallback((content = rawTextContent) => {
    const file = activeFileRef.current;
    const workspaceDir = hittableDirRef.current;
    if (!file || !hasLoadedRef.current) return;
    // Capture the destination before awaiting, and serialize writes instead of
    // dropping keystrokes that arrive while a previous write is in flight.
    writeQueueRef.current = writeQueueRef.current.then(async () => {
      const writable = await file.handle.createWritable();
      await writable.write(content);
      await writable.close();
      if (file.kind === "env" && workspaceDir && workspaceDir === hittableDirRef.current) {
        const env = await readEnvFile(workspaceDir);
        setEnvContent(env);
      }
    }).catch((err) => {
      console.error("Failed to save file:", err);
    });
    return writeQueueRef.current;
  }, [rawTextContent]);

  const deleteEntry = useCallback(async (parentHandle: FileSystemDirectoryHandle, name: string) => {
    await parentHandle.removeEntry(name, { recursive: true });
    await refreshTree();
  }, [refreshTree]);

  const renameEntry = useCallback(async (
    parentHandle: FileSystemDirectoryHandle,
    oldName: string,
    newName: string,
    kind: "file" | "directory"
  ) => {
    const { renameEntry: rename } = await import("@/utils/workspace/fsHelpers");
    await rename(parentHandle, oldName, newName, kind);
    await refreshTree();
  }, [refreshTree]);

  const createEntry = useCallback(async (
    parentHandle: FileSystemDirectoryHandle,
    name: string,
    kind: "file" | "directory"
  ) => {
    const { createFile, createDirectory } = await import("@/utils/workspace/fsHelpers");
    if (kind === "file") {
      const ext = name.split(".").pop();
      const content = ext === "hit" ? serializeHitFile({
        method: "GET",
        url: "",
        headers: {},
        params: {},
        body: "",
        response: null,
      }) : "";
      await createFile(parentHandle, name, content);
    } else {
      await createDirectory(parentHandle, name);
    }
    await refreshTree();
  }, [refreshTree]);

  return (
    <WorkspaceContext.Provider
      value={{
        mode,
        setMode,
        directoryHandle,
        hittableDir,
        tree,
        activeFile,
        rawTextContent,
        envContent,
        isRefreshing,
        isFileLoaded,
        connectingStage,
        openFolder,
        disconnectFolder,
        refreshTree,
        refreshEnv,
        setActiveFile: setActiveFileWithKind,
        updateRawTextContent,
        saveRawTextContent,
        deleteEntry,
        renameEntry,
        createEntry,
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
