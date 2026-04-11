import type { Metadata } from "next";
import Link from "next/link";
import { GlitchButton, ScrollReveal } from "@/components/client";

export const metadata: Metadata = {
	title: "Security - Forged",
	description: "How Forged protects your SSH keys. Zero-knowledge architecture, encryption details, and threat model.",
};

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
					<Link className="hidden text-[12px] text-white uppercase tracking-wider transition-colors sm:block" href="/security">
						Security
					</Link>
					<a
						className="hidden text-[#a1a1aa] text-[12px] uppercase tracking-wider transition-colors hover:text-white md:block"
						href="https://github.com/itzzritik/forged"
					>
						GitHub
					</a>
					<GlitchButton className="h-8 px-5 text-[12px]" href="/login">
						Sign in
					</GlitchButton>
				</div>
			</div>
		</nav>
	);
}

function HeroBackground() {
	// Pre-generated hex lines for the background - deterministic, no runtime randomness
	const hexLines = [
		"e3b0c44298fc1c14 9afbf4c8996fb924 27ae41e4649b934c a495991b7852b855",
		"a7ffc6f8bf1ed766 51c14756a061d662 f580ff4de43b49fa 82d80a4b80f8434a",
		"6b86b273ff34fce1 9d6b804eff5a3f57 47ada4eaa22f1d49 c01e52ddb7875b4b",
		"d4735e3a265e16ee a03239cc719ab018 06b6fa084a4e97eb 22f9fe5e40b7e3e3",
		"4e07408562bedb8b 60ce05c1decfe3ad 16b72230967de01f 640b7e4729b49fce",
		"4b227777d4dd1fc6 1c6f884f48641d02 b4d121d3fd328cb0 8b5f0c25b5e33c7b",
		"ef2d127de37b942b aad06145e54b0c61 9a1f22327b2ebbcf bec78f5564afe39d",
		"e7f6c011776e8db7 cd330b54174fd76f 7d0216b612387a5f faf4e6c34b67ddfe",
		"2c624232cdd221771 294750a4d5e4d4cd 2bf3d5e56b3327e3 19c45e0ad9f1d2b8",
		"19581e27de7ced00 ff1ce50b2047e7a5 67c43b8d4b1ae21e 4db1a0c5c6e47e28",
		"4a44dc15364204a8 0fe80e9039455cc1 608281820fe2b24f 1e5233ade6af1dd5",
		"9f14025af0065b30 6c0a68d2efb68d65 de27cc87fd346d6e a25a9e1c2e6e1e7b",
		"b17ef6d19c7a5b1e e83b907af917c8b5 00c1a073db9c2aad eb44ec546aa3b90b",
		"ca978112ca1bbdca fac231b39a23dc4d a786eff8147c4e72 b9807785afee48bb",
		"3e23e8160039594a 33894f6564e1b134 8bbd7a0088d42c4a cb73eeaed59c009d",
		"2e7d2c03a9507ae2 65ecf5b5356885a5 394c32af5064a65a 2b6577725ec7d8a1",
	];

	return (
		<div className="pointer-events-none absolute inset-0 select-none overflow-hidden">
			{/* Hex digest lines - scrolling upward */}
			<div className="absolute inset-0 flex flex-col justify-center gap-[2px] opacity-[0.06]">
				{hexLines.map((line, i) => (
					<div
						className="animate-slide-up whitespace-nowrap font-mono text-[11px] text-white tracking-[0.3em] lg:text-[13px]"
						key={i}
						style={{
							animationDelay: `${i * 120}ms`,
							animationDuration: "1.2s",
							animationFillMode: "both",
						}}
					>
						{line}
					</div>
				))}
			</div>

			{/* Gradient overlays for depth */}
			<div className="absolute inset-0 bg-gradient-to-b from-black via-transparent to-black" />
			<div className="absolute inset-0 bg-gradient-to-r from-black via-black/60 to-transparent" />
			<div className="absolute inset-0 bg-[radial-gradient(ellipse_at_bottom_left,_rgba(234,88,12,0.1)_0%,_transparent_50%)]" />
		</div>
	);
}

