"use client";

import { useEffect, useRef, useState } from "react";
import { Modal } from "@/components/ui/modal";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

interface VaultUnlockProps {
  onUnlock: (password: string) => Promise<void>;
  error: string | null;
  attemptsRemaining: number | null;
  lockedUntil: string | null;
}

const LockIcon = () => (
  <svg
    width="24"
    height="24"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
    <path d="M7 11V7a5 5 0 0 1 10 0v4" />
  </svg>
);

const EyeIcon = () => (
  <svg
    width="14"
    height="14"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
    <circle cx="12" cy="12" r="3" />
  </svg>
);

const EyeOffIcon = () => (
  <svg
    width="14"
    height="14"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
    <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
    <line x1="1" y1="1" x2="23" y2="23" />
  </svg>
);

const SpinnerIcon = () => (
  <svg
    width="14"
    height="14"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className="animate-spin"
  >
    <path d="M21 12a9 9 0 1 1-6.219-8.56" />
  </svg>
);

const minutesUntil = (iso: string): number => {
  const diff = new Date(iso).getTime() - Date.now();
  return Math.max(0, Math.ceil(diff / 60000));
};

export const VaultUnlock = ({
  onUnlock,
  error,
  attemptsRemaining,
  lockedUntil,
}: VaultUnlockProps) => {
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [minutesLeft, setMinutesLeft] = useState<number | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    if (!lockedUntil) {
      setMinutesLeft(null);
      return;
    }

    const update = () => setMinutesLeft(minutesUntil(lockedUntil));
    update();
    const id = setInterval(update, 10000);
    return () => clearInterval(id);
  }, [lockedUntil]);

  const isLockedOut = lockedUntil != null && new Date(lockedUntil) > new Date();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isLoading || isLockedOut) return;
    setIsLoading(true);
    try {
      await onUnlock(password);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Modal
      title="Vault // Unlock"
      closable={false}
      open={true}
      onOpenChange={() => {}}
    >
      <div className="p-6 flex flex-col gap-5">
        {/* Lock icon with orange glow */}
        <div className="flex justify-center">
          <div className="relative flex items-center justify-center">
            <div className="absolute inset-0 rounded-full bg-primary/30 blur-md" />
            <div className="relative text-primary">
              <LockIcon />
            </div>
          </div>
        </div>

        {/* Heading */}
        <div className="text-center space-y-1">
          <p className="text-lg font-semibold">Unlock Vault</p>
          <p className="text-muted-foreground text-sm">
            Enter your master password to decrypt your keys
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="flex flex-col gap-3">
          <div className="relative">
            <Input
              ref={inputRef}
              type={showPassword ? "text" : "password"}
              placeholder="Master password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={isLoading}
              aria-invalid={error != null ? true : undefined}
              className="pr-9"
            />
            <button
              type="button"
              onClick={() => setShowPassword((v) => !v)}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
              aria-label={showPassword ? "Hide password" : "Show password"}
              tabIndex={-1}
            >
              {showPassword ? <EyeOffIcon /> : <EyeIcon />}
            </button>
          </div>

          {/* Error / lockout message */}
          {isLockedOut ? (
            <p className="text-destructive text-xs">
              Too many attempts. Try again in {minutesLeft ?? 1} minute{minutesLeft !== 1 ? "s" : ""}.
            </p>
          ) : error ? (
            <p className="text-destructive text-xs">
              {error}
              {attemptsRemaining != null && (
                <span className="text-muted-foreground">
                  {" "}({attemptsRemaining} attempt{attemptsRemaining !== 1 ? "s" : ""} remaining)
                </span>
              )}
            </p>
          ) : null}

          <Button
            type="submit"
            disabled={isLoading || isLockedOut}
            className="w-full bg-primary text-primary-foreground"
          >
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
      </div>
    </Modal>
  );
};
