import {
  Binary,
  BookOpen,
  Braces,
  Container,
  Database,
  File,
  FileArchive,
  FileAudio,
  FileCode2,
  FileCog,
  FileImage,
  FileJson,
  FileLock2,
  FileText,
  FileType,
  FileVideo,
  Folder,
  FolderGit2,
  FolderOpen,
  GitBranch,
  Hammer,
  Key,
  NotebookPen,
  Package,
  Scale,
  Settings,
  Sheet,
  Terminal,
  Zap,
  type LucideIcon,
} from "lucide-react";

export type FileIcon = {
  Icon: LucideIcon;
  /** Tailwind text-colour class. Kept as a class so the tree stays themeable. */
  className: string;
};

const DEFAULT_FILE: FileIcon = { Icon: File, className: "text-white/30" };

/**
 * Exact filename wins over extension: `package.json` should read as a manifest,
 * not as generic JSON. Keys are compared lowercased.
 */
const BY_NAME: Record<string, FileIcon> = {
  "package.json": { Icon: Package, className: "text-red-400/80" },
  "package-lock.json": { Icon: FileLock2, className: "text-red-400/50" },
  "pnpm-lock.yaml": { Icon: FileLock2, className: "text-amber-400/50" },
  "yarn.lock": { Icon: FileLock2, className: "text-sky-400/50" },
  "bun.lockb": { Icon: FileLock2, className: "text-amber-200/50" },
  "cargo.lock": { Icon: FileLock2, className: "text-orange-400/50" },
  "go.sum": { Icon: FileLock2, className: "text-cyan-400/50" },
  "go.mod": { Icon: FileCog, className: "text-cyan-400/80" },
  "cargo.toml": { Icon: FileCog, className: "text-orange-400/80" },
  dockerfile: { Icon: Container, className: "text-blue-400/80" },
  "docker-compose.yml": { Icon: Container, className: "text-blue-400/80" },
  "docker-compose.yaml": { Icon: Container, className: "text-blue-400/80" },
  makefile: { Icon: Hammer, className: "text-amber-500/80" },
  license: { Icon: Scale, className: "text-yellow-200/70" },
  "license.md": { Icon: Scale, className: "text-yellow-200/70" },
  "readme.md": { Icon: BookOpen, className: "text-sky-300/80" },
  readme: { Icon: BookOpen, className: "text-sky-300/80" },
  ".gitignore": { Icon: GitBranch, className: "text-orange-400/70" },
  ".gitattributes": { Icon: GitBranch, className: "text-orange-400/70" },
  ".env": { Icon: Key, className: "text-amber-400/80" },
  "env.json": { Icon: Settings, className: "text-amber-400/70" },
  env: { Icon: Settings, className: "text-amber-400/70" },
  "tsconfig.json": { Icon: FileCog, className: "text-blue-400/70" },
  "vercel.json": { Icon: FileCog, className: "text-white/60" },
};