function Hero() {
	const stats = [
		{ label: "Cipher", value: "XChaCha20" },
		{ label: "KDF", value: "Argon2id" },
		{ label: "Key", value: "256-bit" },
		{ label: "Nonce", value: "192-bit" },
	];

	return (
		<section className="relative flex h-dvh flex-col overflow-hidden border-[#27272a] border-b">
			<HeroBackground />

			{/* Content */}
			<div className="relative z-10 flex flex-1 flex-col justify-center px-6 lg:px-16">
				<div className="max-w-5xl">
					<div className="animate-slide-up">
						<div className="inline-flex items-center gap-2 bg-[#ea580c] px-3 py-1.5 font-bold font-mono text-[11px] text-black uppercase tracking-[0.15em]">
							<span className="h-1.5 w-1.5 animate-pulse rounded-full bg-black" />
							Security Whitepaper
						</div>
					</div>

					<h1 className="mt-8 animate-slide-up font-bold text-[clamp(48px,10vw,128px)] text-white leading-[0.85] tracking-[-0.04em] delay-100">
						Security
						<br />
						Protocol
					</h1>

					<p className="mt-8 max-w-2xl animate-slide-up text-[#a1a1aa] text-lg leading-relaxed delay-200 md:text-xl">
						Zero-knowledge architecture. The master password never moves. Key material is heavily insulated. Nothing escapes encrypted buffers.
					</p>

					{/* Cryptographic stat badges */}
					<div className="mt-10 flex animate-slide-up flex-wrap items-center gap-3 delay-300">
						{stats.map((s) => (
							<div
								className="group flex h-10 items-center gap-3 border border-[#27272a] bg-[#09090b] px-4 transition-colors hover:border-[#ea580c]/40"
								key={s.label}
							>
								<span className="font-mono text-[#a1a1aa] text-[9px] uppercase tracking-widest">{s.label}</span>
								<span className="h-4 w-px bg-[#27272a]" />
								<span className="font-bold font-mono text-[12px] text-white tracking-wider transition-colors group-hover:text-[#ea580c]">{s.value}</span>
							</div>
						))}
					</div>
				</div>
			</div>

			{/* Bottom bar */}
			<div className="relative z-10 shrink-0 border-[#27272a] border-t bg-[#050505]/80 backdrop-blur-sm">
				<div className="flex h-14 items-center justify-between px-6 lg:px-16">
					<div className="flex items-center gap-6">
						<div className="flex items-center gap-2">
							<span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#10b981] shadow-[0_0_8px_#10b981]" />
							<span className="font-bold font-mono text-[#10b981] text-[10px] uppercase tracking-widest">Audited</span>
						</div>
						<span className="hidden h-4 w-px bg-[#27272a] sm:block" />
						<span className="hidden font-mono text-[#a1a1aa] text-[10px] uppercase tracking-widest sm:block">Open Source</span>
					</div>
					<div className="flex items-center gap-2">
						<span className="hidden font-mono text-[#3f3f46] text-[9px] uppercase tracking-widest sm:block">golang.org/x/crypto</span>
					</div>
				</div>
			</div>
		</section>
	);
}

