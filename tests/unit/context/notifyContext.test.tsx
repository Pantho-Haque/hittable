// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, renderHook, screen, waitForElementToBeRemoved, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NotificationProvider } from "@/context/notifyContext";
import { useNotification } from "@/hooks/useNotify";

const undo = vi.fn();

function Buttons() {
  const n = useNotification();
  return (
    <div>
      <button onClick={() => n.success({ title: "Saved", desc: "Route saved" })}>ok</button>
      <button onClick={() => n.error({ title: "Failed", desc: "Bad JSON", timeout: 5000 })}>err</button>
      <button onClick={() => n.info({ title: "Note", desc: "FYI", position: "bottom-left" })}>info</button>
      <button onClick={() => n.success({ title: "Imported", desc: "from curl", timeout: 5000, action: { label: "Undo", onClick: undo } })}>act</button>
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

  // The toast used to concatenate its text colour and border classes with no
  // space ("zinc-200border-l-…"), losing both, and followed the OS light theme
  // inside the dark app. Each mode now has its own status-token border.
  it("each mode gets its own colour on the app's dark surface", async () => {
    render(<NotificationProvider><Buttons /></NotificationProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByText("ok"));
    await user.click(screen.getByText("err"));
    const ok = screen.getByText("Saved").closest("div[class*='border-l-4']")!.className;
    const bad = screen.getByText("Failed").closest("div[class*='border-l-4']")!.className;
    expect(ok).toContain("border-l-(--success)");
    expect(bad).toContain("border-l-(--danger)");
    for (const c of [ok, bad]) {
      expect(c).toContain("bg-(--surface-2)");
      expect(c).not.toMatch(/\S(border-l|bg)-/); // no glued-together classes
    }
  });

  it("errors are announced as alerts, the rest as status", async () => {
    render(<NotificationProvider><Buttons /></NotificationProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByText("ok"));
    await user.click(screen.getByText("err"));
    expect(screen.getByRole("alert")).toHaveTextContent("Failed");
    expect(screen.getByRole("status")).toHaveTextContent("Saved");
  });

  it("an action button runs its callback and dismisses the toast", async () => {
    render(<NotificationProvider><Buttons /></NotificationProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByText("act"));
    await user.click(screen.getByRole("button", { name: "Undo" }));
    expect(undo).toHaveBeenCalledOnce();
    await waitFor(() => expect(screen.queryByText("Imported")).toBeNull());
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
    await user.click(within(toast).getByRole("button", { name: "Dismiss notification" }));
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
