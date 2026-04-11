"use client";

import { motion, useInView, AnimatePresence } from "framer-motion";
import Link from "next/link";
import {
  useRef,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
  type AnchorHTMLAttributes,
  type ComponentPropsWithoutRef,
} from "react";

const EASE = [0.16, 1, 0.3, 1] as const;

export function ScrollReveal({
  children,
  delay = 0,
  className = "",
}: {
  children: ReactNode;
  delay?: number;
  className?: string;
}) {
  const ref = useRef(null);
  const inView = useInView(ref, { once: true, margin: "-80px" });

  return (
    <motion.div
      ref={ref}
      className={className}
      initial={{ opacity: 0, y: 30 }}
      animate={inView ? { opacity: 1, y: 0 } : { opacity: 0, y: 30 }}
      transition={{ duration: 0.7, delay, ease: EASE }}
    >
      {children}
    </motion.div>
  );
}

export function StaggerGrid({
  children,
  stagger = 0.1,
  className = "",
}: {
  children: ReactNode;
  stagger?: number;
  className?: string;
}) {
  const ref = useRef(null);
  const inView = useInView(ref, { once: true, margin: "-60px" });

  return (
    <motion.div
      ref={ref}
      className={className}
      initial="hidden"
      animate={inView ? "visible" : "hidden"}
      variants={{
        visible: { transition: { staggerChildren: stagger } },
        hidden: {},
      }}
    >
      {children}
    </motion.div>
  );
}

export function StaggerItem({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <motion.div
      className={className}
      variants={{
        hidden: { opacity: 0, y: 30 },
        visible: {
          opacity: 1,
          y: 0,
          transition: { duration: 0.6, ease: EASE },
        },
      }}
    >
      {children}
    </motion.div>
  );
}

