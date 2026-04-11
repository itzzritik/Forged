import Link from "next/link";
import { AuthCTAButton, AuthNavButton } from "@/components/auth-nav";
import type { TerminalStep } from "@/components/client";
import { AnimatedBigTerminal, AnimatedTerminalGrid, GlitchButton, ScrollReveal, TERMINAL_CARDS, TopologyVisualizer } from "@/components/client";

function Nav() {
	return (
		<nav className="fixed top-0 right-0 left-0 z-50 border-[#27272a] border-b bg-black/80 backdrop-blur-xl">
			<div className="flex h-14 w-full items-center justify-between px-6 lg:px-16">
				<Link className="group flex items-center gap-3" href="/">
					<div className="flex h-7 w-7 items-center justify-center border border-[#27272a] bg-black transition-colors group-hover:border-[#ea580c]">
						<svg
							className="text-white transition-colors group-hover:text-[#ea580c]"
							fill="none"
							height="14"
							stroke="currentColor"
							strokeLinecap="round"
							strokeLinejoin="round"
							strokeWidth="1.5"
							viewBox="0 0 24 24"
							width="14"
						>
							<path d="M15 3h6v6" />
							<path d="M10 14L21 3" />
							<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
						</svg>
					</div>
					<span className="font-bold font-mono text-[13px] text-white uppercase tracking-[0.2em] transition-colors group-hover:text-[#ea580c]">Forged</span>
				</Link>
				<div className="flex items-center gap-6 md:gap-8">
					<Link className="hidden text-[#a1a1aa] text-[12px] uppercase tracking-wider transition-colors hover:text-white sm:block" href="/docs">
						Docs
					</Link>
					<Link className="hidden text-[#a1a1aa] text-[12px] uppercase tracking-wider transition-colors hover:text-white sm:block" href="/security">
						Security
					</Link>
					<a
						className="hidden text-[#a1a1aa] text-[12px] uppercase tracking-wider transition-colors hover:text-white md:block"
						href="https://github.com/itzzritik/forged"
					>
						GitHub
					</a>
					<AuthNavButton />
				</div>
			</div>
		</nav>
	);
}

const ROUTING_DEMO: TerminalStep[] = [
	{
		command: "forged status",
		output: ["Daemon: running (PID 4129)", "Keys:   3 loaded", "Socket: /Users/user/.forged/agent.sock"],
		pauseAfter: 2000,
	},
	{
		command: "forged list",
		output: [
			"  NAME      TYPE         FINGERPRINT                                        SIGNING",
			"  --------  -----------  -------------------------------------------------  -------",
			"  github    ssh-ed25519  SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU  yes",
			"  deploy    ssh-ed25519  SHA256:jRkG8NpL2wQx5vB7tY1fH3sA9dK6uEiO4cX8rZ3bVnw",
			"  personal  ssh-rsa      SHA256:nVxK3mQR9f2QWv43kLwpQ2rBx87mN7pLq2iO8pK2wEzs",
		],
		pauseAfter: 2500,
	},
	{
		command: 'forged host github "github.com" "*.github.com"',
		output: ["Mapped github to [github.com *.github.com]"],
		pauseAfter: 1800,
	},
	{
		command: 'forged host deploy "*.prod.company.com" "10.0.*"',
		output: ["Mapped deploy to [*.prod.company.com 10.0.*]"],
		pauseAfter: 2200,
	},
	{
		command: "forged hosts",
		output: ["  github\tgithub.com\t(exact)", "  github\t*.github.com\t(wildcard)", "  deploy\t*.prod.company.com\t(wildcard)", "  deploy\t10.0.*\t(wildcard)"],
		pauseAfter: 2800,
	},
	{
		command: "ssh git@github.com",
		output: ["Hi user! You've successfully authenticated.", "Connection to github.com closed."],
		pauseAfter: 2200,
	},
	{
		command: "ssh deploy@api.prod.company.com",
		output: ["Welcome to Ubuntu 24.04.1 LTS (GNU/Linux 6.5.0-44-generic x86_64)", "", "Last login: Fri Apr 11 09:42:17 2026 from 10.0.1.5", "deploy@api-prod:~$"],
		pauseAfter: 3000,
	},
];

