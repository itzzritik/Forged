"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import Link from "next/link";
import { GlitchText, AnimatedTerminalGrid, TERMINAL_CARDS } from "@/components/client";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

function GitHubIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
    </svg>
  );
}

function GoogleIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24">
      <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="currentColor" />
      <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="currentColor" />
      <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="currentColor" />
      <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="currentColor" />
    </svg>
  );
}

function BackgroundGrid() {
  return (
    <div className="absolute inset-0 pointer-events-none select-none overflow-hidden bg-black">
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
  const callback = searchParams.get("callback") || "";
  const error = searchParams.get("error");

  const githubUrl = `${API_URL}/api/v1/auth/github?callback=${encodeURIComponent(callback)}`;
  const googleUrl = `${API_URL}/api/v1/auth/google?callback=${encodeURIComponent(callback)}`;

  return (
    <div className="min-h-screen flex flex-col items-center justify-center px-6 relative overflow-hidden bg-black">
      <BackgroundGrid />

      {/* Top nav bar */}
      <nav className="fixed top-0 left-0 right-0 z-50 border-b border-[#27272a] bg-black/80 backdrop-blur-xl">
        <div className="w-full px-6 lg:px-16 h-14 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-3 group">
            <div className="w-7 h-7 bg-black border border-[#27272a] flex items-center justify-center group-hover:border-[#ea580c] transition-colors">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-white group-hover:text-[#ea580c] transition-colors">
                <path d="M15 3h6v6" />
                <path d="M10 14L21 3" />
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
              </svg>
            </div>
            <span className="text-[13px] font-bold tracking-[0.2em] text-white uppercase font-mono group-hover:text-[#ea580c] transition-colors">
              Forged
            </span>
          </Link>
          <Link href="/" className="text-[12px] tracking-wider text-[#a1a1aa] hover:text-white transition-colors uppercase flex items-center gap-2">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="19" y1="12" x2="5" y2="12" />
              <polyline points="12 19 5 12 12 5" />
            </svg>
            Back
          </Link>
        </div>
      </nav>

      {/* Main content */}
      <div className="relative w-full max-w-[440px] z-10 animate-slide-up">
        {/* Header */}
        <div className="flex flex-col items-center mb-10">
          <div className="relative">
            <div className="absolute inset-0 bg-[#ea580c]/15 blur-[24px] scale-150" />
            <div className="relative w-14 h-14 bg-black border border-[#27272a] flex items-center justify-center">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-white">
                <path d="M15 3h6v6" />
                <path d="M10 14L21 3" />
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
              </svg>
            </div>
          </div>
          <h1 className="mt-8 text-3xl sm:text-4xl font-bold tracking-tighter text-white">
            Sign in
          </h1>
          <p className="mt-3 text-sm text-[#a1a1aa] tracking-wide">
            Authenticate to begin your session
          </p>
        </div>

        {/* Auth card */}
        <div className="border border-[#27272a] bg-[#050505] overflow-hidden">
          {/* Card header */}
          <div className="border-b border-[#27272a] bg-[#030303] px-6 h-10 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="w-1.5 h-1.5 rounded-full bg-[#ea580c] animate-pulse shadow-[0_0_8px_#ea580c]" />
              <span className="text-[10px] font-mono tracking-widest text-[#a1a1aa] uppercase">Session // Auth</span>
            </div>
            <span className="text-[9px] font-mono tracking-widest text-[#3f3f46] uppercase">OAuth 2.0</span>
          </div>

          <div className="p-6 sm:p-8">
            {error && (
              <div className="mb-6 p-4 bg-red-950/20 border border-red-500/20 text-red-400 text-[12px] font-mono flex items-center gap-3">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="shrink-0">
                  <circle cx="12" cy="12" r="10" />
                  <line x1="15" y1="9" x2="9" y2="15" />
                  <line x1="9" y1="9" x2="15" y2="15" />
                </svg>
                {decodeURIComponent(error)}
              </div>
            )}

            <div className="space-y-3">
              <a
                href={githubUrl}
                className="group relative flex items-center justify-center gap-3 w-full h-12 bg-white text-black text-[12px] font-bold font-mono tracking-widest uppercase transition-all duration-200 hover:bg-zinc-100 active:scale-[0.98] overflow-hidden"
              >
                <GitHubIcon />
                <GlitchText text="Continue with GitHub" className="relative z-[2]" />
              </a>

              <a
                href={googleUrl}
                className="group relative flex items-center justify-center gap-3 w-full h-12 bg-black border border-[#27272a] text-white text-[12px] font-bold font-mono tracking-widest uppercase transition-all duration-200 hover:border-[#ea580c] hover:text-[#ea580c] active:scale-[0.98] overflow-hidden"
              >
                <GoogleIcon />
                <GlitchText text="Continue with Google" className="relative z-[2]" />
              </a>
            </div>

            {/* Divider */}
            <div className="mt-8 flex items-center gap-4">
              <div className="flex-1 h-px bg-[#27272a]" />
              <span className="text-[9px] text-[#3f3f46] uppercase tracking-widest font-mono">
                or via CLI
              </span>
              <div className="flex-1 h-px bg-[#27272a]" />
            </div>

            {/* CLI block */}
            <div className="mt-6 border border-[#27272a] bg-black overflow-hidden">
              <div className="border-b border-[#18181b] px-4 py-2 flex items-center justify-between">
                <span className="text-[9px] font-mono tracking-widest text-[#3f3f46] uppercase">Terminal</span>
                <span className="w-1.5 h-1.5 rounded-full bg-[#10b981] animate-pulse shadow-[0_0_6px_#10b981]" />
              </div>
              <div className="px-4 py-3 flex items-center gap-3 font-mono text-[13px]">
                <span className="text-[#ea580c]">$</span>
                <span className="text-white">forged login</span>
              </div>
            </div>

            <p className="mt-4 text-[11px] text-[#3f3f46] leading-relaxed">
              Run in your terminal to authenticate via CLI. Opens a browser session for OAuth handshake.
            </p>
          </div>
        </div>

        {/* Footer badges */}
        <div className="mt-8 flex items-center justify-center gap-6">
          <span className="text-[9px] font-mono tracking-widest text-[#27272a] uppercase">E2E Encrypted</span>
          <span className="w-1 h-1 bg-[#27272a]" />
          <span className="text-[9px] font-mono tracking-widest text-[#27272a] uppercase">Zero Knowledge</span>
          <span className="w-1 h-1 bg-[#27272a]" />
          <span className="text-[9px] font-mono tracking-widest text-[#27272a] uppercase">Open Source</span>
        </div>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen flex items-center justify-center bg-black">
          <div className="w-6 h-6 border-2 border-[#27272a] border-t-[#ea580c] rounded-full animate-spin" />
        </div>
      }
    >
      <LoginContent />
    </Suspense>
  );
}