/** Extension (without the dot, lowercased) → icon. */
const BY_EXTENSION: Record<string, FileIcon> = {
  // Hittable's own request format.
  hit: { Icon: Zap, className: "text-cyan-400/80" },

  // Web / JS
  ts: { Icon: FileCode2, className: "text-blue-400/80" },
  tsx: { Icon: FileCode2, className: "text-blue-300/80" },
  mts: { Icon: FileCode2, className: "text-blue-400/80" },
  cts: { Icon: FileCode2, className: "text-blue-400/80" },
  js: { Icon: FileCode2, className: "text-yellow-400/80" },
  jsx: { Icon: FileCode2, className: "text-yellow-300/80" },
  mjs: { Icon: FileCode2, className: "text-yellow-400/80" },
  cjs: { Icon: FileCode2, className: "text-yellow-400/80" },
  html: { Icon: FileCode2, className: "text-orange-400/80" },
  htm: { Icon: FileCode2, className: "text-orange-400/80" },
  css: { Icon: FileCode2, className: "text-sky-400/80" },
  scss: { Icon: FileCode2, className: "text-pink-400/80" },
  sass: { Icon: FileCode2, className: "text-pink-400/80" },
  less: { Icon: FileCode2, className: "text-indigo-400/80" },
  vue: { Icon: FileCode2, className: "text-emerald-400/80" },
  svelte: { Icon: FileCode2, className: "text-orange-500/80" },

  // Data / config
  json: { Icon: FileJson, className: "text-yellow-300/70" },
  jsonc: { Icon: FileJson, className: "text-yellow-300/70" },
  yaml: { Icon: FileCog, className: "text-violet-400/80" },
  yml: { Icon: FileCog, className: "text-violet-400/80" },
  toml: { Icon: FileCog, className: "text-slate-300/70" },
  ini: { Icon: FileCog, className: "text-slate-300/70" },
  conf: { Icon: FileCog, className: "text-slate-300/70" },
  cfg: { Icon: FileCog, className: "text-slate-300/70" },
  xml: { Icon: Braces, className: "text-orange-300/70" },
  csv: { Icon: Sheet, className: "text-emerald-400/70" },
  tsv: { Icon: Sheet, className: "text-emerald-400/70" },
  sql: { Icon: Database, className: "text-teal-400/80" },

  // Other languages
  py: { Icon: FileCode2, className: "text-blue-300/80" },
  go: { Icon: FileCode2, className: "text-cyan-300/80" },
  rs: { Icon: FileCode2, className: "text-orange-400/80" },
  rb: { Icon: FileCode2, className: "text-red-400/80" },
  php: { Icon: FileCode2, className: "text-indigo-300/80" },
  java: { Icon: FileCode2, className: "text-red-500/80" },
  kt: { Icon: FileCode2, className: "text-purple-400/80" },
  swift: { Icon: FileCode2, className: "text-orange-500/80" },
  c: { Icon: FileCode2, className: "text-blue-500/80" },
  h: { Icon: FileCode2, className: "text-blue-500/60" },
  cpp: { Icon: FileCode2, className: "text-blue-500/80" },
  hpp: { Icon: FileCode2, className: "text-blue-500/60" },
  cs: { Icon: FileCode2, className: "text-green-500/80" },
  lua: { Icon: FileCode2, className: "text-blue-400/80" },
  sh: { Icon: Terminal, className: "text-green-400/80" },
  bash: { Icon: Terminal, className: "text-green-400/80" },
  zsh: { Icon: Terminal, className: "text-green-400/80" },
  fish: { Icon: Terminal, className: "text-green-400/80" },

  // Docs
  md: { Icon: FileText, className: "text-emerald-400/70" },
  mdx: { Icon: FileText, className: "text-emerald-300/70" },
  txt: { Icon: FileText, className: "text-slate-300/50" },
  log: { Icon: FileText, className: "text-slate-400/50" },
  pdf: { Icon: FileType, className: "text-red-400/70" },

  // Media
  png: { Icon: FileImage, className: "text-pink-400/70" },
  jpg: { Icon: FileImage, className: "text-pink-400/70" },
  jpeg: { Icon: FileImage, className: "text-pink-400/70" },
  gif: { Icon: FileImage, className: "text-pink-400/70" },
  webp: { Icon: FileImage, className: "text-pink-400/70" },
  avif: { Icon: FileImage, className: "text-pink-400/70" },
  svg: { Icon: FileImage, className: "text-amber-300/70" },
  ico: { Icon: FileImage, className: "text-amber-300/70" },
  mp4: { Icon: FileVideo, className: "text-purple-400/70" },
  mov: { Icon: FileVideo, className: "text-purple-400/70" },
  webm: { Icon: FileVideo, className: "text-purple-400/70" },
  mp3: { Icon: FileAudio, className: "text-purple-300/70" },
  wav: { Icon: FileAudio, className: "text-purple-300/70" },
  flac: { Icon: FileAudio, className: "text-purple-300/70" },

  // Archives / binaries
  zip: { Icon: FileArchive, className: "text-amber-500/70" },
  tar: { Icon: FileArchive, className: "text-amber-500/70" },
  gz: { Icon: FileArchive, className: "text-amber-500/70" },
  rar: { Icon: FileArchive, className: "text-amber-500/70" },
  "7z": { Icon: FileArchive, className: "text-amber-500/70" },
  wasm: { Icon: Binary, className: "text-purple-400/70" },
  exe: { Icon: Binary, className: "text-slate-300/60" },
  bin: { Icon: Binary, className: "text-slate-300/60" },

  // Keys / certs
  pem: { Icon: Key, className: "text-amber-400/70" },
  key: { Icon: Key, className: "text-amber-400/70" },
  crt: { Icon: Key, className: "text-amber-400/70" },
};

/** Folders that earn a distinct glyph. Compared lowercased. */
const FOLDERS_BY_NAME: Record<string, FileIcon> = {
  hittable: { Icon: Zap, className: "text-cyan-400/70" },
  notes: { Icon: NotebookPen, className: "text-emerald-400/60" },
  ".git": { Icon: FolderGit2, className: "text-orange-400/50" },
  node_modules: { Icon: Folder, className: "text-white/15" },
};

/**
 * `.env.local` and `.env.production` should read like `.env`, so dotted
 * variants collapse onto their base name before the lookup tables are hit.
 */
function normaliseName(name: string): string {
  const lower = name.toLowerCase();
  if (lower.startsWith(".env")) return ".env";
  if (lower.startsWith("dockerfile")) return "dockerfile";
  if (lower.startsWith("makefile")) return "makefile";
  return lower;
}

export function getFileIcon(name: string): FileIcon {
  const normalised = normaliseName(name);
  const byName = BY_NAME[normalised];
  if (byName) return byName;

  // `archive.tar.gz` should be an archive, and `env.d.ts` should be TypeScript,
  // so fall back through compound extensions before the last segment alone.
  const segments = normalised.split(".");
  for (let i = 1; i < segments.length; i += 1) {
    const candidate = BY_EXTENSION[segments.slice(i).join(".")];
    if (candidate) return candidate;
  }

  const extension = segments.length > 1 ? segments[segments.length - 1] : "";
  return BY_EXTENSION[extension] ?? DEFAULT_FILE;
}

export function getFolderIcon(name: string, isExpanded: boolean): FileIcon {
  const special = FOLDERS_BY_NAME[name.toLowerCase()];
  if (special) return special;
  return {
    Icon: isExpanded ? FolderOpen : Folder,
    className: "text-amber-400/60",
  };
}
