"use client";

import { AnimatePresence, motion, useInView } from "framer-motion";
import Link from "next/link";
import { type AnchorHTMLAttributes, type ComponentPropsWithoutRef, type ReactNode, useCallback, useEffect, useRef, useState } from "react";

const EASE = [0.16, 1, 0.3, 1] as const;

export function ScrollReveal({ children, delay = 0, className = "" }: { children: ReactNode; delay?: number; className?: string }) {
	const ref = useRef(null);
	const inView = useInView(ref, { once: true, margin: "-80px" });

	return (
		<motion.div
			animate={inView ? { opacity: 1, y: 0 } : { opacity: 0, y: 30 }}
			className={className}
			initial={{ opacity: 0, y: 30 }}
			ref={ref}
			transition={{ duration: 0.7, delay, ease: EASE }}
		>
			{children}
		</motion.div>
	);
}

export function StaggerGrid({ children, stagger = 0.1, className = "" }: { children: ReactNode; stagger?: number; className?: string }) {
	const ref = useRef(null);
	const inView = useInView(ref, { once: true, margin: "-60px" });

	return (
		<motion.div
			animate={inView ? "visible" : "hidden"}
			className={className}
			initial="hidden"
			ref={ref}
			variants={{
				visible: { transition: { staggerChildren: stagger } },
				hidden: {},
			}}
		>
			{children}
		</motion.div>
	);
}

