// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, renderHook, screen, waitForElementToBeRemoved } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NotificationProvider } from "@/context/notifyContext";
import { useNotification } from "@/hooks/useNotify";

function Buttons() {
  const n = useNotification();
  return (
    <div>
      <button onClick={() => n.success({ title: "Saved", desc: "Route saved" })}>ok</button>
      <button onClick={() => n.error({ title: "Failed", desc: "Bad JSON", timeout: 5000 })}>err</button>
      <button onClick={() => n.info({ title: "Note", desc: "FYI", position: "bottom-left" })}>info</button>
    </div>
  );
}

describe("notifications", () => {
  it("useNotification outside the provider throws a clear error", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => renderHook(() => useNotification())).toThrow(/within a NotificationProvider/);
  });

  it("shows a toast with title and description", async () => {
    render(<NotificationProvider><Buttons /></NotificationProvider>);
    await userEvent.setup().click(screen.getByText("ok"));
    expect(screen.getByText("Saved")).toBeInTheDocument();
    expect(screen.getByText("Route saved")).toBeInTheDocument();
  });

  it("each mode gets its own colour", async () => {
    render(<NotificationProvider><Buttons /></NotificationProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByText("ok"));
    await user.click(screen.getByText("err"));
    expect(screen.getByText("Saved").closest("div[class*='border-l-4']")!.className).toContain("green");
    expect(screen.getByText("Failed").closest("div[class*='border-l-4']")!.className).toContain("red");
  });

  it("disappears after its timeout (default 2s), longer timeouts stay", async () => {
    render(<NotificationProvider><Buttons /></NotificationProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByText("ok"));
    await user.click(screen.getByText("err"));
    await new Promise((r) => setTimeout(r, 1500));
    expect(screen.getByText("Route saved")).toBeInTheDocument();
    await waitForElementToBeRemoved(() => screen.queryByText("Route saved"), { timeout: 2000 });
    expect(screen.getByText("Bad JSON")).toBeInTheDocument();
  });

  it("the × button dismisses only that toast", async () => {
    render(<NotificationProvider><Buttons /></NotificationProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByText("ok"));
    await user.click(screen.getByText("err"));
    const toast = screen.getByText("Saved").closest("div[class*='border-l-4']") as HTMLElement;
    await user.click(toast.querySelector("button")!);
    await waitForElementToBeRemoved(() => screen.queryByText("Saved"));
    expect(screen.getByText("Failed")).toBeInTheDocument();
  });

  it("groups toasts by position", async () => {
    render(<NotificationProvider><Buttons /></NotificationProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByText("ok"));
    await user.click(screen.getByText("info"));
    expect(screen.getByText("Saved").closest(".fixed")!.className).toContain("top-0 right-0");
    expect(screen.getByText("Note").closest(".fixed")!.className).toContain("bottom-0 left-0");
  });
});
