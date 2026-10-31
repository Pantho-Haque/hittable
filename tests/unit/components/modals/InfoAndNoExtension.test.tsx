// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import InfoModal from "@/components/modals/InfoModal";
import ExtensionStatus from "@/components/modals/ExtensionStatus";
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

// The old NoExtensionModal opened by itself on every visit, even for people
// who never call localhost. It is now a status button that explains on click;
// a localhost request without it says so in the response panel instead.
describe("ExtensionStatus", () => {
  it("renders nothing until the extension check has finished", () => {
    const { container } = render(<ExtensionStatus checked={false} available={false} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("never opens a dialog on its own", () => {
    render(<ExtensionStatus checked available={false} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Extension not installed" })).toBeInTheDocument();
  });

  it("shows connected when the extension answered", () => {
    render(<ExtensionStatus checked available />);
    expect(screen.getByRole("button", { name: "Extension connected" })).toBeInTheDocument();
  });

  it("explains how to install on click, and Download opens the release", async () => {
    const open = vi.spyOn(window, "open").mockImplementation(() => null);
    const user = userEvent.setup();
    render(<ExtensionStatus checked available={false} />);
    await user.click(screen.getByRole("button", { name: "Extension not installed" }));
    const dialog = within(screen.getByRole("dialog", { name: "Localhost extension" }));
    expect(dialog.getByText(/chrome:\/\/extensions/)).toBeInTheDocument();
    await user.click(dialog.getByRole("button", { name: "Download" }));
    expect(open).toHaveBeenCalledWith(EXTENSION_URL, "_blank");
    await user.click(dialog.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