function EncryptionPrimitives() {
	const primitives = [
		{
			icon: "M12 15v2m-6 4h12a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2zm10-10V7a4 4 0 0 0-8 0v4h8z",
			title: "Argon2id",
			subtitle: "Key derivation",
			desc: "Winner of the Password Hashing Competition. Memory-hard function making GPU and ASIC brute-force attacks economically infeasible.",
			bullets: ["64MB memory cost", "3 iterations, 4 parallelism", "ASIC-resistant by design"],
			cta: "View Parameters",
			href: "#key-hierarchy",
		},
		{
			icon: "M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0 1 12 2.944a11.955 11.955 0 0 1-8.618 3.04A12.02 12.02 0 0 0 3 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z",
			title: "XChaCha20-Poly1305",
			subtitle: "Vault encryption",
			desc: "Extended-nonce AEAD stream cipher. Immune to timing attacks, nonce-misuse resistant, and faster than AES-GCM without hardware acceleration.",
			bullets: ["256-bit encryption key", "192-bit extended nonce", "Authenticated encryption (AEAD)"],
			cta: "View Cipher Details",
			href: "#key-hierarchy",
		},
		{
			icon: "M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4",
			title: "HKDF-SHA256",
			subtitle: "Sync protocol",
			desc: "Deterministic key derivation using HMAC-based Extract-and-Expand. Sync keys are mathematically isolated from vault keys.",
			bullets: ["Context-bound derivation", "Isolated sync key space", "Forward secrecy"],
			cta: "View Sync Protocol",
			href: "/docs#sync",
		},
		{
			icon: "M4 4v5h.582m15.356 2A8.001 8.001 0 0 0 4.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 0 1-15.357-2m15.357 2H15",
			title: "Random 24-byte",
			subtitle: "Nonce strategy",
			desc: "Every vault write generates a cryptographically random 24-byte nonce, entirely preventing collision attacks across billions of operations.",
			bullets: ["Per-write randomization", "No nonce reuse possible", "Collision-resistant by design"],
			cta: "View Nonce Strategy",
			href: "/docs",
		},
	];

	return (
		<section className="relative overflow-hidden border-[#27272a] border-b bg-black px-6 py-24 lg:px-16 lg:py-36">
			<div
				className="pointer-events-none absolute inset-0 opacity-[0.04]"
				style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }}
			/>

			<div className="relative z-10 w-full">
				<ScrollReveal className="mb-4">
					<div className="flex items-center gap-2.5">
						<span className="h-2 w-2 bg-[#ea580c]" />
						<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Encryption Primitives</span>
					</div>
				</ScrollReveal>

				<ScrollReveal>
					<h2 className="text-pretty font-bold text-4xl text-white leading-[0.95] tracking-tighter sm:text-5xl lg:text-7xl xl:text-8xl">
						Cryptographic Foundation
					</h2>
				</ScrollReveal>

				<ScrollReveal className="mt-6">
					<p className="max-w-2xl text-[#a1a1aa] text-base leading-relaxed lg:text-lg">
						Every layer of the vault is built on battle-tested, open-standard cryptographic primitives. No proprietary algorithms, no security through
						obscurity.
					</p>
				</ScrollReveal>

				<div className="relative z-10 mt-16 grid grid-cols-1 border-white/10 border-t border-l md:grid-cols-2">
					{primitives.map((f) => (
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

				<ScrollReveal delay={0.2}>
					<div className="mt-px border border-[#27272a] border-t-0 bg-[#09090b] p-6 lg:p-8">
						<div className="flex items-start gap-4">
							<div className="h-full min-h-[40px] w-1 shrink-0 self-stretch bg-[#ea580c]" />
							<p className="font-mono text-[#a1a1aa] text-[13px] leading-relaxed">
								Derived 256-bit keys encrypt the vault using XChaCha20-Poly1305. The 24-byte nonce is freshly randomized on every local vault sync action,
								entirely preventing collision attacks. All cryptographic operations use Go&apos;s <code className="text-white">golang.org/x/crypto</code>{" "}
								library - no custom implementations.
							</p>
						</div>
					</div>
				</ScrollReveal>
			</div>
		</section>
	);
}

function KeyHierarchy() {
	const steps = [
		{
			id: "01",
			label: "Input",
			value: "Master Password",
			desc: "User-provided passphrase. Never stored on disk, never transmitted. Zeroed from memory immediately after derivation.",
		},
		{
			id: "02",
			label: "KDF",
			value: "Argon2id",
			desc: "Memory-hard function transforms the password into a 256-bit key. 64MB memory, 3 iterations, 4 parallelism. ASIC-resistant.",
		},
		{
			id: "03",
			label: "Output",
			value: "Vault Key",
			desc: "256-bit symmetric key encrypts and decrypts the local vault file via XChaCha20-Poly1305 AEAD. Never persisted.",
		},
		{
			id: "04",
			label: "Expand",
			value: "HKDF-SHA256",
			desc: "Derives a mathematically isolated sync key from the vault key using context-bound HMAC Extract-and-Expand.",
		},
		{ id: "05", label: "Output", value: "Sync Key", desc: "Encrypts the vault blob before upload. The sync server only ever sees opaque, encrypted binary data." },
	];

	return (
		<section className="relative overflow-hidden border-[#27272a] border-b bg-black py-24 lg:py-36" id="key-hierarchy">
			<div
				className="absolute inset-0 opacity-[0.02]"
				style={{ backgroundImage: "linear-gradient(#fff 1px, transparent 1px), linear-gradient(90deg, #fff 1px, transparent 1px)", backgroundSize: "64px 64px" }}
			/>

			<div className="relative z-10 w-full px-6 lg:px-16">
				<ScrollReveal className="mb-4">
					<div className="flex items-center gap-2.5">
						<span className="h-2 w-2 bg-[#ea580c]" />
						<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Key Hierarchy</span>
					</div>
				</ScrollReveal>

				<ScrollReveal>
					<h2 className="text-pretty font-bold text-4xl text-white leading-[0.95] tracking-tighter sm:text-5xl lg:text-7xl xl:text-8xl">Derivation Chain</h2>
				</ScrollReveal>

				<ScrollReveal className="mt-6 mb-16">
					<p className="max-w-2xl text-[#a1a1aa] text-base leading-relaxed lg:text-lg">
						A single master password deterministically generates all encryption keys through a strict, one-way derivation chain. No key is ever stored - they
						are regenerated on demand.
					</p>
				</ScrollReveal>

				<div className="grid grid-cols-1 gap-px bg-[#27272a] sm:grid-cols-2 lg:grid-cols-5">
					{steps.map((step, i) => (
						<ScrollReveal className="group relative flex flex-col overflow-hidden bg-black p-8" delay={i * 0.1} key={step.id}>
							<div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-[#ea580c]/5 to-transparent opacity-0 transition-opacity duration-500 group-hover:opacity-100" />

							<div className="mb-8 flex items-center justify-between">
								<span className="font-mono text-[#a1a1aa] text-xs uppercase tracking-widest">Step // {step.id}</span>
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
							<span className="mb-1 font-mono text-[#ea580c] text-sm uppercase tracking-widest opacity-80">{step.label}</span>
							<span className="mb-6 font-bold text-2xl text-white tracking-tight sm:text-3xl">{step.value}</span>
							<p className="relative z-10 mt-auto text-[#a1a1aa] text-sm leading-relaxed">{step.desc}</p>
						</ScrollReveal>
					))}
				</div>
			</div>
		</section>
	);
}

function PayloadAccess() {
	const groups = [
		{
			status: "Visible",
			color: "#a1a1aa",
			items: [{ label: "Email Address", desc: "Available strictly for account ID metadata (via OAuth)" }],
		},
		{
			status: "Encrypted",
			color: "#f59e0b",
			items: [{ label: "Raw Vault Blob", desc: "Encrypted payload accessible to sync servers - opaque binary data" }],
		},
		{
			status: "Blocked",
			color: "#ef4444",
			items: [
				{ label: "Master Password", desc: "Never leaves local execution. Zeroed from memory after derivation." },
				{ label: "Encryption Key", desc: "Local-only deterministic generation. Invisible to any server." },
				{ label: "Private SSH Keys", desc: "Nested within AEAD-encrypted vault buffers. Physically inaccessible." },
			],
		},
	];

	return (
		<section className="relative overflow-hidden border-[#27272a] border-b bg-black py-24 lg:py-36">
			<div
				className="pointer-events-none absolute inset-0 opacity-[0.04]"
				style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }}
			/>

			<div className="relative z-10 w-full px-6 lg:px-16">
				<ScrollReveal className="mb-4">
					<div className="flex items-center gap-2.5">
						<span className="h-2 w-2 bg-[#ea580c]" />
						<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Access Control</span>
					</div>
				</ScrollReveal>

				<ScrollReveal>
					<h2 className="text-pretty font-bold text-4xl text-white leading-[0.95] tracking-tighter sm:text-5xl lg:text-7xl xl:text-8xl">Payload Visibility</h2>
				</ScrollReveal>

				<ScrollReveal className="mt-6 mb-16">
					<p className="max-w-2xl text-[#a1a1aa] text-base leading-relaxed lg:text-lg">
						What the server can see, what it can&apos;t, and what is physically impossible to access - even with full infrastructure compromise.
					</p>
				</ScrollReveal>

				<div className="flex flex-col gap-4">
					{groups.map((group, gi) => (
						<ScrollReveal delay={gi * 0.1} key={group.status}>
							<div className="flex flex-col overflow-hidden border border-[#27272a] bg-black md:flex-row">
								{/* Left status strip */}
								<div className="flex shrink-0 items-center justify-between gap-3 border-[#27272a] border-b bg-[#050505] p-6 md:w-48 md:flex-col md:items-start md:justify-center md:border-r md:border-b-0">
									<div className="flex items-center gap-2.5">
										<span
											className="h-2 w-2 animate-pulse rounded-full"
											style={{ backgroundColor: group.color, boxShadow: `0 0 8px ${group.color}` }}
										/>
										<span className="font-bold font-mono text-sm uppercase tracking-widest" style={{ color: group.color }}>
											{group.status}
										</span>
									</div>
									<span className="font-mono text-[#3f3f46] text-[10px] tracking-widest">
										{group.items.length} {group.items.length === 1 ? "field" : "fields"}
									</span>
								</div>

								{/* Items */}
								<div className="flex-1 divide-y divide-[#18181b]">
									{group.items.map((item) => (
										<div
											className="group flex flex-col gap-2 p-6 transition-colors hover:bg-[#09090b] sm:flex-row sm:items-center sm:gap-8"
											key={item.label}
										>
											<span className="shrink-0 font-bold font-mono text-[13px] text-white transition-colors duration-300 group-hover:text-[#ea580c] sm:w-48">
												{item.label}
											</span>
											<p className="text-[#a1a1aa] text-[13px] leading-relaxed">{item.desc}</p>
										</div>
									))}
								</div>
							</div>
						</ScrollReveal>
					))}
				</div>
			</div>
		</section>
	);
}

