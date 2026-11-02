// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import HtmlSourceViewer from "@/components/RequestForm/ResponsePanelComponents/HtmlSourceViewer";

const SOURCE = ["<html>", "  <body>", '    <h1 id="greeting">Hi</h1>', "    <p>" + "x".repeat(2000) + "</p>", "  </body>", "</html>"].join("\n");

beforeEach(() => localStorage.clear());

describe("HtmlSourceViewer", () => {
  it("shows the whole source in one read-only editor with one scroll area", () => {
    const { container } = render(<HtmlSourceViewer source={SOURCE} />);
    expect(container.querySelectorAll(".cm-scroller")).toHaveLength(1);
    const content = screen.getByRole("textbox", { name: "Response HTML source" });
    expect(content).toHaveAttribute("aria-readonly", "true");
    expect(content.textContent).toContain('<h1 id="greeting">Hi</h1>');
  });

  it("marks the line picked in the preview and moves the cursor there", () => {
    const { container, rerender } = render(<HtmlSourceViewer source={SOURCE} highlightedLine={null} />);
    expect(container.querySelector(".cm-picked-line")).toBeNull();
    rerender(<HtmlSourceViewer source={SOURCE} highlightedLine={2} />);
    expect(container.querySelector(".cm-picked-line")?.textContent).toBe('    <h1 id="greeting">Hi</h1>');
    expect(screen.getByText("Ln 3")).toBeInTheDocument();
    rerender(<HtmlSourceViewer source={SOURCE} highlightedLine={null} />);
    expect(container.querySelector(".cm-picked-line")).toBeNull();
  });

  it("a line past the end marks nothing", () => {
    const { container } = render(<HtmlSourceViewer source={SOURCE} highlightedLine={99} />);
    expect(container.querySelector(".cm-picked-line")).toBeNull();
  });

  it("wrap toggles and is remembered", () => {
    const { container, unmount } = render(<HtmlSourceViewer source={SOURCE} />);
    const wrap = screen.getByRole("button", { name: "Wrap long lines" });
    expect(wrap).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(wrap);
    expect(wrap).toHaveAttribute("aria-pressed", "true");
    expect(container.querySelector(".cm-lineWrapping")).not.toBeNull();
    unmount();
    const again = render(<HtmlSourceViewer source={SOURCE} />);
    expect(screen.getByRole("button", { name: "Wrap long lines" })).toHaveAttribute("aria-pressed", "true");
    expect(again.container.querySelector(".cm-lineWrapping")).not.toBeNull();
  });

  it("collapse all folds the tags, expand all brings them back", () => {
    render(<HtmlSourceViewer source={SOURCE} />);
    fireEvent.click(screen.getByRole("button", { name: "Collapse all tags" }));
    expect(screen.getByRole("button", { name: "Expand all tags" })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Expand all tags" }));
    expect(screen.getByRole("button", { name: "Collapse all tags" })).toHaveAttribute("aria-pressed", "false");
  });

  it("the search button opens the search panel", () => {
    const { container } = render(<HtmlSourceViewer source={SOURCE} />);
    fireEvent.click(screen.getByRole("button", { name: /Search in source/ }));
    expect(container.querySelector(".cm-search")).not.toBeNull();
  });

  it("offers the pretty-print toggle only when the panel can change it", () => {
    const { unmount } = render(<HtmlSourceViewer source={SOURCE} />);
    expect(screen.queryByRole("button", { name: /pretty-print|as received/i })).toBeNull();
    unmount();
    const change = vi.fn();
    render(<HtmlSourceViewer source={SOURCE} pretty onPrettyChange={change} />);
    fireEvent.click(screen.getByRole("button", { name: "Pretty-printed: show as received" }));
    expect(change).toHaveBeenCalledWith(false);
  });
});
