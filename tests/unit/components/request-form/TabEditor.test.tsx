// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import RequestForm from "@/components/hittable/RequestForm";
import { mountData } from "../../helpers/dataHarness";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

async function setup(route = "List users", folderPath: string[] = []) {
  const user = userEvent.setup();
  const r = mountData(<RequestForm />, { route, folderPath });
  await waitFor(() => expect(screen.getByLabelText("Request URL")).not.toHaveValue(""));
  return { user, ...r };
}

const tab = (name: "Params" | "Body" | "Headers") => screen.getByRole("tab", { name });
const editor = (t: "params" | "body" | "headers") => screen.getByLabelText(`${t} JSON`) as HTMLTextAreaElement;
const send = () => screen.getByRole("button", { name: "Send" });

// user-event treats { and [ as key descriptors; paste raw text instead.
async function replace(user: ReturnType<typeof userEvent.setup>, el: HTMLElement, text: string) {
  await user.clear(el);
  if (text) {
    el.focus();
    await user.paste(text);
  }
}

beforeEach(() => localStorage.clear());

describe("Params / Body / Headers editor", () => {
  it("opens on Params and switches tabs", async () => {
    const { user } = await setup();
    expect(editor("params")).toBeInTheDocument();
    await user.click(tab("Headers"));
    expect(JSON.parse(editor("headers").value)).toEqual({ Authorization: "Bearer <<TOKEN>>" });
    await user.click(tab("Body"));
    expect(editor("body")).toBeInTheDocument();
  });

  it("shows one line number per line", async () => {
    const { user } = await setup();
    await user.click(tab("Headers"));
    const lines = editor("headers").value.split("\n").length;
    const gutter = editor("headers").previousElementSibling as HTMLElement;
    expect(within(gutter).getAllByText(/^\d+$/)).toHaveLength(lines);
  });

  it("invalid JSON shows the parse error and disables Send; fixing it clears both", async () => {
    const { user } = await setup();
    await user.click(tab("Headers"));
    await replace(user, editor("headers"), '{"a": ');
    expect(screen.getByText(/JSON|Unexpected|position/i, { selector: "span.truncate" })).toBeInTheDocument();
    expect(send()).toBeDisabled();
    await replace(user, editor("headers"), '{"a": "1"}');
    expect(screen.queryByText(/JSON|Unexpected|position/i, { selector: "span.truncate" })).not.toBeInTheDocument();
    expect(send()).toBeEnabled();
  });

  it("an empty tab is valid", async () => {
    const { user } = await setup();
    await user.click(tab("Body"));
    await replace(user, editor("body"), "");
    expect(send()).toBeEnabled();
  });

  it("a JSON error in one tab keeps Send disabled after fixing another tab", async () => {
    const { user } = await setup();
    await user.click(tab("Body"));
    await replace(user, editor("body"), "{broken");
    expect(send()).toBeDisabled();
    await user.click(tab("Params"));
    await replace(user, editor("params"), '{"page": "2"}');
    expect(send()).toBeDisabled();
  });

  it("editing Params rewrites the URL query string, encoded", async () => {
    const { user } = await setup();
    await replace(user, editor("params"), '{"q": "a b", "page": "2"}');
    expect(screen.getByLabelText("Request URL")).toHaveValue("https://<<HOST>>/users?q=a%20b&page=2");
    await replace(user, editor("params"), "{}");
    expect(screen.getByLabelText("Request URL")).toHaveValue("https://<<HOST>>/users");
  });

  it("typing a query string into the URL updates the Params tab", async () => {
    const { user } = await setup();
    const url = screen.getByLabelText("Request URL");
    await user.type(url, "?limit=5");
    expect(JSON.parse(editor("params").value)).toEqual({ limit: "5" });
  });

  // Beautify moved from Ctrl/Cmd+J (the browser's Downloads shortcut) to
  // Shift+Alt+F, the VS Code format chord (FR-2).
  it("Beautify (and Shift+Alt+F) formats the active tab's JSON; Ctrl+J no longer does", async () => {
    const { user, ctx } = await setup();
    await user.click(tab("Body"));
    await replace(user, editor("body"), '{"a":1,"b":[1,2]}');
    await user.click(screen.getByRole("button", { name: "Beautify JSON (Shift+Alt+F)" }));
    expect(editor("body").value).toBe('{\n\t"a": 1,\n\t"b": [\n\t\t1,\n\t\t2\n\t]\n}');

    await replace(user, editor("body"), '{"c":true}');
    await user.keyboard("{Control>}j{/Control}");
    expect(ctx().formInput.body).toBe('{"c":true}');
    await user.keyboard("{Shift>}{Alt>}f{/Alt}{/Shift}");
    expect(ctx().formInput.body).toBe('{\n\t"c": true\n}');
  });

  it("Beautify leaves invalid JSON untouched", async () => {
    const { user } = await setup();
    await user.click(tab("Body"));
    await replace(user, editor("body"), '{"a":');
    await user.keyboard("{Shift>}{Alt>}f{/Alt}{/Shift}");
    expect(editor("body").value).toBe('{"a":');
    expect(send()).toBeDisabled();
  });

  describe("table mode", () => {
    it("shows each key/value as a row and editing a value updates the JSON", async () => {
      const { user, ctx } = await setup();
      await user.click(tab("Headers"));
      await user.click(screen.getByTitle("Table mode"));
      expect(screen.getByDisplayValue("Authorization")).toBeInTheDocument();
      const value = screen.getByDisplayValue("Bearer <<TOKEN>>");
      await user.clear(value);
      await user.type(value, "Basic abc");
      expect(JSON.parse(ctx().formInput.headers)).toEqual({ Authorization: "Basic abc" });
    });

    it("Add row and remove row; rows with an empty key are not saved", async () => {
      const { user, ctx } = await setup();
      await user.click(tab("Headers"));
      await user.click(screen.getByTitle("Table mode"));
      await user.click(screen.getByRole("button", { name: "Add row" }));
      const keys = screen.getAllByPlaceholderText("Header name");
      expect(keys).toHaveLength(2);
      await user.type(screen.getAllByPlaceholderText("Header value")[1], "orphan");
      expect(JSON.parse(ctx().formInput.headers)).toEqual({ Authorization: "Bearer <<TOKEN>>" });
      await user.type(keys[1], "X-Trace");
      expect(JSON.parse(ctx().formInput.headers)).toEqual({ Authorization: "Bearer <<TOKEN>>", "X-Trace": "orphan" });

      await user.click(screen.getAllByTitle("Remove row")[0]);
      expect(JSON.parse(ctx().formInput.headers)).toEqual({ "X-Trace": "orphan" });
    });

    it("Params rows rewrite the URL", async () => {
      const { user } = await setup();
      await user.click(screen.getByTitle("Table mode"));
      // empty params show one blank row
      await user.type(screen.getByPlaceholderText("Param name"), "sort");
      await user.type(screen.getByPlaceholderText("Value"), "name");
      expect(screen.getByLabelText("Request URL")).toHaveValue("https://<<HOST>>/users?sort=name");
    });

    it("switching tabs in table mode shows that tab's rows; JSON mode shows the result", async () => {
      const { user } = await setup("Create user", ["Admin"]);
      await user.click(screen.getByTitle("Table mode"));
      await user.click(tab("Body"));
      expect(screen.getByDisplayValue("name")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Ada")).toBeInTheDocument();
      await user.click(screen.getByTitle("JSON mode"));
      expect(JSON.parse(editor("body").value)).toEqual({ name: "Ada" });
    });
  });

  describe("body types (FR-4)", () => {
    it("a Form body is edited as rows and stored URL-encoded with its Content-Type", async () => {
      const { user, ctx } = await setup();
      await user.click(tab("Body"));
      await user.click(screen.getByRole("radio", { name: "Form" }));
      expect(screen.getByRole("radio", { name: "Form" })).toHaveAttribute("aria-checked", "true");
      expect(JSON.parse(ctx().formInput.headers)).toMatchObject({ "Content-Type": "application/x-www-form-urlencoded" });
      const [key] = screen.getAllByPlaceholderText("Field");
      const [value] = screen.getAllByPlaceholderText("Value");
      await user.clear(key);
      await user.type(key, "b");
      await user.clear(value);
      await user.type(value, "x y");
      expect(ctx().formInput.body).toBe("b=x+y");
    });

    it("a Text body is sent as written and never blocks Send", async () => {
      const { user, ctx } = await setup();
      await user.click(tab("Body"));
      await user.click(screen.getByRole("radio", { name: "Text" }));
      const body = screen.getByLabelText("body text");
      await user.clear(body);
      await user.type(body, "not {{json");
      expect(ctx().formInput.body).toBe("not {json"); // user-event types "{{" as "{"
      expect(send()).toBeEnabled();
    });

    it("None clears the body", async () => {
      const { user, ctx } = await setup();
      await user.click(tab("Body"));
      await user.click(screen.getByRole("radio", { name: "None" }));
      expect(ctx().formInput.body).toBe("");
      expect(screen.getByText(/This request has no body/)).toBeInTheDocument();
    });
  });

  it("tabs follow the ARIA tabs pattern: one selected, arrow keys move", async () => {
    const { user } = await setup();
    expect(tab("Params")).toHaveAttribute("aria-selected", "true");
    tab("Params").focus();
    await user.keyboard("{ArrowRight}");
    expect(tab("Body")).toHaveAttribute("aria-selected", "true");
    expect(tab("Body")).toHaveFocus();
    await user.keyboard("{ArrowLeft}{ArrowLeft}");
    expect(tab("Headers")).toHaveAttribute("aria-selected", "true");
  });
});
