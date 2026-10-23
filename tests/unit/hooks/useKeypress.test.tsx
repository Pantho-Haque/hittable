// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import useKeypress from "@/hooks/useKeypress";

// `key` is reserved by React, so the shortcut key travels as `k`.
type Opts = Parameters<typeof useKeypress>[0];
function Harness({ k, ...rest }: Omit<Opts, "key"> & { k: string }) {
  useKeypress({ key: k, ...rest });
  return (
    <div>
      <input aria-label="field" />
      <textarea aria-label="area" />
      <button>outside</button>
    </div>
  );
}

describe("useKeypress", () => {
  it("fires Ctrl+key and Cmd+key shortcuts interchangeably", async () => {
    const func = vi.fn();
    render(<Harness k="Enter" isMeta func={func} />);
    const user = userEvent.setup();
    await user.keyboard("{Control>}{Enter}{/Control}");
    await user.keyboard("{Meta>}{Enter}{/Meta}");
    expect(func).toHaveBeenCalledTimes(2);
  });

  it("fires modifier shortcuts even while typing in an input", async () => {
    const func = vi.fn();
    render(<Harness k="s" isMeta func={func} />);
    const user = userEvent.setup();
    await user.click(screen.getByLabelText("field"));
    await user.keyboard("{Control>}s{/Control}");
    expect(func).toHaveBeenCalledTimes(1);
  });

  it("does not fire a modifier shortcut without the modifier", async () => {
    const func = vi.fn();
    render(<Harness k="s" isMeta func={func} />);
    await userEvent.setup().keyboard("s");
    expect(func).not.toHaveBeenCalled();
  });

  it("Shift+T fires outside fields but never while typing in an input or textarea", async () => {
    const func = vi.fn();
    render(<Harness k="t" isShift func={func} />);
    const user = userEvent.setup();
    await user.keyboard("{Shift>}T{/Shift}");
    expect(func).toHaveBeenCalledTimes(1);

    await user.click(screen.getByLabelText("field"));
    await user.keyboard("{Shift>}T{/Shift}");
    await user.click(screen.getByLabelText("area"));
    await user.keyboard("{Shift>}T{/Shift}");
    expect(func).toHaveBeenCalledTimes(1);
  });

  it("a plain shortcut does not fire when a modifier is held", async () => {
    const func = vi.fn();
    render(<Harness k="t" isShift func={func} />);
    await userEvent.setup().keyboard("{Control>}{Shift>}T{/Shift}{/Control}");
    expect(func).not.toHaveBeenCalled();
  });

  it("calls the latest callback without re-subscribing", async () => {
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = render(<Harness k="j" isMeta func={first} />);
    rerender(<Harness k="j" isMeta func={second} />);
    await userEvent.setup().keyboard("{Control>}j{/Control}");
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
  });

  it("prevents the browser default for a matched shortcut", () => {
    render(<Harness k="s" isMeta func={() => {}} />);
    const ev = new KeyboardEvent("keydown", { key: "s", ctrlKey: true, cancelable: true });
    window.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
  });

  it("stops listening after unmount", async () => {
    const func = vi.fn();
    const { unmount } = render(<Harness k="b" isMeta func={func} />);
    unmount();
    await userEvent.setup().keyboard("{Control>}b{/Control}");
    expect(func).not.toHaveBeenCalled();
  });
});
