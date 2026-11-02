// @vitest-environment jsdom
import { describe, it, expect, beforeEach, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import Split, { type SplitSide } from "@/components/ui/Split";

// jsdom lays nothing out: give every element a 1000×600 box.
beforeEach(() => {
  localStorage.clear();
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
    x: 0, y: 0, left: 0, top: 0, width: 1000, height: 600, right: 1000, bottom: 600, toJSON: () => ({}),
  } as DOMRect);
});

const mount = (props: Partial<React.ComponentProps<typeof Split>> = {}) =>
  render(
    <Split
      id="t"
      orientation="horizontal"
      defaultSize={0.4}
      minFirst={100}
      minSecond={100}
      collapsible={["first", "second"]}
      firstLabel="Request"
      secondLabel="Response"
      first={<p>first pane</p>}
      second={<p>second pane</p>}
      {...props}
    />,
  );
const handle = () => screen.getByRole("separator", { name: "Resize Request and Response" });
const pane = (text: string) => screen.getByText(text).parentElement!;

describe("Split", () => {
  it("is a focusable window splitter with its value and the pane it controls", () => {
    mount();
    const h = handle();
    expect(h).toHaveAttribute("tabindex", "0");
    expect(h).toHaveAttribute("aria-orientation", "vertical"); // side-by-side panes: a vertical line
    expect(h).toHaveAttribute("aria-valuenow", "40");
    expect(h.getAttribute("aria-controls")).toBe(pane("first pane").id);
  });

  it("arrows resize (Shift for big steps), Home and End go to the minimums", () => {
    mount();
    fireEvent.keyDown(handle(), { key: "ArrowRight" });
    expect(handle()).toHaveAttribute("aria-valuenow", "42"); // 400 + 16 px of 1000
    fireEvent.keyDown(handle(), { key: "ArrowLeft", shiftKey: true });
    expect(handle()).toHaveAttribute("aria-valuenow", "35"); // 416 − 64
    fireEvent.keyDown(handle(), { key: "Home" });
    expect(handle()).toHaveAttribute("aria-valuenow", "10");
    fireEvent.keyDown(handle(), { key: "End" });
    expect(handle()).toHaveAttribute("aria-valuenow", "90");
  });

  it("stacked panes resize with Up and Down", () => {
    mount({ orientation: "vertical" });
    expect(handle()).toHaveAttribute("aria-orientation", "horizontal");
    fireEvent.keyDown(handle(), { key: "ArrowDown" });
    expect(Number(handle().getAttribute("aria-valuenow"))).toBeGreaterThan(40);
  });

  it("Enter collapses the primary pane and brings it back, keeping it mounted", () => {
    mount();
    fireEvent.keyDown(handle(), { key: "Enter" });
    expect(pane("first pane")).toHaveStyle({ display: "none" });
    expect(handle()).toHaveAttribute("aria-valuenow", "0");
    fireEvent.keyDown(handle(), { key: "Enter" });
    expect(pane("first pane")).not.toHaveStyle({ display: "none" });
    expect(handle()).toHaveAttribute("aria-valuenow", "40");
  });

  it("a collapsed pane leaves a button that restores it", () => {
    mount();
    fireEvent.keyDown(handle(), { key: "Enter" });
    fireEvent.click(screen.getByRole("button", { name: "Show Request" }));
    expect(pane("first pane")).not.toHaveStyle({ display: "none" });
    expect(screen.queryByRole("button", { name: /Show/ })).toBeNull();
  });

  it("dragging past half a pane's minimum collapses it; dragging back restores", () => {
    mount();
    const h = handle();
    h.setPointerCapture = () => {};
    fireEvent.pointerDown(h, { button: 0, pointerId: 1, clientX: 400 });
    fireEvent.pointerMove(h, { pointerId: 1, clientX: 600 });
    expect(handle()).toHaveAttribute("aria-valuenow", "60");
    expect(document.body).toHaveClass("is-splitting-x"); // frames ignore the pointer mid-drag
    fireEvent.pointerMove(h, { pointerId: 1, clientX: 970 });
    expect(pane("second pane")).toHaveStyle({ display: "none" });
    fireEvent.pointerMove(h, { pointerId: 1, clientX: 500 });
    expect(pane("second pane")).not.toHaveStyle({ display: "none" });
    fireEvent.pointerUp(h, { pointerId: 1 });
    expect(document.body).not.toHaveClass("is-splitting-x");
  });

  it("double-click resets to the default size", () => {
    mount();
    fireEvent.keyDown(handle(), { key: "End" });
    fireEvent.doubleClick(handle());
    expect(handle()).toHaveAttribute("aria-valuenow", "40");
  });

  it("remembers size and collapse per id", () => {
    const { unmount } = mount();
    fireEvent.keyDown(handle(), { key: "Home" });
    unmount();
    mount();
    expect(handle()).toHaveAttribute("aria-valuenow", "10");
    fireEvent.keyDown(handle(), { key: "Enter" });
    expect(JSON.parse(localStorage.getItem("hittable:split:t")!)).toEqual({ size: 0.1, collapsed: "first" });
  });

  it("a broken saved layout falls back to the default", () => {
    localStorage.setItem("hittable:split:t", "{not json");
    mount();
    expect(handle()).toHaveAttribute("aria-valuenow", "40");
  });

  it("a controlled pane (the explorer) reports collapse instead of owning it, with no restore bar", () => {
    function Explorer() {
      const [collapsed, setCollapsed] = useState<SplitSide | null>(null);
      return (
        <>
          <span data-testid="state">{collapsed ?? "open"}</span>
          <Split id="x" orientation="horizontal" unit="px" defaultSize={256} collapsible={["first"]}
            collapsed={collapsed} onCollapsedChange={setCollapsed} restoreBar={false}
            firstLabel="Explorer" secondLabel="Editor" first={<p>tree</p>} second={<p>editor</p>} />
        </>
      );
    }
    render(<Explorer />);
    const h = screen.getByRole("separator");
    expect(pane("tree")).toHaveStyle({ width: "256px" });
    fireEvent.keyDown(h, { key: "Enter" });
    expect(screen.getByTestId("state")).toHaveTextContent("first");
    expect(screen.queryByRole("button", { name: /Show/ })).toBeNull();
    fireEvent.keyDown(h, { key: "ArrowRight" });
    expect(screen.getByTestId("state")).toHaveTextContent("open");
    expect(localStorage.getItem("hittable:split:x")).toContain('"collapsed":null');
  });

  it("a parent that only flips (Ctrl/Cmd+B's toggle) stays in step through a long drag", () => {
    function Flip() {
      const [shut, setShut] = useState(false);
      return (
        <>
          <span data-testid="shut">{String(shut)}</span>
          <Split id="f" orientation="horizontal" unit="px" defaultSize={256} minFirst={160} collapsible={["first"]}
            collapsed={shut ? "first" : null} onCollapsedChange={() => setShut((v) => !v)} restoreBar={false}
            firstLabel="Explorer" secondLabel="Editor" first={<p>tree</p>} second={<p>editor</p>} />
        </>
      );
    }
    render(<Flip />);
    const h = screen.getByRole("separator");
    h.setPointerCapture = () => {};
    fireEvent.pointerDown(h, { button: 0, pointerId: 1, clientX: 256 });
    for (const x of [60, 40, 20, 10]) fireEvent.pointerMove(h, { pointerId: 1, clientX: x });
    expect(screen.getByTestId("shut")).toHaveTextContent("true");
    for (const x of [200, 220]) fireEvent.pointerMove(h, { pointerId: 1, clientX: x });
    fireEvent.pointerUp(h, { pointerId: 1 });
    expect(screen.getByTestId("shut")).toHaveTextContent("false");
  });
});
