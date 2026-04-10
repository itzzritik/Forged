"use client";

import { motion, useInView } from "framer-motion";
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
