// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import NoteEditor from "@/components/notes/NoteEditor";

function Editor({ initial = "", selected = true }: { initial?: string; selected?: boolean }) {
  const [content, setContent] = useState(initial);
  return (
    <NoteEditor selectedId={selected ? "1" : null} selectedTitle={selected ? "API notes" : null}
      content={content} isUnsaved={content !== initial} onChange={setContent} />
  );
}
const preview = (c: HTMLElement) => c.querySelector(".note-preview") as HTMLElement;

describe("NoteEditor: sanitised markdown preview", () => {
  it("strips <script>, event handlers and javascript: links", () => {
    const hit = vi.fn();
    (window as unknown as { hit: () => void }).hit = hit;
    const md = [
      "<script>hit()</script>",
      '<img src="x" onerror="hit()">',
      "[click](javascript:hit())",
      '<a href="javascript:hit()">raw link</a>',
      '<iframe src="https://evil.dev"></iframe>',
      '<div onclick="hit()">div</div>',
      '<svg><script>hit()</script></svg>',
    ].join("\n\n");
    const { container } = render(<Editor initial={md} />);
    const p = preview(container);
    expect(p.querySelector("script, iframe")).toBeNull();
    expect(p.innerHTML).not.toMatch(/onerror|onclick|javascript:/i);
    for (const a of p.querySelectorAll("a")) expect(a.getAttribute("href") ?? "").not.toMatch(/^javascript:/i);
    expect(hit).not.toHaveBeenCalled();
  });

  it("still renders normal GitHub-flavoured markdown", () => {
    const md = "# Title\n\n## Sub\n\n- one\n- two\n\n1. first\n\n`inline`\n\n```json\n{\"a\": 1}\n```\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n[docs](https://hittable.dev) **bold** _em_ ~~gone~~\n\n> quote";
    const { container } = render(<Editor initial={md} />);
    const p = preview(container);
    expect(p.querySelector("h1")).toHaveTextContent("Title");
    expect(p.querySelector("h2")).toHaveTextContent("Sub");
    expect(p.querySelectorAll("ul li")).toHaveLength(2);
    expect(p.querySelector("ol li")).toHaveTextContent("first");
    expect(p.querySelector("pre code")).toHaveTextContent('{"a": 1}');
    expect(p.querySelector("table td")).toHaveTextContent("1");
    expect(p.querySelector("a")).toHaveAttribute("href", "https://hittable.dev");
    expect(p.querySelector("strong")).toHaveTextContent("bold");
    expect(p.querySelector("del")).toHaveTextContent("gone");
    expect(p.querySelector("blockquote")).toHaveTextContent("quote");
  });

  it("keeps single line breaks (breaks: true)", () => {
    const { container } = render(<Editor initial={"line one\nline two"} />);
    expect(preview(container).querySelector("br")).not.toBeNull();
  });

  it("shows a placeholder for an empty note", () => {
    render(<Editor />);
    expect(screen.getByText("Nothing to preview")).toBeInTheDocument();
  });

  it("switches between preview, edit and split; typing updates the preview and counts", async () => {
    const user = userEvent.setup();
    const { container } = render(<Editor initial="# Hi" />);
    expect(screen.getByText("Preview mode")).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Switch to edit" }));
    expect(screen.getByText("Edit mode")).toBeInTheDocument();
    const box = screen.getByRole("textbox", { name: "Markdown source" });
    await user.clear(box);
    await user.type(box, "**two words**");
    expect(screen.getByText("13 chars · 2 words")).toBeInTheDocument();
    expect(screen.getByText("unsaved")).toBeInTheDocument();

    const split = screen.getByRole("button", { name: "Split" });
    expect(split).toHaveAttribute("aria-pressed", "false");
    await user.click(split);
    expect(screen.getByText("Split mode")).toBeInTheDocument();
    expect(split).toHaveAttribute("aria-pressed", "true");
    expect(preview(container).querySelector("strong")).toHaveTextContent("two words");
    expect(screen.getAllByRole("textbox", { name: "Markdown source" }).length).toBeGreaterThan(0);
  });

  it("without a selected note there are no mode buttons", () => {
    render(<Editor selected={false} />);
    expect(screen.getByText("Select a note")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