function ThreatModel() {
	const threats = [
		{
			vector: "Disk theft",
			mitigation:
				"Vault is encrypted via Argon2id + XChaCha20-Poly1305. Physical data is cryptographically opaque without brute-forcing memory-hard key derivation.",
		},
		{
			vector: "Network node capture",
			mitigation: "Zero-knowledge architecture. Captured infrastructure contains exclusively encrypted binary blobs with no decryption capability.",
		},
		{
			vector: "Memory swap leak",
			mitigation: "Key memory pages locked with unix.Mlock(). Daemon actively zeroes all sensitive memory regions upon shutdown or lock.",
		},
		{ vector: "Socket interception", mitigation: "Daemon socket permissions strictly enforced at 0600 with owner-only access. No remote socket exposure." },
		{
			vector: "MITM on TLS sync",
			mitigation: "Forced TLS 1.3 transit with secondary vault-level encryption. Dual-layer protection makes MITM cryptographically useless.",
		},
		{
			vector: "Brute force attack",
			mitigation: "Argon2id parameters (64MB, 3 iterations) make each guess cost ~300ms. Rate limiting strictly enforced server-side.",
		},
		{
			vector: "File corruption",
			mitigation: "Atomic write logic (tmp + fsync + rename) ensures vault integrity. Write failures never corrupt existing vault state.",
		},
	];

	return (
		<section className="relative overflow-hidden border-[#27272a] border-b bg-black py-24 lg:py-36">
			<div
				className="pointer-events-none absolute inset-0 opacity-[0.04]"
				style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }}
			/>

			<div className="relative z-10 w-full px-6 lg:px-16">
				<ScrollReveal className="mb-16">
					<div className="mb-4 flex items-center gap-2.5">
						<span className="h-2 w-2 bg-[#ea580c]" />
						<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Threat Model</span>
					</div>
					<h2 className="mb-6 font-bold text-4xl text-white leading-[0.95] tracking-tighter sm:text-5xl lg:text-7xl">Attack Surface</h2>
					<p className="max-w-2xl text-[#a1a1aa] text-base leading-relaxed lg:text-lg">
						Every known attack vector, mapped to its operational mitigation. If you find a gap, we want to hear about it.
					</p>
				</ScrollReveal>

				{/* Desktop: Table */}
				<ScrollReveal className="hidden md:block">
					<div className="overflow-hidden border border-[#27272a]">
						<table className="w-full bg-black text-sm">
							<thead>
								<tr className="bg-[#050505]">
									<th className="w-[280px] border-[#27272a] border-r py-5 pl-8 text-left font-bold font-mono text-[#ea580c] text-[10px] uppercase tracking-widest">
										Threat Vector
									</th>
									<th className="py-5 pl-8 text-left font-bold font-mono text-[#ea580c] text-[10px] uppercase tracking-widest">
										Operational Mitigation
									</th>
								</tr>
							</thead>
							<tbody>
								{threats.map((t) => (
									<tr className="group border-[#18181b] border-t bg-black transition-colors hover:bg-[#09090b]" key={t.vector}>
										<td className="border-[#27272a] border-r py-6 pr-6 pl-8 align-top font-mono text-sm text-white transition-colors duration-300 group-hover:text-[#ea580c]">
											{t.vector}
										</td>
										<td className="py-6 pr-8 pl-8 text-[#a1a1aa] text-sm leading-relaxed">{t.mitigation}</td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				</ScrollReveal>

				{/* Mobile: Stacked cards */}
				<div className="space-y-px md:hidden">
					{threats.map((t, i) => (
						<ScrollReveal delay={i * 0.05} key={t.vector}>
							<div className="group border border-[#27272a] bg-black p-6 transition-colors hover:border-[#ea580c]/30">
								<span className="mb-3 block font-bold font-mono text-sm text-white transition-colors group-hover:text-[#ea580c]">{t.vector}</span>
								<p className="text-[#a1a1aa] text-sm leading-relaxed">{t.mitigation}</p>
							</div>
						</ScrollReveal>
					))}
				</div>
			</div>
		</section>
	);
}

function SecurityCTA() {
	return (
		<section className="relative flex flex-col items-center justify-center overflow-hidden bg-black py-36 text-center">
			<div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,_rgba(234,88,12,0.06)_0%,_transparent_60%)]" />
			<div
				className="pointer-events-none absolute inset-0 opacity-[0.04]"
				style={{ background: "repeating-linear-gradient(135deg,transparent,transparent 4px,rgba(255,255,255,0.5) 4px,rgba(255,255,255,0.5) 5px)" }}
			/>

			<ScrollReveal className="relative z-10 flex w-full max-w-3xl flex-col items-center px-6 lg:px-16">
				<div className="mb-6 flex items-center justify-center gap-2.5">
					<span className="h-2 w-2 animate-pulse bg-[#ea580c]" />
					<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-[0.2em]">Open Source</span>
				</div>
				<h2 className="mb-8 text-pretty font-bold text-4xl text-white leading-[0.95] tracking-tighter sm:text-5xl lg:text-7xl">
					Don&apos;t trust us.
					<br />
					Audit us.
				</h2>
				<p className="mb-12 max-w-2xl text-[#a1a1aa] text-base leading-relaxed lg:text-xl">
					The entire Forged core is open source. Every cryptographic implementation, every daemon operation, every vault interaction - fully inspectable.
				</p>
				<div className="flex flex-col items-center justify-center gap-4 sm:flex-row">
					<GlitchButton className="h-14 px-10 text-sm" external href="https://github.com/itzzritik/forged">
						Examine Source Code
					</GlitchButton>
					<GlitchButton className="h-14 px-10 text-sm" href="/docs" variant="secondary">
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
					<Link className="transition-colors hover:text-[#ea580c]" href="/">
						Home
					</Link>
				</div>
			</div>
		</footer>
	);
}

export default function SecurityPage() {
	return (
		<div className="min-h-screen bg-black">
			<Nav />
			<Hero />
			<EncryptionPrimitives />
			<KeyHierarchy />
			<PayloadAccess />
			<ThreatModel />
			<SecurityCTA />
			<Footer />
		</div>
	);
}
