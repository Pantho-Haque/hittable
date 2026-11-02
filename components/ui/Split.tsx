"use client";

import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { ChevronDown, ChevronLeft, ChevronRight, ChevronUp } from "lucide-react";

export type SplitSide = "first" | "second";

type Props = {
  /** Sizes and collapsed state are remembered per browser under this id. */
  id: string;
  /** "horizontal" puts the panes side by side, "vertical" stacks them. */
  orientation: "horizontal" | "vertical";
  first: ReactNode;
  second: ReactNode;
  firstLabel: string;
  secondLabel: string;
  /** The first pane's size: pixels with unit "px", else a 0–1 share. */
  defaultSize: number;
  unit?: "px" | "fraction";
  minFirst?: number;
  minSecond?: number;
  /** Panes that collapse when dragged past half their minimum, or with Enter. */
  collapsible?: SplitSide[];
  /** Controlled collapsed state (e.g. the explorer, also toggled by Ctrl/Cmd+B). */
  collapsed?: SplitSide | null;
  onCollapsedChange?: (side: SplitSide | null) => void;
  /** A collapsed pane leaves a bar to bring it back; off when something else restores it. */
  restoreBar?: boolean;
  className?: string;
};

const KEY_STEP = 16;
const KEY_STEP_LARGE = 64;

type Saved = { size: number; collapsed: SplitSide | null };
function load(id: string): Partial<Saved> {
  try {
    return JSON.parse(localStorage.getItem(`hittable:split:${id}`) ?? "{}");
  } catch {
    return {};
  }
}
function save(id: string, value: Saved) {
  try {
    localStorage.setItem(`hittable:split:${id}`, JSON.stringify(value));
  } catch {
    // not remembered; the layout still applies now
  }
}

/**
 * Two panes and a draggable separator: the WAI-ARIA window splitter pattern.
 * Drag (mouse, pen or touch), double-click to reset, arrows to resize, Home /
 * End for the extremes, Enter to collapse or restore. Both panes stay mounted
 * while collapsed, so an editor keeps its state and a preview does not reload.
 */
