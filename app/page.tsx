import { ElementType } from "react";

import Link from "next/link";
import { Github, ArrowRight } from "lucide-react";
import { TerminalDemo } from "../components/homepage/TerminalDemo";
import { LANDING_PAGE_DATA } from "../constants/landing";
import { GetResume } from "@/services";
import { PortfolioSection } from "@/components";

function FeatureCard({
  icon: Icon,
  title,
  desc,
}: {
  icon: ElementType;
  title: string;
  desc: string;
}) {
  return (
    <div className="flex flex-col gap-4 p-6 rounded-2xl border border-white/5 bg-white/2 hover:bg-white/4 transition-colors">
      <div className="w-10 h-10 rounded-lg bg-zinc-800/50 border border-white/5 flex items-center justify-center text-zinc-300">
        <Icon size={20} />
      </div>
      <div>
        <h3 className="text-base font-semibold text-zinc-100 mb-1.5">
          {title}
        </h3>
        <p className="text-sm text-zinc-400 leading-relaxed">{desc}</p>
      </div>
    </div>
  );
}

function KbdBadge({ keys, label }: { keys: string[]; label: string }) {
  return (
    <div className="flex items-center justify-between py-3 border-b border-white/5 last:border-0">
      <span className="text-sm text-zinc-400">{label}</span>
      <div className="flex items-center gap-1.5">
        {keys.map((k, i) => (
          <span
            key={i}
            className="px-2 py-1 rounded bg-zinc-800 border border-zinc-700 text-xs font-mono text-zinc-300 min-w-[28px] text-center shadow-sm"
          >
            {k}
          </span>
        ))}
      </div>
    </div>
  );
}

export default async function HomePage() {
  // const { hero, skills } = PORTFOLIO_DATA;
  const { appInfo, features } = LANDING_PAGE_DATA;
  const res = await GetResume();
  const hero = res.hero;
  const skills = res.experience[0].stack;

  return (
    <main className="min-h-full bg-[#09090b] text-zinc-100 overflow-x-hidden font-sans selection:bg-zinc-800">
      {/* Subtle Background Pattern */}
      <div
        className="pointer-events-none fixed inset-0 z-0 opacity-[0.15]"
        style={{
          backgroundImage:
            "radial-gradient(circle at 2px 2px, rgba(255,255,255,0.15) 1px, transparent 0)",
          backgroundSize: "32px 32px",
        }}
      />
      <div className="pointer-events-none fixed inset-0 z-0 bg-linear-to-b from-transparent via-[#09090b]/80 to-[#09090b]" />

      <div className="relative z-10">
        {/* ── Hero ── */}
        <section className="max-w-6xl mx-auto px-6 pt-24 pb-20 flex flex-col lg:flex-row items-center gap-16">
          <div className="flex-1 flex flex-col">
            <a
              href={appInfo.githubUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border border-white/10 bg-white/5 hover:bg-white/10 transition-colors w-fit text-xs font-medium text-zinc-400 tracking-wide uppercase"
            >
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
              Open Source
            </a>
            <h1 className="text-5xl lg:text-7xl font-bold tracking-tight text-white leading-[1.1]">
              <span className="text-zinc-300 text-2xl mt-0 -ml-7">
                {" "}
                ✨ Introducing
              </span>{" "}
              <br />
              <span className="text-yellow-200">
                {appInfo.introducing}
              </span>{" "}
              <br />
              <span className="text-zinc-300">{appInfo.title}</span> <br />
              <span className="text-zinc-500">{appInfo.subtitle}</span>
            </h1>
            <p className="text-lg text-zinc-400 leading-relaxed max-w-lg font-light my-8">
              {appInfo.description}
            </p>
            <div className="flex items-center gap-4">
              <Link
                href="/hittable"
                className="flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-full bg-white text-black hover:bg-zinc-200 transition-colors shadow-[0_0_0_1px_rgba(255,255,255,1)]"
              >
                Launch App
                <ArrowRight size={16} />
              </Link>
              <a
                href={appInfo.githubUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-full border border-zinc-700 bg-zinc-900 text-white hover:bg-zinc-800 transition-colors"
              >
                <Github size={16} />
                GitHub
              </a>
            </div>
          </div>
          <div className="flex-1 w-full flex justify-center lg:justify-end">
            <TerminalDemo />
          </div>
        </section>

        {/* ── Features ── */}
        <section
          id="features"
          className="max-w-6xl mx-auto px-6 py-20 border-t border-white/5"
        >
          <div className="mb-12">
            <h2 className="text-3xl font-bold text-white tracking-tight">
              Built for developers who move fast.
            </h2>
            <p className="text-zinc-400 mt-3 text-lg font-light">
              A refined toolset focusing on speed and simplicity.
            </p>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {features.map((feature, idx) => (
              <FeatureCard
                key={idx}
                icon={feature.icon}
                title={feature.title}
                desc={feature.desc}
              />
            ))}
          </div>
        </section>

        {/* ── Keyboard shortcuts ── */}
        <section
          id="shortcuts"
          className="max-w-6xl mx-auto px-6 py-20 border-t border-white/5"
        >
          <div className="flex flex-col lg:flex-row gap-12 items-start">
            <div className="flex-1">
              <h2 className="text-3xl font-bold text-white tracking-tight mb-4">
                Stay in the flow.
              </h2>
              <p className="text-lg text-zinc-400 leading-relaxed font-light">
                Every action is a keystroke away. No clicking through menus —
                just code and send.
              </p>
            </div>
            <div className="flex-1 w-full flex flex-col gap-2 bg-zinc-900/30 p-6 rounded-2xl border border-white/5">
              <KbdBadge keys={["Ctrl", "↵"]} label="Send request" />
              <KbdBadge
                keys={["Ctrl", "S"]}
                label="Save changes to collection"
              />
              <KbdBadge keys={["Ctrl", "F"]} label="Search response payload" />
              <KbdBadge keys={["Ctrl", "B"]} label="Toggle sidebar" />
              <KbdBadge keys={["Shift", "T"]} label="Create new route" />
            </div>
          </div>
        </section>

        {/* ── Professional Profile / Built By ── */}
        <PortfolioSection hero={hero} skills={skills} />

        {/* ── Footer ── */}
        <footer className="max-w-6xl mx-auto px-6 py-8 border-t border-white/5 flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="w-6 h-6 rounded flex items-center justify-center border border-white/20 bg-white/10 text-white text-[10px] font-bold">
              H
            </div>
            <span className="text-xs text-zinc-500 font-medium">
              Hittable · MIT License
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-6">
            <a
              href={
                res.hero.contactLinks.find(
                  (link: { icon: string }) => link.icon === "github",
                ).href
              }
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-zinc-500 hover:text-zinc-300 transition-colors font-medium"
            >
              Pantho Haque
            </a>
            <Link
              href="/hittable"
              className="text-xs text-zinc-500 hover:text-zinc-300 transition-colors font-medium"
            >
              Launch App
            </Link>
          </div>
        </footer>
      </div>
    </main>
  );
}
