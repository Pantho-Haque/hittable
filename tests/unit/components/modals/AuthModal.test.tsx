// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import AuthModal from "@/components/modals/AuthModal";
import { renderWithData } from "./renderWithData";

function setup(headers = '{\n\t"Accept": "application/json"\n}') {
  const utils = renderWithData(<AuthModal />, { formInput: { headers } });
  return { ...utils, user: userEvent.setup() };
}

const dialog = () => within(screen.getByRole("dialog"));

async function pick(user: ReturnType<typeof userEvent.setup>, preset: RegExp) {
  await user.click(screen.getByTitle("Auth"));
  await user.click(dialog().getByRole("button", { name: preset }));
}

beforeEach(() => localStorage.clear());

describe("AuthModal", () => {
  it("adds a Bearer token header and keeps the other headers", async () => {
    const { user, ctx } = setup();
    await pick(user, /^Bearer Token/);
    await user.type(screen.getByPlaceholderText(/eyJhbGci/), "tok-123");
    await user.click(dialog().getByRole("button", { name: "Apply" }));
    expect(JSON.parse(ctx().formInput.headers)).toEqual({ Accept: "application/json", Authorization: "Bearer tok-123" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("encodes Basic auth as base64 of user:pass", async () => {
    const { user, ctx } = setup();
    await pick(user, /^Basic Auth/);
    await user.type(screen.getByPlaceholderText("admin"), "aladdin");
    await user.type(screen.getByPlaceholderText("••••••••"), "opensesame");
    await user.click(dialog().getByRole("button", { name: "Apply" }));
    expect(JSON.parse(ctx().formInput.headers).Authorization).toBe("Basic YWxhZGRpbjpvcGVuc2VzYW1l");
  });

  it("encodes non-ASCII Basic credentials as UTF-8 instead of crashing", async () => {
    const { user, ctx } = setup();
    await pick(user, /^Basic Auth/);
    await user.type(screen.getByPlaceholderText("admin"), "jürgen");
    await user.type(screen.getByPlaceholderText("••••••••"), "pässwörd✓");
    await user.click(dialog().getByRole("button", { name: "Apply" }));
    const value = JSON.parse(ctx().formInput.headers).Authorization as string;
    expect(Buffer.from(value.replace("Basic ", ""), "base64").toString("utf8")).toBe("jürgen:pässwörd✓");
  });

  it("masks the Basic auth password", async () => {
    const { user } = setup();
    await pick(user, /^Basic Auth/);
    expect(screen.getByPlaceholderText("••••••••")).toHaveAttribute("type", "password");
  });

  it("adds an API key under X-API-Key by default", async () => {
    const { user, ctx } = setup();
    await pick(user, /^API Key/);
    expect(screen.getByPlaceholderText("X-API-Key")).toHaveValue("X-API-Key");
    await user.type(screen.getByPlaceholderText("your-api-key"), "k-1");
    await user.click(dialog().getByRole("button", { name: "Apply" }));
    expect(JSON.parse(ctx().formInput.headers)).toEqual({ Accept: "application/json", "X-API-Key": "k-1" });
  });

  it("uses a custom API key header name", async () => {
    const { user, ctx } = setup();
    await pick(user, /^API Key/);
    const name = screen.getByPlaceholderText("X-API-Key");
    await user.clear(name);
    await user.type(name, "api-token");
    await user.type(screen.getByPlaceholderText("your-api-key"), "k-2");
    await user.click(dialog().getByRole("button", { name: "Apply" }));
    expect(JSON.parse(ctx().formInput.headers)).toEqual({ Accept: "application/json", "api-token": "k-2" });
  });

  it("replaces an existing Authorization header whatever its case", async () => {
    const { user, ctx } = setup('{"authorization":"Bearer old","Accept":"*/*"}');
    await pick(user, /^Bearer Token/);
    await user.type(screen.getByPlaceholderText(/eyJhbGci/), "new");
    await user.click(dialog().getByRole("button", { name: "Apply" }));
    expect(JSON.parse(ctx().formInput.headers)).toEqual({ Accept: "*/*", Authorization: "Bearer new" });
  });

  it("switching from Bearer to API key removes the old Authorization header", async () => {
    const { user, ctx } = setup('{"Authorization":"Bearer old"}');
    await pick(user, /^API Key/);
    await user.type(screen.getByPlaceholderText("your-api-key"), "k");
    await user.click(dialog().getByRole("button", { name: "Apply" }));
    expect(JSON.parse(ctx().formInput.headers)).toEqual({ "X-API-Key": "k" });
  });

  it("does not add an empty Bearer header", async () => {
    const { user, ctx } = setup();
    await pick(user, /^Bearer Token/);
    await user.click(dialog().getByRole("button", { name: "Apply" }));
    expect(JSON.parse(ctx().formInput.headers)).toEqual({ Accept: "application/json" });
  });

  it("Clear Auth removes auth headers and keeps the rest", async () => {
    const { user, ctx } = setup('{"Authorization":"Bearer x","X-API-Key":"k","Accept":"*/*"}');
    await user.click(screen.getByTitle("Auth"));
    await user.click(dialog().getByRole("button", { name: "Clear Auth" }));
    expect(JSON.parse(ctx().formInput.headers)).toEqual({ Accept: "*/*" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("Cancel leaves the headers untouched", async () => {
    const { user, ctx } = setup('{"Accept":"*/*"}');
    await pick(user, /^Bearer Token/);
    await user.type(screen.getByPlaceholderText(/eyJhbGci/), "t");
    await user.click(dialog().getByRole("button", { name: "Cancel" }));
    expect(ctx().formInput.headers).toBe('{"Accept":"*/*"}');
  });

  it("explains that No Auth adds nothing", async () => {
    const { user } = setup();
    await user.click(screen.getByTitle("Auth"));
    expect(dialog().getByText("No authentication will be added to requests")).toBeInTheDocument();
  });
});
