// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CopyAsMenu from "@/components/RequestForm/CopyAsMenu";
import { NotificationProvider } from "@/context/notifyContext";
import { decodeShare } from "@/utils/shareLink";
import { generateCode } from "@/utils/codegen";
import { THittableCurlJson } from "@/types";

const formInput: THittableCurlJson = {
  method: "POST",
  url: "https://<<HOST>>/users",
  headers: JSON.stringify({ Authorization: "Bearer <<TOKEN>>", "Content-Type": "application/json" }),
  body: '{"name":"Ann"}',
  params: "{}",
};
const env = { HOST: "api.io", TOKEN: "env-token" };
const secrets = { TOKEN: "secret-token" };

// The menu shows toasts for share links (FR-15), so it needs the provider the app has.
function setup(input: THittableCurlJson = formInput) {
  const user = userEvent.setup();
  const onCopied = vi.fn();
  render(
    <NotificationProvider>
      <CopyAsMenu formInput={input} env={env} secrets={secrets} onCopied={onCopied} />
      <p>outside</p>
    </NotificationProvider>,
  );
  const trigger = screen.getByRole("button", { name: /copy as/i });
  return { user, onCopied, trigger };
}

describe("CopyAsMenu", () => {
  it("opens a menu of languages with the first item focused", async () => {
    const { user, trigger } = setup();
    expect(trigger).toHaveAttribute("aria-haspopup", "menu");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    await user.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    const items = screen.getAllByRole("menuitem");
    // FR-15 adds "Share link" after the code languages.
    expect(items.map((i) => i.textContent)).toEqual(["cURL", "JavaScript fetch", "Python requests", "HTTPie", "Share link"]);
    await waitFor(() => expect(items[0]).toHaveFocus());
  });

  it("moves focus with the arrow keys, wrapping at the ends", async () => {
    const { user, trigger } = setup();
    trigger.focus();
    await user.keyboard("{ArrowDown}");
    const items = await screen.findAllByRole("menuitem");
    await waitFor(() => expect(items[0]).toHaveFocus());
    await user.keyboard("{ArrowDown}");
    expect(items[1]).toHaveFocus();
    // Wraps to the last item, which became "Share link" with FR-15.
    await user.keyboard("{ArrowUp}{ArrowUp}");
    expect(items[4]).toHaveFocus();
    await user.keyboard("{Home}");
    expect(items[0]).toHaveFocus();
    await user.keyboard("{End}");
    expect(items[4]).toHaveFocus();
  });

  it("copies the generated code with variables resolved and calls onCopied", async () => {
    const { user, onCopied, trigger } = setup();
    await user.click(trigger);
    await user.click(screen.getByRole("menuitem", { name: "Python requests" }));
    await waitFor(() => expect(onCopied).toHaveBeenCalledWith("Python requests"));
    const copied = await navigator.clipboard.readText();
    expect(copied).toBe(generateCode("python", formInput, { ...env, ...secrets }));
    expect(copied).toContain("secret-token");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("copies via the keyboard (Enter on a focused item)", async () => {
    const { user, onCopied, trigger } = setup();
    await user.click(trigger);
    await user.keyboard("{ArrowDown}{Enter}");
    await waitFor(() => expect(onCopied).toHaveBeenCalledWith("JavaScript fetch"));
    expect(await navigator.clipboard.readText()).toContain("await fetch(");
  });

  it("Esc closes the menu and returns focus to the trigger", async () => {
    const { user, trigger } = setup();
    await user.click(trigger);
    await screen.findByRole("menu");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });

  it("closes on a click outside", async () => {
    const { user, trigger, onCopied } = setup();
    await user.click(trigger);
    await screen.findByRole("menu");
    await user.click(screen.getByText("outside"));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(onCopied).not.toHaveBeenCalled();
  });

  it("Share link copies a #share= URL without secrets and toasts what was replaced", async () => {
    const { user, onCopied, trigger } = setup({
      ...formInput,
      headers: JSON.stringify({ Authorization: "Bearer abc", "X-Api-Key": "<<KEY>>" }),
    });
    await user.click(trigger);
    await user.click(screen.getByRole("menuitem", { name: "Share link" }));
    expect(await screen.findByText("Share link copied")).toBeInTheDocument();
    expect(screen.getByText(/Authorization was replaced with Bearer <<TOKEN>>/)).toBeInTheDocument();
    const copied = await navigator.clipboard.readText();
    expect(copied).toMatch(new RegExp(`^${window.location.origin}/hittable#share=`));
    const d = decodeShare(copied.split("#")[1]);
    expect(d.ok && JSON.parse(d.request.headers)).toEqual({ Authorization: "Bearer <<TOKEN>>", "X-Api-Key": "<<KEY>>" });
    expect(d.ok && d.request.url).toBe("https://<<HOST>>/users");
    expect(copied).not.toContain("secret-token");
    expect(onCopied).not.toHaveBeenCalled(); // its toast says variables are filled in, which a link never does
  });

  it("Share link shows an error toast when the request is too large", async () => {
    let body = "";
    for (let i = 0; body.length < 60_000; i++) body += Math.sin(i).toString(36);
    const { user, trigger } = setup({ ...formInput, body });
    await navigator.clipboard.writeText("before");
    await user.click(trigger);
    await user.click(screen.getByRole("menuitem", { name: "Share link" }));
    expect(await screen.findByText("Can't make a share link")).toBeInTheDocument();
    expect(await navigator.clipboard.readText()).toBe("before");
  });
});
