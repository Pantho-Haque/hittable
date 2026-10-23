// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import EnvModal from "@/components/modals/EnvModal";
import type { THittableCollections } from "@/types";
import { renderWithData, stored } from "./renderWithData";

const seed: THittableCollections = [
  { collectionName: "api", env: { BASE: "https://x.dev", TOKEN: "abc" }, secrets: { PASS: "hunter2" }, items: [] },
  { collectionName: "empty", env: {}, secrets: {}, items: [] },
];

function setup(collectionName = "api") {
  const utils = renderWithData(<EnvModal collectionName={collectionName} />, {
    collections: seed,
    selector: {
      collectionName,
      folderPath: [],
      curlName: "r",
      env: {},
      secrets: {},
      curlJson: { method: "GET", url: "", headers: "{}", body: "{}", params: "{}" },
    },
  });
  return { ...utils, user: userEvent.setup() };
}

const keys = () => screen.getAllByPlaceholderText("KEY").map((i) => (i as HTMLInputElement).value);
const values = () => screen.getAllByPlaceholderText("value").map((i) => (i as HTMLInputElement).value);
const save = () => screen.getByRole("button", { name: /Save/ });

beforeEach(() => localStorage.clear());

describe("EnvModal", () => {
  it("lists the collection's variables and explains the <<KEY>> syntax", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Environment Variables"));
    expect(screen.getByRole("dialog", { name: "Environment: api" })).toBeInTheDocument();
    expect(screen.getByText("<<KEY>>")).toBeInTheDocument();
    expect(keys()).toEqual(["BASE", "TOKEN"]);
    expect(values()).toEqual(["https://x.dev", "abc"]);
  });

  it("saves an edited value and a new variable, and the open request sees them", async () => {
    const { user, ctx } = setup();
    await user.click(screen.getByTitle("Environment Variables"));
    const [, token] = screen.getAllByPlaceholderText("value");
    await user.clear(token);
    await user.type(token, "xyz");
    await user.click(screen.getByRole("button", { name: "Add" }));
    await user.type(screen.getAllByPlaceholderText("KEY")[2], "NEW");
    await user.type(screen.getAllByPlaceholderText("value")[2], "1");
    await user.click(save());

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(stored()[0].env).toEqual({ BASE: "https://x.dev", TOKEN: "xyz", NEW: "1" });
    expect(stored()[0].secrets).toEqual({ PASS: "hunter2" });
    expect(ctx().selectorResponse?.env).toEqual({ BASE: "https://x.dev", TOKEN: "xyz", NEW: "1" });
  });

  it("keeps secrets in their own tab and saves them separately", async () => {
    const { user, ctx } = setup();
    await user.click(screen.getByTitle("Environment Variables"));
    await user.click(screen.getByRole("button", { name: "Secrets" }));
    expect(screen.getByText(/Secrets are not resolved during export/)).toBeInTheDocument();
    expect(keys()).toEqual(["PASS"]);
    await user.click(screen.getByRole("button", { name: "Add" }));
    await user.type(screen.getAllByPlaceholderText("KEY")[1], "API_KEY");
    await user.type(screen.getAllByPlaceholderText("value")[1], "k-1");
    await user.click(save());
    expect(stored()[0].secrets).toEqual({ PASS: "hunter2", API_KEY: "k-1" });
    expect(stored()[0].env).toEqual({ BASE: "https://x.dev", TOKEN: "abc" });
    expect(ctx().selectorResponse?.secrets).toEqual({ PASS: "hunter2", API_KEY: "k-1" });
  });

  it("removes a variable with its row's delete button", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Environment Variables"));
    await user.click(screen.getByRole("button", { name: "Remove BASE" }));
    expect(keys()).toEqual(["TOKEN"]);
    await user.click(save());
    expect(stored()[0].env).toEqual({ TOKEN: "abc" });
  });

  it("drops rows whose key is blank", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Environment Variables"));
    await user.click(screen.getByRole("button", { name: "Add" }));
    await user.type(screen.getAllByPlaceholderText("value")[2], "orphan");
    await user.click(save());
    expect(stored()[0].env).toEqual({ BASE: "https://x.dev", TOKEN: "abc" });
  });

  it("saves with Ctrl/Cmd+S", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Environment Variables"));
    await user.click(screen.getByRole("button", { name: "Remove TOKEN" }));
    await user.keyboard("{Control>}s{/Control}");
    expect(stored()[0].env).toEqual({ BASE: "https://x.dev" });
    await user.click(screen.getByTitle("Environment Variables"));
    await user.click(screen.getByRole("button", { name: "Remove BASE" }));
    await user.keyboard("{Meta>}s{/Meta}");
    expect(stored()[0].env).toEqual({});
  });

  it("discards edits on Cancel, Escape and backdrop click", async () => {
    const { user } = setup();
    for (const close of [
      () => user.click(screen.getByRole("button", { name: "Cancel" })),
      () => user.keyboard("{Escape}"),
      async () => fireEvent.click(screen.getByRole("dialog").parentElement!),
    ]) {
      await user.click(screen.getByTitle("Environment Variables"));
      await user.click(screen.getByRole("button", { name: "Remove BASE" }));
      await close();
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    }
    expect(stored()[0].env).toEqual({ BASE: "https://x.dev", TOKEN: "abc" });
    // Reopening shows the saved state, not the abandoned edit.
    await user.click(screen.getByTitle("Environment Variables"));
    expect(keys()).toEqual(["BASE", "TOKEN"]);
  });

  it("shows empty states for a collection with no variables", async () => {
    const { user } = setup("empty");
    await user.click(screen.getByTitle("Environment Variables"));
    expect(screen.getByText("No environment variables yet")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Secrets" }));
    expect(screen.getByText("No secrets yet")).toBeInTheDocument();
  });

  it("only changes the collection it was opened for", async () => {
    const { user } = setup("empty");
    await user.click(screen.getByTitle("Environment Variables"));
    await user.click(screen.getByRole("button", { name: "Add" }));
    await user.type(screen.getByPlaceholderText("KEY"), "X");
    await user.click(save());
    expect(stored()[1].env).toEqual({ X: "" });
    expect(stored()[0].env).toEqual(seed[0].env);
  });
});