export function StaggerItem({ children, className = "" }: { children: ReactNode; className?: string }) {
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

export function SpotlightCard({ children, className = "" }: { children: ReactNode; className?: string }) {
	const ref = useRef<HTMLDivElement>(null);
	const [pos, setPos] = useState({ x: 0, y: 0 });
	const [hovering, setHovering] = useState(false);

	return (
		<div
			className={`relative overflow-hidden ${className}`}
			onMouseEnter={() => setHovering(true)}
			onMouseLeave={() => setHovering(false)}
			onMouseMove={(e) => {
				if (!ref.current) return;
				const rect = ref.current.getBoundingClientRect();
				setPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
			}}
			ref={ref}
			role="presentation"
		>
			{hovering && (
				<div
					className="pointer-events-none absolute inset-0 z-0 transition-opacity"
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

export interface TerminalCardDef {
	brightness: number;
	lines: string[];
	pace: "fast" | "normal" | "slow";
	status: "ok" | "warn" | "error";
	title: string;
}

const PACE_CONFIG = {
	fast: { lineInterval: 160, holdTime: 2000 },
	normal: { lineInterval: 460, holdTime: 4000 },
	slow: { lineInterval: 640, holdTime: 5600 },
} as const;

const DOT_COLORS = {
	ok: { idle: "bg-emerald-500/80", active: "bg-emerald-400 animate-pulse" },
	warn: { idle: "bg-amber-500/80", active: "bg-amber-400 animate-pulse" },
	error: { idle: "bg-red-500/80", active: "bg-red-400 animate-pulse" },
} as const;

function lineColor(line: string) {
	if (line.startsWith(">")) return "#999";
	if (line.includes("[WARN]")) return "rgb(251 191 36 / 0.8)";
	if (line.includes("[ERR]")) return "rgb(248 113 113 / 0.8)";
	return "#666";
}

export const TERMINAL_CARDS: TerminalCardDef[] = [
	{
		title: "SETUP // BOOTSTRAP",
		status: "ok",
		brightness: 1.0,
		pace: "normal",
		lines: [
			"> forged setup",
			"[INIT] Creating vault at ~/.forged/",
			"[INIT] Master password: ********",
			"[SCAN] Found 5 keys in ~/.ssh/",
			"[IMPORT] id_ed25519 ... ok",
			"[IMPORT] id_rsa ... ok",
			"[IMPORT] deploy_key ... ok",
			"[DAEMON] Binding socket 0600",
		],
	},
	{
		title: "AGENT // STATUS",
		status: "ok",
		brightness: 0.75,
		pace: "slow",
		lines: ["> forged status", "Daemon: running (PID 4821)", "Keys:   4 loaded", "Socket: ~/.forged/agent.sock"],
	},
	{
		title: "KEYGEN // ED25519",
		status: "ok",
		brightness: 1.1,
		pace: "fast",
		lines: [
			"> forged generate deploy-prod",
			"[GEN] Algorithm: Ed25519",
			"[GEN] Comment: deploy@prod",
			"[VAULT] Encrypting with XChaCha20",
			"[VAULT] Nonce: random 24-byte",
			"[OK] Key deploy-prod created",
			"[OK] Vault synced to disk",
		],
	},
	{
		title: "SYNC // CLOUD",
		status: "ok",
		brightness: 1.1,
		pace: "fast",
		lines: [
			"> forged sync",
			"[AUTH] Token valid (exp 2026-04-11)",
			"[HKDF] Deriving sync key...",
			"[UPLOAD] Encrypting vault blob",
			"[UPLOAD] 4.2 KB -> blob storage",
			"[OK] Sync complete (312ms)",
			"[OK] 4 keys propagated",
		],
	},
	{
		title: "ROUTING // LEARN",
		status: "ok",
		brightness: 0.75,
		pace: "slow",
		lines: [
			"> forged doctor",
			"[CHECK] SSH include ... ok",
			"[CHECK] Agent socket ... ok",
			"[ROUTE] github.com -> github",
			"[ROUTE] prod.company.com -> deploy",
			"[OK] Local routing is healthy",
		],
	},
	{
		title: "SSH // CONNECT",
		status: "ok",
		brightness: 1.1,
		pace: "fast",
		lines: [
			"> ssh git@github.com",
			"[AGENT] Request from ssh (pid 9102)",
			"[MATCH] github.com -> github key",
			"[AUTH] Ed25519 challenge-response",
			"[OK] Authenticated as git",
			"Hi user! You've successfully",
			"authenticated with key: github",
		],
	},
	{
		title: "MIGRATE // IMPORT",
		status: "warn",
		brightness: 1.0,
		pace: "normal",
		lines: [
			"> forged migrate --from ssh",
			"[SCAN] Reading ~/.ssh/ ...",
			"[FOUND] id_ed25519 (Ed25519)",
			"[FOUND] id_rsa (RSA 2048-bit)",
			"[WARN] id_rsa uses weak RSA-2048",
			"[IMPORT] 2 keys ingested",
			"[VAULT] Re-encrypted with Argon2id",
		],
	},
	{
		title: "GIT // SIGNING",
		status: "ok",
		brightness: 0.75,
		pace: "normal",
		lines: [
			'> git commit -m "fix auth flow"',
			"[SIGN] Request from git (pid 3401)",
			"[MATCH] git -> signing key",
			"[SIGN] SSH signature created",
			"[OK] Commit signed: a3f2b1c",
			"[OK] Verified: ssh-ed25519",
			"1 file changed, 12 insertions(+)",
		],
	},
	{
		title: "VAULT // ENCRYPT",
		status: "ok",
		brightness: 0.75,
		pace: "slow",
		lines: [
			"> forged lock",
			"[LOCK] Zeroing memory pages...",
			"[LOCK] mlock() released 4 pages",
			"[LOCK] Agent socket suspended",
			"[OK] Vault locked, 0 keys in mem",
			"> forged unlock",
			"Master password: ********",
		],
	},
	{
		title: "DAEMON // LOGS",
		status: "ok",
		brightness: 1.0,
		pace: "fast",
		lines: [
			"> forged logs",
			"14:23:07 [INFO] github.com -> ok",
			"14:23:08 [INFO] key: github",
			"14:23:08 [INFO] auth: success",
			"14:23:09 [INFO] session: active",
			"14:25:11 [INFO] prod.co -> ok",
			"14:25:12 [INFO] key: deploy",
		],
	},
	{
		title: "AGENT // READY",
		status: "ok",
		brightness: 1.1,
		pace: "slow",
		lines: [
			"> forged status",
			"Daemon: running (PID 4821)",
			"Keys:   4 loaded",
			"Socket: /Users/user/.forged/agent.sock",
			"[OK] SSH agent available",
		],
	},
	{
		title: "DOCTOR // CHECK",
		status: "ok",
		brightness: 1.0,
		pace: "normal",
		lines: [
			"> forged doctor",
			"[CHECK] Vault integrity ... ok",
			"[CHECK] Daemon running ... ok",
			"[CHECK] Socket perms 0600 ... ok",
			"[CHECK] SSH config ... ok",
			"[CHECK] Argon2id params ... ok",
			"[OK] All 5 checks passed",
		],
	},
	{
		title: "LIST // KEYS",
		status: "ok",
		brightness: 0.75,
		pace: "slow",
		lines: [
			"> forged list",
			"  NAME       TYPE          FINGERPRINT",
			"  github     ssh-ed25519   SHA256:xK3...",
			"  deploy     ssh-ed25519   SHA256:mN7...",
			"  personal   ssh-rsa       SHA256:pQ2...",
			"  signing    ssh-ed25519   SHA256:vB9...",
		],
	},
	{
		title: "VIEW // KEY",
		status: "ok",
		brightness: 1.1,
		pace: "normal",
		lines: [
			"> forged view github",
			"Name         Github (Personal)",
			"Type         ssh-ed25519",
			"Fingerprint  SHA256:xK3mQR9f2QWv43kL...",
			"Public key",
			"  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5...",
		],
	},
	{
		title: "BENCHMARK // ARGON",
		status: "warn",
		brightness: 1.0,
		pace: "normal",
		lines: [
			"> forged benchmark",
			"[BENCH] Argon2id 64MB 3 iter",
			"[BENCH] Derive: 287ms avg",
			"[BENCH] Encrypt: 0.4ms avg",
			"[BENCH] Decrypt: 0.3ms avg",
			"[BENCH] Total: 288ms per unlock",
			"[OK] Within security threshold",
		],
	},
	{
		title: "SYNC // STATUS",
		status: "ok",
		brightness: 1.1,
		pace: "fast",
		lines: [
			"> forged sync status",
			"account:  user@forged.dev",
			"last_sync: 2m ago",
			"blob_size: 4.2 KB",
			"devices:   3 linked",
			"conflicts: 0",
			"[OK] Vault in sync",
		],
	},
	{
		title: "SECURITY // DOCTOR",
		status: "ok",
		brightness: 1.0,
		pace: "normal",
		lines: [
			"> forged doctor --fix",
			"[PASS] Vault exists",
			"[PASS] Config exists",
			"[PASS] Daemon running (PID 4821)",
			"[PASS] Agent socket 0600",
			"[PASS] IPC socket",
			"[FIXED] SSH agent IdentityAgent",
		],
	},
	{
		title: "RENAME // KEY",
		status: "ok",
		brightness: 0.75,
		pace: "slow",
		lines: [
			"> forged rename personal backup",
			"Renamed personal -> backup",
			"",
			"> forged list --json",
			'{"keys":[{"name":"backup",',
			'"type":"ssh-rsa",',
			'"fingerprint":"SHA256:pQ2"}]}',
		],
	},
	{
		title: "EXPORT // VAULT",
		status: "ok",
		brightness: 1.1,
		pace: "fast",
		lines: [
			"> forged export",
			"[AUTH] Touch ID approved",
			"[SAVE] /Users/user/Downloads/forged-export.json",
			"[OK] Export written",
		],
	},
	{ title: "DAEMON // START", status: "ok", brightness: 1.0, pace: "fast", lines: ["> forged start", "Daemon started (PID 4821)"] },
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

		// biome-ignore lint/complexity/noExcessiveCognitiveComplexity: animation tick function requires complex state management
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
			{ threshold: 0.1 }
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
		<div className="grid h-full grid-cols-2 grid-rows-5 gap-px bg-[#1a1a1a] md:grid-cols-3 lg:grid-cols-4" ref={gridRef}>
			{cards.map((card: TerminalCardDef) => (
				<div
					className="flex flex-col overflow-hidden border border-[#1a1a1a] bg-black"
					data-terminal
					key={card.title}
					style={{ filter: `brightness(${card.brightness})` }}
				>
					<div className="flex h-7 shrink-0 items-center justify-between border-[#1a1a1a] border-b px-3">
						<span className="font-mono text-[#555] text-[10px] tracking-[0.5px]">{card.title}</span>
						<span className={`h-1.5 w-1.5 rounded-full ${DOT_COLORS[card.status].idle}`} data-dot />
					</div>
					<div className="flex-1 overflow-hidden p-3">
						{card.lines.map((line: string, i: number) => (
							<div
								className="truncate whitespace-pre font-mono text-[11px] leading-[1.7]"
								data-line
								key={i}
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
				chars
					.map((c, i) => {
						if (c === " ") return " ";
						if (i < resolved) return c;
						return rand();
					})
					.join("")
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

	return (
		<span className={className} ref={ref}>
			{display}
		</span>
	);
}

type GlitchButtonProps = {
	children: string;
	variant?: "primary" | "secondary";
	href?: string;
	external?: boolean;
} & Omit<AnchorHTMLAttributes<HTMLAnchorElement> & ComponentPropsWithoutRef<typeof Link>, "children">;

export function GlitchButton({ children, variant = "primary", href, external, className = "", ...rest }: GlitchButtonProps) {
	const { display, scramble } = useGlitchText(children);

	const isPrimary = variant === "primary";
	const base =
		"group relative inline-flex items-center justify-center gap-2 font-mono text-sm font-bold tracking-wider uppercase overflow-hidden transition-colors active:scale-[0.97]";
	const style = isPrimary ? "bg-white text-black hover:bg-zinc-200" : "bg-transparent text-white border border-[#27272a] hover:border-[#ea580c] hover:text-[#ea580c]";
	const overlayOpacity = isPrimary ? "group-hover:opacity-[0.12]" : "group-hover:opacity-[0.30]";

	const inner = (
		<>
			<span className="relative z-[2]">{display}</span>
			{/* Crosshatch overlay */}
			<div
				aria-hidden
				className={`pointer-events-none absolute inset-0 z-[1] opacity-0 ${overlayOpacity} transition-opacity duration-200`}
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
			<a className={classes} href={href} onMouseEnter={scramble} {...rest}>
				{inner}
			</a>
		);
	}

	if (href) {
		return (
			<Link className={classes} href={href} onMouseEnter={scramble} {...rest}>
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

export interface TerminalStep {
	command: string;
	output: string[];
	pauseAfter?: number;
}

const SEPARATOR_RE = /^\s+-+\s+-+/;
const HEADER_RE = /^\s+(NAME|TYPE|FINGERPRINT|SIGNING)/;
const SHELL_PROMPT_RE = /^\w+@[\w-]+:[~/]/;

function TerminalOutputLine({ text }: { text: string }) {
	if (SEPARATOR_RE.test(text)) return <span className="text-[#27272a]">{text}</span>;
	if (HEADER_RE.test(text)) return <span className="text-[#52525b]">{text}</span>;
	if (text.startsWith("Mapped ")) return <span className="text-[#10b981]">{text}</span>;
	if (text.includes("Connection") && text.includes("closed")) return <span className="text-[#52525b]">{text}</span>;
	if (SHELL_PROMPT_RE.test(text)) return <span className="text-[#ea580c]">{text}</span>;
	return <span className="text-[#a1a1aa]">{text}</span>;
}

export function AnimatedBigTerminal({ steps }: { steps: TerminalStep[] }) {
	const containerRef = useRef<HTMLDivElement>(null);
	const [lines, setLines] = useState<{ id: string; text: string; type: "cmd" | "out"; ts: string }[]>([]);
	const [typing, setTyping] = useState("");
	const [cursorOn, setCursorOn] = useState(true);
	const tsRef = useRef({ base: Date.now(), offset: 0 });

	useEffect(() => {
		const id = setInterval(() => setCursorOn((v) => !v), 530);
		return () => clearInterval(id);
	}, []);

	useEffect(() => {
		let alive = true;
		let step = 0;
		const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

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
			if ("\"'*@".includes(ch)) return 60 + Math.random() * 30;
			if (".-/~".includes(ch)) return 28 + Math.random() * 14;
			if (prev === " " && Math.random() < 0.12) return 130 + Math.random() * 90;
			return 25 + Math.random() * 28;
		};

		// biome-ignore lint/complexity/noExcessiveCognitiveComplexity: terminal animation requires sequential async state machine
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

				setLines((p) => [...p, { id: `c${step}-${Date.now()}`, text: s.command, type: "cmd", ts: ts() }]);
				setTyping("");

				await sleep(160 + Math.random() * 120);

				for (let i = 0; i < s.output.length; i++) {
					if (!alive) return;
					const line = s.output[i];
					setLines((p) => [...p, { id: `o${step}-${i}-${Date.now()}`, text: line, type: "out", ts: ts() }]);

					let d = 45;
					if (SEPARATOR_RE.test(line)) d = 12;
					else if (i === 0 && s.output.length > 3) d = 20;
					else if (line.includes("Welcome") || line.includes("Last login")) d = 110;
					await sleep(d + Math.random() * 25);
				}
				setLines((p) => [...p, { id: `g${step}-${Date.now()}`, text: "\u00A0", type: "out", ts: "" }]);

				await sleep(s.pauseAfter ?? 2500);

				step = (step + 1) % steps.length;
			}
		}

		animate();
		return () => {
			alive = false;
		};
	}, [steps]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: containerRef is a stable ref, intentionally omitted
	useEffect(() => {
		containerRef.current?.scrollTo({ top: containerRef.current.scrollHeight, behavior: "smooth" });
	}, [lines, typing]);

	return (
		<div className="relative flex h-full w-full flex-col overflow-hidden">
			<div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_top,_rgba(234,88,12,0.04)_0%,_transparent_55%)]" />

			<div
				className="relative z-10 flex-1 overflow-hidden p-5 font-mono text-[11px] leading-[1.9] md:px-6 md:py-5 md:text-[13px] [&::-webkit-scrollbar]:hidden"
				ref={containerRef}
				style={{ tabSize: 8 }}
			>
				<AnimatePresence initial={false}>
					{lines.map((line) => (
						<motion.div
							animate={{ opacity: 1, y: 0 }}
							className="flex whitespace-pre"
							initial={{ opacity: 0, y: 5 }}
							key={line.id}
							transition={{ duration: 0.12, ease: "easeOut" }}
						>
							{line.ts && <span className="mr-4 hidden shrink-0 select-none text-[#1e1e21] lg:inline">{line.ts}</span>}
							<span className="flex-1">
								{line.type === "cmd" ? (
									<>
										<span className="select-none text-[#ea580c]">$ </span>
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
				<div className="flex whitespace-pre">
					<span className="mr-4 hidden shrink-0 select-none text-[#1e1e21] lg:inline">
						&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
					</span>
					<span className="flex-1">
						<span className="select-none text-[#ea580c]">$ </span>
						<span className="text-white">{typing}</span>
						<span
							className="ml-px inline-block h-[14px] w-[7px] translate-y-[2px] transition-opacity duration-75"
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
		<div className="relative mt-16 w-full md:mt-24">
			{/* Abstract Background SVG Pattern */}
			<div
				className="pointer-events-none absolute inset-0 opacity-[0.03]"
				style={{ backgroundImage: "radial-gradient(#ea580c 1px, transparent 1px)", backgroundSize: "32px 32px" }}
			/>

			{/* Grid Layout */}
			<div className="relative z-10 mb-8 grid w-full grid-cols-1 gap-8 md:mb-12 md:grid-cols-12">
				{/* Node 01: UNIX SOCKET (Small) */}
				<motion.div
					className="group relative flex h-[280px] w-full flex-col border border-[#27272a] bg-[#050505] p-6 shadow-2xl transition-colors hover:border-[#10b981]/30 md:col-span-5"
					initial={{ opacity: 0, y: 20 }}
					viewport={{ once: true }}
					whileInView={{ opacity: 1, y: 0 }}
				>
					<div className="mb-6 flex items-center justify-between border-[#27272a] border-b pb-4">
						<div className="flex items-center gap-3">
							<span className="font-mono text-[#a1a1aa] text-[10px] tracking-[0.2em]">01 {/* // */}</span>
							<span className="font-bold font-mono text-[11px] text-white uppercase tracking-widest">Ingress Socket</span>
						</div>
						<div className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#10b981] shadow-[0_0_8px_#10b981]" />
					</div>
					<p className="mb-6 flex-1 text-[#a1a1aa] text-sm leading-relaxed">
						Replaces the standard ssh-agent. Exposes a native UNIX socket locally, dropping perfectly into your existing ecosystem without requiring custom
						clients.
					</p>
					<div className="mt-auto flex h-16 w-full shrink-0 items-center justify-between overflow-hidden border border-[#18181b] bg-black px-4">
						<span className="flex shrink-0 items-center gap-2 truncate font-mono text-[#10b981] text-[10px] tracking-widest">
							<span className="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-[#10b981] shadow-[0_0_8px_#10b981]" />
							<span className="hidden font-bold sm:inline">LISTENING</span>
						</span>
						<span className="ml-4 shrink-0 truncate font-mono text-[#a1a1aa] text-[10px] tracking-widest opacity-60">/tmp/forged.sock</span>
					</div>
				</motion.div>

				{/* Node 02: SYNC NODE (Big) */}
				<motion.div
					className="group relative flex h-[280px] w-full flex-col border border-[#27272a] bg-[#050505] p-6 shadow-2xl transition-colors hover:border-[#ea580c]/30 md:col-span-7"
					initial={{ opacity: 0, y: 20 }}
					transition={{ delay: 0.1 }}
					viewport={{ once: true }}
					whileInView={{ opacity: 1, y: 0 }}
				>
					<div className="mb-6 flex items-center justify-between border-[#27272a] border-b pb-4">
						<div className="flex items-center gap-3">
							<span className="font-mono text-[#a1a1aa] text-[10px] tracking-[0.2em]">02 {/* // */}</span>
							<span className="font-bold font-mono text-[11px] text-white uppercase tracking-widest">E2E Sync Node</span>
						</div>
						<div className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#ea580c] shadow-[0_0_8px_#ea580c]" />
					</div>
					<p className="mb-6 flex-1 text-[#a1a1aa] text-sm leading-relaxed">
						Before ever leaving your machine, the vault is heavily encrypted. The central server only routes opaque, impenetrable binary blobs across your
						devices.
					</p>
					<div className="mt-auto flex h-16 w-full shrink-0 items-center justify-between overflow-hidden border border-[#18181b] bg-black px-4">
						<span className="z-10 shrink-0 truncate font-mono text-[#a1a1aa] text-[10px] tracking-widest">wss://forged.dev</span>

						{/* Single Segmented Uplink Array */}
						<div className="mx-4 flex flex-1 justify-center gap-1 md:mx-6 lg:gap-1.5">
							{Array.from({ length: 24 }).map((_, i) => (
								<motion.div
									animate={{ opacity: [0.15, 1, 0.15] }}
									className="h-[2px] flex-1 rounded-full bg-[#ea580c] shadow-[0_0_5px_rgba(234,88,12,0.5)]"
									key={`sync-${i}`}
									transition={{ repeat: Number.POSITIVE_INFINITY, duration: 1.5, delay: i * 0.05, ease: "linear" }}
								/>
							))}
						</div>

						<span className="z-10 shrink-0 font-bold font-mono text-[#ea580c] text-[10px] uppercase tracking-widest">12 KB/S</span>
					</div>
				</motion.div>
				{/* Node 03: PATTERN ROUTER (Big) */}
				<motion.div
					className="group relative flex h-[280px] w-full flex-col border border-[#27272a] bg-[#050505] p-6 shadow-2xl transition-colors hover:border-[#10b981]/30 md:col-span-7"
					initial={{ opacity: 0, y: 20 }}
					transition={{ delay: 0.2 }}
					viewport={{ once: true }}
					whileInView={{ opacity: 1, y: 0 }}
				>
					<div className="mb-6 flex items-center justify-between border-[#27272a] border-b pb-4">
						<div className="flex items-center gap-3">
							<span className="font-mono text-[#a1a1aa] text-[10px] tracking-[0.2em]">03 {/* // */}</span>
							<span className="font-bold font-mono text-[11px] text-white uppercase tracking-widest">Pattern Engine</span>
						</div>
						<div className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#10b981] shadow-[0_0_8px_#10b981]" />
					</div>
					<p className="mb-6 flex-1 text-[#a1a1aa] text-sm leading-relaxed">
						Intercepts the raw SSH connection challenge before injection. Evaluates destination masks via PCRE regex and routes to the exact corresponding
						ethnographic identity.
					</p>

					<div className="mt-auto flex h-16 w-full shrink-0 items-center justify-between overflow-hidden border border-[#18181b] bg-black px-4">
						{/* Left: Target */}
						<span className="max-w-[80px] shrink-0 truncate font-mono text-[#a1a1aa] text-[10px] tracking-widest sm:max-w-none">prod.aws</span>

						{/* Center: Animated Horizontal Pipeline */}
						<div className="mx-4 flex min-w-0 flex-1 items-center">
							<div className="relative isolate h-[2px] flex-1 overflow-hidden rounded-l-full bg-[#27272a]">
								<motion.div
									animate={{ left: "200%" }}
									className="absolute inset-y-0 w-12 bg-gradient-to-r from-transparent via-[#10b981] to-transparent"
									initial={{ left: "-100%" }}
									transition={{ repeat: Number.POSITIVE_INFINITY, duration: 1.5, ease: "linear" }}
								/>
							</div>

							<div className="mx-2 flex shrink-0 items-center gap-1.5 bg-[#10b981]/10 px-2 py-0.5 shadow-[0_0_10px_rgba(16,185,129,0.1)]">
								<span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#10b981]" />
								<span className="hidden font-bold font-mono text-[#10b981] text-[9px] tracking-widest sm:block">MATCH</span>
							</div>

							<div className="relative isolate h-[2px] flex-1 overflow-hidden rounded-r-full bg-[#27272a]">
								<motion.div
									animate={{ left: "200%" }}
									className="absolute inset-y-0 w-12 bg-gradient-to-r from-transparent via-[#ea580c] to-transparent"
									initial={{ left: "-100%" }}
									transition={{ repeat: Number.POSITIVE_INFINITY, duration: 1.5, ease: "linear", delay: 0.75 }}
								/>
							</div>
						</div>

						{/* Right: Key */}
						<span className="max-w-[80px] shrink-0 truncate text-right font-bold font-mono text-[#ea580c] text-[10px] tracking-widest sm:max-w-none">
							aws-yubikey
						</span>
					</div>
				</motion.div>

				{/* Node 04: VAULT (Small) */}
				<motion.div
					className="group relative flex h-[280px] w-full flex-col border border-[#27272a] bg-[#050505] p-6 shadow-2xl transition-colors hover:border-[#ea580c]/30 md:col-span-5"
					initial={{ opacity: 0, y: 20 }}
					transition={{ delay: 0.3 }}
					viewport={{ once: true }}
					whileInView={{ opacity: 1, y: 0 }}
				>
					<div className="mb-6 flex items-center justify-between border-[#27272a] border-b pb-4">
						<div className="flex items-center gap-3">
							<span className="font-mono text-[#a1a1aa] text-[10px] tracking-[0.2em]">04 {/* // */}</span>
							<span className="font-bold font-mono text-[11px] text-white uppercase tracking-widest">Memory Vault</span>
						</div>
						<div className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#ea580c] shadow-[0_0_8px_#ea580c]" />
					</div>

					<p className="mb-6 flex-1 text-[#a1a1aa] text-sm leading-relaxed">
						Keys sit encrypted at-rest using military-grade AEAD standard ciphers, explicitly decrypted only ephemerally in RAM upon strict pattern match.
					</p>

					<div className="mt-auto flex h-16 w-full shrink-0 items-center justify-between overflow-hidden border border-[#18181b] bg-black px-4">
						<span className="flex shrink-0 items-center gap-2 truncate font-mono text-[#ea580c] text-[10px] tracking-widest">
							<span className="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-[#ea580c] shadow-[0_0_8px_#ea580c]" />
							<span className="hidden font-bold sm:inline">0xVAULT</span>
						</span>

						{/* Horizontal Right-to-Left hex scroller */}
						<div
							className="isolate ml-4 flex h-full flex-1 flex-col justify-center overflow-hidden pl-4 text-right"
							style={{
								WebkitMaskImage: "linear-gradient(to right, transparent 0%, black 20%)",
								maskImage: "linear-gradient(to right, transparent 0%, black 20%)",
							}}
						>
							<motion.div
								animate={{ x: ["0%", "-50%"] }}
								className="inline-flex gap-4 whitespace-nowrap font-mono text-[#3f3f46] text-[10px] tracking-widest"
								transition={{ repeat: Number.POSITIVE_INFINITY, duration: 25, ease: "linear" }}
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
