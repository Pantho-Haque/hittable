import { PORTFOLIO_DATA } from "@/constants/landing";
import { Code2, Github, Linkedin, Mail } from "lucide-react";

export default function PortfolioSection({
  hero,
  skills,
}: {
  hero: {
    name: string;
    current_position: string;
    company_name: string;
    comment_one: string;
    comment_two: string;
    contactLinks: Record<string, string>[];
  };
  skills: string[];
}) {
  return (
    <section className="max-w-6xl mx-auto px-6 py-20 border-t border-white/5">
      <div className="rounded-3xl border border-white/10 bg-(--ink-800)/60 p-8 md:p-12 relative overflow-hidden flex flex-col md:flex-row items-center gap-10 shadow-2xl">
        <div className="absolute top-0 right-0 p-8 opacity-[0.03] pointer-events-none">
          <Code2 size={240} />
        </div>

        {/* Image */}
        <div className="shrink-0 relative">
          <div className="w-32 h-32 md:w-44 md:h-44 rounded-full overflow-hidden border-2 border-white/10 relative z-10 shadow-xl">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={PORTFOLIO_DATA.hero.photo}
              alt={hero.name || "Pantho Haque"}
              className="w-full h-full object-cover scale-[1.7]"
            />
          </div>
          <div className="absolute inset-0 rounded-full shadow-[0_0_40px_rgba(255,255,255,0.08)] z-0" />
        </div>

        {/* Content */}
        <div className="flex-1 relative z-10">
          <div className="flex items-center gap-3 mb-3">
            <span className="px-3 py-1 rounded-full bg-emerald-500/10 text-emerald-400 text-xs font-semibold border border-emerald-500/20 tracking-wide uppercase">
              {hero.current_position || PORTFOLIO_DATA.hero.current_position} @{" "}
              {hero.company_name || PORTFOLIO_DATA.hero.company_name}
            </span>
          </div>
          <h2 className="text-3xl md:text-4xl font-bold text-white mb-4 tracking-tight">
            {hero.name || PORTFOLIO_DATA.hero.name}
          </h2>
          <p className="text-slate-400 leading-relaxed mb-6 max-w-2xl text-sm md:text-base font-light">
            {hero.comment_one || PORTFOLIO_DATA.hero.comment_one}
            <br />
            <br />
            {PORTFOLIO_DATA.hero.comment_two}
          </p>

          <div className="flex flex-wrap gap-2 mb-8">
            {skills.map((skill: string) => (
              <span
                key={skill}
                className="px-3 py-1.5 rounded border border-white/10 bg-white/5 text-xs text-slate-300 font-medium"
              >
                {skill}
              </span>
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-6">
            {hero.contactLinks?.map((link: Record<string, string>) => {
              let Icon = Mail;
              if (link.icon === "github") Icon = Github;
              else if (link.icon === "linkedin") Icon = Linkedin;

              return (
                <a
                  key={link.label}
                  href={link.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-2 text-sm font-medium text-slate-400 hover:text-white transition-colors"
                >
                  <Icon size={18} /> {link.label}
                </a>
              );
            })}
          </div>
        </div>
      </div>
    </section>
  );
}