export default function Split({
  id,
  orientation,
  first,
  second,
  firstLabel,
  secondLabel,
  defaultSize,
  unit = "fraction",
  minFirst = 120,
  minSecond = 120,
  collapsible = [],
  collapsed: controlled,
  onCollapsedChange,
  restoreBar = true,
  className = "",
}: Props) {
  const horizontal = orientation === "horizontal";
  const firstId = useId();
  const boxRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState(defaultSize);
  const [ownCollapsed, setOwnCollapsed] = useState<SplitSide | null>(null);
  const collapsed = controlled !== undefined ? controlled : ownCollapsed;
  // The box's length along the split: pixel-sized panes report their position as a share of it.
  const [boxSize, setBoxSize] = useState(0);
  useLayoutEffect(() => {
    const el = boxRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(([entry]) => setBoxSize(horizontal ? entry.contentRect.width : entry.contentRect.height));
    ro.observe(el);
    return () => ro.disconnect();
  }, [horizontal]);

  // Read before paint, so a remembered layout never flashes the default.
  useLayoutEffect(() => {
    const saved = load(id);
    if (typeof saved.size === "number" && Number.isFinite(saved.size)) {
      setSize(saved.size);
    }
    if (controlled === undefined && (saved.collapsed === "first" || saved.collapsed === "second")) setOwnCollapsed(saved.collapsed);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once per split
  }, [id]);

  const sizeRef = useRef(size);
  sizeRef.current = size;
  const collapsedRef = useRef(collapsed);
  collapsedRef.current = collapsed;

  // Reports only real changes: a drag past the collapse point fires many
  // moves before the next render, and a parent's toggle must hear it once.
  const setCollapsed = useCallback(
    (side: SplitSide | null) => {
      if (collapsedRef.current === side) return;
      collapsedRef.current = side;
      if (controlled === undefined) setOwnCollapsed(side);
      onCollapsedChange?.(side);
    },
    [controlled, onCollapsedChange],
  );
  const persist = useCallback(() => save(id, { size: sizeRef.current, collapsed: controlled === undefined ? collapsedRef.current : null }), [id, controlled]);
  useEffect(persist, [persist, collapsed]);

  const total = () => {
    const box = boxRef.current?.getBoundingClientRect();
    return box ? (horizontal ? box.width : box.height) : 0;
  };
  const toPx = (s: number, t: number) => (unit === "px" ? s : s * t);
  const fromPx = (px: number, t: number) => (unit === "px" ? px : t ? px / t : defaultSize);

  /** Move the separator to `px` from the start, collapsing a pane dragged well past its minimum. */
  const moveTo = (px: number, t: number) => {
    if (collapsible.includes("first") && px < minFirst / 2) return setCollapsed("first");
    if (collapsible.includes("second") && t - px < minSecond / 2) return setCollapsed("second");
    const clamped = Math.max(minFirst, Math.min(t - minSecond, px));
    const next = fromPx(clamped, t);
    setSize(next);
    setCollapsed(null);
  };

  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return;
    e.preventDefault();
    const handle = e.currentTarget;
    handle.setPointerCapture(e.pointerId);
    // Frames (the HTML preview) swallow pointer events mid-drag; switch them off.
    document.body.classList.add(horizontal ? "is-splitting-x" : "is-splitting-y");
    const start = boxRef.current!.getBoundingClientRect();
    const onMove = (ev: PointerEvent) => moveTo(horizontal ? ev.clientX - start.left : ev.clientY - start.top, horizontal ? start.width : start.height);
    const onUp = () => {
      document.body.classList.remove("is-splitting-x", "is-splitting-y");
      handle.removeEventListener("pointermove", onMove);
      handle.removeEventListener("pointerup", onUp);
      handle.removeEventListener("pointercancel", onUp);
      persist();
    };
    handle.addEventListener("pointermove", onMove);
    handle.addEventListener("pointerup", onUp);
    handle.addEventListener("pointercancel", onUp);
  };

  const toggleCollapse = () => {
    if (collapsed) return setCollapsed(null);
    const side = collapsible[0];
    if (side) setCollapsed(side);
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const t = total();
    const now = collapsed === "first" ? 0 : collapsed === "second" ? t : toPx(size, t);
    const step = e.shiftKey ? KEY_STEP_LARGE : KEY_STEP;
    const back = horizontal ? "ArrowLeft" : "ArrowUp";
    const fwd = horizontal ? "ArrowRight" : "ArrowDown";
    let target: number | null = null;
    if (e.key === back) target = Math.max(minFirst, now - step);
    else if (e.key === fwd) target = Math.min(t - minSecond, now + step);
    else if (e.key === "Home") target = minFirst;
    else if (e.key === "End") target = t - minSecond;
    else if (e.key === "Enter") {
      e.preventDefault();
      return toggleCollapse();
    } else return;
    e.preventDefault();
    const next = fromPx(target, t);
    setSize(next);
    setCollapsed(null);
    sizeRef.current = next;
    persist();
  };

  const reset = () => {
    setSize(defaultSize);
    setCollapsed(null);
    sizeRef.current = defaultSize;
    persist();
  };

  const dim = horizontal ? "width" : "height";
  const minDim = horizontal ? "minWidth" : "minHeight";
  const firstStyle: React.CSSProperties =
    collapsed === "first" ? { display: "none" }
    : collapsed === "second" ? { flex: "1 1 0" }
    : unit === "px" ? { [dim]: size, flex: "none", maxWidth: horizontal ? `calc(100% - ${minSecond}px)` : undefined, maxHeight: horizontal ? undefined : `calc(100% - ${minSecond}px)` }
    : { flex: `${size} 1 0`, [minDim]: minFirst };
  const secondStyle: React.CSSProperties =
    collapsed === "second" ? { display: "none" }
    : collapsed === "first" || unit === "px" ? { flex: "1 1 0", [minDim]: unit === "px" ? minSecond : undefined }
    : { flex: `${1 - size} 1 0`, [minDim]: minSecond };

  const percent =
    collapsed === "first" ? 0
    : collapsed === "second" ? 100
    : unit === "px" ? (boxSize ? Math.min(100, Math.round((size / boxSize) * 100)) : 0)
    : Math.round(size * 100);
  const hiddenLabel = collapsed === "first" ? firstLabel : collapsed === "second" ? secondLabel : null;
  const RestoreIcon = horizontal ? (collapsed === "first" ? ChevronRight : ChevronLeft) : collapsed === "first" ? ChevronDown : ChevronUp;

  return (
    <div ref={boxRef} className={`flex min-h-0 min-w-0 ${horizontal ? "flex-row" : "flex-col"} ${className}`}>
      <div id={firstId} className="flex min-h-0 min-w-0 flex-col overflow-hidden" style={firstStyle}>
        {first}
      </div>
      <div
        role="separator"
        tabIndex={0}
        aria-orientation={horizontal ? "vertical" : "horizontal"}
        aria-controls={firstId}
        aria-label={`Resize ${firstLabel} and ${secondLabel}`}
        aria-valuenow={percent}
        aria-valuemin={0}
        aria-valuemax={100}
        title="Drag to resize · double-click to reset · Enter to collapse"
        onPointerDown={onPointerDown}
        onDoubleClick={reset}
        onKeyDown={onKeyDown}
        className={`split-handle ${horizontal ? "split-handle-x" : "split-handle-y"}`}
      />
      {restoreBar && hiddenLabel && (
        <button
          type="button"
          onClick={() => setCollapsed(null)}
          className={`split-restore ${horizontal ? "split-restore-x" : "split-restore-y"}`}
          style={{ order: collapsed === "first" ? -1 : 1 }}
          title={`Show ${hiddenLabel}`}
        >
          <RestoreIcon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span>Show {hiddenLabel}</span>
        </button>
      )}
      <div className="flex min-h-0 min-w-0 flex-col overflow-hidden" style={secondStyle}>
        {second}
      </div>
    </div>
  );
}