function Hero() {
	return (
		<section className="relative flex h-dvh flex-col border-[#27272a] border-b">
			<section className="relative flex-1 overflow-hidden">
				{/* Terminal Grid Background */}
				<div className="pointer-events-none absolute inset-0 select-none overflow-hidden">
					<div className="h-full">
						<div className="relative h-full">
							<AnimatedTerminalGrid cards={TERMINAL_CARDS} />
							{/* Bottom gradient fade - 40% height, fully opaque at bottom */}
							<div className="pointer-events-none absolute right-0 bottom-0 left-0 h-[40%] bg-gradient-to-b from-transparent to-black" />
							{/* Left-side gradient for text readability */}
							<div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-black via-40% via-black/80 to-transparent" />
							{/* Top edge fade */}
							<div className="pointer-events-none absolute top-0 right-0 left-0 h-20 bg-gradient-to-b from-black to-transparent" />
						</div>
					</div>
				</div>

				{/* Hero Content */}
				<div className="relative z-10 flex h-full flex-col justify-center px-6 lg:px-16">
					<div className="max-w-4xl">
						<div className="animate-slide-up">
							<div className="inline-flex items-center gap-2 bg-[#ea580c] px-3 py-1.5 font-bold font-mono text-[11px] text-black uppercase tracking-[0.15em]">
								~/.ssh is a mess
							</div>
						</div>

						<h1 className="mt-8 animate-slide-up font-bold text-[clamp(48px,8vw,96px)] text-white leading-[0.9] tracking-[-0.035em] delay-100">
							Your Keys, <br />
							One Vault, <br />
							Every Machine.
						</h1>

						<p className="mt-8 mb-12 max-w-2xl animate-slide-up text-[#a1a1aa] text-lg leading-relaxed delay-200 md:text-xl">
							Encrypt, sync, and manage all your SSH keys across every device. One command to install. One binary to run. You never touch ~/.ssh again.
						</p>

						<div className="flex animate-slide-up flex-col items-start gap-4 delay-300 sm:flex-row sm:items-center">
							<GlitchButton className="h-12 px-8" href="/docs">
								Get Started
							</GlitchButton>
							<div className="group flex h-12 items-center border border-[#27272a] bg-[#09090b] transition-colors hover:border-[#a1a1aa]/30">
								<span className="mx-3 select-none font-mono text-[#27272a] text-lg">$</span>
								<code className="pr-4 font-mono text-sm text-white tracking-wide">brew install forged</code>
								<div
									className="flex h-full w-12 cursor-pointer items-center justify-center border-[#27272a] border-l transition-colors group-hover:bg-white/5"
									title="Copy"
								>
									<svg
										className="text-[#a1a1aa] transition-colors group-hover:text-white"
										fill="none"
										height="14"
										stroke="currentColor"
										strokeWidth="2"
										viewBox="0 0 24 24"
										width="14"
									>
										<rect height="13" width="13" x="9" y="9" />
										<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
									</svg>
								</div>
							</div>
						</div>
					</div>
				</div>
			</section>
		</section>
	);
}

