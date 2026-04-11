"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { GlitchButton } from "@/components/client";

export function AuthNavButton() {
  const [loggedIn, setLoggedIn] = useState(false);

  useEffect(() => {
    setLoggedIn(document.cookie.includes("forged_logged_in=1"));
  }, []);

  if (loggedIn) {
    return (
      <Link href="/dashboard" className="flex items-center gap-2 group">
        <div className="w-7 h-7 bg-[#ea580c] flex items-center justify-center text-[11px] font-bold font-mono text-black">
          F
        </div>
      </Link>
    );
  }

  return (
    <GlitchButton href="/login" className="px-5 h-8 text-[12px]">Sign in</GlitchButton>
  );
}

export function AuthCTAButton() {
  const [loggedIn, setLoggedIn] = useState(false);

  useEffect(() => {
    setLoggedIn(document.cookie.includes("forged_logged_in=1"));
  }, []);

  return (
    <GlitchButton
      href={loggedIn ? "/dashboard" : "/login"}
      className="h-14 px-12 text-sm max-w-full"
    >
      {loggedIn ? "Dashboard" : "Create Account"}
    </GlitchButton>
  );
}
