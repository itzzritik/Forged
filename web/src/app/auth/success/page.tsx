import { getSession, parseJWTPayload } from "@/lib/auth";
import { redirect } from "next/navigation";
import Link from "next/link";

export default async function AuthSuccessPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const { error } = await searchParams;

  if (error) {
    return (
      <div className="min-h-screen bg-black flex flex-col items-center justify-center px-6">
        <div className="w-full max-w-[440px] text-center">
          <div className="relative inline-block mb-8">
            <div className="absolute inset-0 bg-red-500/15 blur-[24px] scale-150" />
            <div className="relative w-14 h-14 bg-black border border-[#27272a] flex items-center justify-center">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#ef4444" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10" />
                <line x1="15" y1="9" x2="9" y2="15" />
                <line x1="9" y1="9" x2="15" y2="15" />
              </svg>
            </div>
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-white mb-3">Authentication Failed</h1>
          <p className="text-sm text-[#a1a1aa] font-mono mb-8">{decodeURIComponent(error)}</p>
          <Link href="/login" className="text-xs text-[#a1a1aa] hover:text-white font-mono tracking-wider uppercase transition-colors">
            Try again
          </Link>
        </div>
      </div>
    );
  }

  const token = await getSession();
  if (!token) redirect("/login");

  const payload = parseJWTPayload(token);
  const name = (payload?.name || "") as string;
  const email = (payload?.email || payload?.sub || "") as string;

  return (
    <div className="min-h-screen bg-black flex flex-col items-center justify-center px-6">
      <div className="w-full max-w-[440px]">
        <div className="flex flex-col items-center mb-8">
          <div className="relative">
            <div className="absolute inset-0 bg-[#ea580c]/15 blur-[24px] scale-150" />
            <div className="relative w-14 h-14 bg-black border border-[#27272a] flex items-center justify-center">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
                <polyline points="22 4 12 14.01 9 11.01" />
              </svg>
            </div>
          </div>
          <h1 className="mt-8 text-2xl font-bold tracking-tight text-white">CLI Authenticated</h1>
          <p className="mt-2 text-sm text-[#a1a1aa] font-mono">{name || email}</p>
        </div>

        <div className="border border-[#27272a] bg-[#050505] overflow-hidden">
          <div className="border-b border-[#27272a] bg-[#030303] px-6 h-10 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="w-1.5 h-1.5 rounded-full bg-[#10b981] animate-pulse shadow-[0_0_8px_#10b981]" />
              <span className="text-[10px] font-mono tracking-widest text-[#a1a1aa] uppercase">Session // Authenticated</span>
            </div>
            <span className="text-[9px] font-mono tracking-widest text-[#3f3f46] uppercase">CLI</span>
          </div>
          <div className="p-6 text-center">
            <p className="text-sm text-[#a1a1aa] font-mono">
              You can close this tab and return to your terminal.
            </p>
          </div>
        </div>

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
