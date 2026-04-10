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
