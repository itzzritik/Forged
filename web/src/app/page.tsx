"use client";

import Link from "next/link";
import { useEffect, useState, useRef } from "react";

// ============================================
// FLOATING TERMINAL CODE BLOCKS DATA
// ============================================
const terminalData = [
  {
    id: "agent-01",
    title: "AGENT-01 // REFACTOR",
    position: { top: "5%", left: "2%" },
    lines: [
      { type: "command", text: '> blackbox agent run --task refactor-auth' },
      { type: "output", text: '[INIT] Loading codebase context...' },
      { type: "output", text: '[SCAN] 47 files analyzed in 1.2s' },
      { type: "output", text: '[PLAN] Extracting auth middleware → /lib/auth.ts' },
      { type: "output", text: '[EDIT] src/routes/api.ts — removed inline checks' },
      { type: "output", text: '[EDIT] src/middleware/auth.ts — created guard' },
      { type: "success", text: '[TEST] 12/12 passing' },
      { type: "done", text: '[DONE] Refactor complete. PR ready.' },
    ],
  },
  {
    id: "agent-02",
    title: "AGENT-02 // MIGRATE",
    position: { top: "8%", left: "25%" },
    lines: [
      { type: "command", text: '> blackbox agent run --task db-migration' },
      { type: "output", text: '[INIT] Connecting to schema registry...' },
      { type: "output", text: '[DIFF] 3 tables modified, 1 added' },
      { type: "output", text: '[GEN] Migration 0047_add_teams.sql' },
      { type: "success", text: '[VALIDATE] Foreign keys ............. OK' },
      { type: "success", text: '[VALIDATE] Indexes .................. OK' },
      { type: "output", text: '[APPLY] Dry run successful' },
      { type: "done", text: '[DONE] Migration staged.' },
    ],
  },
  {
    id: "agent-03",
    title: "AGENT-03 // TEST-GEN",
    position: { top: "3%", left: "50%" },
    lines: [
      { type: "command", text: '> blackbox agent run --task generate-tests' },
      { type: "output", text: '[SCAN] Uncovered functions: 23' },
      { type: "output", text: '[GEN] tests/auth.test.ts (8 cases)' },
      { type: "output", text: '[GEN] tests/billing.test.ts (6 cases)' },
      { type: "output", text: '[GEN] tests/api.test.ts (9 cases)' },
      { type: "success", text: '[RUN] 23/23 passing' },
      { type: "success", text: '[COV] Coverage: 47% → 89%' },
      { type: "done", text: '[DONE] Test suite committed.' },
    ],
  },
  {
    id: "agent-04",
    title: "AGENT-04 // DEPLOY",
    position: { top: "12%", left: "75%" },
    lines: [
      { type: "command", text: '> blackbox agent run --task deploy-staging' },
      { type: "success", text: '[BUILD] next build .................. OK' },
      { type: "success", text: '[LINT] 0 errors, 0 warnings' },
      { type: "success", text: '[TYPE] tsc --noEmit ................ OK' },
      { type: "output", text: '[PUSH] Deploying to staging...' },
      { type: "output", text: '[DNS] https://staging.blackbox.dev' },
      { type: "success", text: '[HEALTH] 200 OK — 43ms' },
      { type: "done", text: '[DONE] Live on staging.' },
    ],
  },
  {
    id: "chairman",
    title: "CHAIRMAN LLM // JUDGE",
    position: { top: "35%", left: "65%" },
    lines: [
      { type: "command", text: '> chairman evaluate --round 1' },
      { type: "output", text: '[RECV] 4 agent submissions received' },
      { type: "output", text: '[EVAL] Agent-01: refactor .......... 9.2/10' },
      { type: "output", text: '[EVAL] Agent-02: migration ......... 8.8/10' },
      { type: "output", text: '[EVAL] Agent-03: test-gen .......... 9.5/10' },
      { type: "output", text: '[EVAL] Agent-04: deploy ............ 9.1/10' },
      { type: "success", text: '[RANK] Best: Agent-03 (test coverage)' },
      { type: "done", text: '[VERDICT] All agents passed. Merging.' },
    ],
  },
  {
    id: "system",
    title: "SYSTEM // MONITOR",
    position: { top: "40%", left: "0%" },
    lines: [
      { type: "command", text: '> blackbox monitor --live' },
      { type: "output", text: '[SYS] CPU: 12% | MEM: 3.2GB / 16GB' },
      { type: "output", text: '[SYS] Active agents: 4' },
      { type: "output", text: '[SYS] Queue depth: 0' },
      { type: "output", text: '[NET] API latency p99: 89ms' },
      { type: "output", text: '[NET] Requests/min: 2,847' },
      { type: "success", text: '[COST] Session: $0.42' },
      { type: "done", text: '[STATUS] All systems nominal.' },
    ],
  },
  {
    id: "agent-05",
    title: "AGENT-05 // REVIEW",
    position: { top: "55%", left: "78%" },
    lines: [
      { type: "command", text: '> blackbox agent run --task code-review' },
      { type: "output", text: '[LOAD] PR #247 — 14 files changed' },
      { type: "success", text: '[SCAN] Security patterns ........... OK' },
      { type: "warning", text: '[SCAN] Performance anti-patterns ... 1 found' },
      { type: "warning", text: '[WARN] Avoid N+1 in /api/teams.ts:34' },
      { type: "success", text: '[SCAN] Type coverage ............... 100%' },
      { type: "success", text: '[APPROVE] No blockers found' },
      { type: "done", text: '[DONE] Review posted.' },
    ],
  },
  {
    id: "agent-06",
    title: "AGENT-06 // DOCS",
    position: { top: "60%", left: "20%" },
    lines: [
      { type: "command", text: '> blackbox agent run --task update-docs' },
      { type: "output", text: '[SCAN] 7 undocumented exports found' },
      { type: "output", text: '[GEN] docs/api-reference.md updated' },
      { type: "output", text: '[GEN] docs/auth-guide.md created' },
      { type: "output", text: '[GEN] README.md — added quickstart' },
      { type: "success", text: '[LINK] Cross-references validated' },
      { type: "success", text: '[SPELL] 0 issues' },
      { type: "done", text: '[DONE] Documentation shipped.' },
    ],
  },
];

