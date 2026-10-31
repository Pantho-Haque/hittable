"use client";

import { useState } from "react";
import { Plug } from "lucide-react";
import { ModalActions, ModalShell } from "@/components/ui/SharedModal";
import { EXTENSION_URL } from "@/config";

/**
 * Whether the localhost extension is connected, as a small button. It used
 * to be a dialog that opened by itself on every visit, even for people who
 * never call localhost; now the explanation is one click away and a
 * localhost request that needs it says so in the response panel.
 */
export default function ExtensionStatus({ checked, available }: { checked: boolean; available: boolean }) {
  const [open, setOpen] = useState(false);
  if (!checked) return null;

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label={available ? "Extension connected" : "Extension not installed"}
        title={available ? "Localhost requests go through the extension" : "Install the extension to call localhost"}
        className="flex shrink-0 items-center gap-1.5 rounded-md border border-white/10 px-2 py-1 text-xs text-white/70 hover:text-white hover:border-white/20 cursor-pointer"
      >
        <span
          aria-hidden="true"
          className={`h-1.5 w-1.5 rounded-full ${available ? "bg-(--success)" : "bg-(--muted)"}`}
        />
        <Plug className="h-3 w-3" aria-hidden="true" />
        <span className="hidden sm:inline">{available ? "Extension" : "No extension"}</span>
      </button>
      {open && (
        <ModalShell
          title={available ? "Extension connected" : "Localhost extension"}
          subtitle={
            available
              ? "Requests to localhost go through the extension; everything else goes through the proxy."
              : "Only needed for localhost: browsers stop a web page from calling your own machine. Every other URL works without it."
          }
          onClose={() => setOpen(false)}
        >
          {!available && (
            <ol className="text-xs text-white/70 space-y-1">
              <li>1. Download and unzip the extension</li>
              <li>2. Go to <code>chrome://extensions</code></li>
              <li>3. Enable <strong>Developer mode</strong></li>
              <li>4. Click <strong>Load unpacked</strong> → select the folder</li>
            </ol>
          )}
          <ModalActions
            onCancel={() => setOpen(false)}
            onConfirm={available ? () => setOpen(false) : () => window.open(EXTENSION_URL, "_blank")}
            confirmLabel={available ? "Done" : "Download"}
            cancelLabel="Close"
          />
        </ModalShell>
      )}
    </>
  );
}
