// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useExtension } from "@/hooks/useExtension";

const ready = (source: MessageEventSource | null = window) =>
  act(() => { window.dispatchEvent(new MessageEvent("message", { data: { type: "HITTABLE_EXTENSION_READY" }, source })); });

let post: ReturnType<typeof vi.spyOn>;
beforeEach(() => {
  vi.useFakeTimers();
  post = vi.spyOn(window, "postMessage").mockImplementation(() => {});
  delete window.__hittableExtension;
});
afterEach(() => vi.useRealTimers());

describe("useExtension", () => {
  it("pings every 500 ms to the page's own origin and gives up after 10 tries (~5 s)", () => {
    const { result } = renderHook(() => useExtension());
    expect(result.current).toEqual({ available: false, checked: false });
    act(() => vi.advanceTimersByTime(4500));
    expect(post).toHaveBeenCalledTimes(9);
    expect(post).toHaveBeenCalledWith({ type: "HITTABLE_PING" }, window.location.origin);
    expect(result.current.checked).toBe(false);
    act(() => vi.advanceTimersByTime(500));
    expect(result.current).toEqual({ available: false, checked: true });
    act(() => vi.advanceTimersByTime(5000));
    expect(post).toHaveBeenCalledTimes(10);
  });

  it("detects the extension when it answers, and stops pinging", () => {
    const { result } = renderHook(() => useExtension());
    act(() => vi.advanceTimersByTime(1000));
    ready();
    expect(result.current).toEqual({ available: true, checked: true });
    expect(window.__hittableExtension).toBe(true);
    const calls = post.mock.calls.length;
    act(() => vi.advanceTimersByTime(5000));
    expect(post).toHaveBeenCalledTimes(calls);
  });

  it("ignores a READY message from another window", () => {
    const { result } = renderHook(() => useExtension());
    ready({} as MessageEventSource);
    ready(null);
    expect(result.current.available).toBe(false);
  });

  it("stops listening and pinging after unmount", () => {
    const { result, unmount } = renderHook(() => useExtension());
    unmount();
    act(() => vi.advanceTimersByTime(5000));
    expect(post).not.toHaveBeenCalled();
    ready();
    expect(result.current.available).toBe(false);
  });
});
