// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { ModalActions, ModalInput, ModalShell } from "@/components/ui/SharedModal";

function Demo({ onConfirm = () => {}, backdropCloses = true }: { onConfirm?: () => void; backdropCloses?: boolean }) {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState("");
  return (
    <>
      <button onClick={() => setOpen(true)}>Open</button>
      {open && (
        <ModalShell title="Demo" subtitle="sub" onClose={() => setOpen(false)} isOnBlurPressClose={backdropCloses}>
          <ModalInput value={value} onChange={setValue} placeholder="name" />
          <ModalActions onCancel={() => setOpen(false)} onConfirm={onConfirm} confirmLabel="OK" />
        </ModalShell>
      )}
    </>
  );
}

describe("ModalShell", () => {
  it("is an accessible modal dialog named by its title and rendered at the body", async () => {
    const user = userEvent.setup();
    render(<Demo />);
    await user.click(screen.getByRole("button", { name: "Open" }));
    const dialog = screen.getByRole("dialog", { name: "Demo" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog.closest("[data-modal]")?.parentElement).toBe(document.body);
    expect(dialog).toHaveFocus();
  });

  it("traps Tab and Shift+Tab inside the dialog", async () => {
    const user = userEvent.setup();
    render(<Demo />);
    await user.click(screen.getByRole("button", { name: "Open" }));
    const input = screen.getByLabelText("name");
    const ok = screen.getByRole("button", { name: "OK" });
    await user.tab();
    expect(input).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
    await user.tab();
    expect(ok).toHaveFocus();
    await user.tab();
    expect(input).toHaveFocus();
    await user.tab({ shift: true });
    expect(ok).toHaveFocus();
  });

  it("returns focus to the opener when it closes", async () => {
    const user = userEvent.setup();
    render(<Demo />);
    const opener = screen.getByRole("button", { name: "Open" });
    await user.click(opener);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("closes on a backdrop click but not on a click inside", async () => {
    const user = userEvent.setup();
    render(<Demo />);
    await user.click(screen.getByRole("button", { name: "Open" }));
    await user.click(screen.getByText("sub"));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.click(document.querySelector("[data-modal]")!);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("can opt out of backdrop dismissal", async () => {
    const user = userEvent.setup();
    render(<Demo backdropCloses={false} />);
    await user.click(screen.getByRole("button", { name: "Open" }));
    await user.click(document.querySelector("[data-modal]")!);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
});

describe("ModalActions", () => {
  it("runs the confirm action and shows a disabled loading state", async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(<ModalActions onCancel={() => {}} onConfirm={onConfirm} confirmLabel="Save" />);
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(onConfirm).toHaveBeenCalledOnce();
    rerender(<ModalActions onCancel={() => {}} onConfirm={onConfirm} confirmLabel="Save" loading />);
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Save" })).toHaveTextContent("Loading...");
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
  });
});
