// Reading order of the docs sidebar. Titles come from each page's
// frontmatter (content/docs/<slug>.md); only the grouping lives here.
export const DOCS_NAV: { title: string; slugs: string[] }[] = [
  { title: "Start here", slugs: ["index", "getting-started", "project-format"] },
  { title: "Web app", slugs: ["web-app", "browser-extension", "import-export", "self-hosting"] },
  {
    title: "Terminal app",
    slugs: ["shellapp", "shellapp-cli", "shellapp-keyboard", "shellapp-git", "shellapp-ai", "shellapp-editor-terminal"],
  },
  { title: "Project", slugs: ["development", "faq"] },
];

export const DOCS_REPO_EDIT_BASE = "https://github.com/Pantho-Haque/hittable/edit/main/content/docs/";
