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
  pace: "fast" | "normal" | "slow" | "aggressive";
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

export function AnimatedBigTerminal({ cards }: { cards: TerminalCardDef[] }) {
  const [lines, setLines] = useState<{ id: string; text: string; isCommand: boolean; timestamp: string }[]>([]);
  const [typingCommand, setTypingCommand] = useState("");
  const [showCursor, setShowCursor] = useState(true);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const int = setInterval(() => setShowCursor(c => !c), 500);
    return () => clearInterval(int);
  }, []);

  useEffect(() => {
    let active = true;
    let cardIndex = 0;

    const generateTimestamp = () => {
      const now = new Date();
      return `[${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}.${Math.floor(Math.random() * 99).toString().padStart(2, '0')}]`;
    };

    const wait = (ms: number) => new Promise(r => setTimeout(r, ms));

    async function run() {
      await wait(1000);
      
      while (active) {
        const card = cards[cardIndex];
        const rawCmd = card.lines[0];
        const cmd = rawCmd.startsWith("> ") ? rawCmd.slice(2) : rawCmd;
        const output = card.lines.slice(1);

        let charDelay = 30;
        let lineDelay = 150;
        if (card.pace === "fast") {
          charDelay = 15;
          lineDelay = 50;
        } else if (card.pace === "slow") {
          charDelay = 50;
          lineDelay = 300;
        } else if (card.pace === "aggressive") {
          charDelay = 10;
          lineDelay = 100;
        }

        let currentCmd = "";
        for (let i = 0; i < cmd.length; i++) {
          if (!active) return;
          currentCmd += cmd[i];
          setTypingCommand(currentCmd);
          if (charDelay > 0) {
            await wait(charDelay + Math.random() * (charDelay / 2));
          }
        }

        if (!active) return;
        setLines(prev => {
           const next = [...prev, { id: Math.random().toString(), text: cmd, isCommand: true, timestamp: generateTimestamp() }];
           return next.slice(-40);
        });
        setTypingCommand("");
        
        await wait(200);

        for (const outLine of output) {
          if (!active) return;
          setLines(prev => {
             const next = [...prev, { id: Math.random().toString(), text: outLine, isCommand: false, timestamp: generateTimestamp() }];
             return next.slice(-40);
          });
          if (lineDelay > 0) {
            await wait(lineDelay + Math.random() * (lineDelay / 2));
          }
        }

        await wait(3000);
        
        cardIndex = (cardIndex + 1) % cards.length;
      }
    }

    run();

    return () => { active = false; };
  }, [cards]);

  // Smooth synchronized scrolling via effect
  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTo({
        top: containerRef.current.scrollHeight,
        behavior: 'smooth'
      });
    }
  }, [lines, typingCommand]);

  const getLineColor = (lineText: string) => {
    if (lineText.includes("[WARN]") || lineText.includes("⚠️")) return "#f59e0b";
    if (lineText.includes("[ERR]") || lineText.match(/\[error\]/i) || lineText.includes("failed on channel")) return "#ef4444";
    if (lineText.match(/\[\+\]|\[OK\]| ok | success/i)) return "#10b981"; // success
    if (lineText.match(/\[\~\]|\[INFO\]|^\[[A-Z]+\]/)) return "#0ea5e9";
    if (lineText.startsWith("forged:")) return "#a1a1aa";
    return "#a1a1aa";
  };

  return (
    <div className="w-full h-full bg-transparent relative flex flex-col overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_top,_rgba(234,88,12,0.05)_0%,_transparent_60%)] pointer-events-none" />
      
      <div ref={containerRef} className="p-4 md:p-6 flex-1 overflow-hidden [&::-webkit-scrollbar]:hidden relative z-10 font-mono text-[11px] md:text-xs pb-24">
        <AnimatePresence initial={false}>
          {lines.map((line) => (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2, ease: "easeOut" }}
              key={line.id} 
              className="leading-[1.7] whitespace-pre-wrap break-all mb-1.5 flex items-start group"
            >
              {/* Timestamp Gutter */}
              <span className="text-[#3f3f46] mr-4 select-none shrink-0">{line.timestamp}</span>

              {line.isCommand ? (
                <div className="flex items-start">
                  <span className="text-[#ea580c] mr-3 select-none flex-shrink-0">root@forged:~$</span>
                  <span className="text-white font-medium">{line.text}</span>
                </div>
              ) : (
                <div className="flex items-start">
                   <span className="text-transparent mr-3 select-none hidden md:inline flex-shrink-0">root@forged:~$</span>
                   <span style={{ color: getLineColor(line.text) }}>
                     {line.text.startsWith('forged:') ? (
                       <>
                         <span className="text-[#ea580c] font-bold">forged:</span>
                         <span className="text-white ml-2">{line.text.replace('forged:', '')}</span>
                       </>
                     ) : (
                       line.text
                     )}
                   </span>
                </div>
              )}
            </motion.div>
          ))}
        </AnimatePresence>
        
        <div className="leading-[1.7] whitespace-pre-wrap break-all mt-1.5 flex items-start">
          <span className="text-[#3f3f46] mr-4 select-none shrink-0 opacity-0">[00:00:00.00]</span>
          <span className="text-[#ea580c] mr-3 select-none flex-shrink-0">root@forged:~$</span>
          <span className="text-white font-medium">{typingCommand}</span>
          <span className={`inline-block w-2.5 h-3.5 ml-1 bg-[#ea580c] align-middle translate-y-[2px] shadow-[0_0_8px_rgba(234,88,12,0.6)] ${showCursor ? 'opacity-100' : 'opacity-0'}`} />
        </div>
        <div className="h-32" />
      </div>

      <div className="absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-[#050505] via-[#050505]/90 to-transparent pointer-events-none w-full z-10" />
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