function GridFeatures() {
	const features = [
		{
			icon: "M12 15v2m-6 4h12a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2zm10-10V7a4 4 0 0 0-8 0v4h8z",
			title: "Encrypted Vaults",
			subtitle: "Zero-knowledge storage",
			desc: "Your keys sit unprotected in ~/.ssh. Forged wraps every private key in an Argon2id + XChaCha20-Poly1305 vault that never decrypts on disk.",
			bullets: ["Argon2id key derivation", "XChaCha20-Poly1305 encryption", "mlock() memory protection"],
			cta: "Read Security Paper",
			href: "/security",
		},
		{
			icon: "M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4",
			title: "Cross-Platform Sync",
			subtitle: "Every device, one vault",
			desc: "Moving between machines means manually copying key files. Forged syncs encrypted blobs across all your devices automatically.",
			bullets: ["Zero-knowledge cloud sync", "HKDF-SHA256 derived sync keys", "Conflict-free propagation"],
			cta: "View Sync Docs",
			href: "/docs#sync",
		},
		{
			icon: "M12 8v4m0 4h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0z",
			title: "Intelligent Binding",
			subtitle: "Pattern-matched routing",
			desc: "SSH throws every key at the server until banned. Forged binds specific keys to specific hosts using wildcard and regex patterns.",
			bullets: ["Wildcard host matching", "Regex pattern support", "Eliminates auth failures"],
			cta: "Configure Hosts",
			href: "/docs#host-matching",
		},
		{
			icon: "M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 1 1 3.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z",
			title: "Git Signatures",
			subtitle: "Verified commits",
			desc: "A built-in SSH agent allows frictionless, automatic verified signatures on every git commit across all your workflows.",
			bullets: ["Automatic commit signing", "SSH-based GPG alternative", "forged-sign helper binary"],
			cta: "Setup Signing",
			href: "/docs#git-signing",
		},
		{
			icon: "M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2H7a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2zM9 9h6v6H9V9z",
			title: "Unix Daemon",
			subtitle: "Always running, always ready",
			desc: "A single ~13MB Go binary runs a background daemon that emulates the ssh-agent protocol. No Electron, no browser extensions.",
			bullets: ["Pure Go socket agent", "launchctl/systemd binding", "0600 socket permissions"],
			cta: "View Architecture",
			href: "/docs#setup",
		},
		{
			icon: "M4 16l4.586-4.586a2 2 0 0 1 2.828 0L16 16m-2-2l1.586-1.586a2 2 0 0 1 2.828 0L20 14m-6-6h.01M6 20h12a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2z",
			title: "Key Migration",
			subtitle: "Import from anywhere",
			desc: "Migrate keys from ~/.ssh or 1Password in a single command. Inspect your running ssh-agent to plan the move.",
			bullets: ["Import from ~/.ssh", "1Password CLI integration", "Agent key discovery"],
			cta: "Migration Guide",
			href: "/docs#key-management",
		},
	];

	return (
		<section className="relative overflow-hidden border-white/10 border-t bg-black px-6 py-36 lg:px-16">
			<div
				className="pointer-events-none absolute inset-0 opacity-[0.04]"
				style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }}
			/>

			<div className="relative z-10 w-full">
				<ScrollReveal className="mb-4">
					<div className="flex items-center gap-2.5">
						<span className="h-2 w-2 bg-[#ea580c]" />
						<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">The SSH Platform</span>
					</div>
				</ScrollReveal>

				<ScrollReveal>
					<h2 className="text-pretty font-bold text-4xl text-white leading-[0.95] tracking-tighter sm:text-5xl lg:text-7xl xl:text-8xl">
						Manage keys from anywhere, anytime, autonomously.
					</h2>
				</ScrollReveal>

				<ScrollReveal className="mt-6">
					<p className="max-w-2xl text-[#a1a1aa] text-base leading-relaxed lg:text-lg">
						One binary, six capabilities. Generate, encrypt, sync, bind, sign, and migrate your SSH keys from a single daemon - while you focus on shipping.
					</p>
				</ScrollReveal>

				<div className="relative z-10 mt-16 grid grid-cols-1 border-white/10 border-t border-l md:grid-cols-2 lg:grid-cols-3">
					{features.map((f) => (
						<article
							className="group relative flex cursor-pointer flex-col border-white/10 border-r border-b bg-white/[0.03] transition-colors duration-300 hover:border-[#ea580c]/20"
							key={f.title}
						>
							{/* Hover overlay */}
							<div className="pointer-events-none absolute inset-0 bg-[#ea580c]/[0.07] opacity-0 transition-opacity duration-300 group-hover:opacity-100" />

							{/* Icon + Title */}
							<div className="relative flex items-center gap-3.5 px-6 pt-6 pb-3">
								<div className="flex h-9 w-9 items-center justify-center border border-white/10 bg-black text-white/70 transition-colors duration-300 group-hover:border-[#ea580c]/40 group-hover:text-orange-400">
									<svg fill="none" height="20" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" width="20">
										<path d={f.icon} />
									</svg>
								</div>
								<div>
									<h3 className="font-semibold text-sm text-white tracking-tight transition-colors duration-300 group-hover:text-orange-400">
										{f.title}
									</h3>
									<p className="text-[#a1a1aa] text-xs transition-colors duration-300 group-hover:text-orange-400/60">{f.subtitle}</p>
								</div>
							</div>

							{/* Description */}
							<p className="relative px-6 text-sm text-white leading-relaxed transition-colors duration-300 group-hover:text-orange-300">{f.desc}</p>

							{/* Bullets */}
							<ul className="relative flex-1 space-y-2 px-6 pt-4 pb-2">
								{f.bullets.map((b) => (
									<li className="flex items-start gap-2.5 text-white text-xs transition-colors duration-300 group-hover:text-orange-300" key={b}>
										<span className="mt-1.5 h-1 w-1 shrink-0 bg-[#ea580c]/80" />
										{b}
									</li>
								))}
							</ul>

							{/* CTA */}
							<div className="relative mt-auto px-6 pt-3 pb-6">
								<Link
									className="inline-flex items-center gap-2 bg-[length:0%_1px] bg-[linear-gradient(to_right,rgba(234,88,12,0.4),rgba(234,88,12,0.4))] bg-[position:0%_100%] bg-no-repeat font-medium text-white text-xs transition-all duration-500 ease-in-out group-hover:bg-[length:100%_1px] group-hover:text-[#ea580c]"
									href={f.href}
								>
									{f.cta}
									<svg
										className="opacity-0 transition-opacity group-hover:opacity-100"
										fill="none"
										height="14"
										stroke="currentColor"
										strokeWidth="2"
										viewBox="0 0 24 24"
										width="14"
									>
										<line x1="5" x2="19" y1="12" y2="12" />
										<polyline points="12 5 19 12 12 19" />
									</svg>
								</Link>
							</div>
						</article>
					))}
				</div>
			</div>
		</section>
	);
}

