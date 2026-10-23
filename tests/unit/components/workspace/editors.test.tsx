// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { WorkspaceProvider, useWorkspace } from "@/context/workspaceContext";
import { NotificationProvider } from "@/context/notifyContext";
import { DataProvider } from "@/context/dataContext";
import HitFileEditor from "@/components/workspace/HitFileEditor";
import MarkdownEditor from "@/components/workspace/MarkdownEditor";
import PlainTextEditor from "@/components/workspace/PlainTextEditor";
import DirectoryPicker from "@/components/workspace/DirectoryPicker";
import WorkspaceConnecting from "@/components/workspace/WorkspaceConnecting";
import { FakeDir, FakeFile, asDir } from "../../helpers/fakeFs";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/services/workspace/HandleStore", () => ({
  saveDirectoryHandle: vi.fn(async () => {}),
  loadDirectoryHandle: vi.fn(async () => null),
  removeDirectoryHandle: vi.fn(async () => {}),
  verifyPermission: vi.fn(async () => true),
}));

const hit = (over: object = {}) =>
  JSON.stringify({ method: "GET", url: "<<BASE_URL>>/users", headers: { Authorization: "Bearer <<TOKEN>>" }, params: {}, body: "", response: null, ...over }, null, 2) + "\n";

let root: FakeDir;
let fetchMock: ReturnType<typeof vi.fn>;

function OpenFile({ path }: { path: string }) {
  const { openFolder, setActiveFile, hittableDir } = useWorkspace();
  useEffect(() => { openFolder(); }, [openFolder]);
  useEffect(() => {
    if (hittableDir) setActiveFile({ path: path.split("/"), handle: root.at(path) as unknown as FileSystemFileHandle });
  }, [hittableDir, path, setActiveFile]);
  return null;
}

function mount(path: string, editor: React.ReactNode) {
  render(
    <NotificationProvider>
      <DataProvider>
        <WorkspaceProvider>
          <OpenFile path={path} />
          {editor}
        </WorkspaceProvider>
      </DataProvider>
    </NotificationProvider>,
  );
  return userEvent.setup();
}

const file = (path: string) => root.at(path) as FakeFile;
const saved = (path: string) => JSON.parse(file(path).content);

