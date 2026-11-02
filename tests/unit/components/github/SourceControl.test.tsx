// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SourceControlPanel from "@/components/github/SourceControlPanel";
import { BranchSwitcher, SyncPill } from "@/components/github/RepoHeader";
import { WorkspaceProvider } from "@/context/workspaceContext";
import { NotificationProvider } from "@/context/notifyContext";
import { createGitHubBackend, type GitHubBackend } from "@/services/github/githubBackend";
import { memoryStore } from "@/services/github/workingCopy";
import { fakeGitHub, META } from "../../helpers/fakeGitHubClient";

const push = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

// The branch switcher lists branches through the api module; point it at the fake.
const ghRef = vi.hoisted(() => ({ current: null as null | { branches: () => Promise<unknown> } }));
vi.mock("@/utils/github/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/utils/github/api")>()),
  api: { branches: () => ghRef.current!.branches() },
}));

const SEED = { "README.md": "# hi\n", "hittable/env.json": "{}\n", "hittable/users/get.hit": '{"method":"GET"}\n' };
let gh: ReturnType<typeof fakeGitHub>;
let b: GitHubBackend;
const onOpenDiff = vi.fn();

async function setup(branch = "main", meta: Partial<typeof META> = {}) {
  b = await createGitHubBackend({ owner: "octo", repo: "api-tests", branch, meta: { ...META, ...meta }, client: gh.client, kv: memoryStore() });
  ghRef.current = { branches: () => gh.client.branches("octo", "api-tests") };
  render(
    <NotificationProvider>
      <WorkspaceProvider>
        <SyncPill backend={b} onCompare={onOpenDiff} />
        <BranchSwitcher backend={b} />
        <SourceControlPanel backend={b} onOpenDiff={onOpenDiff} onClose={vi.fn()} />
      </WorkspaceProvider>
    </NotificationProvider>,
  );
  return userEvent.setup();
}

const panel = () => screen.getByRole("complementary", { name: "Source control" });
// Each row: the status letter shown, and the button's accessible name.
const changed = () =>
  within(panel()).queryAllByRole("listitem").map((li) => {
    const open = within(li).getAllByRole("button")[0];
    return `${open.textContent?.[0]} ${open.getAttribute("aria-label")}`;
  });

beforeEach(() => {
  gh = fakeGitHub(SEED, { protectedBranches: ["protected"] });
  push.mockReset();
  onOpenDiff.mockReset();
  vi.spyOn(window, "open").mockImplementation(() => null);
});

