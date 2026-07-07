"use client";

import { Upload } from "lucide-react";
import { useCallback, useState } from "react";
import { ModalShell, ModalActions } from "@/components";
import { compressString } from "@/utils/compressString";
import { THittableCollection } from "@/types";
import { exportToPostmanCollection } from "@/utils/importers/postmanExporter";
import { exportToInsomniaCollection } from "@/utils/importers/insomniaExporter";

type ExportFormat = "hittable" | "postman" | "insomnia";

const FORMAT_OPTIONS: { value: ExportFormat; label: string; description: string }[] = [
  { value: "hittable", label: "Hittable native", description: "Compressed string for Hittable import" },
  { value: "postman", label: "Postman Collection v2.1", description: "JSON for Postman import" },
  { value: "insomnia", label: "Insomnia export", description: "JSON for Insomnia import" },
];

export default function ExportModal({
  exportString,
  collectionName,
}: {
  exportString: string;
  collectionName: string;
}) {
  const [open, setOpen] = useState(false);
  const [error, setError] = useState("");
  const [format, setFormat] = useState<ExportFormat>("hittable");

  const generateExport = useCallback(() => {
    try {
      const parsed = JSON.parse(exportString) as THittableCollection;
      const stripped: THittableCollection = {
        ...parsed,
        items: parsed.items?.map((item) =>
          item.type === "route" ? { ...item, response: "" } : item
        ) ?? [],
      };

      switch (format) {
        case "hittable":
          return compressString(JSON.stringify(stripped));
        case "postman":
          return exportToPostmanCollection(stripped);
        case "insomnia":
          return exportToInsomniaCollection(stripped);
        default:
          return null;
      }
    } catch {
      return null;
    }
  }, [exportString, format]);

  const copyToClipboard = () => {
    const code = generateExport();
    if (!code) {
      setError("Failed to generate export");
      return;
    }
    navigator.clipboard.writeText(code);
    setOpen(false);
  };

  const downloadFile = () => {
    const code = generateExport();
    if (!code) {
      setError("Failed to generate export");
      return;
    }

    const filenames: Record<ExportFormat, string> = {
      hittable: `${collectionName}-hittable.txt`,
      postman: `${collectionName}-postman.json`,
      insomnia: `${collectionName}-insomnia.json`,
    };

    const blob = new Blob([code], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filenames[format];
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    setOpen(false);
  };

  return (
    <>
      <button
        onClick={(e) => {
          e.stopPropagation();
          setError("");
          setOpen(true);
        }}
        className="flex items-center gap-2 px-3 py-2 text-xs text-white/50 hover:bg-white/5 hover:text-cyan-400 transition-colors w-full text-left cursor-pointer"
      >
        <Upload size={12} />
        Export
      </button>

      {open && (
        <ModalShell
          title="Export"
          subtitle={`Export "${collectionName}" in your preferred format`}
          onClose={() => setOpen(false)}
        >
          <div className="w-full h-full flex flex-col gap-3">
            {error && <p className="text-[10px] text-red-400">{error}</p>}

            {/* Format selector */}
            <div className="flex flex-col gap-1.5">
              <span className="text-[9px] text-white/25 uppercase tracking-wider">Format</span>
              <div className="flex flex-col gap-1">
                {FORMAT_OPTIONS.map((opt) => (
                  <label
                    key={opt.value}
                    className={`flex items-center gap-2 px-2 py-1.5 rounded-md border cursor-pointer transition-colors ${
                      format === opt.value
                        ? "border-cyan-500/30 bg-cyan-500/5 text-cyan-400"
                        : "border-white/5 bg-white/2 text-white/40 hover:border-white/10 hover:text-white/60"
                    }`}
                  >
                    <input
                      type="radio"
                      name="exportFormat"
                      value={opt.value}
                      checked={format === opt.value}
                      onChange={(e) => setFormat(e.target.value as ExportFormat)}
                      className="hidden"
                    />
                    <div className="w-3 h-3 rounded-full border border-current flex items-center justify-center shrink-0">
                      {format === opt.value && <div className="w-1.5 h-1.5 rounded-full bg-current" />}
                    </div>
                    <div className="flex flex-col">
                      <span className="text-[10px] font-medium">{opt.label}</span>
                      <span className="text-[8px] text-white/25">{opt.description}</span>
                    </div>
                  </label>
                ))}
              </div>
            </div>
          </div>

          <div className="flex gap-2">
            <ModalActions
              onCancel={() => setOpen(false)}
              onConfirm={copyToClipboard}
              confirmLabel="Copy"
            />
            {format !== "hittable" && (
              <button
                onClick={downloadFile}
                className="flex-1 px-4 py-1.5 text-xs rounded-md border border-white/10 text-white/40 hover:bg-white/5 hover:text-white/70 transition-colors cursor-pointer"
              >
                Download .json
              </button>
            )}
          </div>
        </ModalShell>
      )}
    </>
  );
}