// ============================================
// FLOATING TERMINAL COMPONENT
// ============================================
function FloatingTerminal({
  data,
  delay = 0,
}: {
  data: typeof terminalData[0];
  delay?: number;
}) {
  const [visibleLines, setVisibleLines] = useState(0);

  useEffect(() => {
    const timer = setTimeout(() => {
      const interval = setInterval(() => {
        setVisibleLines((prev) => {
          if (prev >= data.lines.length) {
            clearInterval(interval);
            return prev;
          }
          return prev + 1;
        });
      }, 150);
      return () => clearInterval(interval);
    }, delay);
    return () => clearTimeout(timer);
  }, [data.lines.length, delay]);

  const getLineColor = (type: string) => {
    switch (type) {
      case "command":
        return "text-zinc-300";
      case "success":
        return "text-emerald-400";
      case "warning":
        return "text-amber-400";
      case "done":
        return "text-emerald-400";
      default:
        return "text-zinc-500";
    }
  };

  return (
    <div
      className="absolute w-[320px] bg-[#0a0a0a]/90 backdrop-blur-sm border border-white/10 rounded-lg overflow-hidden shadow-2xl opacity-60 hover:opacity-100 transition-opacity duration-300 pointer-events-auto"
      style={{
        top: data.position.top,
        left: data.position.left,
        animation: `fadeInUp 0.8s ease-out ${delay}ms both`,
      }}
    >
      <div className="px-3 py-2 border-b border-white/10 bg-white/5">
        <span className="text-[10px] font-mono text-zinc-500 tracking-wider">
          {data.title}
        </span>
      </div>
      <div className="p-3 font-mono text-[11px] leading-relaxed max-h-[200px] overflow-hidden">
        {data.lines.slice(0, visibleLines).map((line, i) => (
          <div key={i} className={`${getLineColor(line.type)} whitespace-nowrap`}>
            {line.text}
          </div>
        ))}
        {visibleLines < data.lines.length && (
          <span className="inline-block w-2 h-3 bg-white/50 animate-pulse" />
        )}
      </div>
    </div>
  );
}