describe("SourceControlPanel", () => {
  it("lists the changes with their status; a click opens the diff", async () => {
    const user = await setup();
    await b.writeFile(["hittable", "users", "get.hit"], "edited");
    await b.remove(["README.md"]);
    await waitFor(() => expect(changed()).toHaveLength(2));
    expect(changed()).toEqual(["D README.md, Deleted", "M hittable/users/get.hit, Modified"]);
    await user.click(screen.getByRole("button", { name: /hittable\/users\/get\.hit, Modified/ }));
    expect(onOpenDiff).toHaveBeenCalledWith("hittable/users/get.hit");
    // The prefill follows the changes.
    expect(screen.getByRole("textbox", { name: "Commit message" })).toHaveValue("Update 2 files\n\n- Delete README.md\n- Update hittable/users/get.hit");
  });

  it("discard asks first, then restores the file", async () => {
    const user = await setup();
    await b.writeFile(["README.md"], "edited");
    await user.click(await screen.findByRole("button", { name: "Discard changes to README.md" }));
    const dialog = screen.getByRole("dialog", { name: /Discard changes to README.md/ });
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(b.status()).toEqual({ "README.md": "M" });

    await user.click(screen.getByRole("button", { name: "Discard changes to README.md" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Discard" }));
    await waitFor(() => expect(b.status()).toEqual({}));
    expect(screen.getByText(/No changes/)).toBeInTheDocument();
  });

  it("Discard all asks first", async () => {
    const user = await setup();
    await b.writeFile(["README.md"], "edited");
    await b.createFile(["new.txt"], "x");
    await user.click(await screen.findByRole("button", { name: "Discard all" }));
    await user.click(within(screen.getByRole("dialog", { name: "Discard all 2 changes?" })).getByRole("button", { name: "Discard" }));
    await waitFor(() => expect(b.changes()).toEqual([]));
  });

  it("commits, clears the changes and toasts the short SHA with View on GitHub", async () => {
    const user = await setup();
    await b.writeFile(["README.md"], "edited");
    const commit = await screen.findByRole("button", { name: /Commit to main/ });
    await user.click(commit);
    const toast = await screen.findByText(/^Committed [0-9a-f]{7} to main$/);
    expect(gh.files()["README.md"]).toBe("edited");
    expect(gh.committed[0].message).toBe("Update README.md");
    await user.click(within(toast.closest("[role=status]") as HTMLElement).getByRole("button", { name: "View on GitHub" }));
    expect(window.open).toHaveBeenCalledWith(expect.stringMatching(/^https:\/\/github.com\/o\/r\/commit\//), "_blank", "noopener,noreferrer");
    expect(b.changes()).toEqual([]);
    expect(screen.getByRole("button", { name: /Commit to main/ })).toBeDisabled();
  });

  it("head moved on the same file: marked C, nothing overwritten, then Keep mine / Take theirs", async () => {
    const user = await setup();
    await b.writeFile(["README.md"], "mine");
    gh.push({ "README.md": "theirs" });
    await user.click(await screen.findByRole("button", { name: /Commit to main/ }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/changed the same files/);
    expect(gh.files()["README.md"]).toBe("theirs");
    expect(changed()).toEqual(["C README.md, Conflict"]);
    expect(screen.getByRole("button", { name: /Commit to main/ })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "1 conflict" }));
    await user.click(screen.getByRole("button", { name: "Compare README.md" }));
    expect(onOpenDiff).toHaveBeenCalledWith("README.md");
    await user.click(screen.getByRole("button", { name: "1 conflict" }));
    await user.click(screen.getByRole("button", { name: "Take theirs" }));
    await waitFor(() => expect(b.changes()).toEqual([]));
    expect(screen.getByRole("button", { name: /Up to date/ })).toBeInTheDocument();
  });

  it("head moved elsewhere: the commit goes on top of the new head", async () => {
    const user = await setup();
    await b.writeFile(["README.md"], "mine");
    gh.push({ "hittable/env.json": '{"a":1}\n' });
    await user.click(await screen.findByRole("button", { name: /Commit to main/ }));
    await screen.findByText(/^Committed/);
    expect(gh.files()).toMatchObject({ "README.md": "mine", "hittable/env.json": '{"a":1}\n' });
  });

  it("a protected branch offers the pull request path with the same changes", async () => {
    gh.branches.set("protected", gh.branches.get("main")!);
    const user = await setup("protected");
    await b.writeFile(["README.md"], "via PR");
    await user.click(await screen.findByRole("button", { name: /Commit to protected/ }));
    const dialog = await screen.findByRole("dialog", { name: /new branch & open a pull request/ });
    expect(within(dialog).getByText(/protected is protected/)).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: /New branch/ })).toHaveValue("hittable/update-readme-md");
    expect(within(dialog).getByRole("textbox", { name: "Pull request title" })).toHaveValue("Update README.md");

    gh.branches.set("hittable/update-readme-md", gh.branches.get("main")!); // taken
    await user.click(within(dialog).getByRole("button", { name: /Commit & open pull request/ }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(/already exists/);
    expect(within(dialog).getByRole("textbox", { name: /New branch/ })).toHaveValue("hittable/update-readme-md-2");

    await user.click(within(dialog).getByRole("button", { name: /Commit & open pull request/ }));
    expect(await screen.findByText("Opened pull request #1")).toBeInTheDocument();
    expect(gh.pulls[0]).toMatchObject({ head: "hittable/update-readme-md-2", base: "protected" });
    expect(gh.files("hittable/update-readme-md-2")["README.md"]).toBe("via PR");
    expect(b.changes()).toEqual([]);
  });

  it("read-only: no commit box", async () => {
    await setup("main", { canPush: false });
    expect(screen.queryByRole("button", { name: /Commit to/ })).toBeNull();
    expect(screen.getByText(/can't push to this repository/)).toBeInTheDocument();
  });

  it("env.local.json is shown as local only and never listed", async () => {
    await setup();
    await b.createFile(["hittable", "env.local.json"], "{}");
    expect(await screen.findByText(/is local only/)).toBeInTheDocument();
    expect(changed()).toEqual([]);
  });
});

describe("BranchSwitcher", () => {
  it("searches and switches with the keyboard", async () => {
    gh.branches.set("dev", gh.branches.get("main")!);
    const user = await setup();
    await user.click(screen.getByRole("button", { name: /Branch: main/ }));
    const input = screen.getByRole("combobox", { name: /Find or create a branch/ });
    await screen.findByRole("option", { name: /dev/ });
    await user.type(input, "dev");
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([expect.stringContaining("dev")]);
    await user.keyboard("{Enter}");
    expect(push).toHaveBeenCalledWith("/hittable/gh/octo/api-tests?ref=dev");
  });

  it("creates a branch from the current commit and switches to it", async () => {
    const user = await setup();
    await b.writeFile(["README.md"], "wip");
    await user.click(screen.getByRole("button", { name: /Branch: main/ }));
    await screen.findByRole("option", { name: /main/ });
    await user.type(screen.getByRole("combobox"), "feature/x");
    await user.click(screen.getByRole("option", { name: /Create branch feature\/x from main/ }));
    await waitFor(() => expect(push).toHaveBeenCalledWith("/hittable/gh/octo/api-tests?ref=feature%2Fx"));
    expect(gh.branches.get("feature/x")).toBe(b.baseSha());
  });

  it("Escape closes it", async () => {
    const user = await setup();
    await user.click(screen.getByRole("button", { name: /Branch: main/ }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "Switch branch" })).toBeNull();
    expect(screen.getByRole("button", { name: /Branch: main/ })).toHaveFocus();
  });
});