function TerminalSection() {
	return (
		<section className="relative overflow-hidden border-white/10 border-t bg-black py-24 lg:py-36">
			<div className="relative z-10 flex w-full flex-col px-6 lg:px-16">
				{/* TOP COMPONENT - Chairman LLM Header Style */}
				<ScrollReveal className="mb-12 flex w-full max-w-3xl flex-col items-start text-left">
					<div className="mb-4 flex items-center gap-2.5">
						<span className="h-2 w-2 bg-[#ea580c]" />
						<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Pattern Matching</span>
					</div>
					<h2 className="mb-6 font-bold text-5xl text-white leading-[0.95] tracking-tighter sm:text-6xl lg:text-7xl">Context Aware Routing.</h2>
					<p className="mb-10 max-w-2xl text-[#a1a1aa] text-lg leading-relaxed lg:text-xl">
						Never write another ~/.ssh/config file again. Forged uses wildcard and regex patterns to instantly route the correct cryptographic key to the
						right server, automatically.
					</p>
					<div className="flex flex-wrap items-center gap-4">
						<GlitchButton className="h-12 px-8 text-sm" href="/docs">
							Configure Patterns
						</GlitchButton>
						<GlitchButton className="h-12 px-8 text-sm" href="/docs" variant="secondary">
							View Docs
						</GlitchButton>
					</div>
				</ScrollReveal>

				{/* BOTTOM COMPONENT - Brutalist Data-Grid Terminal */}
				<ScrollReveal className="w-full" delay={0.2}>
					<div className="group relative flex h-[500px] flex-col overflow-hidden border border-[#27272a] bg-[#050505] p-2 shadow-2xl md:h-[600px]">
						{/* Inner Screen Bezel */}
						<div className="relative flex h-full w-full flex-col border border-[#18181b] bg-black">
							{/* Mac-Style Header & Tab */}
							<div className="z-20 flex h-12 shrink-0 items-center justify-between border-[#18181b] border-b bg-[#030303] px-4">
								<div className="flex items-center gap-4">
									<div className="flex gap-2">
										<div className="h-3 w-3 rounded-full bg-white/10 transition-colors group-hover:bg-red-500/80" />
										<div className="h-3 w-3 rounded-full bg-white/10 transition-colors group-hover:bg-amber-500/80" />
										<div className="h-3 w-3 rounded-full bg-white/10 transition-colors group-hover:bg-emerald-500/80" />
									</div>

									{/* Simple Path */}
									<div className="mx-2 h-4 w-px bg-[#27272a]" />
									<span className="mt-0.5 font-mono text-[#a1a1aa] text-[11px] uppercase tracking-widest">root@forged: ~</span>
								</div>

								<div className="flex items-center gap-4">
									<div className="flex items-center gap-2 border border-[#10b981]/30 bg-[#10b981]/10 px-2 py-1">
										<span className="h-1.5 w-1.5 animate-pulse bg-[#10b981]" />
										<span className="font-mono text-[#10b981] text-[9px] uppercase tracking-widest">ACTIVE</span>
									</div>
								</div>
							</div>

							{/* Terminal Body content */}
							<div className="relative flex-1 overflow-hidden bg-black">
								<AnimatedBigTerminal steps={ROUTING_DEMO} />
							</div>

							{/* Data-Dense Footer */}
							<div className="z-20 flex h-8 shrink-0 items-center justify-between border-[#18181b] border-t bg-[#050505] px-4">
								<div className="flex items-center gap-3">
									<span className="font-mono text-[#a1a1aa] text-[9px] uppercase tracking-widest">MEM: 14.2MB</span>
									<span className="hidden font-mono text-[#a1a1aa] text-[9px] uppercase tracking-widest sm:inline">| CPU: 0.1%</span>
								</div>
								<div className="flex items-center gap-1">
									<span className="h-3 w-1.5 bg-[#ea580c]" />
									<span className="h-3 w-1.5 bg-[#ea580c]" />
									<span className="h-3 w-1.5 bg-[#ea580c]" />
									<span className="h-3 w-1.5 bg-[#ea580c]/30" />
									<span className="h-3 w-1.5 bg-[#ea580c]/30" />
								</div>
							</div>
						</div>
					</div>
				</ScrollReveal>
			</div>
		</section>
	);
}

