// The hero figure: one request on its way from "sent" to "committed", drawn as
// a git graph. Every row is always rendered (no layout shift); CSS plays the
// sequence once via animation-delay (styles/app.css, .hit-*). No JS.

const Node = ({ kind, delay }: { kind: "sent" | "working" | "commit" | "old"; delay?: string }) => {
  const look = {
    sent: "border-(--brand) bg-(--ink-900)",
    working: "border-dashed border-(--git-modified) bg-(--ink-900)",
    commit: "border-(--brand) bg-(--brand)",
    old: "border-(--ink-400) bg-(--ink-900)",
  }[kind];
  return (
    <span
      aria-hidden="true"
      className={`absolute -left-7 top-3.5 h-[11px] w-[11px] rounded-full border-2 ${look} ${delay ? "hit-node" : ""}`}
      style={delay ? { animationDelay: delay } : undefined}
    />
  );
};

const Json = ({ lines }: { lines: [string, string][] }) => (
  <pre className="overflow-hidden px-3 py-2.5 leading-6">
    <span className="text-(--muted)">{"{"}</span>
    {lines.map(([k, v], i) => (
      <span key={k} className="block pl-4">
        <span className="text-(--info)">&quot;{k}&quot;</span>
        <span className="text-(--muted)">: </span>
        <span className="text-(--text)">&quot;{v}&quot;</span>
        {i < lines.length - 1 && <span className="text-(--muted)">,</span>}
      </span>
    ))}
    <span className="text-(--muted)">{"}"}</span>
  </pre>
);

export function RequestCommitDemo() {
  return (
    <figure
      aria-label="A request in hittable/users/create.hit is sent, returns 201 Created, and is committed to main"
      className="w-full max-w-[520px] font-mono text-xs sm:text-[13px]"
    >
      <ol className="relative pl-7">
        {/* the branch line */}
        <span aria-hidden="true" className="absolute bottom-4 left-[5px] top-4 w-px bg-(--border-strong)" />

        {/* 1. the request, sent */}
        <li className="relative">
          <Node kind="sent" />
          <div className="overflow-hidden rounded-md border border-(--border-strong) bg-(--ink-800)">
            <div className="flex h-9 items-center justify-between gap-3 border-b border-(--border) bg-(--ink-700) px-3">
              <span className="truncate text-(--muted)">
                hittable/users/<span className="text-(--text)">create.hit</span>
              </span>
              <span className="hit-reveal font-semibold text-(--git-modified)" style={{ animationDelay: "2.6s" }} title="Modified">
                M
              </span>
            </div>
            <div className="flex items-center gap-2.5 border-b border-(--border) px-3 py-2">
              <span className="font-semibold text-(--method-post)">POST</span>
              <span className="min-w-0 flex-1 truncate">
                <span className="text-(--brand)">&lt;&lt;BASE_URL&gt;&gt;</span>
                <span className="text-(--text)">/users</span>
              </span>
              <span className="rounded bg-(--brand) px-2 py-0.5 font-sans text-xs font-semibold text-(--accent-ink)">Send</span>
            </div>
            <Json lines={[["name", "Ada Lovelace"], ["role", "admin"]]} />
            <div className="relative h-px bg-(--border)">
              <span className="hit-progress absolute inset-0 bg-(--brand)" style={{ animationDelay: "0.6s" }} />
            </div>
            <div className="hit-reveal" style={{ animationDelay: "1.35s" }}>
              <div className="flex items-center gap-3 px-3 pt-2.5 tabular-nums">
                <span className="font-semibold text-(--success)">201 Created</span>
                <span className="text-(--muted)">142 ms</span>
                <span className="text-(--muted)">318 B</span>
              </div>
              <Json lines={[["id", "usr_01j9xk"], ["name", "Ada Lovelace"]]} />
            </div>
          </div>
        </li>

        {/* 2. the change, in the working copy */}
        <li className="hit-reveal relative mt-4" style={{ animationDelay: "2.6s" }}>
          <Node kind="working" />
          <div className="rounded-md border border-(--border) bg-(--ink-800) px-3 py-2.5">
            <div className="flex items-center justify-between font-sans text-xs text-(--muted)">
              <span>Changes</span>
              <span className="tabular-nums">1</span>
            </div>
            <div className="mt-1.5 flex items-center gap-2.5">
              <span className="font-semibold text-(--git-modified)">M</span>
              <span className="truncate text-(--text)">hittable/users/create.hit</span>
            </div>
            <div className="mt-2.5 flex items-center gap-2">
              <span className="min-w-0 flex-1 truncate rounded border border-(--border) bg-(--ink-700) px-2 py-1 font-sans text-(--text)">
                Add create-user request
              </span>
              <span className="shrink-0 rounded border border-(--border-strong) px-2 py-1 font-sans text-(--text)">Commit to main</span>
            </div>
          </div>
        </li>

        {/* 3. the commit, then history */}
        <li className="relative mt-3 flex h-10 items-center gap-3">
          <Node kind="commit" delay="3.7s" />
          <span className="hit-reveal flex min-w-0 items-center gap-3" style={{ animationDelay: "3.7s" }}>
            <span className="text-(--brand)">3f2a9c1</span>
            <span className="truncate font-sans text-(--text)">Add create-user request</span>
            <span className="shrink-0 rounded border border-(--brand) px-1.5 text-(--brand)">main</span>
          </span>
        </li>
        {[
          ["9b0e4d7", "Point BASE_URL at staging", "2 days ago"],
          ["c41d2e8", "Add hittable/env.json", "last week"],
        ].map(([sha, msg, when]) => (
          <li key={sha} className="relative flex h-10 items-center gap-3">
            <Node kind="old" />
            <span className="text-(--muted)">{sha}</span>
            <span className="min-w-0 truncate font-sans text-(--muted)">{msg}</span>
            <span className="ml-auto hidden shrink-0 font-sans text-(--muted) sm:inline">{when}</span>
          </li>
        ))}
      </ol>
    </figure>
  );
}
