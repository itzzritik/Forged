"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { AnimatedTerminalGrid, GlitchText, TERMINAL_CARDS } from "@/components/client";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

function GitHubIcon() {
	return (
		<svg aria-label="GitHub" fill="currentColor" height="16" role="img" viewBox="0 0 24 24" width="16">
			<path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
		</svg>
	);
}

function GoogleIcon() {
	return (
		<svg aria-label="Google" height="16" role="img" viewBox="0 0 24 24" width="16">
			<path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="currentColor" />
			<path
				d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
				fill="currentColor"
			/>
			<path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="currentColor" />
			<path
				d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
				fill="currentColor"
			/>
		</svg>
	);
}

function BackgroundGrid() {
	return (
		<div className="pointer-events-none absolute inset-0 select-none overflow-hidden bg-black">
			{/* Reduced grid opacity from 70% to 45% to keep it visible but non-distracting */}
			<div className="absolute inset-0 opacity-[0.45]">
				<AnimatedTerminalGrid cards={TERMINAL_CARDS} />
			</div>

			{/* Increased dark overlay slightly to 40% to soften the contrast further */}
			<div className="absolute inset-0 bg-black/40" />

			{/* Radial gradient mask pushed further out (70% instead of 90%) to keep center text readable without dimming the whole page */}
			<div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_transparent_0%,_black_100%)]" />

			{/* Top/bottom fade edges */}
			<div className="absolute inset-x-0 top-0 h-32 bg-gradient-to-b from-black to-transparent" />
			<div className="absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-black to-transparent" />
		</div>
	);
}

