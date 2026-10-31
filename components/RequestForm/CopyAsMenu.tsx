"use client";
import { useEffect, useRef, useState } from "react";
import { Check, ChevronDown, Code2 } from "lucide-react";
import { THittableCurlJson } from "@/types";
import { CODE_LANGUAGES, CodeLanguage, copyAs } from "@/utils/codegen";
import { encodeShare, shareUrl } from "@/utils/shareLink";
import { useNotification } from "@/hooks/useNotify";

type Props = {
  formInput: THittableCurlJson;

  env?: Record<string, string>;
  secrets?: Record<string, string>;
  onCopied?: (label: string) => void;
};

export default function CopyAsMenu({ formInput, env, secrets, onCopied }: Props) {
  const notify = useNotification();
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const itemsRef = useRef<(HTMLButtonElement | null)[]>([]);

  useEffect(() => {
    if (!open) return;
    itemsRef.current[0]?.focus();
    const onDown = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const close = () => {
    setOpen(false);
    triggerRef.current?.focus();
  };

  const pick = async (id: CodeLanguage, label: string) => {
    close();
    try {
      await copyAs(id, formInput, { ...env, ...secrets });
    } catch {
      return; // clipboard denied: nothing was copied, so no confirmation
    }
    onCopied?.(label);
    flashCopied();
  };

  const flashCopied = () => {
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  // FR-15: env and secret values are never resolved into the link.
  const share = async () => {
    close();
    const res = encodeShare(formInput);
    if (!res.ok) return notify.error({ title: "Can't make a share link", desc: res.error });
    try {
      await navigator.clipboard.writeText(shareUrl(window.location.origin, res.fragment));
    } catch {
      return notify.error({ title: "Couldn't copy the share link", desc: "The browser blocked clipboard access." });
    }
    flashCopied(); // not onCopied: its "variables are filled in" toast is wrong for a link
    notify.success({
      title: "Share link copied",
      desc: res.redacted.length ? `${res.redacted.join(". ")}.` : "Anyone with the link can preview and import this request.",
    });
  };

  const onMenuKey = (e: React.KeyboardEvent) => {
    const items = itemsRef.current.filter((x): x is HTMLButtonElement => !!x);
    const i = items.indexOf(document.activeElement as HTMLButtonElement);
    const go = (n: number) => {
      e.preventDefault();
      items[(n + items.length) % items.length]?.focus();
    };
    if (e.key === "ArrowDown") go(i + 1);
    else if (e.key === "ArrowUp") go(i - 1);
    else if (e.key === "Home") go(0);
    else if (e.key === "End") go(items.length - 1);
    else if (e.key === "Escape" || e.key === "Tab") {
      e.preventDefault();
      close();
    }
  };

  return (
    <div ref={rootRef} className="relative">
      <button
        ref={triggerRef}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown" && !open) {
            e.preventDefault();
            setOpen(true);
          }
        }}
        className="flex h-full items-center gap-1 rounded-md border border-white/10 bg-(--surface-1) px-2 py-1.5 text-xs text-white/80 hover:bg-(--surface-2) hover:text-white focus-visible:outline-2 focus-visible:outline-(--brand)"
      >
        {copied ? <Check className="h-3.5 w-3.5 text-(--brand)" aria-hidden /> : <Code2 className="h-3.5 w-3.5" aria-hidden />}
        Copy as
        <ChevronDown className="h-3 w-3" aria-hidden />
      </button>
      {open && (
        <div
          role="menu"
          aria-label="Copy request as"
          onKeyDown={onMenuKey}
          className="absolute right-0 top-full z-50 mt-1 min-w-44 rounded-md border border-white/10 bg-(--surface-1) py-1 shadow-lg"
        >
          {CODE_LANGUAGES.map(({ id, label }, i) => (
            <button
              key={id}
              ref={(el) => {
                itemsRef.current[i] = el;
              }}
              type="button"
              role="menuitem"
              tabIndex={-1}
              onClick={() => pick(id, label)}
              className="block w-full px-3 py-1.5 text-left text-xs text-white/80 hover:bg-(--surface-2) hover:text-white focus:bg-(--surface-2) focus:text-(--brand) focus:outline-none"
            >
              {label}
            </button>
          ))}
          <div role="separator" className="my-1 border-t border-white/10" />
          <button
            ref={(el) => {
              itemsRef.current[CODE_LANGUAGES.length] = el;
            }}
            type="button"
            role="menuitem"
            tabIndex={-1}
            onClick={share}
            className="block w-full px-3 py-1.5 text-left text-xs text-white/80 hover:bg-(--surface-2) hover:text-white focus:bg-(--surface-2) focus:text-(--brand) focus:outline-none"
          >
            Share link
          </button>
        </div>
      )}
    </div>
  );
}