function Architecture() {
	const items = [
		{
			label: "Module 1",
			name: "Encrypted By Default",
			desc: "Argon2id + XChaCha20-Poly1305. The protocol standard for high risk key derivation.",
		},
		{
			label: "Module 2",
			name: "Unix Socket Agent",
			desc: "Emulates the exact ssh-agent protocol. Pure Go daemon, drops perfectly into any setup.",
		},
		{
			label: "Module 3",
			name: "Zero Knowledge Sync",
			desc: "Server architecture only stores heavily encrypted blobs. Vault is physically inaccessible.",
		},
	];

	return (
		<section className="relative overflow-hidden border-white/10 border-t bg-black py-36">
			<div
				className="pointer-events-none absolute inset-0 opacity-[0.04]"
				style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }}
			/>

			<div className="relative z-10 w-full px-6 lg:px-16">
				<ScrollReveal className="mb-4">
					<div className="flex items-center gap-2.5">
						<span className="h-2 w-2 bg-[#ea580c]" />
						<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Architecture</span>
					</div>
				</ScrollReveal>

				<ScrollReveal>
					<h2 className="mb-6 text-pretty font-bold text-4xl text-white leading-[0.95] tracking-tighter sm:text-5xl lg:text-7xl xl:text-8xl">Architecture</h2>
					<p className="mb-16 max-w-2xl text-[#a1a1aa] text-base leading-relaxed lg:text-lg">
						No Electron. No bloated browser extensions. Strictly terminal and background daemons written in modern Go.
					</p>
				</ScrollReveal>

				<ScrollReveal>
					<TopologyVisualizer />
				</ScrollReveal>
			</div>
		</section>
	);
}

