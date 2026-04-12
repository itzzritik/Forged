"use client";

import { useEffect, useRef, useState } from "react";
import { motion } from "framer-motion";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { clearSyncKey } from "@/lib/vault-store";

interface VaultUnlockProps {
  attemptsRemaining: number | null;
  error: string | null;
  lockedUntil: string | null;
  onUnlock: (password: string) => Promise<void>;
}

const LockIcon = () => (
  <svg fill="none" height="24" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="24">
    <rect height="11" rx="2" ry="2" width="18" x="3" y="11" />
    <path d="M7 11V7a5 5 0 0 1 10 0v4" />
  </svg>
);

const EyeIcon = () => (
  <svg fill="none" height="14" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="14">
    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
    <circle cx="12" cy="12" r="3" />
  </svg>
);

const EyeOffIcon = () => (
  <svg fill="none" height="14" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="14">
    <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
    <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
    <line x1="1" x2="23" y1="1" y2="23" />
  </svg>
);

const SpinnerIcon = () => (
  <svg className="animate-spin" fill="none" height="14" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="14">
    <path d="M21 12a9 9 0 1 1-6.219-8.56" />
  </svg>
);

const minutesUntil = (iso: string): number => {
  const diff = new Date(iso).getTime() - Date.now();
  return Math.max(0, Math.ceil(diff / 60_000));
};

export const VaultUnlock = ({ onUnlock, error, attemptsRemaining, lockedUntil }: VaultUnlockProps) => {
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [shake, setShake] = useState(false);
  const [minutesLeft, setMinutesLeft] = useState<number | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    if (error) {
      setShake(true);
      const id = setTimeout(() => setShake(false), 600);
      return () => clearTimeout(id);
    }
  }, [error]);

  useEffect(() => {
    if (!lockedUntil) {
      setMinutesLeft(null);
      return;
    }
    const update = () => setMinutesLeft(minutesUntil(lockedUntil));
    update();
    const id = setInterval(update, 10_000);
    return () => clearInterval(id);
  }, [lockedUntil]);

  const isLockedOut = lockedUntil != null && new Date(lockedUntil) > new Date();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isLoading || isLockedOut || !password) return;
    setIsLoading(true);
    try {
      await onUnlock(password);
    } finally {
      setIsLoading(false);
      setPassword("");
    }
  };

  const handleLogout = async () => {
    await clearSyncKey();
    window.location.href = "/api/auth/logout";
  };

  return (
    <motion.div
      animate={shake ? { x: [0, -12, 12, -8, 8, -4, 4, 0] } : { x: 0 }}
      className="fixed top-1/2 left-1/2 z-50 -translate-x-1/2 -translate-y-1/2 overflow-hidden border border-border bg-card shadow-2xl w-full max-w-md font-mono"
      transition={{ duration: 0.5 }}
    >
      <div className="flex h-[38px] items-center border-border border-b bg-[#0a0a0a] px-3.5">
        <div className="flex items-center gap-[7px]">
          <div aria-hidden className="h-[11px] w-[11px] rounded-full border border-[#3f1c20] bg-[#2a1215]" />
          <div aria-hidden className="h-[11px] w-[11px] rounded-full border border-[#3f3615] bg-[#2a2510]" />
          <div aria-hidden className="h-[11px] w-[11px] rounded-full border border-[#1a3f25] bg-[#0f2a18]" />
        </div>
        <div className="flex-1 text-center text-[10px] text-muted-foreground uppercase tracking-[0.1em]">{"Vault // Unlock"}</div>
        <div className="w-[51px]" />
      </div>
        <div className="flex flex-col gap-5 p-6">
          <div className="flex justify-center">
            <div className="relative flex items-center justify-center">
              <div className="absolute inset-0 rounded-full bg-primary/30 blur-md" />
              <div className="relative text-primary">
                <LockIcon />
              </div>
            </div>
          </div>

          <div className="space-y-1 text-center">
            <p className="font-semibold text-lg">Unlock Vault</p>
            <p className="text-muted-foreground text-sm">Enter your master password to decrypt your keys</p>
          </div>

          <form className="flex flex-col gap-3" onSubmit={handleSubmit}>
            <div className="relative">
              <Input
                aria-invalid={error != null}
                className={error ? "border-destructive pr-9" : "pr-9"}
                disabled={isLoading}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Master password"
                ref={inputRef}
                type={showPassword ? "text" : "password"}
                value={password}
              />
              <button
                className="absolute top-1/2 right-2.5 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                onClick={() => setShowPassword((v) => !v)}
                tabIndex={-1}
                type="button"
              >
                {showPassword ? <EyeOffIcon /> : <EyeIcon />}
              </button>
            </div>

            {isLockedOut && (
              <p className="text-destructive text-xs">
                Too many attempts. Try again in {minutesLeft ?? 1} minute{minutesLeft === 1 ? "" : "s"}.
              </p>
            )}
            {!isLockedOut && error && (
              <p className="text-destructive text-xs">
                {error}
                {attemptsRemaining != null && (
                  <span className="text-muted-foreground">
                    {" "}({attemptsRemaining} attempt{attemptsRemaining === 1 ? "" : "s"} remaining)
                  </span>
                )}
              </p>
            )}

            <Button className="w-full bg-primary text-primary-foreground" disabled={isLoading || isLockedOut} type="submit">
              {isLoading ? (
                <>
                  <SpinnerIcon />
                  Deriving key...
                </>
              ) : (
                "Unlock"
              )}
            </Button>
          </form>

          <div className="text-center">
            <button
              className="text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
              onClick={handleLogout}
              type="button"
            >
              Sign out
            </button>
          </div>
        </div>
    </motion.div>
  );
};