function LoginContent() {
	const searchParams = useSearchParams();
	const code = searchParams.get("code") || "";
	const error = searchParams.get("error");
	const [verification, setVerification] = useState<string | null>(null);

	useEffect(() => {
		if (!code) return;
		fetch(`${API_URL}/api/v1/auth/sessions/${code}/verification`)
			.then((r) => r.json())
			.then((data) => setVerification(data.verification || null))
			.catch(() => setVerification(null));
	}, [code]);

	const codeParam = code ? `?code=${encodeURIComponent(code)}` : "";
	const githubUrl = `${API_URL}/api/v1/auth/github${codeParam}`;
	const googleUrl = `${API_URL}/api/v1/auth/google${codeParam}`;

	return (
		<div className="relative flex min-h-screen flex-col items-center justify-center overflow-hidden bg-black px-6">
			<BackgroundGrid />

			{/* Top nav bar */}
			<nav className="fixed top-0 right-0 left-0 z-50 border-[#27272a] border-b bg-black/80 backdrop-blur-xl">
				<div className="flex h-14 w-full items-center justify-between px-6 lg:px-16">
					<Link className="group flex items-center gap-3" href="/">
						<div className="flex h-7 w-7 items-center justify-center border border-[#27272a] bg-black transition-colors group-hover:border-[#ea580c]">
							<svg
								aria-label="Forged logo"
								className="text-white transition-colors group-hover:text-[#ea580c]"
								fill="none"
								height="14"
								role="img"
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
					<Link className="flex items-center gap-2 text-[#a1a1aa] text-[12px] uppercase tracking-wider transition-colors hover:text-white" href="/">
						<svg aria-label="Back" fill="none" height="12" role="img" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" width="12">
							<line x1="19" x2="5" y1="12" y2="12" />
							<polyline points="12 19 5 12 12 5" />
						</svg>
						Back
					</Link>
				</div>
			</nav>

			{/* Main content */}
			<div className="relative z-10 w-full max-w-[440px] animate-slide-up">
				{/* Header */}
				<div className="mb-10 flex flex-col items-center">
					<div className="relative">
						<div className="absolute inset-0 scale-150 bg-[#ea580c]/15 blur-[24px]" />
						<div className="relative flex h-14 w-14 items-center justify-center border border-[#27272a] bg-black">
							<svg
								aria-label="Forged logo"
								className="text-white"
								fill="none"
								height="24"
								role="img"
								stroke="currentColor"
								strokeLinecap="round"
								strokeLinejoin="round"
								strokeWidth="1.5"
								viewBox="0 0 24 24"
								width="24"
							>
								<path d="M15 3h6v6" />
								<path d="M10 14L21 3" />
								<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
							</svg>
						</div>
					</div>
					<h1 className="mt-8 font-bold text-3xl text-white tracking-tighter sm:text-4xl">Sign in</h1>
					<p className="mt-3 text-[#a1a1aa] text-sm tracking-wide">Authenticate to begin your session</p>
				</div>

				{/* Auth card */}
				<div className="overflow-hidden border border-[#27272a] bg-[#050505]">
					{/* Card header */}
					<div className="flex h-10 items-center justify-between border-[#27272a] border-b bg-[#030303] px-6">
						<div className="flex items-center gap-3">
							<span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#ea580c] shadow-[0_0_8px_#ea580c]" />
							<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-widest">Session {/* // */} Auth</span>
						</div>
						<span className="font-mono text-[#3f3f46] text-[9px] uppercase tracking-widest">OAuth 2.0</span>
					</div>

					<div className="p-6 sm:p-8">
						{error && (
							<div className="mb-6 flex items-center gap-3 border border-red-500/20 bg-red-950/20 p-4 font-mono text-[12px] text-red-400">
								<svg
									aria-label="Error icon"
									className="shrink-0"
									fill="none"
									height="14"
									role="img"
									stroke="currentColor"
									strokeWidth="2"
									viewBox="0 0 24 24"
									width="14"
								>
									<circle cx="12" cy="12" r="10" />
									<line x1="15" x2="9" y1="9" y2="15" />
									<line x1="9" x2="15" y1="9" y2="15" />
								</svg>
								{decodeURIComponent(error)}
							</div>
						)}

						{code && verification && (
							<div className="mb-6 border border-border-line bg-surface-hover p-4 text-center">
								<p className="mb-2 font-mono text-[#3f3f46] text-[10px] uppercase tracking-widest">Verify this code matches your terminal</p>
								<p className="font-bold font-mono text-accent text-lg tracking-[0.15em]">FORGE-{verification.toUpperCase()}</p>
							</div>
						)}

						{code && !verification && (
							<div className="mb-6 flex items-center justify-center border border-border-line bg-surface-hover p-4">
								<div className="h-4 w-4 animate-spin rounded-full border-2 border-border-line border-t-accent" />
							</div>
						)}

						<div className="space-y-3">
							<a
								className="group relative flex h-12 w-full items-center justify-center gap-3 overflow-hidden bg-white font-bold font-mono text-[12px] text-black uppercase tracking-widest transition-all duration-200 hover:bg-zinc-100 active:scale-[0.98]"
								href={githubUrl}
							>
								<GitHubIcon />
								<GlitchText className="relative z-[2]" text="Continue with GitHub" />
							</a>

							<a
								className="group relative flex h-12 w-full items-center justify-center gap-3 overflow-hidden border border-[#27272a] bg-black font-bold font-mono text-[12px] text-white uppercase tracking-widest transition-all duration-200 hover:border-[#ea580c] hover:text-[#ea580c] active:scale-[0.98]"
								href={googleUrl}
							>
								<GoogleIcon />
								<GlitchText className="relative z-[2]" text="Continue with Google" />
							</a>
						</div>

						{/* Divider */}
						<div className="mt-8 flex items-center gap-4">
							<div className="h-px flex-1 bg-[#27272a]" />
							<span className="font-mono text-[#3f3f46] text-[9px] uppercase tracking-widest">or via CLI</span>
							<div className="h-px flex-1 bg-[#27272a]" />
						</div>

						{/* CLI block */}
						<div className="mt-6 overflow-hidden border border-[#27272a] bg-black">
							<div className="flex items-center justify-between border-[#18181b] border-b px-4 py-2">
								<span className="font-mono text-[#3f3f46] text-[9px] uppercase tracking-widest">Terminal</span>
								<span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#10b981] shadow-[0_0_6px_#10b981]" />
							</div>
							<div className="flex items-center gap-3 px-4 py-3 font-mono text-[13px]">
								<span className="text-[#ea580c]">$</span>
								<span className="text-white">forged login</span>
							</div>
						</div>

						<p className="mt-4 text-[#3f3f46] text-[11px] leading-relaxed">
							Run in your terminal to authenticate via CLI. Opens a browser session for OAuth handshake.
						</p>
					</div>
				</div>

				{/* Footer badges */}
				<div className="mt-8 flex items-center justify-center gap-6">
					<span className="font-mono text-[#27272a] text-[9px] uppercase tracking-widest">E2E Encrypted</span>
					<span className="h-1 w-1 bg-[#27272a]" />
					<span className="font-mono text-[#27272a] text-[9px] uppercase tracking-widest">Zero Knowledge</span>
					<span className="h-1 w-1 bg-[#27272a]" />
					<span className="font-mono text-[#27272a] text-[9px] uppercase tracking-widest">Open Source</span>
				</div>
			</div>
		</div>
	);
}

export default function LoginPage() {
	return (
		<Suspense
			fallback={
				<div className="flex min-h-screen items-center justify-center bg-black">
					<div className="h-6 w-6 animate-spin rounded-full border-2 border-[#27272a] border-t-[#ea580c]" />
				</div>
			}
		>
			<LoginContent />
		</Suspense>
	);
}