function EnterpriseSecurity() {
	const specs = [
		{
			id: "01",
			title: "CipherSuite",
			value: "XChaCha20",
			desc: "All vault data is encrypted using XChaCha20-Poly1305 AEAD. Extremely fast, deeply secure, and completely immune to timing attacks.",
		},
		{
			id: "02",
			title: "Derivation",
			value: "Argon2id",
			desc: "Master keys are mathematically generated through Argon2id, the winner of the Password Hashing Competition. Highly ASIC resistant.",
		},
		{
			id: "03",
			title: "Isolation",
			value: "M-Lock",
			desc: "The agent daemon uses unix.Mlock() to pin all decrypted memory pages, ensuring host OS swap-to-disk leaks are physically impossible.",
		},
		{
			id: "04",
			title: "Auditability",
			value: "Open Core",
			desc: "The entire core daemon and CLI is open source. No proprietary telemetry, no opaque cryptographic implementations.",
		},
	];

	return (
		<section className="relative overflow-hidden border-[#27272a] border-t bg-black py-36">
			{/* Brutalist Grid Background overlay */}
			<div
				className="absolute inset-0 opacity-[0.02]"
				style={{ backgroundImage: "linear-gradient(#fff 1px, transparent 1px), linear-gradient(90deg, #fff 1px, transparent 1px)", backgroundSize: "64px 64px" }}
			/>

			<div className="relative z-10 mx-auto w-full max-w-[1400px] px-6 lg:px-16">
				{/* Deep industrial header */}
				<ScrollReveal className="mb-4">
					<div className="flex items-center gap-2.5">
						<span className="h-2 w-2 bg-[#ea580c]" />
						<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Enterprise Security</span>
					</div>
				</ScrollReveal>

				<ScrollReveal>
					<h2 className="mb-6 text-pretty font-bold text-4xl text-white leading-[0.95] tracking-tighter sm:text-5xl lg:text-7xl xl:text-8xl">
						Zero Knowledge.
					</h2>
					<p className="mb-16 max-w-2xl text-[#a1a1aa] text-base leading-relaxed lg:text-lg">
						We believe security through obscurity is no security at all. Forged is built entirely on open, mathematically auditable cryptographic standards.
						Your private keys never touch a disk unencrypted, and never leave your machine without end-to-end encryption.
					</p>
				</ScrollReveal>

				{/* The Specs Grid (Data-dense, purely typographical, massive impact) */}
				<div className="grid grid-cols-1 gap-px bg-[#27272a] md:grid-cols-2 lg:grid-cols-4">
					{specs.map((spec, i) => (
						<ScrollReveal className="group relative flex flex-col overflow-hidden bg-black p-8" delay={i * 0.1} key={spec.id}>
							{/* Internal hover glow */}
							<div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-[#ea580c]/5 to-transparent opacity-0 transition-opacity duration-500 group-hover:opacity-100" />

							<div className="mb-8 flex items-center justify-between">
								<span className="font-mono text-[#a1a1aa] text-xs uppercase tracking-widest">Spec // {spec.id}</span>
								<svg
									className="text-[#3f3f46] transition-colors duration-500 group-hover:text-[#ea580c]"
									fill="none"
									height="14"
									stroke="currentColor"
									strokeWidth="1.5"
									viewBox="0 0 24 24"
									width="14"
								>
									<path d="M5 12h14" />
									<path d="M12 5l7 7-7 7" />
								</svg>
							</div>
							<span className="mb-1 font-mono text-[#ea580c] text-sm uppercase tracking-widest opacity-80">{spec.title}</span>
							<span className="mb-6 font-bold text-3xl text-white tracking-tight sm:text-4xl">{spec.value}</span>
							<p className="relative z-10 mt-auto text-[#a1a1aa] text-sm leading-relaxed">{spec.desc}</p>
						</ScrollReveal>
					))}
				</div>

				{/* Audit Badges / Trust center */}
				<ScrollReveal
					className="relative mt-8 flex flex-col items-center justify-between gap-8 overflow-hidden border border-[#27272a] bg-[#020202] p-8 md:flex-row md:p-12"
					delay={0.2}
				>
					{/* Scanline effect */}
					<div className="pointer-events-none absolute inset-0 h-[1px] w-full animate-scan bg-gradient-to-r from-transparent via-[#ea580c]/20 to-transparent" />

					<div className="relative z-10 flex-1">
						<h3 className="mb-3 font-bold text-2xl text-white tracking-tight md:text-3xl">Enterprise Ready. Fully Auditable.</h3>
						<p className="max-w-2xl text-[#a1a1aa] text-sm md:text-base">
							Read the complete cryptographic breakdown of our vault structure in the security whitepaper, or dive directly into the repository to audit the
							Go implementation yourself.
						</p>
					</div>

					<div className="relative z-10 flex w-full flex-col gap-4 sm:w-auto sm:flex-row">
						<GlitchButton className="h-14 border-[#ea580c] px-8 text-sm" href="/security">
							Read Security Paper
						</GlitchButton>
						<GlitchButton className="h-14 px-8 text-sm" href="https://github.com/itzzritik/forged" variant="secondary">
							Audit Source Code
						</GlitchButton>
					</div>
				</ScrollReveal>
			</div>
		</section>
	);
}

function CTA() {
	return (
		<section className="relative flex flex-col items-center justify-center overflow-hidden border-white/10 border-t bg-black py-36 text-center">
			<div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,_rgba(234,88,12,0.06)_0%,_transparent_60%)]" />
			<div
				className="pointer-events-none absolute inset-0 opacity-[0.04]"
				style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }}
			/>

			<ScrollReveal className="relative z-10 flex w-full max-w-4xl flex-col items-center px-6 lg:px-16">
				<div className="mb-6 flex items-center justify-center gap-2.5">
					<span className="h-2 w-2 animate-pulse bg-[#ea580c]" />
					<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Get Started</span>
				</div>
				<h2 className="mb-8 text-pretty font-bold text-5xl text-white leading-[0.9] tracking-tighter sm:text-7xl lg:text-8xl xl:text-[100px]">
					Secure your keys
					<br />
					Ship everything else
				</h2>
				<p className="mb-12 max-w-2xl text-[#a1a1aa] text-base leading-relaxed lg:text-xl">Install Forged. Never think about SSH key management again.</p>
				<div className="flex flex-col flex-wrap items-center justify-center gap-6 sm:flex-row">
					<AuthCTAButton />
					<GlitchButton className="h-14 max-w-full px-12 text-sm" href="/docs" variant="secondary">
						Read Docs
					</GlitchButton>
				</div>
			</ScrollReveal>
		</section>
	);
}

