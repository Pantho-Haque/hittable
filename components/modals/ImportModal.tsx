"use client";

import { Import } from "lucide-react";
import { useState } from "react";
import { ModalShell, ModalActions } from "@/components";
import { decompressString } from "@/utils/compressString";
import { THittableCollection } from "@/types";
import { useDataContext } from "@/context/dataContext";
import { useNotification } from "@/hooks";
import { parsePostmanCollection, isPostmanCollection } from "@/utils/importers/postmanImporter";
import { parseInsomniaExport, isInsomniaExport } from "@/utils/importers/insomniaImporter";

type ImportFormat = "hittable" | "postman" | "insomnia" | "unknown";

function detectFormat(input: string): { format: ImportFormat; data: unknown } {
  // Try JSON parse first
  let parsed: unknown;
  try {
    parsed = JSON.parse(input);
  } catch {
    // Not JSON — might be compressed Hittable format
    return { format: "hittable", data: input };
  }

  if (isPostmanCollection(parsed)) {
    return { format: "postman", data: parsed };
  }

  if (isInsomniaExport(parsed)) {
    return { format: "insomnia", data: parsed };
  }

  // Check if it looks like a Hittable collection
  if (parsed && typeof parsed === "object" && "collectionName" in parsed && "curls" in parsed) {
    return { format: "hittable", data: parsed };
  }

  return { format: "unknown", data: parsed };
}

function decompressHittable(code: string): THittableCollection {
  const decompressed = decompressString(code);
  const parsed = JSON.parse(decompressed) as THittableCollection;
  if (!parsed || typeof parsed.collectionName !== "string" || !Array.isArray(parsed.curls)) {
    throw new Error("Invalid collection structure");
  }
  return parsed;
}

export default function ImportModal() {
  const { collections, setCollections } = useDataContext();
  const toast = useNotification();

  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const [error, setError] = useState("");

  const getUniqueName = (baseName: string) => {
    const existingNames = collections.map((c) => c.collectionName);
    if (!existingNames.includes(baseName)) return baseName;
    let counter = 1;
    while (existingNames.includes(`${baseName} - ${counter}`)) {
      counter++;
    }
    return `${baseName} - ${counter}`;
  };

  const importCollection = () => {
    setError("");
    try {
      const { format, data } = detectFormat(input.trim());

      switch (format) {
        case "hittable": {
          const collection = decompressHittable(input.trim());
          const uniqueName = getUniqueName(collection.collectionName);
          setCollections((prev) => [...prev, { ...collection, collectionName: uniqueName }]);
          toast.success({ title: "Imported", desc: `Collection "${uniqueName}" imported (Hittable native)` });
          break;
        }

        case "postman": {
          const parsed = data as unknown;
          const collectionsToImport = parsePostmanCollection(parsed);
          if (collectionsToImport.length === 0) {
            setError("No importable requests found in Postman collection");
            return;
          }
          for (const col of collectionsToImport) {
            const uniqueName = getUniqueName(col.collectionName);
            setCollections((prev) => [...prev, { ...col, collectionName: uniqueName }]);
          }
          const totalRoutes = collectionsToImport.reduce((sum, c) => sum + c.curls.length, 0);
          // Note: Postman scripts, pre-request scripts, tests, and advanced auth types
          // are not supported and silently dropped
          const summary = `Imported ${collectionsToImport.length} collection(s), ${totalRoutes} route(s) from Postman`;
          const warnings: string[] = [];
          // Check for scripts (pre-request, test) — these can't be imported
          warnings.push("Postman scripts (pre-request/test) are not supported and were skipped");
          warnings.push("Advanced auth types (OAuth1, OAuth2, AWS, etc.) are not supported");
          toast.success({
            title: "Imported from Postman",
            desc: warnings.length > 0 ? `${summary}. ${warnings.join("; ")}` : summary,
          });
          break;
        }

        case "insomnia": {
          const collectionsToImport = parseInsomniaExport(data);
          if (collectionsToImport.length === 0) {
            setError("No importable requests found in Insomnia export");
            return;
          }
          for (const col of collectionsToImport) {
            const uniqueName = getUniqueName(col.collectionName);
            setCollections((prev) => [...prev, { ...col, collectionName: uniqueName }]);
          }
          const totalRoutes = collectionsToImport.reduce((sum, c) => sum + c.curls.length, 0);
          toast.success({
            title: "Imported from Insomnia",
            desc: `Imported ${collectionsToImport.length} collection(s), ${totalRoutes} route(s). Insomnia plugins, certificate configs, and cookie jars were not imported.`,
          });
          break;
        }

        default:
          setError("Unrecognized format. Paste a Hittable export string, Postman collection JSON, or Insomnia export JSON.");
          return;
      }

      setOpen(false);
      setInput("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to import");
    }
  };

  return (
    <>
      <button
        onClick={(e) => {
          e.stopPropagation();
          setError("");
          setOpen(true);
        }}
        className="modal-button-mini mt-2"
      >
        <Import size={14} />
      </button>

      {open && (
        <ModalShell
          title="Import"
          subtitle="Paste Hittable, Postman, or Insomnia export data"
          onClose={() => setOpen(false)}
        >
          <div className="w-full h-full flex flex-col gap-2">
            {error && <p className="text-[10px] text-red-400">{error}</p>}
            <textarea
              value={input}
              onChange={(e) => { setInput(e.target.value); setError(""); }}
              className="w-full h-full bg-transparent border border-white/10 rounded-lg p-2 text-xs text-white/50 focus:outline-none focus:border-cyan-500/50 resize-none"
              placeholder="Paste collection data here (Hittable, Postman, or Insomnia format)..."
            />
          </div>
          <ModalActions
            onCancel={() => setOpen(false)}
            onConfirm={importCollection}
            confirmLabel="Import"
          />
        </ModalShell>
      )}
    </>
  );
}