// ============================================
// COMPANY LOGOS
// ============================================
const companyLogos = [
  { name: "Apple", svg: <svg className="h-6 w-auto" viewBox="0 0 24 24" fill="currentColor"><path d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.81-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z"/></svg> },
  { name: "Amazon", svg: <svg className="h-5 w-auto" viewBox="0 0 24 24" fill="currentColor"><path d="M.045 18.02c.072-.116.187-.124.348-.022 3.636 2.11 7.594 3.166 11.87 3.166 2.852 0 5.668-.533 8.447-1.595l.315-.14c.138-.06.234-.1.293-.13.226-.088.39-.046.493.126.108.18.05.36-.173.543-.472.39-1.17.9-2.097 1.527l-.013.01c-1.936 1.31-4.057 2.098-6.362 2.366-.06.007-.12.01-.18.01-.26 0-.45-.08-.57-.24-.146-.2-.14-.464.017-.79l.064-.135c.068-.138.14-.28.213-.417l.158-.29c.045-.084.092-.166.138-.245l.088-.153c.098-.167.197-.33.294-.49l.09-.145c.237-.388.46-.772.665-1.147.243-.447.47-.906.683-1.377l.028-.06c.048-.108.08-.22.08-.337 0-.167-.084-.31-.25-.43-.56-.403-1.17-.756-1.82-1.063-.635-.3-1.298-.556-1.987-.772-.23-.072-.45-.056-.67.05l-.012.005c-.083.036-.157.078-.225.125a1.15 1.15 0 0 0-.195.163l-.022.024c-.163.18-.293.376-.39.586l-.025.054c-.085.185-.16.375-.225.567l-.012.034c-.058.173-.113.35-.164.53l-.01.038c-.076.284-.14.575-.19.874l-.005.026c-.037.224-.068.45-.094.68l-.003.025c-.03.28-.048.563-.054.847v.027c-.003.18.002.36.012.538l.002.034c.017.316.052.63.104.94.028.18.072.35.13.52.057.166.127.32.212.466.105.178.23.333.378.467.142.127.303.228.48.3.23.095.47.142.72.142.15 0 .31-.018.48-.053.26-.057.5-.15.73-.277.255-.144.48-.32.677-.526.31-.328.548-.708.715-1.14.156-.404.283-.822.38-1.25.035-.153.065-.307.09-.462.025-.15.045-.3.06-.45.03-.3.045-.6.045-.9 0-.137-.003-.27-.01-.4-.008-.165-.02-.325-.037-.48-.017-.16-.04-.315-.066-.467-.027-.155-.058-.306-.094-.454-.035-.15-.075-.296-.12-.44-.044-.145-.093-.286-.146-.423-.053-.14-.11-.275-.173-.406-.063-.133-.13-.262-.202-.387-.145-.25-.31-.485-.494-.705-.185-.22-.39-.42-.612-.6-.37-.3-.78-.54-1.226-.72-.448-.18-.92-.3-1.415-.36-.12-.015-.24-.022-.36-.022-.12 0-.24.007-.36.022-.34.04-.668.115-.985.224-.318.11-.62.25-.907.42-.37.22-.707.486-1.01.795-.304.31-.57.655-.797 1.036-.23.38-.42.79-.567 1.22-.148.43-.25.88-.307 1.35-.028.235-.043.47-.043.71 0 .3.022.594.065.884.04.272.098.54.17.8.068.26.15.515.25.763.1.247.22.485.353.716.134.23.28.45.443.662.133.168.274.33.422.486l.054.055c.214.214.44.41.68.59l.014.01c.164.123.335.238.514.345l.034.02c.2.12.407.228.622.328l.007.003c.347.16.71.29 1.09.39.074.02.15.03.226.03.15 0 .28-.047.39-.14l.007-.007z"/></svg> },
  { name: "Uber", svg: <svg className="h-4 w-auto" viewBox="0 0 24 24" fill="currentColor"><path d="M0 7.97v4.958c0 1.867 1.302 3.101 3 3.101.826 0 1.562-.316 2.094-.87v.736H6.27V7.97H5.094v4.702c0 1.257-.792 2.131-1.947 2.131-1.154 0-1.947-.874-1.947-2.131V7.97zm9.437 0v.736c-.532-.554-1.268-.87-2.094-.87-1.698 0-3 1.234-3 3.101s1.302 3.101 3 3.101c.826 0 1.562-.316 2.094-.87v.736h1.177V7.97zm-1.947 4.902c-1.155 0-1.947-.874-1.947-2.131 0-1.257.792-2.131 1.947-2.131s1.947.874 1.947 2.131c0 1.257-.792 2.131-1.947 2.131zm6.282-4.902v4.958c0 1.867 1.302 3.101 3 3.101.826 0 1.562-.316 2.094-.87v.736h1.177V7.97h-1.177v4.702c0 1.257-.792 2.131-1.947 2.131-1.154 0-1.947-.874-1.947-2.131V7.97zm9.33 0h-1.243l-2.326 3.471v-3.47h-1.177v8.034h1.177v-3.413l2.326 3.412h1.403l-2.672-3.898z"/></svg> },
  { name: "Google", svg: <svg className="h-5 w-auto" viewBox="0 0 24 24" fill="currentColor"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/></svg> },
  { name: "Stripe", svg: <svg className="h-5 w-auto" viewBox="0 0 24 24" fill="currentColor"><path d="M13.976 9.15c-2.172-.806-3.356-1.426-3.356-2.409 0-.831.683-1.305 1.901-1.305 2.227 0 4.515.858 6.09 1.631l.89-5.494C18.252.975 15.697 0 12.165 0 9.667 0 7.589.654 6.104 1.872 4.56 3.147 3.757 4.992 3.757 7.218c0 4.039 2.467 5.76 6.476 7.219 2.585.92 3.445 1.574 3.445 2.583 0 .98-.84 1.545-2.354 1.545-1.875 0-4.965-.921-6.99-2.109l-.9 5.555C5.175 22.99 8.385 24 11.714 24c2.641 0 4.843-.624 6.328-1.813 1.664-1.305 2.525-3.236 2.525-5.732 0-4.128-2.524-5.851-6.591-7.305z"/></svg> },
  { name: "Oracle", svg: <svg className="h-4 w-auto" viewBox="0 0 24 24" fill="currentColor"><path d="M7.076 8.142c-2.143 0-3.873 1.734-3.873 3.858s1.73 3.858 3.873 3.858h9.848c2.142 0 3.873-1.734 3.873-3.858s-1.73-3.858-3.873-3.858zm9.616 6.153H7.308c-1.263 0-2.29-1.027-2.29-2.295s1.027-2.295 2.29-2.295h9.384c1.264 0 2.29 1.027 2.29 2.295s-1.026 2.295-2.29 2.295z"/></svg> },
  { name: "Supabase", svg: <svg className="h-5 w-auto" viewBox="0 0 24 24" fill="currentColor"><path d="M11.9 1.036c-.015-.986-1.26-1.41-1.874-.637L.764 12.05C-.33 13.427.65 15.455 2.409 15.455h9.579l.113 7.51c.014.985 1.259 1.408 1.873.636l9.262-11.653c1.093-1.375.113-3.403-1.645-3.403h-9.642z"/></svg> },
];

// ============================================
// HEADER COMPONENT
// ============================================
function Header() {
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const handleScroll = () => setScrolled(window.scrollY > 20);
    window.addEventListener("scroll", handleScroll);
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  return (
    <header
      className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ${
        scrolled ? "bg-black/80 backdrop-blur-md border-b border-white/10" : ""
      }`}
    >
      <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
        <Link href="/" className="flex items-center gap-2">
          <div className="flex items-center gap-1.5">
            <div className="w-5 h-5 bg-white rounded-sm flex items-center justify-center">
              <span className="text-black font-bold text-xs">B</span>
            </div>
            <span className="text-white font-semibold text-sm tracking-tight">
              BLACKBOX.AI
            </span>
          </div>
        </Link>

        <nav className="hidden md:flex items-center gap-8">
          <Link
            href="#"
            className="text-sm text-zinc-400 hover:text-white transition-colors"
          >
            PLATFORM
          </Link>
          <Link
            href="#"
            className="text-sm text-zinc-400 hover:text-white transition-colors"
          >
            PRICING
          </Link>
          <Link
            href="/docs"
            className="text-sm text-zinc-400 hover:text-white transition-colors"
          >
            DOCS
          </Link>
        </nav>

        <div className="flex items-center gap-4">
          <Link
            href="/login"
            className="text-sm text-zinc-400 hover:text-white transition-colors hidden sm:block"
          >
            LOGIN
          </Link>
          <Link
            href="#"
            className="h-9 px-4 bg-white text-black text-sm font-medium rounded-md flex items-center justify-center hover:bg-zinc-200 transition-colors"
          >
            GET STARTED
          </Link>
        </div>
      </div>
    </header>
  );
}

// ============================================
// HERO SECTION
// ============================================
function HeroSection() {
  return (
    <section className="relative min-h-screen pt-32 pb-20 overflow-hidden">
      {/* Floating Terminal Background */}
      <div className="absolute inset-0 pointer-events-none">
        {terminalData.map((terminal, i) => (
          <FloatingTerminal key={terminal.id} data={terminal} delay={i * 300} />
        ))}
      </div>

      {/* Gradient Overlay */}
      <div className="absolute inset-0 bg-gradient-to-b from-black via-black/90 to-black pointer-events-none" />
      <div className="absolute inset-0 bg-gradient-to-r from-black via-transparent to-black pointer-events-none" />

      {/* Main Content */}
      <div className="relative z-10 max-w-7xl mx-auto px-6">
        <div className="max-w-3xl">
          {/* Badge */}
          <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border border-orange-500/30 bg-orange-500/10 mb-8">
            <span className="text-orange-500 text-xs font-medium tracking-wide">
              BLACKBOX AI
            </span>
          </div>

          {/* Main Headline */}
          <h1 className="text-5xl sm:text-6xl md:text-7xl lg:text-8xl font-bold tracking-tight leading-[0.95] mb-8">
            <span className="text-white">Claude Code,</span>
            <br />
            <span className="text-white">Codex,</span>
            <br />
            <span className="text-orange-500">Blackbox</span>
          </h1>

          {/* Subheadline */}
          <p className="text-lg md:text-xl text-zinc-400 max-w-xl mb-10 leading-relaxed">
            Enterprise-grade AI agents with frontier and open-source model access.
            Ship faster with autonomous multi-agent execution through one API.
          </p>

          {/* CTA Button */}
          <Link
            href="#"
            className="inline-flex items-center gap-2 h-12 px-6 bg-white text-black font-medium rounded-md hover:bg-zinc-200 transition-colors group"
          >
            GET STARTED
            <svg
              className="w-4 h-4 group-hover:translate-x-1 transition-transform"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M17 8l4 4m0 0l-4 4m4-4H3"
              />
            </svg>
          </Link>
        </div>
      </div>

      {/* Company Logos */}
      <div className="relative z-10 mt-32 border-t border-white/10 pt-10">
        <div className="max-w-7xl mx-auto px-6">
          <p className="text-xs text-zinc-600 uppercase tracking-widest mb-8 text-center">
            Teams at Fortune 500 companies that depend on{" "}
            <span className="text-zinc-400">BLACKBOX.AI</span>
          </p>
          <div className="flex flex-wrap items-center justify-center gap-8 md:gap-16 opacity-50">
            {companyLogos.map((logo) => (
              <div key={logo.name} className="text-zinc-500 hover:text-white transition-colors">
                {logo.svg}
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

// ============================================
// FEATURES DATA
// ============================================
const features = [
  {
    id: "cli",
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 7.5l3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0021 18V6a2.25 2.25 0 00-2.25-2.25H5.25A2.25 2.25 0 003 6v12a2.25 2.25 0 002.25 2.25z" />
      </svg>
    ),
    title: "CLI",
    subtitle: "Your terminal, supercharged",
    description:
      "Dispatch competing agents from a single command. They analyze your codebase, generate solutions in parallel, and open PRs — no browser needed.",
    bullets: [
      "Multi-agent parallel execution",
      "Automatic PR creation",
      "CI/CD pipeline integration",
    ],
    link: { text: "Try CLI", href: "#" },
  },
  {
    id: "ide",
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M17.25 6.75L22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3l-4.5 16.5" />
      </svg>
    ),
    title: "IDE",
    subtitle: "Agents in your editor",
    description:
      "Agents work alongside you inside VS Code or Blackbox IDE. Real-time code generation, refactoring, and testing — right where you write code.",
    bullets: [
      "Inline code generation",
      "Context-aware refactoring",
      "Integrated test runner",
    ],
    link: { text: "Try IDE", href: "#" },
  },
  {
    id: "cloud",
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 15a4.5 4.5 0 004.5 4.5H18a3.75 3.75 0 001.332-7.257 3 3 0 00-3.758-3.848 5.25 5.25 0 00-10.233 2.33A4.502 4.502 0 002.25 15z" />
      </svg>
    ),
    title: "Cloud",
    subtitle: "Always-on, always working",
    description:
      "Deploy autonomous agents to the cloud. They monitor, fix, and optimize your codebase 24/7 — even while your team sleeps.",
    bullets: [
      "24/7 autonomous operation",
      "Automated monitoring & fixes",
      "Team dashboards & controls",
    ],
    link: { text: "Explore Cloud", href: "#" },
  },
  {
    id: "api",
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M14.25 9.75L16.5 12l-2.25 2.25m-4.5 0L7.5 12l2.25-2.25M6 20.25h12A2.25 2.25 0 0020.25 18V6A2.25 2.25 0 0018 3.75H6A2.25 2.25 0 003.75 6v12A2.25 2.25 0 006 20.25z" />
      </svg>
    ),
    title: "API",
    subtitle: "Programmable agent execution",
    description:
      "Integrate agent execution into any workflow with OpenAI-compatible endpoints. Chat completions, multi-agent orchestration, and real-time streaming.",
    bullets: [
      "OpenAI-compatible endpoints",
      "Multi-agent orchestration",
      "WebSocket streaming",
    ],
    link: { text: "View API Docs", href: "#" },
  },
  {
    id: "mobile",
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 1.5H8.25A2.25 2.25 0 006 3.75v16.5a2.25 2.25 0 002.25 2.25h7.5A2.25 2.25 0 0018 20.25V3.75a2.25 2.25 0 00-2.25-2.25H13.5m-3 0V3h3V1.5m-3 0h3m-3 18.75h3" />
      </svg>
    ),
    title: "Mobile",
    subtitle: "Ship code from your pocket",
    description:
      "Review agent work, approve PRs, and dispatch new tasks from anywhere. The Blackbox mobile app keeps you in control on the go.",
    bullets: [
      "Push notification alerts",
      "PR review & approval",
      "One-tap agent dispatch",
    ],
    link: { text: "Get the App", href: "#" },
  },
  {
    id: "builder",
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M11.42 15.17L17.25 21A2.652 2.652 0 0021 17.25l-5.877-5.877M11.42 15.17l2.496-3.03c.317-.384.74-.626 1.208-.766M11.42 15.17l-4.655 5.653a2.548 2.548 0 11-3.586-3.586l6.837-5.63m5.108-.233c.55-.164 1.163-.188 1.743-.14a4.5 4.5 0 004.486-6.336l-3.276 3.277a3.004 3.004 0 01-2.25-2.25l3.276-3.276a4.5 4.5 0 00-6.336 4.486c.091 1.076-.071 2.264-.904 2.95l-.102.085m-1.745 1.437L5.909 7.5H4.5L2.25 3.75l1.5-1.5L7.5 4.5v1.409l4.26 4.26m-1.745 1.437l1.745-1.437m6.615 8.206L15.75 15.75M4.867 19.125h.008v.008h-.008v-.008z" />
      </svg>
    ),
    title: "Builder",
    subtitle: "Describe it, agents build it",
    description:
      "Go from natural language to a deployed application. Agents handle the architecture, code, testing, and deployment — you describe the vision.",
    bullets: [
      "Natural language to app",
      "Full-stack generation",
      "One-click deployment",
    ],
    link: { text: "Try Builder", href: "#" },
  },
];

// ============================================
// FEATURES SECTION
// ============================================
function FeaturesSection() {
  return (
    <section className="py-32 px-6 bg-white text-black">
      <div className="max-w-7xl mx-auto">
        {/* Section Header */}
        <div className="mb-20">
          <p className="text-xs text-zinc-500 uppercase tracking-widest mb-4 font-mono">
            Your Agent Platform
          </p>
          <h2 className="text-4xl md:text-5xl lg:text-6xl font-bold tracking-tight leading-tight max-w-4xl">
            Run agents from anywhere,
            <br />
            <span className="text-zinc-400">anytime, autonomously.</span>
          </h2>
          <p className="text-lg text-zinc-600 mt-6 max-w-2xl leading-relaxed">
            One platform, six surfaces. Dispatch autonomous coding agents from your
            terminal, IDE, cloud, API, phone, or browser. They compete, collaborate,
            and ship code — while you focus on what matters.
          </p>
        </div>

        {/* Features Grid */}
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
          {features.map((feature) => (
            <div
              key={feature.id}
              className="group p-8 border border-zinc-200 rounded-xl hover:border-zinc-400 hover:shadow-lg transition-all duration-300 bg-white"
            >
              {/* Icon */}
              <div className="w-10 h-10 rounded-lg bg-zinc-100 flex items-center justify-center mb-6 group-hover:bg-zinc-200 transition-colors text-zinc-700">
                {feature.icon}
              </div>

              {/* Title */}
              <h3 className="text-xl font-bold mb-1">{feature.title}</h3>
              <p className="text-sm text-zinc-500 mb-4">{feature.subtitle}</p>

              {/* Description */}
              <p className="text-sm text-zinc-600 mb-6 leading-relaxed">
                {feature.description}
              </p>

              {/* Bullets */}
              <ul className="space-y-2 mb-6">
                {feature.bullets.map((bullet) => (
                  <li
                    key={bullet}
                    className="text-sm text-zinc-500 flex items-center gap-2"
                  >
                    <span className="w-1 h-1 rounded-full bg-zinc-400" />
                    {bullet}
                  </li>
                ))}
              </ul>

              {/* Link */}
              <Link
                href={feature.link.href}
                className="text-sm font-medium text-black hover:underline inline-flex items-center gap-1 group/link"
              >
                {feature.link.text}
                <svg
                  className="w-4 h-4 group-hover/link:translate-x-1 transition-transform"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M17 8l4 4m0 0l-4 4m4-4H3"
                  />
                </svg>
              </Link>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

// ============================================
// PARALLEL AGENTS SECTION
// ============================================
function ParallelAgentsSection() {
  const [activeAgent, setActiveAgent] = useState("claude");

  const agents = [
    {
      id: "claude",
      name: "Claude Code",
      badge: "winner",
      confidence: 0.94,
      response:
        "I'll use a sliding window algorithm with Redis MULTI/EXEC for atomicity. The middleware checks req count per IP in a 60s window, returns 429 when exceeded.",
    },
    {
      id: "codex",
      name: "Codex",
      confidence: 0.81,
      response:
        "Implementing token bucket via Redis INCR + EXPIRE. Each request decrements the bucket; refill rate is configurable per route. Includes retry-after header.",
    },
    {
      id: "blackbox",
      name: "Blackbox",
      confidence: 0.78,
      response:
        "I recommend a distributed rate limiter using Redis sorted sets for precise sliding windows. Supports per-user and per-endpoint limits with graceful degradation.",
    },
  ];

  return (
    <section className="py-32 px-6 bg-black text-white">
      <div className="max-w-7xl mx-auto">
        {/* Section Header */}
        <div className="mb-16">
          <p className="text-xs text-zinc-500 uppercase tracking-widest mb-4 font-mono">
            Chairman LLM
          </p>
          <h2 className="text-4xl md:text-5xl lg:text-6xl font-bold tracking-tight leading-tight">
            Run Agents in Parallel.
          </h2>
          <p className="text-lg text-zinc-400 mt-6 max-w-2xl leading-relaxed">
            Dispatch the same task to multiple AI agents, then let Chairman LLM
            evaluate every candidate on correctness, performance, risk, and
            complexity. Best output wins.
          </p>
        </div>

        {/* Interactive Demo */}
        <div className="grid lg:grid-cols-2 gap-8">
          {/* Left: Task + Agent Responses */}
          <div className="space-y-6">
            {/* Task Card */}
            <div className="p-6 bg-zinc-900 border border-zinc-800 rounded-xl">
              <p className="text-xs text-zinc-500 uppercase tracking-widest mb-3 font-mono">
                Task
              </p>
              <p className="text-white leading-relaxed">
                Implement rate limiting middleware with Redis backend for the API
                gateway
              </p>
            </div>

            {/* Agent Responses */}
            {agents.map((agent) => (
              <div
                key={agent.id}
                onClick={() => setActiveAgent(agent.id)}
                className={`p-6 border rounded-xl cursor-pointer transition-all duration-300 ${
                  activeAgent === agent.id
                    ? "bg-zinc-900 border-orange-500/50"
                    : "bg-zinc-900/50 border-zinc-800 hover:border-zinc-700"
                }`}
              >
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-3">
                    <span className="font-semibold">{agent.name}</span>
                    {agent.badge === "winner" && (
                      <span className="px-2 py-0.5 text-[10px] uppercase tracking-wider bg-orange-500/20 text-orange-500 rounded">
                        winner
                      </span>
                    )}
                  </div>
                  <span className="text-sm text-zinc-500">
                    {agent.confidence.toFixed(2)}
                  </span>
                </div>
                <p className="text-sm text-zinc-400 leading-relaxed">
                  {agent.response}
                </p>
              </div>
            ))}
          </div>

          {/* Right: Code Diff + Stats */}
          <div className="space-y-6">
            {/* Winner Card */}
            <div className="p-6 bg-zinc-900 border border-zinc-800 rounded-xl">
              <div className="flex items-center justify-between mb-4">
                <p className="text-xs text-zinc-500 uppercase tracking-widest font-mono">
                  Winner Selected
                </p>
                <div className="flex items-center gap-2">
                  <span className="text-orange-500 font-semibold">claude code</span>
                  <span className="text-sm text-zinc-500">confidence: 0.94</span>
                </div>
              </div>
              <div className="flex items-center gap-4 text-sm">
                <span className="text-emerald-400">TESTS: 46/46</span>
                <span className="text-zinc-500">correctness: 0.97</span>
              </div>
            </div>

            {/* PR Card */}
            <div className="p-6 bg-zinc-900 border border-zinc-800 rounded-xl">
              <div className="flex items-center gap-2 mb-4">
                <div className="w-2 h-2 rounded-full bg-emerald-500" />
                <span className="text-sm text-zinc-400">PR #218 opened</span>
              </div>

              <div className="space-y-2 font-mono text-sm">
                <div className="flex items-center justify-between text-zinc-400">
                  <span>src/middleware/rate-limit.ts</span>
                  <span className="text-emerald-400">+47 -12</span>
                </div>
                <div className="flex items-center justify-between text-zinc-400">
                  <span>src/config/redis.ts</span>
                  <span className="text-emerald-400">+18 -3</span>
                </div>
                <div className="flex items-center justify-between text-zinc-400">
                  <span>tests/rate-limit.test.ts</span>
                  <span className="text-emerald-400">+94 -0</span>
                </div>
              </div>

              <div className="mt-4 pt-4 border-t border-zinc-800 flex items-center justify-between text-sm">
                <span className="text-zinc-500">3 files changed</span>
                <span className="text-emerald-400">+159 -15</span>
              </div>
            </div>

            {/* Agent Status */}
            <div className="p-6 bg-zinc-900 border border-zinc-800 rounded-xl">
              <p className="text-xs text-zinc-500 uppercase tracking-widest mb-4 font-mono">
                Agent Status
              </p>
              <div className="space-y-3">
                {agents.map((agent) => (
                  <div
                    key={agent.id}
                    className="flex items-center justify-between text-sm"
                  >
                    <span className="text-zinc-400">{agent.name}</span>
                    <span className="text-zinc-600">queued</span>
                  </div>
                ))}
              </div>

              <div className="mt-6 pt-4 border-t border-zinc-800 space-y-2 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-zinc-500">Execution</span>
                  <span className="text-white">3 agents in parallel</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-zinc-500">Sequential</span>
                  <span className="text-zinc-400">~18 min</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-zinc-500">Parallel</span>
                  <span className="text-emerald-400">~6 min</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* CTA Buttons */}
        <div className="mt-16 flex flex-wrap items-center gap-4">
          <Link
            href="#"
            className="inline-flex items-center gap-2 h-12 px-6 bg-white text-black font-medium rounded-md hover:bg-zinc-200 transition-colors"
          >
            RUN AGENTS
            <svg
              className="w-4 h-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M17 8l4 4m0 0l-4 4m4-4H3"
              />
            </svg>
          </Link>
          <Link
            href="/docs"
            className="inline-flex items-center gap-2 h-12 px-6 border border-zinc-700 text-white font-medium rounded-md hover:bg-zinc-900 transition-colors"
          >
            Read Docs
          </Link>
        </div>
      </div>
    </section>
  );
}

// ============================================
// CODE COMPARISON DEMO
// ============================================
function CodeComparisonSection() {
  const codeLines = [
    { num: 1, text: "import { Redis } from '@upstash/redis';", type: "normal" },
    { num: 2, text: "import type { NextRequest } from 'next/server';", type: "normal" },
    { num: 3, text: "", type: "normal" },
    { num: 4, text: "-const RATE_LIMIT = 100;", type: "removed" },
    { num: 5, text: "-const WINDOW_MS = 60_000;", type: "removed" },
    { num: 6, text: "+interface SlidingWindowConfig {", type: "added" },
    { num: 7, text: "+  maxRequests: number;", type: "added" },
    { num: 8, text: "+  windowMs: number;", type: "added" },
    { num: 9, text: "+  keyPrefix?: string;", type: "added" },
    { num: 10, text: "+}", type: "added" },
    { num: 11, text: "+", type: "added" },
    { num: 12, text: "+const DEFAULT_CONFIG: SlidingWindowConfig = {", type: "added" },
    { num: 13, text: "+  maxRequests: 100,", type: "added" },
    { num: 14, text: "+  windowMs: 60_000,", type: "added" },
    { num: 15, text: "+  keyPrefix: 'rl:sw',", type: "added" },
    { num: 16, text: "+};", type: "added" },
  ];

  return (
    <section className="py-32 px-6 bg-zinc-950 text-white border-t border-zinc-900">
      <div className="max-w-7xl mx-auto">
        {/* Section Header */}
        <div className="mb-16">
          <p className="text-xs text-zinc-500 uppercase tracking-widest mb-4 font-mono">
            Multi-Harness
          </p>
          <h2 className="text-4xl md:text-5xl lg:text-6xl font-bold tracking-tight leading-tight">
            Every agent harness.
            <br />
            <span className="text-orange-500">One platform.</span>
          </h2>
          <p className="text-lg text-zinc-400 mt-6 max-w-2xl leading-relaxed">
            Claude Code, Codex, Blackbox — access every coding agent through a
            single API. Compare harnesses side by side and ship the best result.
          </p>
        </div>

        {/* Code Comparison UI */}
        <div className="grid lg:grid-cols-3 gap-4 mb-8">
          {/* Agent Tabs */}
          {[
            { name: "Claude Code", file: "src/middleware/rate-limiter.ts", diff: "+35-8", score: 0.94 },
            { name: "Codex", file: "src/lib/cache.ts", diff: "+30-9", score: 0.81 },
            { name: "Blackbox", file: "src/hooks/use-debounce.ts", diff: "+27-8", score: 0.78 },
          ].map((agent, i) => (
            <div
              key={agent.name}
              className={`p-4 rounded-lg border cursor-pointer transition-all ${
                i === 0
                  ? "bg-zinc-900 border-orange-500/50"
                  : "bg-zinc-900/50 border-zinc-800 hover:border-zinc-700"
              }`}
            >
              <div className="flex items-center justify-between mb-2">
                <span className="font-medium text-sm">{agent.name}</span>
                <span
                  className={`text-sm ${
                    i === 0 ? "text-orange-500" : "text-zinc-500"
                  }`}
                >
                  {agent.score}
                </span>
              </div>
              <div className="flex items-center justify-between text-xs text-zinc-500">
                <span className="font-mono truncate">{agent.file}</span>
                <span className="text-emerald-400">{agent.diff}</span>
              </div>
            </div>
          ))}
        </div>

        {/* Code Viewer */}
        <div className="bg-[#0a0a0a] border border-zinc-800 rounded-xl overflow-hidden">
          <div className="px-4 py-3 border-b border-zinc-800 flex items-center justify-between">
            <span className="text-sm text-zinc-400 font-mono">
              src/middleware/rate-limiter.ts
            </span>
            <span className="text-sm text-emerald-400 font-mono">+35 -8</span>
          </div>
          <div className="p-4 overflow-x-auto">
            <pre className="font-mono text-sm leading-6">
              {codeLines.map((line) => (
                <div
                  key={line.num}
                  className={`flex ${
                    line.type === "added"
                      ? "bg-emerald-500/10"
                      : line.type === "removed"
                      ? "bg-red-500/10"
                      : ""
                  }`}
                >
                  <span className="w-12 text-zinc-600 text-right pr-4 select-none">
                    {line.num}
                  </span>
                  <span
                    className={
                      line.type === "added"
                        ? "text-emerald-400"
                        : line.type === "removed"
                        ? "text-red-400"
                        : "text-zinc-400"
                    }
                  >
                    {line.text}
                  </span>
                </div>
              ))}
            </pre>
          </div>
        </div>

        {/* Feature Cards */}
        <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-6 mt-16">
          {[
            {
              title: "Claude Code",
              desc: "The best model from Anthropic, fine-tuned on coding patterns, and more.",
            },
            {
              title: "Codex",
              desc: "Model created by OpenAI to translate natural language to code.",
            },
            {
              title: "Blackbox",
              desc: "Deploy autonomous agents to the cloud. They monitor fix, and optimize.",
            },
            {
              title: "Builder",
              desc: "Go from natural language to a deployed application. Agents handle it all.",
            },
          ].map((card) => (
            <div
              key={card.title}
              className="p-6 border border-zinc-800 rounded-xl hover:border-zinc-700 transition-colors"
            >
              <h3 className="font-semibold mb-2">{card.title}</h3>
              <p className="text-sm text-zinc-500 leading-relaxed">{card.desc}</p>
              <Link
                href="#"
                className="text-sm text-zinc-400 hover:text-white mt-4 inline-block"
              >
                Explore {card.title} &rarr;
              </Link>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

// ============================================
// CTA SECTION
// ============================================
function CTASection() {
  return (
    <section className="py-32 px-6 bg-black text-white text-center border-t border-zinc-900">
      <div className="max-w-4xl mx-auto">
        <h2 className="text-4xl md:text-5xl lg:text-6xl font-bold tracking-tight leading-tight mb-6">
          Start building with
          <br />
          <span className="text-orange-500">BLACKBOX AI.</span>
        </h2>
        <p className="text-lg text-zinc-400 mb-10 max-w-xl mx-auto leading-relaxed">
          Multi-agent execution, AI-native IDE, CLI, API, and mobile — all free
          to start.
        </p>
        <div className="flex flex-wrap items-center justify-center gap-4">
          <Link
            href="#"
            className="inline-flex items-center gap-2 h-12 px-6 bg-white text-black font-medium rounded-md hover:bg-zinc-200 transition-colors"
          >
            GET STARTED FREE
            <svg
              className="w-4 h-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M17 8l4 4m0 0l-4 4m4-4H3"
              />
            </svg>
          </Link>
          <Link
            href="#"
            className="inline-flex items-center gap-2 h-12 px-6 border border-zinc-700 text-white font-medium rounded-md hover:bg-zinc-900 transition-colors"
          >
            VIEW PRICING
          </Link>
        </div>
      </div>
    </section>
  );
}

// ============================================
// FOOTER
// ============================================
function Footer() {
  const footerLinks = {
    Resources: ["Pricing", "Blog", "Docs", "AI Experts", "Careers"],
    Products: ["Cloud", "IDE", "CLI", "API", "Mobile", "Builder", "VS Code"],
    Legal: ["Terms of Service", "Privacy Policy", "Contact Us"],
  };

  return (
    <footer className="py-16 px-6 bg-black text-white border-t border-zinc-900">
      <div className="max-w-7xl mx-auto">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-10 mb-16">
          {/* Logo */}
          <div>
            <div className="flex items-center gap-1.5 mb-6">
              <div className="w-5 h-5 bg-white rounded-sm flex items-center justify-center">
                <span className="text-black font-bold text-xs">B</span>
              </div>
              <span className="text-white font-semibold text-sm tracking-tight">
                BLACKBOX.AI
              </span>
            </div>
          </div>

          {/* Links */}
          {Object.entries(footerLinks).map(([category, links]) => (
            <div key={category}>
              <h4 className="text-sm font-semibold mb-4 text-zinc-400">
                {category}
              </h4>
              <ul className="space-y-3">
                {links.map((link) => (
                  <li key={link}>
                    <Link
                      href="#"
                      className="text-sm text-zinc-500 hover:text-white transition-colors"
                    >
                      {link}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        {/* Bottom */}
        <div className="pt-8 border-t border-zinc-900 flex flex-col sm:flex-row items-center justify-between gap-4">
          <p className="text-xs text-zinc-600">
            &copy; 2024 Blackbox. 548 Market Street, San Francisco CA 94104
          </p>
          <div className="flex items-center gap-6">
            <Link href="#" className="text-zinc-600 hover:text-white transition-colors">
              <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
              </svg>
            </Link>
            <Link href="#" className="text-zinc-600 hover:text-white transition-colors">
              <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
              </svg>
            </Link>
            <Link href="#" className="text-zinc-600 hover:text-white transition-colors">
              <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                <path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433c-1.144 0-2.063-.926-2.063-2.065 0-1.138.92-2.063 2.063-2.063 1.14 0 2.064.925 2.064 2.063 0 1.139-.925 2.065-2.064 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z" />
              </svg>
            </Link>
          </div>
        </div>
      </div>
    </footer>
  );
}

// ============================================
// MAIN PAGE
// ============================================
export default function Home() {
  return (
    <div className="bg-black min-h-screen">
      <style jsx global>{`
        @keyframes fadeInUp {
          from {
            opacity: 0;
            transform: translateY(20px);
          }
          to {
            opacity: 0.6;
            transform: translateY(0);
          }
        }
      `}</style>
      <Header />
      <HeroSection />
      <FeaturesSection />
      <ParallelAgentsSection />
      <CodeComparisonSection />
      <CTASection />
      <Footer />
    </div>
  );
}