function Footer() {
	return (
		<footer className="border-[#27272a] border-t bg-black py-16">
			<div className="flex w-full flex-col items-center justify-between gap-6 px-6 sm:flex-row lg:px-16">
				<div className="flex items-center gap-3">
					<div className="flex h-6 w-6 items-center justify-center border border-[#27272a] bg-black">
						<svg className="text-white" fill="none" height="12" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" width="12">
							<path d="M15 3h6v6" />
							<path d="M10 14L21 3" />
							<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
						</svg>
					</div>
					<span className="font-bold text-[#a1a1aa] text-[11px] uppercase tracking-widest">&copy; Forged Inc {new Date().getFullYear()}</span>
				</div>
				<div className="flex items-center gap-10 text-[#a1a1aa] text-[11px] uppercase tracking-widest">
					<a className="transition-colors hover:text-[#ea580c]" href="https://github.com/itzzritik/forged">
						GitHub
					</a>
					<Link className="transition-colors hover:text-[#ea580c]" href="/docs">
						Docs
					</Link>
					<Link className="transition-colors hover:text-[#ea580c]" href="/security">
						Privacy
					</Link>
				</div>
			</div>
		</footer>
	);
}

export default function Home() {
	return (
		<div className="bg-black">
			<Nav />
			<Hero />
			<GridFeatures />
			<TerminalSection />
			<Architecture />
			<EnterpriseSecurity />
			<CTA />
			<Footer />
		</div>
	);
}
