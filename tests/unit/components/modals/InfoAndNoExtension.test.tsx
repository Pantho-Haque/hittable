// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import InfoModal from "@/components/modals/InfoModal";
import NoExtensionModal from "@/components/modals/NoExtensionModal";
import { EXTENSION_URL } from "@/config";
import { KEYBINDINGS } from "@/constants";

describe("InfoModal", () => {
  it("lists every keybinding, including Esc to close", async () => {
    const user = userEvent.setup();
    render(<InfoModal />);
    await user.click(screen.getByTitle("Info"));
    const dialog = within(screen.getByRole("dialog", { name: "Hittable Info" }));
    await user.click(dialog.getByRole("button", { name: /Keybindings/ }));
    for (const { description } of KEYBINDINGS) expect(dialog.getByText(description)).toBeInTheDocument();
    expect(dialog.getByText("Close modals")).toBeInTheDocument();
  });

  it("links the extension download to the official release asset", async () => {
    const user = userEvent.setup();
    render(<InfoModal />);
    await user.click(screen.getByTitle("Info"));
    await user.click(screen.getByRole("button", { name: /Localhost Extension/ }));
    const link = screen.getByRole("link", { name: "Download Extension" });
    expect(link).toHaveAttribute("href", EXTENSION_URL);
    expect(link).toHaveAttribute("rel", expect.stringContaining("noreferrer"));
  });

  it("closes with Got it or Escape", async () => {
    const user = userEvent.setup();
    render(<InfoModal />);
    await user.click(screen.getByTitle("Info"));
    await user.click(screen.getByRole("button", { name: "Got it" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(screen.getByTitle("Info"));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});

describe("NoExtensionModal", () => {
  it("stays hidden until the extension check has finished", () => {
    render(<NoExtensionModal checked={false} available={false} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("stays hidden when the extension answered", () => {
    render(<NoExtensionModal checked available />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("explains how to install when the check gave up, and Download opens the release", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const open = vi.spyOn(window, "open").mockImplementation(() => null);
    const user = userEvent.setup();
    render(<NoExtensionModal checked available={false} />);
    const dialog = within(screen.getByRole("dialog", { name: "No Extension Found" }));
    expect(dialog.getByText(/chrome:\/\/extensions/)).toBeInTheDocument();
    await user.click(dialog.getByRole("button", { name: "Download" }));
    expect(open).toHaveBeenCalledWith(EXTENSION_URL, "_blank");
    await user.click(dialog.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("is not dismissed by a stray click on the backdrop", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const user = userEvent.setup();
    render(<NoExtensionModal checked available={false} />);
    await user.click(document.querySelector("[data-modal]")!);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
});
