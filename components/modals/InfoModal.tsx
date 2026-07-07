"use client";

import { Info, Keyboard, Puzzle, Database, Variable, Server } from "lucide-react";
import { useState } from "react";
import { ModalShell, ModalActions } from "@/components";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/Accordion";
import { KEYBINDINGS } from "@/constants";

const acordionItems = [
    {
      value: "keybindings",
      triggerIcon: <Keyboard size={12} className="text-cyan-500/50" />,
      triggerText: "Keybindings",
      content: <Keybindings />,
    },
    {
      value: "extension",
      triggerIcon: <Puzzle size={12} className="text-cyan-500/50" />,
      triggerText: "Localhost Extension",
      content: <LocalExtension />,
    },
    {
      value: "proxy",
      triggerIcon: <Server size={12} className="text-cyan-500/50" />,
      triggerText: "How the Proxy Works",
      content: <ProxyInfo />,
    },
    {
      value: "envvars",
      triggerIcon: <Variable size={12} className="text-cyan-500/50" />,
      triggerText: "Environment Variables",
      content: <EnvVarsInfo />,
    },
    {
      value: "storage",
      triggerIcon: <Database size={12} className="text-cyan-500/50" />,
      triggerText: "Data & Privacy",
      content: <StorageInfo />,
    },
  ];
  
export default function InfoModal() {
  const [open, setOpen] = useState(false);

  
  return (
    <>
      <button
        onClick={(e) => {
          e.stopPropagation();
          setOpen(true);
        }}
        className="modal-button-mini mt-auto mb-2"
      >
        <Info size={14} />
      </button>

      {open && (
        <ModalShell
          title="Hittable Info"
          subtitle=""
          onClose={() => setOpen(false)}
          size="md"
        >
          <div className="w-full font-mono overflow-y-auto">
            <Accordion type="multiple">
              {acordionItems.map((item) => (
                <AccordionItem key={item.value} value={item.value}>
                  <AccordionTrigger className="border-l border-b border-white/10">
                    <span className="flex items-center gap-2">
                      {item.triggerIcon}
                      {item.triggerText}
                    </span>
                  </AccordionTrigger>
                  <AccordionContent className="pt-2">{item.content}</AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </div>

          <ModalActions
            onCancel={() => setOpen(false)}
            onConfirm={() => setOpen(false)}
            confirmLabel="Got it"
          />
        </ModalShell>
      )}
    </>
  );
}

function Keybindings() {
  return (
    <div className="flex flex-col gap-2">
      {KEYBINDINGS.map(({ keys, description }) => (
        <div key={description} className="flex items-center justify-between">
          <span className="text-white/30">{description}</span>
          <div className="flex items-center gap-1">
            {keys.map((k, i) => (
              <span key={k} className="flex items-center gap-1">
                <kbd className="kbd">{k}</kbd>
                {i < keys.length - 1 && (
                  <span className="text-white/20 text-[10px]">+</span>
                )}
              </span>
            ))}
          </div>
        </div>
      ))}
      <div className="mt-2 pt-2 border-t border-white/5">
        <div className="flex items-center justify-between">
          <span className="text-white/30">Close modals</span>
          <div className="flex items-center gap-1">
            <kbd className="kbd">Esc</kbd>
          </div>
        </div>
      </div>
    </div>
  );
}

function LocalExtension() {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-white/30 leading-relaxed">
        To send requests to your local servers, install the Hittable browser
        extension.
      </p>
      <ol className="flex flex-col gap-1.5 text-white/30">
        {[
          "Download and unzip the extension",
          "Go to chrome://extensions",
          "Enable Developer mode",
          "Click Load unpacked → select the folder",
        ].map((step, i) => (
          <li key={i} className="flex items-start gap-2">
            <span className="text-cyan-500/40 shrink-0">{i + 1}.</span>
            {step}
          </li>
        ))}
      </ol>
      <a
        href="https://github.com/pantho-haque/hittable-extension/releases/latest/download/hittable-extension.zip"
        target="_blank"
        rel="noreferrer"
        className="flex items-center justify-center gap-2 rounded-md border border-cyan-500/20 bg-cyan-500/5 px-3 py-1.5 text-[10px] font-semibold text-cyan-400 transition-colors hover:bg-cyan-500/10"
      >
        Download Extension
      </a>
    </div>
  );
}

function ProxyInfo() {
  return (
    <div className="flex flex-col gap-3 text-white/30 leading-relaxed text-[11px]">
      <p>
        Hittable routes external API requests through a Next.js server-side proxy
        at <code className="text-cyan-400/60 bg-white/5 px-1 py-0.5 rounded text-[10px]">/api/proxy</code>. This bypasses
        CORS restrictions that would normally block browser-to-server requests.
      </p>
      <p>
        For <strong className="text-white/50">localhost</strong> requests, the
        browser extension takes over — it intercepts requests to <code className="text-cyan-400/60 bg-white/5 px-1 py-0.5 rounded text-[10px]">localhost</code> / <code className="text-cyan-400/60 bg-white/5 px-1 py-0.5 rounded text-[10px]">127.0.0.1</code> and
        makes them directly from the extension&apos;s background service worker, which
        is not subject to CORS.
      </p>
      <p className="text-white/20">
        Requests never leave your machine for localhost. External requests pass
        through the Vercel-hosted proxy server.
      </p>
    </div>
  );
}

function EnvVarsInfo() {
  return (
    <div className="flex flex-col gap-3 text-white/30 leading-relaxed text-[11px]">
      <p>
        Define environment variables per collection in the Env Vars modal. Reference them in URLs,
        headers, and body using double angle brackets:
      </p>
      <div className="bg-white/5 rounded-md p-3 font-mono text-[10px]">
        <div className="text-white/20 mb-1"># Example usage in a URL:</div>
        <div className="text-cyan-400/70">https://api.example.com/{'<<'}host{'>>'}/users</div>
        <div className="text-white/20 mt-2 mb-1"># Example usage in headers:</div>
        <div className="text-cyan-400/70">Authorization: Bearer {'<<'}token{'>>'}</div>
      </div>
      <p>
        Variable names can contain letters, numbers, and underscores — e.g. <code className="text-cyan-400/60 bg-white/5 px-1 py-0.5 rounded text-[10px]">{'<<'}API_KEY{'>>'}</code>, <code className="text-cyan-400/60 bg-white/5 px-1 py-0.5 rounded text-[10px]">{'<<'}BASE_URL{'>>'}</code>.
      </p>
    </div>
  );
}

function StorageInfo() {
  return (
    <div className="flex flex-col gap-3 text-white/30 leading-relaxed text-[11px]">
      <p>
        <strong className="text-white/50">Everything stays on your machine.</strong> Hittable
        stores all data in your browser&apos;s localStorage — collections, environment
        variables, notes, and request history are never sent to any server.
      </p>
      <p>
        The only network requests made are:
      </p>
      <ul className="flex flex-col gap-1 ml-3">
        <li>• API requests you explicitly send (routed through the proxy or extension)</li>
        <li>• Landing page resume data (fetched from the developer&apos;s portfolio API)</li>
      </ul>
      <p className="text-white/20">
        Clearing your browser data will delete all Hittable data. Use the Export
        feature to back up collections.
      </p>
    </div>
  );
}
