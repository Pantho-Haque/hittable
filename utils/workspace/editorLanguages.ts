import { LanguageDescription, LanguageSupport, StreamLanguage } from "@codemirror/language";

/**
 * Every language is behind a dynamic `load()` so the grammars are code-split
 * out of the main bundle and only fetched when a matching file is opened.
 */
const DESCRIPTIONS: LanguageDescription[] = [
  LanguageDescription.of({
    name: "TypeScript",
    extensions: ["ts", "mts", "cts"],
    load: async () => (await import("@codemirror/lang-javascript")).javascript({ typescript: true }),
  }),
  LanguageDescription.of({
    name: "TSX",
    extensions: ["tsx"],
    load: async () =>
      (await import("@codemirror/lang-javascript")).javascript({ typescript: true, jsx: true }),
  }),
  LanguageDescription.of({
    name: "JavaScript",
    extensions: ["js", "mjs", "cjs"],
    load: async () => (await import("@codemirror/lang-javascript")).javascript(),
  }),
  LanguageDescription.of({
    name: "JSX",
    extensions: ["jsx"],
    load: async () => (await import("@codemirror/lang-javascript")).javascript({ jsx: true }),
  }),
  LanguageDescription.of({
    name: "JSON",
    // `.hit` is Hittable's own request format, which is JSON on the wire.
    extensions: ["json", "jsonc", "hit", "webmanifest"],
    filename: /^(env|\.babelrc|\.prettierrc)$/,
    load: async () => (await import("@codemirror/lang-json")).json(),
  }),
  LanguageDescription.of({
    name: "Markdown",
    extensions: ["md", "mdx", "markdown"],
    load: async () => (await import("@codemirror/lang-markdown")).markdown(),
  }),
  LanguageDescription.of({
    name: "HTML",
    extensions: ["html", "htm", "xhtml"],
    load: async () => (await import("@codemirror/lang-html")).html(),
  }),
  LanguageDescription.of({
    name: "CSS",
    extensions: ["css", "scss", "sass", "less"],
    load: async () => (await import("@codemirror/lang-css")).css(),
  }),
  LanguageDescription.of({
    name: "XML",
    extensions: ["xml", "svg", "plist", "xsd"],
    load: async () => (await import("@codemirror/lang-xml")).xml(),
  }),
  LanguageDescription.of({
    name: "YAML",
    extensions: ["yaml", "yml"],
    load: async () => (await import("@codemirror/lang-yaml")).yaml(),
  }),
  LanguageDescription.of({
    name: "Python",
    extensions: ["py", "pyi", "pyw"],
    load: async () => (await import("@codemirror/lang-python")).python(),
  }),
  LanguageDescription.of({
    name: "Go",
    extensions: ["go"],
    load: async () => (await import("@codemirror/lang-go")).go(),
  }),
  LanguageDescription.of({
    name: "Rust",
    extensions: ["rs"],
    load: async () => (await import("@codemirror/lang-rust")).rust(),
  }),
  LanguageDescription.of({
    name: "SQL",
    extensions: ["sql"],
    load: async () => (await import("@codemirror/lang-sql")).sql(),
  }),
  LanguageDescription.of({
    name: "Shell",
    extensions: ["sh", "bash", "zsh", "fish", "ksh"],
    filename: /^(\.?(bash|zsh)(rc|_profile|env)|Makefile|makefile)$/,
    load: async () => stream("shell"),
  }),
  LanguageDescription.of({
    name: "TOML",
    extensions: ["toml"],
    load: async () => stream("toml"),
  }),
  LanguageDescription.of({
    name: "INI",
    extensions: ["ini", "conf", "cfg", "properties"],
    load: async () => stream("properties"),
  }),
  LanguageDescription.of({
    name: "Dockerfile",
    extensions: ["dockerfile"],
    filename: /^Dockerfile/i,
    load: async () => stream("dockerFile"),
  }),
  LanguageDescription.of({
    name: "Ruby",
    extensions: ["rb", "gemspec"],
    load: async () => stream("ruby"),
  }),
  LanguageDescription.of({
    name: "Lua",
    extensions: ["lua"],
    load: async () => stream("lua"),
  }),
  LanguageDescription.of({
    name: "C/C++",
    extensions: ["c", "h", "cpp", "hpp", "cc", "cxx"],
    load: async () => stream("c"),
  }),
  LanguageDescription.of({
    name: "Java",
    extensions: ["java"],
    load: async () => stream("java"),
  }),
  LanguageDescription.of({
    name: "C#",
    extensions: ["cs"],
    load: async () => stream("csharp"),
  }),
  LanguageDescription.of({
    name: "Swift",
    extensions: ["swift"],
    load: async () => stream("swift"),
  }),
  LanguageDescription.of({
    name: "Diff",
    extensions: ["diff", "patch"],
    load: async () => stream("diff"),
  }),
];

type StreamParser = Parameters<typeof StreamLanguage.define>[0];

/**
 * Loaders are spelled out one by one rather than built from a template string:
 * a computed `import()` makes the bundler pull in every legacy mode, and the
 * clike family (C, Java, C#) all share a single module with differing exports.
 */
const STREAM_MODES: Record<string, () => Promise<StreamParser>> = {
  shell: async () => (await import("@codemirror/legacy-modes/mode/shell")).shell,
  toml: async () => (await import("@codemirror/legacy-modes/mode/toml")).toml,
  properties: async () => (await import("@codemirror/legacy-modes/mode/properties")).properties,
  dockerFile: async () => (await import("@codemirror/legacy-modes/mode/dockerfile")).dockerFile,
  ruby: async () => (await import("@codemirror/legacy-modes/mode/ruby")).ruby,
  lua: async () => (await import("@codemirror/legacy-modes/mode/lua")).lua,
  swift: async () => (await import("@codemirror/legacy-modes/mode/swift")).swift,
  diff: async () => (await import("@codemirror/legacy-modes/mode/diff")).diff,
  c: async () => (await import("@codemirror/legacy-modes/mode/clike")).c,
  java: async () => (await import("@codemirror/legacy-modes/mode/clike")).java,
  csharp: async () => (await import("@codemirror/legacy-modes/mode/clike")).csharp,
};

/**
 * The legacy modes ship as CodeMirror 5 stream parsers, which have to be
 * adapted before CodeMirror 6 will accept them.
 */
async function stream(mode: keyof typeof STREAM_MODES): Promise<LanguageSupport> {
  return new LanguageSupport(StreamLanguage.define(await STREAM_MODES[mode]()));
}

/** Resolves a filename to a grammar, or null when nothing matches. */
export function languageForFile(filename: string): LanguageDescription | null {
  return LanguageDescription.matchFilename(DESCRIPTIONS, filename);
}

/**
 * Grammars that contribute their own completion source. Layering buffer-word
 * completion on top of these lists the same identifier twice, and their
 * scope-aware suggestions are strictly better than a flat word scan.
 */
const SELF_COMPLETING = new Set(["TypeScript", "TSX", "JavaScript", "JSX", "HTML", "CSS", "Python", "SQL"]);

export function languageHasCompletion(filename: string): boolean {
  const description = languageForFile(filename);
  return description !== null && SELF_COMPLETING.has(description.name);
}

/** Human-readable label for the editor status bar. */
export function languageNameForFile(filename: string): string {
  return languageForFile(filename)?.name ?? "Plain Text";
}