beforeEach(() => {
  localStorage.clear();
  // jsdom has no layout; CodeMirror measures text ranges.
  Element.prototype.scrollIntoView = () => {};
  Range.prototype.getClientRects = () => [] as unknown as DOMRectList;
  Range.prototype.getBoundingClientRect = () => new DOMRect();
  root = new FakeDir("repo", {
    hittable: {
      "env.json": '{"BASE_URL":"https://api.example.com","TOKEN":"t0k"}',
      users: { "list.hit": hit(), "broken.hit": '{"method": "GET",' },
      notes: { "api.md": "# API\n\nUse **auth**." },
    },
    "todo.txt": "ship it",
  });
  window.showDirectoryPicker = vi.fn(async () => asDir(root)) as unknown as typeof window.showDirectoryPicker;
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe(".hit files: Text and Runner", () => {
  it("a valid file opens in the Runner, and Text shows the raw JSON", async () => {
    const user = mount("hittable/users/list.hit", <HitFileEditor />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Runner" })).toHaveAttribute("aria-pressed", "true"));
    await user.click(screen.getByRole("button", { name: "Text" }));
    await waitFor(() => expect(document.querySelector(".cm-content")?.textContent).toContain('"<<BASE_URL>>/users"'));
  });

  it("Runner shows the request from the file", async () => {
    const user = mount("hittable/users/list.hit", <HitFileEditor />);
    await waitFor(() => expect(screen.getByLabelText("Request URL")).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "Runner" }));
    expect(screen.getByRole("button", { name: "Runner" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByLabelText("Request URL")).toHaveValue("<<BASE_URL>>/users");
    expect(screen.getByLabelText("HTTP method")).toHaveValue("GET");
  });

  it("invalid JSON blocks the Runner until it is fixed in Text mode", async () => {
    const user = mount("hittable/users/broken.hit", <HitFileEditor />);
    await waitFor(() => expect(document.querySelector(".cm-content")?.textContent).toContain("method"));
    await user.click(screen.getByRole("button", { name: "Runner" }));
    expect(screen.getByText(/isn't valid Hittable JSON right now/)).toBeInTheDocument();
    expect(screen.queryByLabelText("Request URL")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Switch to Text mode to fix" }));
    expect(screen.getByRole("button", { name: "Text" })).toHaveAttribute("aria-pressed", "true");
  });

  it("edits in the Runner are saved to the file in .hit format", async () => {
    const user = mount("hittable/users/list.hit", <HitFileEditor />);
    await waitFor(() => expect(screen.getByLabelText("Request URL")).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "Runner" }));
    await user.selectOptions(screen.getByLabelText("HTTP method"), "DELETE");
    await waitFor(() => expect(saved("hittable/users/list.hit").method).toBe("DELETE"));
    const content = file("hittable/users/list.hit").content;
    expect(Object.keys(JSON.parse(content))).toEqual(["method", "url", "headers", "params", "body", "response"]);
    expect(content.endsWith("}\n")).toBe(true);
    // templates stay literal on disk
    expect(content).toContain('"<<BASE_URL>>/users"');
  });

  it("sending resolves variables from env.json and writes the response into the file", async () => {
    fetchMock.mockResolvedValue({ status: 200, ok: true, json: async () => ({ status: 200, statusText: "OK", ok: true, data: { id: 3 } }) });
    const user = mount("hittable/users/list.hit", <HitFileEditor />);
    await waitFor(() => expect(screen.getByLabelText("Request URL")).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "Runner" }));
    await user.click(screen.getByRole("button", { name: "Send" }));
    const sent = JSON.parse(fetchMock.mock.calls[0][1].body);
    expect(sent.url).toBe("https://api.example.com/users");
    expect(JSON.parse(sent.headers)).toEqual({ Authorization: "Bearer t0k" });
    await waitFor(() => expect(saved("hittable/users/list.hit").response).toMatchObject({ status: 200, data: { id: 3 } }));
    expect(saved("hittable/users/list.hit").url).toBe("<<BASE_URL>>/users");
  });
});

describe("other editors", () => {
  it("markdown notes render a preview and autosave edits to disk", async () => {
    const user = mount("hittable/notes/api.md", <MarkdownEditor />);
    expect(await screen.findByRole("heading", { name: "API" })).toBeInTheDocument();
    await user.click(screen.getByTitle(/edit/i));
    const area = screen.getByRole("textbox");
    await user.type(area, "!");
    await waitFor(() => expect(file("hittable/notes/api.md").content).toBe("# API\n\nUse **auth**.!"));
  });

  it("plain files open in the code editor", async () => {
    mount("todo.txt", <PlainTextEditor />);
    await waitFor(() => expect(document.querySelector(".cm-content")?.textContent).toBe("ship it"));
  });

  it("the picker invites you to choose a folder, then reports the workspace is ready", async () => {
    render(
      <NotificationProvider>
        <WorkspaceProvider>
          <DirectoryPicker />
        </WorkspaceProvider>
      </NotificationProvider>,
    );
    const user = userEvent.setup();
    expect(screen.getByRole("heading", { name: "Open a workspace" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Choose folder" }));
    expect(await screen.findByRole("heading", { name: "Your workspace is ready" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Change folder" })).toBeInTheDocument();
  });

  it("the connecting overlay names the stage", () => {
    const { rerender } = render(<WorkspaceConnecting stage="preparing" />);
    expect(screen.getByRole("status")).toHaveTextContent("Preparing workspace…");
    expect(screen.getByRole("status")).toHaveTextContent("Large folders take a moment to scan.");
    rerender(<WorkspaceConnecting stage="reconnecting" />);
    expect(screen.getByRole("status")).toHaveTextContent("Reconnecting to your workspace…");
    expect(screen.getByRole("status")).toHaveTextContent("Restoring the workspace you had open.");
  });
});
