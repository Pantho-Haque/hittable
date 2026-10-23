// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ExportModal from "@/components/modals/ExportModal";
import { decompressString } from "@/utils/compressString";
import type { THittableCollection } from "@/types";

const SAVED = '{"data":"SAVED-RESPONSE-BODY"}';
const collection: THittableCollection = {
  collectionName: "Shop",
  env: { base: "https://x.dev" },
  secrets: {},
  items: [
    { type: "route", name: "list", curl: "curl -X GET https://x.dev/items", response: SAVED },
    {
      type: "folder",
      name: "orders",
      items: [
        { type: "route", name: "create", curl: "curl -X POST https://x.dev/o", response: SAVED },
        { type: "folder", name: "deep", items: [{ type: "route", name: "get", curl: "curl -X GET https://x.dev/o/1", response: SAVED }] },
      ],
    },
  ],
};

function setup(exportString = JSON.stringify(collection)) {
  const user = userEvent.setup();
  render(<ExportModal exportString={exportString} collectionName="Shop" />);
  return user;
}

async function open(user: ReturnType<typeof userEvent.setup>, format?: RegExp) {
  await user.click(screen.getByRole("button", { name: "Export" }));
  if (format) await user.click(screen.getByText(format));
}

const dialog = () => within(screen.getByRole("dialog"));

function captureDownload() {
  const blobs: Blob[] = [];
  const names: string[] = [];
  URL.createObjectURL = vi.fn((b: Blob) => (blobs.push(b), "blob:x"));
  URL.revokeObjectURL = vi.fn();
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
    names.push(this.download);
  });
  return { blobs, names };
}

beforeEach(() => vi.restoreAllMocks());

describe("ExportModal", () => {
  it("copies a Hittable native string with every saved response stripped, nested ones too", async () => {
    const user = setup();
    await open(user);
    expect(dialog().getByText('Export "Shop" in your preferred format')).toBeInTheDocument();
    await user.click(dialog().getByRole("button", { name: "Copy" }));

    const copied = await navigator.clipboard.readText();
    const roundTrip = JSON.parse(decompressString(copied)) as THittableCollection;
    expect(copied).toMatch(/^[A-Za-z0-9_-]+$/); // URL-safe, no padding
    expect(JSON.stringify(roundTrip)).not.toContain("SAVED-RESPONSE-BODY");
    expect(roundTrip.collectionName).toBe("Shop");
    expect(roundTrip.env).toEqual(collection.env);
    const folder = roundTrip.items[1];
    expect(folder.type === "folder" && folder.items.map((i) => i.name)).toEqual(["create", "deep"]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("offers no file download for the native string", async () => {
    const user = setup();
    await open(user);
    expect(dialog().queryByRole("button", { name: "Download .json" })).not.toBeInTheDocument();
  });

  it("copies a Postman v2.1 collection", async () => {
    const user = setup();
    await open(user, /Postman Collection v2\.1/);
    await user.click(dialog().getByRole("button", { name: "Copy" }));
    const json = JSON.parse(await navigator.clipboard.readText());
    expect(json.info.name).toBe("Shop");
    expect(json.info.schema).toContain("schema.getpostman.com");
    expect(json.item.map((i: { name: string }) => i.name)).toEqual(["list", "orders"]);
    expect(JSON.stringify(json)).not.toContain("SAVED-RESPONSE-BODY");
  });

  it("downloads a Postman file named after the collection", async () => {
    const { blobs, names } = captureDownload();
    const user = setup();
    await open(user, /Postman Collection v2\.1/);
    await user.click(dialog().getByRole("button", { name: "Download .json" }));
    expect(names).toEqual(["Shop-postman.json"]);
    const json = JSON.parse(await blobs[0].text());
    expect(json.info.name).toBe("Shop");
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:x");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("downloads an Insomnia export with a workspace and every request", async () => {
    const { blobs, names } = captureDownload();
    const user = setup();
    await open(user, /Insomnia export/);
    await user.click(dialog().getByRole("button", { name: "Download .json" }));
    expect(names).toEqual(["Shop-insomnia.json"]);
    const { resources } = JSON.parse(await blobs[0].text());
    expect(resources.find((r: { _type: string }) => r._type === "workspace").name).toBe("Shop");
    const requests = resources.filter((r: { _type: string }) => r._type === "request").map((r: { name: string }) => r.name);
    expect(requests.sort()).toEqual(["create", "get", "list"]);
    expect(JSON.stringify(resources)).not.toContain("SAVED-RESPONSE-BODY");
  });

  it("shows an error instead of copying when the collection cannot be read", async () => {
    const user = setup("{not json");
    await open(user);
    await user.click(dialog().getByRole("button", { name: "Copy" }));
    expect(dialog().getByText("Failed to generate export")).toBeInTheDocument();
  });

  it("closes on Escape without copying", async () => {
    const user = setup();
    await navigator.clipboard.writeText("untouched");
    await open(user);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(await navigator.clipboard.readText()).toBe("untouched");
  });
});
