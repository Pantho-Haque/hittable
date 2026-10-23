// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import Menu from "@/components/hittable/Menu";
import { NotificationProvider } from "@/context/notifyContext";
import { DataProvider } from "@/context/dataContext";

function mount(props: Partial<Parameters<typeof Menu>[0]> = {}) {
  render(
    <NotificationProvider>
      <DataProvider>
      <Menu
        type="collection"
        currentName="Users"
        collectionCurlList={{ Users: [] }}
        setCollections={vi.fn()}
        setSelection={vi.fn()}
        {...props}
      />
      <p>elsewhere</p>
      </DataProvider>
    </NotificationProvider>,
  );
  return userEvent.setup();
}

const trigger = () => screen.getByRole("button", { name: "Actions for Users" });

describe("item ⋮ menu", () => {
  it("is a keyboard-reachable button that opens and closes", async () => {
    const user = mount();
    expect(trigger()).toHaveAttribute("aria-expanded", "false");
    await user.tab();
    expect(trigger()).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(trigger()).toHaveAttribute("aria-expanded", "true");
    await user.click(trigger());
    expect(trigger()).toHaveAttribute("aria-expanded", "false");
  });

  it("clicking outside closes it", async () => {
    const user = mount();
    await user.click(trigger());
    await user.click(screen.getByText("elsewhere"));
    expect(trigger()).toHaveAttribute("aria-expanded", "false");
  });

  it("offers Rename and Delete, and Export only when there is something to export", async () => {
    const user = mount({ type: "route" });
    await user.click(trigger());
    expect(screen.getByRole("button", { name: /Rename/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Delete/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Export/ })).not.toBeInTheDocument();
  });

  it("a collection menu can export", async () => {
    const user = mount({ exportString: JSON.stringify({ collectionName: "Users", items: [], env: {}, secrets: {} }) });
    await user.click(trigger());
    expect(screen.getByRole("button", { name: /Export/ })).toBeInTheDocument();
  });
});
