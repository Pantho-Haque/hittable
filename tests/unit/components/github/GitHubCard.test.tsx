// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import GitHubCard, { relativeTime } from "@/components/launcher/GitHubCard";
import { GitHubError, type RepoMeta } from "@/utils/github/api";
import { rememberRecent } from "@/utils/github/local";

const push = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

const gh = vi.hoisted(() => ({ session: vi.fn(), repos: vi.fn() }));
vi.mock("@/utils/github/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/utils/github/api")>()),
  api: gh,
}));

const repo = (fullName: string, extra: Partial<RepoMeta> = {}): RepoMeta => {
  const [owner, name] = fullName.split("/");
  return {
    fullName, owner, name, private: false, fork: false, archived: false, canPush: true,
    defaultBranch: "main", pushedAt: new Date(Date.now() - 3 * 86400_000).toISOString(), description: null, ...extra,
  };
};

const REPOS = [
  repo("octo/api-tests", { private: true, description: "Requests for the users API" }),
  repo("octo/website"),
  repo("acme/shared", { canPush: false }),
  repo("octo/old-archive", { archived: true }),
  repo("octo/forked", { fork: true }),
];

beforeEach(() => {
  localStorage.clear();
  push.mockReset();
  gh.session.mockResolvedValue({ login: "octo", name: "Octo", avatarUrl: "" });
  gh.repos.mockResolvedValue(REPOS);
});

const options = () => screen.getAllByRole("option").map((o) => o.textContent ?? "");

describe("GitHubCard", () => {
  it("signed out: the sign-in pitch", async () => {
    gh.session.mockResolvedValue(null);
    render(<GitHubCard />);
    expect(screen.getByRole("link", { name: "Sign in with GitHub" })).toHaveAttribute("href", "/api/github/login?next=/hittable");
    await Promise.resolve();
    expect(gh.repos).not.toHaveBeenCalled();
  });

  it("signed in: lists repositories with badges and when they were pushed", async () => {
    render(<GitHubCard />);
    await screen.findByRole("listbox", { name: "Repositories" });
    expect(options()).toHaveLength(5);
    const api = screen.getByRole("option", { name: /octo\/api-tests/ });
    expect(within(api).getByText("Private")).toBeInTheDocument();
    expect(within(api).getByText(/pushed 3 days ago/)).toBeInTheDocument();
    expect(within(screen.getByRole("option", { name: /acme\/shared/ })).getByText("Read-only")).toBeInTheDocument();
    expect(within(screen.getByRole("option", { name: /old-archive/ })).getByText("Archived")).toBeInTheDocument();
    expect(within(screen.getByRole("option", { name: /forked/ })).getByText("Fork")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Grant access on GitHub/ })).toHaveAttribute("href", "https://github.com/settings/applications");
  });

  it("search narrows the list; the owner chips filter", async () => {
    const user = userEvent.setup();
    render(<GitHubCard />);
    await screen.findByRole("listbox");
    await user.type(screen.getByRole("combobox", { name: "Search repositories" }), "web");
    expect(options()).toEqual([expect.stringContaining("octo/website")]);
    await user.clear(screen.getByRole("combobox"));
    await user.type(screen.getByRole("combobox"), "zzzz");
    expect(screen.getByText(/No repositories match/)).toBeInTheDocument();
    await user.clear(screen.getByRole("combobox"));

    await user.click(screen.getByRole("button", { name: "acme" }));
    expect(screen.getByRole("button", { name: "acme" })).toHaveAttribute("aria-pressed", "true");
    expect(options()).toEqual([expect.stringContaining("acme/shared")]);
  });

  it("Recent lists the last repository opened first", async () => {
    rememberRecent("octo/website");
    rememberRecent("acme/shared");
    render(<GitHubCard />);
    const recent = await screen.findByRole("group", { name: "Recent" });
    expect(within(recent).getAllByRole("option").map((o) => o.textContent)).toEqual([
      expect.stringContaining("acme/shared"),
      expect.stringContaining("octo/website"),
    ]);
  });

  it("is keyboard operable: arrows move, Enter opens", async () => {
    const user = userEvent.setup();
    render(<GitHubCard />);
    await screen.findByRole("listbox");
    const input = screen.getByRole("combobox");
    await user.click(input);
    await user.keyboard("{ArrowDown}");
    expect(screen.getAllByRole("option")[1]).toHaveAttribute("aria-selected", "true");
    expect(input).toHaveAttribute("aria-activedescendant", screen.getAllByRole("option")[1].id);
    await user.keyboard("{Enter}");
    expect(push).toHaveBeenCalledWith("/hittable/gh/octo/website");
  });

  it("an error offers Retry; signed out falls back to the pitch", async () => {
    localStorage.setItem("hittable_gh_signed_in", "1");
    gh.repos.mockRejectedValueOnce(new GitHubError("upstream", 502, "GitHub is down"));
    const user = userEvent.setup();
    render(<GitHubCard />);
    expect(await screen.findByRole("alert")).toHaveTextContent("GitHub is down");
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("listbox")).toBeInTheDocument();
    expect(gh.repos).toHaveBeenLastCalledWith(true);
  });

  it("a signed-out answer while listing shows the sign-in pitch", async () => {
    localStorage.setItem("hittable_gh_signed_in", "1");
    gh.repos.mockRejectedValue(new GitHubError("signed_out", 401));
    render(<GitHubCard />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading repositories…");
    expect(await screen.findByRole("link", { name: "Sign in with GitHub" })).toBeInTheDocument();
  });

  it("relativeTime", () => {
    const now = Date.UTC(2026, 10, 1);
    expect(relativeTime(new Date(now - 3 * 86400_000).toISOString(), now)).toBe("3 days ago");
    expect(relativeTime(new Date(now - 86400_000).toISOString(), now)).toBe("yesterday");
    expect(relativeTime(new Date(now - 5000).toISOString(), now)).toBe("just now");
  });
});