export function SpotlightCard({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState({ x: 0, y: 0 });
  const [hovering, setHovering] = useState(false);

  return (
    <div
      ref={ref}
      onMouseMove={(e) => {
        if (!ref.current) return;
        const rect = ref.current.getBoundingClientRect();
        setPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
      }}
      onMouseEnter={() => setHovering(true)}
      onMouseLeave={() => setHovering(false)}
      className={`relative overflow-hidden ${className}`}
    >
      {hovering && (
        <div
          className="absolute inset-0 pointer-events-none z-0 transition-opacity"
          style={{
            background: `radial-gradient(400px circle at ${pos.x}px ${pos.y}px, rgba(234,88,12,0.07), transparent 50%)`,
          }}
        />
      )}
      <div className="relative z-10">{children}</div>
    </div>
  );
}

// --- Animated Terminal Grid ---

export type TerminalCardDef = {
  title: string;
  status: "ok" | "warn" | "error";
  brightness: number;
  pace: "fast" | "normal" | "slow";
  lines: string[];
};

const PACE_CONFIG = {
  fast:   { lineInterval: 160, holdTime: 2000 },
  normal: { lineInterval: 460, holdTime: 4000 },
  slow:   { lineInterval: 640, holdTime: 5600 },
} as const;

const DOT_COLORS = {
  ok:    { idle: "bg-emerald-500/80", active: "bg-emerald-400 animate-pulse" },
  warn:  { idle: "bg-amber-500/80",   active: "bg-amber-400 animate-pulse" },
  error: { idle: "bg-red-500/80",     active: "bg-red-400 animate-pulse" },
} as const;

function lineColor(line: string) {
  if (line.startsWith(">")) return "#999";
  if (line.includes("[WARN]")) return "rgb(251 191 36 / 0.8)";
  if (line.includes("[ERR]")) return "rgb(248 113 113 / 0.8)";
  return "#666";
}

export const TERMINAL_CARDS: TerminalCardDef[] = [
  { title: "SETUP // BOOTSTRAP", status: "ok", brightness: 1.0, pace: "normal", lines: [
    "> forged setup",
    "[INIT] Creating vault at ~/.forged/",
    "[INIT] Master password: ********",
    "[SCAN] Found 5 keys in ~/.ssh/",
    "[IMPORT] id_ed25519 ... ok",
    "[IMPORT] id_rsa ... ok",
    "[IMPORT] deploy_key ... ok",
    "[DAEMON] Binding socket 0600",
  ]},
  { title: "AGENT // STATUS", status: "ok", brightness: 0.75, pace: "slow", lines: [
    "> forged status",
    "Daemon: running (PID 4821)",
    "Keys:   4 loaded",
    "Socket: ~/.forged/agent.sock",
  ]},
  { title: "KEYGEN // ED25519", status: "ok", brightness: 1.1, pace: "fast", lines: [
    "> forged generate deploy-prod",
    "[GEN] Algorithm: Ed25519",
    "[GEN] Comment: deploy@prod",
    "[VAULT] Encrypting with XChaCha20",
    "[VAULT] Nonce: random 24-byte",
    "[OK] Key deploy-prod created",
    "[OK] Vault synced to disk",
  ]},
  { title: "SYNC // CLOUD", status: "ok", brightness: 1.1, pace: "fast", lines: [
    "> forged sync",
    "[AUTH] Token valid (exp 2026-04-11)",
    "[HKDF] Deriving sync key...",
    "[UPLOAD] Encrypting vault blob",
    "[UPLOAD] 4.2 KB -> blob storage",
    "[OK] Sync complete (312ms)",
    "[OK] 4 keys propagated",
  ]},
  { title: "HOST // BINDING", status: "ok", brightness: 0.75, pace: "slow", lines: [
    "> forged hosts",
    "  NAME        PATTERNS",
    "  github      *.github.com",
    "  deploy      *.prod.company.com",
    "  personal    *",
    "> forged host github \"github.com\"",
    "[OK] Pattern bound: github.com",
  ]},
  { title: "SSH // CONNECT", status: "ok", brightness: 1.1, pace: "fast", lines: [
    "> ssh git@github.com",
    "[AGENT] Request from ssh (pid 9102)",
    "[MATCH] github.com -> github key",
    "[AUTH] Ed25519 challenge-response",
    "[OK] Authenticated as git",
    "Hi user! You've successfully",
    "authenticated with key: github",
  ]},
  { title: "MIGRATE // IMPORT", status: "warn", brightness: 1.0, pace: "normal", lines: [
    "> forged migrate --from ssh",
    "[SCAN] Reading ~/.ssh/ ...",
    "[FOUND] id_ed25519 (Ed25519)",
    "[FOUND] id_rsa (RSA 2048-bit)",
    "[WARN] id_rsa uses weak RSA-2048",
    "[IMPORT] 2 keys ingested",
    "[VAULT] Re-encrypted with Argon2id",
  ]},
  { title: "GIT // SIGNING", status: "ok", brightness: 0.75, pace: "normal", lines: [
    "> git commit -m \"fix auth flow\"",
    "[SIGN] Request from git (pid 3401)",
    "[MATCH] git -> signing key",
    "[SIGN] SSH signature created",
    "[OK] Commit signed: a3f2b1c",
    "[OK] Verified: ssh-ed25519",
    "1 file changed, 12 insertions(+)",
  ]},
  { title: "VAULT // ENCRYPT", status: "ok", brightness: 0.75, pace: "slow", lines: [
    "> forged lock",
    "[LOCK] Zeroing memory pages...",
    "[LOCK] mlock() released 4 pages",
    "[LOCK] Agent socket suspended",
    "[OK] Vault locked, 0 keys in mem",
    "> forged unlock",
    "Master password: ********",
  ]},
  { title: "DAEMON // LOGS", status: "ok", brightness: 1.0, pace: "fast", lines: [
    "> forged logs",
    "14:23:07 [INFO] github.com -> ok",
    "14:23:08 [INFO] key: github",
    "14:23:08 [INFO] auth: success",
    "14:23:09 [INFO] session: active",
    "14:25:11 [INFO] prod.co -> ok",
    "14:25:12 [INFO] key: deploy",
  ]},
  { title: "CONFIG // TOML", status: "ok", brightness: 1.1, pace: "slow", lines: [
    "[[hosts]]",
    "name = \"GitHub\"",
    "match = [\"github.com\"]",
    "key = \"github\"",
    "git_signing = true",
    "",
    "[[hosts]]",
  ]},
  { title: "DOCTOR // CHECK", status: "ok", brightness: 1.0, pace: "normal", lines: [
    "> forged doctor",
    "[CHECK] Vault integrity ... ok",
    "[CHECK] Daemon running ... ok",
    "[CHECK] Socket perms 0600 ... ok",
    "[CHECK] SSH config ... ok",
    "[CHECK] Argon2id params ... ok",
    "[OK] All 5 checks passed",
  ]},
  { title: "LIST // KEYS", status: "ok", brightness: 0.75, pace: "slow", lines: [
    "> forged list",
    "  NAME       TYPE          FINGERPRINT",
    "  github     ssh-ed25519   SHA256:xK3...",
    "  deploy     ssh-ed25519   SHA256:mN7...",
    "  personal   ssh-rsa       SHA256:pQ2...",
    "  signing    ssh-ed25519   SHA256:vB9...",
  ]},
  { title: "EXPORT // KEY", status: "ok", brightness: 1.1, pace: "normal", lines: [
    "> forged export github",
    "ssh-ed25519 AAAAC3NzaC1lZDI1",
    "NTE5AAAAIG8f3kR7vKJzMnL+hW2",
    "Kf9mN3pQ5xR1tY6uI0oP8aS2dF4",
    "gH7jK1lZ3xC5vB9nM2qW8eR6tY0",
    "uI4oP7aS1dF3gH6jK0 github@f",
    "[OK] Public key written to stdout",
  ]},
  { title: "BENCHMARK // ARGON", status: "warn", brightness: 1.0, pace: "normal", lines: [
    "> forged benchmark",
    "[BENCH] Argon2id 64MB 3 iter",
    "[BENCH] Derive: 287ms avg",
    "[BENCH] Encrypt: 0.4ms avg",
    "[BENCH] Decrypt: 0.3ms avg",
    "[BENCH] Total: 288ms per unlock",
    "[OK] Within security threshold",
  ]},
  { title: "SYNC // STATUS", status: "ok", brightness: 1.1, pace: "fast", lines: [
    "> forged sync status",
    "account:  user@forged.dev",
    "last_sync: 2m ago",
    "blob_size: 4.2 KB",
    "devices:   3 linked",
    "conflicts: 0",
    "[OK] Vault in sync",
  ]},
  { title: "SECURITY // DOCTOR", status: "ok", brightness: 1.0, pace: "normal", lines: [
    "> forged doctor --fix",
    "[PASS] Vault exists",
    "[PASS] Config exists",
    "[PASS] Daemon running (PID 4821)",
    "[PASS] Agent socket 0600",
    "[PASS] IPC socket",
    "[FIXED] SSH agent IdentityAgent",
  ]},
  { title: "RENAME // KEY", status: "ok", brightness: 0.75, pace: "slow", lines: [
    "> forged rename personal backup",
    "Renamed personal -> backup",
    "",
    "> forged list --json",
    "{\"keys\":[{\"name\":\"backup\",",
    "\"type\":\"ssh-rsa\",",
    "\"fingerprint\":\"SHA256:pQ2\"}]}",
  ]},
  { title: "UNHOST // UNBIND", status: "ok", brightness: 1.1, pace: "fast", lines: [
    "> forged unhost deploy \"10.0.*\"",
    "[ROUTE] Removing pattern...",
    "[OK] Unbound: 10.0.*",
    "> forged hosts",
    "  NAME     PATTERNS",
    "  github   *.github.com",
    "  deploy   *.prod.company.com",
  ]},
  { title: "DAEMON // START", status: "ok", brightness: 1.0, pace: "fast", lines: [
    "> forged start",
    "Daemon started (PID 4821)",
  ]},
];

export function AnimatedTerminalGrid({ cards }: { cards: TerminalCardDef[] }) {
  const gridRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const grid = gridRef.current;
    if (!grid) return;

    const terminals = grid.querySelectorAll<HTMLElement>("[data-terminal]");
    const visible = new Int32Array(cards.length);
    const BLANK = 300;
    let rafId: number | null = null;
    let active = true;
    const start = performance.now();

    function tick(now: number) {
      for (let t = 0; t < cards.length; t++) {
        const card: TerminalCardDef = cards[t];
        const { lineInterval, holdTime } = PACE_CONFIG[card.pace];
        const delay = t * 120;
        const elapsed = now - start - delay;
        if (elapsed < 0) continue;

        const revealDuration = card.lines.length * lineInterval;
        const cycle = BLANK + revealDuration + holdTime;
        const phase = elapsed % cycle;

        let count: number;
        if (phase < BLANK) {
          count = 0;
        } else if (phase < BLANK + revealDuration) {
          count = Math.floor((phase - BLANK) / lineInterval) + 1;
        } else {
          count = card.lines.length;
        }

        if (count === visible[t]) continue;
        visible[t] = count;

        const el = terminals[t];
        if (!el) continue;

        const lineEls = el.querySelectorAll<HTMLElement>("[data-line]");
        for (let i = 0; i < lineEls.length; i++) {
          lineEls[i].style.visibility = i < count ? "visible" : "hidden";
        }

        const dot = el.querySelector<HTMLElement>("[data-dot]");
        if (dot) {
          const streaming = count > 0 && count < card.lines.length;
          const colors = DOT_COLORS[card.status];
          dot.className = `w-1.5 h-1.5 rounded-full ${streaming ? colors.active : colors.idle}`;
        }
      }

      if (active && document.visibilityState === "visible") {
        rafId = requestAnimationFrame(tick);
      }
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && !rafId) {
          rafId = requestAnimationFrame(tick);
        } else if (!entry.isIntersecting && rafId) {
          cancelAnimationFrame(rafId);
          rafId = null;
        }
      },
      { threshold: 0.1 },
    );
    observer.observe(grid);

    const onVis = () => {
      if (document.visibilityState === "visible" && !rafId) {
        rafId = requestAnimationFrame(tick);
      } else if (document.visibilityState === "hidden" && rafId) {
        cancelAnimationFrame(rafId);
        rafId = null;
      }
    };
    document.addEventListener("visibilitychange", onVis);

    return () => {
      active = false;
      if (rafId) cancelAnimationFrame(rafId);
      observer.disconnect();
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [cards]);

  return (
    <div
      ref={gridRef}
      className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 grid-rows-5 gap-px bg-[#1a1a1a] h-full"
    >
      {cards.map((card: TerminalCardDef) => (
        <div
          key={card.title}
          data-terminal
          className="flex flex-col bg-black border border-[#1a1a1a] overflow-hidden"
          style={{ filter: `brightness(${card.brightness})` }}
        >
          <div className="flex items-center justify-between h-7 px-3 border-b border-[#1a1a1a] shrink-0">
            <span className="text-[10px] text-[#555] font-mono tracking-[0.5px]">{card.title}</span>
            <span data-dot className={`w-1.5 h-1.5 rounded-full ${DOT_COLORS[card.status].idle}`} />
          </div>
          <div className="flex-1 p-3 overflow-hidden">
            {card.lines.map((line: string, i: number) => (
              <div
                key={i}
                data-line
                className="font-mono text-[11px] leading-[1.7] whitespace-pre truncate"
                style={{ visibility: "hidden", color: lineColor(line) }}
              >
                {line}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

// --- Glitch Button ---

const GLITCH_CHARS = "!@#$%^&*0123456789";
const GLITCH_DURATION = 400;

function useGlitchText(text: string) {
  const [display, setDisplay] = useState(text);
  const animating = useRef(false);

  useEffect(() => {
    setDisplay(text);
  }, [text]);

  const scramble = useCallback(() => {
    if (animating.current) return;
    if (typeof window !== "undefined") {
      if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
      if (window.matchMedia("(max-width: 768px)").matches) return;
    }
    animating.current = true;

    const chars = text.split("");
    const rand = () => GLITCH_CHARS[Math.floor(Math.random() * GLITCH_CHARS.length)];
    setDisplay(chars.map((c) => (c === " " ? " " : rand())).join(""));

    const start = performance.now();
    const tick = (now: number) => {
      const progress = Math.min(1, (now - start) / GLITCH_DURATION);
      const resolved = Math.floor(progress * chars.length);
      setDisplay(
        chars.map((c, i) => (c === " " ? " " : i < resolved ? c : rand())).join(""),
      );
      if (progress < 1) {
        requestAnimationFrame(tick);
      } else {
        setDisplay(text);
        animating.current = false;
      }
    };
    requestAnimationFrame(tick);
  }, [text]);

  return { display, scramble };
}

export function GlitchText({ text, className = "" }: { text: string; className?: string }) {
  const { display, scramble } = useGlitchText(text);
  const ref = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const target = el.closest("button, a, [role='button']");
    if (!target) return;
    const handler = () => scramble();
    target.addEventListener("mouseenter", handler);
    return () => target.removeEventListener("mouseenter", handler);
  }, [scramble]);

  return <span ref={ref} className={className}>{display}</span>;
}

type GlitchButtonProps = {
  children: string;
  variant?: "primary" | "secondary";
  href?: string;
  external?: boolean;
} & Omit<AnchorHTMLAttributes<HTMLAnchorElement> & ComponentPropsWithoutRef<typeof Link>, "children">;

export function GlitchButton({
  children,
  variant = "primary",
  href,
  external,
  className = "",
  ...rest
}: GlitchButtonProps) {
  const { display, scramble } = useGlitchText(children);

  const isPrimary = variant === "primary";
  const base =
    "group relative inline-flex items-center justify-center gap-2 font-mono text-sm font-bold tracking-wider uppercase overflow-hidden transition-colors active:scale-[0.97]";
  const style = isPrimary
    ? "bg-white text-black hover:bg-zinc-200"
    : "bg-transparent text-white border border-[#27272a] hover:border-[#ea580c] hover:text-[#ea580c]";
  const overlayOpacity = isPrimary
    ? "group-hover:opacity-[0.12]"
    : "group-hover:opacity-[0.30]";

  const inner = (
    <>
      <span className="relative z-[2]">{display}</span>
      {/* Crosshatch overlay */}
      <div
        aria-hidden
        className={`absolute inset-0 z-[1] pointer-events-none opacity-0 ${overlayOpacity} transition-opacity duration-200`}
        style={{
          background: [
            "repeating-linear-gradient(135deg, transparent, transparent 4px, currentColor 4px, currentColor 5px)",
            "repeating-linear-gradient(45deg, transparent, transparent 4px, currentColor 4px, currentColor 5px)",
          ].join(", "),
          backgroundSize: "8px 8px",
        }}
      />
    </>
  );

  const classes = `${base} ${style} ${className}`;

  if (external && href) {
    return (
      <a href={href} className={classes} onMouseEnter={scramble} {...rest}>
        {inner}
      </a>
    );
  }

  if (href) {
    return (
      <Link href={href} className={classes} onMouseEnter={scramble} {...rest}>
        {inner}
      </Link>
    );
  }

  return (
    <button className={classes} onMouseEnter={scramble} {...(rest as React.ButtonHTMLAttributes<HTMLButtonElement>)}>
      {inner}
    </button>
  );
}

export type TerminalStep = {
  command: string;
  output: string[];
  pauseAfter?: number;
};

function TerminalOutputLine({ text }: { text: string }) {
  if (/^\s+-+\s+-+/.test(text)) return <span className="text-[#27272a]">{text}</span>;
  if (/^\s+(NAME|TYPE|FINGERPRINT|SIGNING)/.test(text)) return <span className="text-[#52525b]">{text}</span>;
  if (text.startsWith("Mapped ")) return <span className="text-[#10b981]">{text}</span>;
  if (text.includes("Connection") && text.includes("closed")) return <span className="text-[#52525b]">{text}</span>;
  if (/^\w+@[\w-]+:[~\/]/.test(text)) return <span className="text-[#ea580c]">{text}</span>;
  return <span className="text-[#a1a1aa]">{text}</span>;
}

export function AnimatedBigTerminal({ steps }: { steps: TerminalStep[] }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [lines, setLines] = useState<{ id: string; text: string; type: "cmd" | "out"; ts: string }[]>([]);
  const [typing, setTyping] = useState("");
  const [cursorOn, setCursorOn] = useState(true);
  const tsRef = useRef({ base: Date.now(), offset: 0 });

  useEffect(() => {
    const id = setInterval(() => setCursorOn(v => !v), 530);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    let alive = true;
    let step = 0;
    const sleep = (ms: number) => new Promise<void>(r => setTimeout(r, ms));

    const ts = () => {
      const t = tsRef.current;
      const d = new Date(t.base + t.offset);
      t.offset += 20 + Math.floor(Math.random() * 180);
      const hh = d.getHours().toString().padStart(2, "0");
      const mm = d.getMinutes().toString().padStart(2, "0");
      const ss = d.getSeconds().toString().padStart(2, "0");
      const ms = d.getMilliseconds().toString().padStart(3, "0");
      return `${hh}:${mm}:${ss}.${ms}`;
    };

    const charDelay = (ch: string, prev: string): number => {
      if (ch === " ") return 15 + Math.random() * 12;
      if ('"\'*@'.includes(ch)) return 60 + Math.random() * 30;
      if (".-/~".includes(ch)) return 28 + Math.random() * 14;
      if (prev === " " && Math.random() < 0.12) return 130 + Math.random() * 90;
      return 25 + Math.random() * 28;
    };

    async function animate() {
      await sleep(700);

      while (alive) {
        const s = steps[step];

        let typed = "";
        let prev = "";
        for (const ch of s.command) {
          if (!alive) return;
          typed += ch;
          setTyping(typed);
          await sleep(charDelay(ch, prev));
          prev = ch;
        }

        if (!alive) return;

        setLines(p => [...p, { id: `c${step}-${Date.now()}`, text: s.command, type: "cmd", ts: ts() }]);
        setTyping("");

        await sleep(160 + Math.random() * 120);

        for (let i = 0; i < s.output.length; i++) {
          if (!alive) return;
          const line = s.output[i];
          setLines(p => [...p, { id: `o${step}-${i}-${Date.now()}`, text: line, type: "out", ts: ts() }]);

          let d = 45;
          if (/^\s+-+/.test(line)) d = 12;
          else if (i === 0 && s.output.length > 3) d = 20;
          else if (line.includes("Welcome") || line.includes("Last login")) d = 110;
          await sleep(d + Math.random() * 25);
        }
        setLines(p => [...p, { id: `g${step}-${Date.now()}`, text: "\u00A0", type: "out", ts: "" }]);

        await sleep(s.pauseAfter ?? 2500);

        step = (step + 1) % steps.length;
      }
    }

    animate();
    return () => { alive = false; };
  }, [steps]);

  useEffect(() => {
    containerRef.current?.scrollTo({ top: containerRef.current.scrollHeight, behavior: "smooth" });
  }, [lines, typing]);

  return (
    <div className="w-full h-full relative flex flex-col overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_top,_rgba(234,88,12,0.04)_0%,_transparent_55%)] pointer-events-none" />

      <div
        ref={containerRef}
        className="p-5 md:px-6 md:py-5 flex-1 overflow-hidden [&::-webkit-scrollbar]:hidden relative z-10 font-mono text-[11px] md:text-[13px] leading-[1.9]"
        style={{ tabSize: 8 }}
      >
        <AnimatePresence initial={false}>
          {lines.map((line) => (
            <motion.div
              key={line.id}
              initial={{ opacity: 0, y: 5 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.12, ease: "easeOut" }}
              className="whitespace-pre flex"
            >
              {line.ts && <span className="text-[#1e1e21] select-none mr-4 shrink-0 hidden lg:inline">{line.ts}</span>}
              <span className="flex-1">
                {line.type === "cmd" ? (
                  <>
                    <span className="text-[#ea580c] select-none">$ </span>
                    <span className="text-white">{line.text}</span>
                  </>
                ) : (
                  <TerminalOutputLine text={line.text} />
                )}
              </span>
            </motion.div>
          ))}
        </AnimatePresence>

        {/* Active typing line with cursor */}
        <div className="whitespace-pre flex">
          <span className="text-[#1e1e21] select-none mr-4 shrink-0 hidden lg:inline">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</span>
          <span className="flex-1">
            <span className="text-[#ea580c] select-none">$ </span>
            <span className="text-white">{typing}</span>
            <span
              className="inline-block w-[7px] h-[14px] ml-px translate-y-[2px] transition-opacity duration-75"
              style={{
                backgroundColor: cursorOn ? "#ea580c" : "transparent",
                boxShadow: cursorOn ? "0 0 8px rgba(234,88,12,0.5)" : "none",
              }}
            />
          </span>
        </div>

      </div>
    </div>
  );
}



// --- Topology Blueprint (Architecture) ---

export function TopologyVisualizer() {
  return (
    <div className="relative w-full mt-16 md:mt-24">
      {/* Abstract Background SVG Pattern */}
      <div className="absolute inset-0 opacity-[0.03] pointer-events-none" style={{ backgroundImage: "radial-gradient(#ea580c 1px, transparent 1px)", backgroundSize: "32px 32px" }} />

      {/* Grid Layout */}
      <div className="grid grid-cols-1 md:grid-cols-12 gap-8 relative z-10 w-full mb-8 md:mb-12">
        {/* Node 01: UNIX SOCKET (Small) */}
        <motion.div 
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          className="md:col-span-5 relative border border-[#27272a] bg-[#050505] p-6 shadow-2xl flex flex-col group hover:border-[#10b981]/30 transition-colors h-[280px] w-full"
        >
          <div className="border-b border-[#27272a] pb-4 mb-6 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="text-[#a1a1aa] font-mono text-[10px] tracking-[0.2em]">01 //</span>
              <span className="text-white font-mono text-[11px] tracking-widest uppercase font-bold">Ingress Socket</span>
            </div>
            <div className="w-1.5 h-1.5 bg-[#10b981] shadow-[0_0_8px_#10b981] animate-pulse rounded-full" />
          </div>
          <p className="text-sm text-[#a1a1aa] leading-relaxed mb-6 flex-1">
            Replaces the standard ssh-agent. Exposes a native UNIX socket locally, dropping perfectly into your existing ecosystem without requiring custom clients.
          </p>
          <div className="h-16 bg-black border border-[#18181b] flex items-center justify-between px-4 mt-auto shrink-0 w-full overflow-hidden">
            <span className="text-[#10b981] font-mono text-[10px] tracking-widest flex items-center gap-2 truncate shrink-0">
              <span className="w-1.5 h-1.5 bg-[#10b981] shadow-[0_0_8px_#10b981] rounded-full animate-pulse shrink-0" />
              <span className="hidden sm:inline font-bold">LISTENING</span>
            </span>
            <span className="text-[#a1a1aa] font-mono text-[10px] tracking-widest opacity-60 truncate ml-4 shrink-0">/tmp/forged.sock</span>
          </div>
        </motion.div>

        {/* Node 02: SYNC NODE (Big) */}
        <motion.div 
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ delay: 0.1 }}
          className="md:col-span-7 relative border border-[#27272a] bg-[#050505] p-6 shadow-2xl flex flex-col group hover:border-[#ea580c]/30 transition-colors h-[280px] w-full"
        >
          <div className="border-b border-[#27272a] pb-4 mb-6 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="text-[#a1a1aa] font-mono text-[10px] tracking-[0.2em]">02 //</span>
              <span className="text-white font-mono text-[11px] tracking-widest uppercase font-bold">E2E Sync Node</span>
            </div>
            <div className="w-1.5 h-1.5 bg-[#ea580c] shadow-[0_0_8px_#ea580c] animate-pulse rounded-full" />
          </div>
          <p className="text-sm text-[#a1a1aa] leading-relaxed mb-6 flex-1">
            Before ever leaving your machine, the vault is heavily encrypted. The central server only routes opaque, impenetrable binary blobs across your devices.
          </p>
          <div className="h-16 bg-black border border-[#18181b] flex items-center justify-between px-4 overflow-hidden mt-auto shrink-0 w-full">
            <span className="text-[#a1a1aa] font-mono text-[10px] tracking-widest z-10 shrink-0 truncate">wss://forged.dev</span>
            
            {/* Single Segmented Uplink Array */}
            <div className="flex-1 flex gap-1 lg:gap-1.5 mx-4 md:mx-6 justify-center">
               {Array.from({ length: 24 }).map((_, i) => (
                  <motion.div 
                    key={`sync-${i}`}
                    animate={{ opacity: [0.15, 1, 0.15] }}
                    transition={{ repeat: Infinity, duration: 1.5, delay: i * 0.05, ease: "linear" }}
                    className="flex-1 h-[2px] bg-[#ea580c] rounded-full shadow-[0_0_5px_rgba(234,88,12,0.5)]"
                  />
               ))}
            </div>
            
            <span className="text-[#ea580c] font-mono text-[10px] tracking-widest z-10 shrink-0 uppercase font-bold">12 KB/S</span>
          </div>
        </motion.div> 
      {/* Node 03: PATTERN ROUTER (Big) */}
        <motion.div 
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ delay: 0.2 }}
          className="md:col-span-7 relative border border-[#27272a] bg-[#050505] p-6 shadow-2xl flex flex-col group hover:border-[#10b981]/30 transition-colors h-[280px] w-full"
        >
          <div className="border-b border-[#27272a] pb-4 mb-6 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="text-[#a1a1aa] font-mono text-[10px] tracking-[0.2em]">03 //</span>
              <span className="text-white font-mono text-[11px] tracking-widest uppercase font-bold">Pattern Engine</span>
            </div>
            <div className="w-1.5 h-1.5 bg-[#10b981] shadow-[0_0_8px_#10b981] animate-pulse rounded-full" />
          </div>
          <p className="text-sm text-[#a1a1aa] leading-relaxed mb-6 flex-1">
            Intercepts the raw SSH connection challenge before injection. Evaluates destination masks via PCRE regex and routes to the exact corresponding ethnographic identity.
          </p>

          <div className="h-16 bg-black border border-[#18181b] flex items-center justify-between px-4 mt-auto shrink-0 w-full overflow-hidden">
             {/* Left: Target */}
             <span className="text-[#a1a1aa] font-mono text-[10px] tracking-widest shrink-0 truncate max-w-[80px] sm:max-w-none">prod.aws</span>
               
             {/* Center: Animated Horizontal Pipeline */}
             <div className="flex-1 flex items-center mx-4 min-w-0">
               <div className="flex-1 h-[2px] bg-[#27272a] relative overflow-hidden rounded-l-full isolate">
                 <motion.div 
                   initial={{ left: "-100%" }}
                   animate={{ left: "200%" }}
                   transition={{ repeat: Infinity, duration: 1.5, ease: "linear" }}
                   className="absolute inset-y-0 w-12 bg-gradient-to-r from-transparent via-[#10b981] to-transparent" 
                 />
               </div>
                 
               <div className="shrink-0 bg-[#10b981]/10 px-2 py-0.5 mx-2 flex items-center gap-1.5 shadow-[0_0_10px_rgba(16,185,129,0.1)]">
                 <span className="w-1.5 h-1.5 rounded-full bg-[#10b981] animate-pulse" />
                 <span className="text-[#10b981] font-mono text-[9px] tracking-widest font-bold hidden sm:block">MATCH</span>
               </div>

               <div className="flex-1 h-[2px] bg-[#27272a] relative overflow-hidden rounded-r-full isolate">
                 <motion.div 
                   initial={{ left: "-100%" }}
                   animate={{ left: "200%" }}
                   transition={{ repeat: Infinity, duration: 1.5, ease: "linear", delay: 0.75 }}
                   className="absolute inset-y-0 w-12 bg-gradient-to-r from-transparent via-[#ea580c] to-transparent" 
                 />
               </div>
             </div>

             {/* Right: Key */}
             <span className="text-[#ea580c] font-mono text-[10px] tracking-widest shrink-0 truncate max-w-[80px] sm:max-w-none font-bold text-right">aws-yubikey</span>
          </div>
        </motion.div>

        {/* Node 04: VAULT (Small) */}
        <motion.div 
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ delay: 0.3 }}
          className="md:col-span-5 relative border border-[#27272a] bg-[#050505] p-6 shadow-2xl flex flex-col group hover:border-[#ea580c]/30 transition-colors h-[280px] w-full"
        >
          <div className="border-b border-[#27272a] pb-4 mb-6 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="text-[#a1a1aa] font-mono text-[10px] tracking-[0.2em]">04 //</span>
              <span className="text-white font-mono text-[11px] tracking-widest uppercase font-bold">Memory Vault</span>
            </div>
            <div className="w-1.5 h-1.5 bg-[#ea580c] shadow-[0_0_8px_#ea580c] animate-pulse rounded-full" />
          </div>
          
          <p className="text-sm text-[#a1a1aa] leading-relaxed mb-6 flex-1">
            Keys sit encrypted at-rest using military-grade AEAD standard ciphers, explicitly decrypted only ephemerally in RAM upon strict pattern match.
          </p>

          <div className="h-16 bg-black border border-[#18181b] flex items-center justify-between px-4 mt-auto shrink-0 w-full overflow-hidden">
             <span className="text-[#ea580c] font-mono text-[10px] tracking-widest flex items-center gap-2 shrink-0 truncate">
                <span className="w-1.5 h-1.5 bg-[#ea580c] shadow-[0_0_8px_#ea580c] animate-pulse rounded-full shrink-0" />
                <span className="hidden sm:inline font-bold">0xVAULT</span>
             </span>
             
             {/* Horizontal Right-to-Left hex scroller */}
             <div className="flex-1 overflow-hidden ml-4 pl-4 text-right h-full flex flex-col justify-center isolate" style={{ WebkitMaskImage: 'linear-gradient(to right, transparent 0%, black 20%)', maskImage: 'linear-gradient(to right, transparent 0%, black 20%)' }}>
               <motion.div 
                  animate={{ x: ["0%", "-50%"] }}
                  transition={{ repeat: Infinity, duration: 25, ease: "linear" }}
                  className="whitespace-nowrap font-mono text-[10px] tracking-widest text-[#3f3f46] inline-flex gap-4"
               >
                 <span>e3b0c44298fc1c149afbf4c8996fd41d8cd98f00b204e9800998ecf80a4d55a8</span>
                 <span>e3b0c44298fc1c149afbf4c8996fd41d8cd98f00b204e9800998ecf80a4d55a8</span>
                 <span>e3b0c44298fc1c149afbf4c8996fd41d8cd98f00b204e9800998ecf80a4d55a8</span>
                 <span>e3b0c44298fc1c149afbf4c8996fd41d8cd98f00b204e9800998ecf80a4d55a8</span>
               </motion.div>
             </div>
          </div>
        </motion.div>
      </div>
    </div>
  );
}
