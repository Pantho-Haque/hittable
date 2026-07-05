"use client";

import { useEffect, useRef, useId } from "react";
import { createPortal } from "react-dom";

const sizes = {
  sm: "w-[90vw] max-w-[380px]",
  md: "w-[90vw] max-w-[60%] h-[60%]",
  lg: "w-[95vw] max-w-[90%] h-[80%]",
};

export function ModalShell({
  title,
  subtitle,
  isOnBlurPressClose = true,
  onClose,
  children,
  size = "sm",
}: {
  title: string;
  subtitle?: string;
  isOnBlurPressClose?: boolean;
  onClose: () => void;
  children: React.ReactNode;
  size?: keyof typeof sizes;
}) {
  const titleId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    previousFocusRef.current = document.activeElement as HTMLElement;
    dialogRef.current?.focus();

    return () => {
      previousFocusRef.current?.focus();
    };
  }, []);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key !== "Tab" || !dialogRef.current) return;

      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;

      const first = focusable[0];
      const last = focusable[focusable.length - 1];

      if (e.shiftKey) {
        if (document.activeElement === first) {
          e.preventDefault();
          last.focus();
        }
      } else {
        if (document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);

  return createPortal(
    <div
      data-modal
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onMouseDown={(e) => {
        if (isOnBlurPressClose && e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className={`relative bg-[#0a1628] border border-white/10 rounded-xl shadow-2xl shadow-black/80 p-6 ${sizes[size]}  flex flex-col gap-5 font-mono outline-none`}
        onMouseDown={(e) => e.stopPropagation()}
        onClick={(e) => e.stopPropagation()}
        style={{
          boxShadow:
            "0 0 0 1px rgba(0,229,204,0.08), 0 24px 80px rgba(0,0,0,0.8)",
        }}
      >
        {/* Corner brackets */}
        <span className="absolute top-0 left-0 w-4 h-4 border-t border-l border-cyan-500/30 rounded-tl-xl" aria-hidden="true" />
        <span className="absolute top-0 right-0 w-4 h-4 border-t border-r border-cyan-500/30 rounded-tr-xl" aria-hidden="true" />
        <span className="absolute bottom-0 left-0 w-4 h-4 border-b border-l border-cyan-500/30 rounded-bl-xl" aria-hidden="true" />
        <span className="absolute bottom-0 right-0 w-4 h-4 border-b border-r border-cyan-500/30 rounded-br-xl" aria-hidden="true" />

        <div>
          <p className="text-[9px] tracking-[0.3em] uppercase text-cyan-500/60 mb-1">
            Hittable
          </p>
          <h2 id={titleId} className="text-sm font-bold text-white/90">{title}</h2>
          {subtitle && (
            <p className="text-xs text-white/30 mt-1 capitalize">{subtitle}</p>
          )}
        </div>

        {children}
      </div>
    </div>,
    document.body,
  );
}

export function ModalInput({
  value,
  onChange,
  onKeyDown,
  placeholder,
  autoFocus,
  ariaLabel,
}: {
  value: string;
  onChange: (v: string) => void;
  onKeyDown?: (e: React.KeyboardEvent) => void;
  placeholder?: string;
  autoFocus?: boolean;
  ariaLabel?: string;
}) {
  return (
    <input
      autoFocus={autoFocus}
      aria-label={ariaLabel || placeholder || "Input"}
      className="w-full border-b border-cyan-500/30 bg-transparent outline-none py-2 text-sm text-white/80 placeholder-white/20 focus:border-cyan-400 transition-colors"
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      onKeyDown={onKeyDown}
    />
  );
}

export function ModalActions({
  onCancel,
  onConfirm,
  confirmLabel,
  cancelLabel = "Cancel",
  confirmDanger,
  loading,
}: {
  onCancel: () => void;
  onConfirm: () => void;
  confirmLabel: string;
  cancelLabel?: string;
  confirmDanger?: boolean;
  loading?: boolean;
}) {
  useEffect(() => {
    const handleKeydown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onCancel();
      }
    };
    document.addEventListener("keydown", handleKeydown);
    return () => document.removeEventListener("keydown", handleKeydown);
  }, [onConfirm, onCancel]);
  return (
    <div className="flex justify-end gap-2 pt-1 mt-auto">
      <button
        onClick={onCancel}
        disabled={loading}
        aria-label={cancelLabel}
        className="px-4 py-1.5 text-xs rounded-md border border-white/10 text-white/40 hover:bg-white/5 hover:text-white/70 transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
      >
        {cancelLabel}
      </button>
      <button
        onClick={onConfirm}
        disabled={loading}
        aria-label={confirmLabel}
        className="px-4 py-1.5 text-xs rounded-md font-bold transition-all active:scale-95 cursor-pointer disabled:cursor-not-allowed disabled:opacity-60 flex items-center gap-1.5"
        style={
          confirmDanger
            ? {
                background: "rgba(248,113,113,0.15)",
                border: "1px solid rgba(248,113,113,0.3)",
                color: "#f87171",
              }
            : {
                background: "rgba(0,229,204,0.15)",
                border: "1px solid rgba(0,229,204,0.3)",
                color: "#00e5cc",
              }
        }
      >
        {loading && (
          <svg
            className="animate-spin h-3 w-3 shrink-0"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            />
          </svg>
        )}
        {loading ? "Loading..." : confirmLabel}
      </button>
    </div>
  );
}
