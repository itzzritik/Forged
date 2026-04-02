"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

function GitHubIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
    </svg>
  );
}

function GoogleIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24">
      <path
        d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"
        fill="#4285F4"
      />
      <path
        d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
        fill="#34A853"
      />
      <path
        d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
        fill="#FBBC05"
      />
      <path
        d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
        fill="#EA4335"
      />
    </svg>
  );
}

function ForgedMark() {
  return (
    <div className="relative w-12 h-12 flex items-center justify-center">
      <div
        className="absolute inset-0 rounded-xl bg-gradient-to-br from-amber-500/20 to-orange-600/10 blur-xl"
        style={{ animation: "glow-pulse 3s ease-in-out infinite" }}
      />
      <div className="relative w-12 h-12 rounded-xl bg-gradient-to-br from-amber-500/10 to-transparent border border-amber-500/20 flex items-center justify-center">
        <svg
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="text-amber-400"
        >
          <path d="M15 3h6v6" />
          <path d="M10 14L21 3" />
          <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
        </svg>
      </div>
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
    <div className="flex-1 flex items-center justify-center px-6 relative overflow-hidden">
      {/* Background texture */}
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_top,_rgba(245,158,11,0.03)_0%,_transparent_50%)]" />
      <div
        className="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-[1px]"
        style={{
          background:
            "linear-gradient(90deg, transparent, rgba(245,158,11,0.15), transparent)",
        }}
      />

      <div
        className="relative w-full max-w-[360px]"
        style={{ animation: "fade-in 0.6s ease-out" }}
      >
        {/* Logo */}
        <div className="flex flex-col items-center mb-10">
          <ForgedMark />
          <h1
            className="mt-5 text-xl font-medium tracking-tight text-foreground"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            forged
          </h1>
          <p className="mt-2 text-sm text-muted">
            Sign in to sync your SSH keys
          </p>
        </div>

        {/* Error */}
        {error && (
          <div className="mb-6 p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm text-center">
            {decodeURIComponent(error)}
          </div>
        )}

        {/* OAuth buttons */}
        <div className="space-y-3">
          <a
            href={githubUrl}
            className="group flex items-center justify-center gap-3 w-full h-12 rounded-lg bg-white text-zinc-900 text-sm font-medium transition-all duration-200 hover:bg-zinc-100 hover:scale-[1.01] active:scale-[0.99]"
          >
            <GitHubIcon />
            Continue with GitHub
          </a>

          <a
            href={googleUrl}
            className="group flex items-center justify-center gap-3 w-full h-12 rounded-lg bg-surface border border-border text-foreground text-sm font-medium transition-all duration-200 hover:bg-surface-hover hover:border-border-hover hover:scale-[1.01] active:scale-[0.99]"
          >
            <GoogleIcon />
            Continue with Google
          </a>
        </div>

        {/* Divider */}
        <div className="mt-8 flex items-center gap-3">
          <div className="flex-1 h-px bg-border" />
          <span
            className="text-xs text-muted uppercase tracking-widest"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            or
          </span>
          <div className="flex-1 h-px bg-border" />
        </div>

        {/* CLI hint */}
        <div className="mt-6 p-4 rounded-lg bg-surface border border-border">
          <p className="text-xs text-muted mb-2">
            Opened from the CLI? Sign in above and you&apos;ll be redirected
            back automatically.
          </p>
          <div
            className="flex items-center gap-2 text-xs text-muted"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            <span className="text-accent-dim">$</span>
            <span className="text-zinc-400">forged login</span>
          </div>
        </div>

        {/* Footer */}
        <p className="mt-8 text-center text-xs text-muted">
          By signing in, you agree to our{" "}
          <a href="/terms" className="text-zinc-400 hover:text-foreground transition-colors">
            Terms
          </a>{" "}
          and{" "}
          <a href="/privacy" className="text-zinc-400 hover:text-foreground transition-colors">
            Privacy Policy
          </a>
        </p>

        {/* Security note */}
        <div className="mt-4 flex items-center justify-center gap-1.5 text-xs text-muted">
          <svg
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="text-accent-dim"
          >
            <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
            <path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
          Zero-knowledge encryption. We never see your keys.
        </div>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <div className="flex-1 flex items-center justify-center">
          <div className="w-6 h-6 border-2 border-accent/30 border-t-accent rounded-full animate-spin" />
        </div>
      }
    >
      <LoginContent />
    </Suspense>
  );
}
