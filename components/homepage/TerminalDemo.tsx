"use client";
import { useEffect, useState } from "react";

const DEMO_LINES = [
  { delay: 0,    type: "label",    text: "POST  /api/users" },
  { delay: 400,  type: "key",      text: '  "Authorization":', value: ' "Bearer <<token>>"' },
  { delay: 700,  type: "key",      text: '  "Content-Type":', value: ' "application/json"' },
  { delay: 1100, type: "divider",  text: "" },
  { delay: 1300, type: "key",      text: '  "name":', value: ' "Pantho Haque"' },
  { delay: 1600, type: "key",      text: '  "role":', value: ' "admin"' },
  { delay: 2000, type: "divider",  text: "" },
  { delay: 2200, type: "status",   text: "200 OK", value: "  32ms" },
  { delay: 2500, type: "response", text: '  "id":', value: ' "usr_01j9xk"' },
  { delay: 2800, type: "response", text: '  "created":', value: ' "2025-03-08"' },
];

export function TerminalDemo() {
  const [visible, setVisible] = useState<number[]>([]);

  useEffect(() => {
    const timers = DEMO_LINES.map((line, i) =>
      setTimeout(() => setVisible((p) => [...p, i]), line.delay + 600),
    );
    return () => timers.forEach(clearTimeout);
  }, []);

  return (
    <div className="relative w-full max-w-lg rounded-2xl border border-white/10 bg-(--ink-700) shadow-2xl overflow-hidden font-mono text-[13px]">
      {/* Top Bar */}
      <div className="flex items-center gap-2 px-4 py-3 border-b border-white/5 bg-(--ink-600)">
        <div className="flex gap-1.5">
          <span className="w-2.5 h-2.5 rounded-full bg-(--ink-400)" />
          <span className="w-2.5 h-2.5 rounded-full bg-(--ink-400)" />
          <span className="w-2.5 h-2.5 rounded-full bg-(--ink-400)" />
        </div>
        <span className="ml-3 text-xs text-slate-500 font-sans">hittable — new request</span>
      </div>
      <div className="p-5 flex flex-col gap-1.5 min-h-[260px]">
        {DEMO_LINES.map((line, i) => (
          <div
            key={i}
            className="transition-all duration-300 flex"
            style={{
              opacity: visible.includes(i) ? 1 : 0,
              transform: visible.includes(i) ? "translateY(0)" : "translateY(4px)",
            }}
          >
            {line.type === "label" && <span className="text-emerald-400 font-medium">{line.text}</span>}
            {line.type === "divider" && <div className="w-full h-px bg-white/5 my-2" />}
            {(line.type === "key" || line.type === "response") && (
              <span>
                <span className="text-slate-400">{line.text}</span>
                <span className="text-slate-300">{line.value}</span>
              </span>
            )}
            {line.type === "status" && (
              <div className="flex items-center gap-2 mt-2">
                <span className="px-2 py-0.5 rounded text-xs font-medium bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">{line.text}</span>
                <span className="text-slate-500 text-xs">{line.value}</span>
              </div>
            )}
          </div>
        ))}
        <span className="inline-block w-2 h-4 bg-(--brand) animate-pulse mt-2" />
      </div>
    </div>
  );
}
