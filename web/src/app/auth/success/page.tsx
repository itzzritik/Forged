import Link from "next/link";
import { redirect } from "next/navigation";
import { getSession, parseJWTPayload } from "@/lib/auth";

export default async function AuthSuccessPage({ searchParams }: { searchParams: Promise<{ error?: string }> }) {
	const { error } = await searchParams;

	if (error) {
		return (
			<div className="flex min-h-screen flex-col items-center justify-center bg-black px-6">
				<div className="w-full max-w-[440px] text-center">
					<div className="relative mb-8 inline-block">
						<div className="absolute inset-0 scale-150 bg-red-500/15 blur-[24px]" />
						<div className="relative flex h-14 w-14 items-center justify-center border border-[#27272a] bg-black">
							<svg
								aria-label="Error"
								fill="none"
								height="24"
								role="img"
								stroke="#ef4444"
								strokeLinecap="round"
								strokeLinejoin="round"
								strokeWidth="1.5"
								viewBox="0 0 24 24"
								width="24"
							>
								<circle cx="12" cy="12" r="10" />
								<line x1="15" x2="9" y1="9" y2="15" />
								<line x1="9" x2="15" y1="9" y2="15" />
							</svg>
						</div>
					</div>
					<h1 className="mb-3 font-bold text-2xl text-white tracking-tight">Authentication Failed</h1>
					<p className="mb-8 font-mono text-[#a1a1aa] text-sm">{decodeURIComponent(error)}</p>
					<Link className="font-mono text-[#a1a1aa] text-xs uppercase tracking-wider transition-colors hover:text-white" href="/login">
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
		<div className="flex min-h-screen flex-col items-center justify-center bg-black px-6">
			<div className="w-full max-w-[440px]">
				<div className="mb-8 flex flex-col items-center">
					<div className="relative">
						<div className="absolute inset-0 scale-150 bg-[#ea580c]/15 blur-[24px]" />
						<div className="relative flex h-14 w-14 items-center justify-center border border-[#27272a] bg-black">
							<svg
								aria-label="Success"
								fill="none"
								height="24"
								role="img"
								stroke="white"
								strokeLinecap="round"
								strokeLinejoin="round"
								strokeWidth="1.5"
								viewBox="0 0 24 24"
								width="24"
							>
								<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
								<polyline points="22 4 12 14.01 9 11.01" />
							</svg>
						</div>
					</div>
					<h1 className="mt-8 font-bold text-2xl text-white tracking-tight">CLI Authenticated</h1>
					<p className="mt-2 font-mono text-[#a1a1aa] text-sm">{name || email}</p>
				</div>

				<div className="overflow-hidden border border-[#27272a] bg-[#050505]">
					<div className="flex h-10 items-center justify-between border-[#27272a] border-b bg-[#030303] px-6">
						<div className="flex items-center gap-3">
							<span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[#10b981] shadow-[0_0_8px_#10b981]" />
							<span className="font-mono text-[#a1a1aa] text-[10px] uppercase tracking-widest">Session {/* // */} Authenticated</span>
						</div>
						<span className="font-mono text-[#3f3f46] text-[9px] uppercase tracking-widest">CLI</span>
					</div>
					<div className="p-6 text-center">
						<p className="font-mono text-[#a1a1aa] text-sm">You can close this tab and return to your terminal.</p>
					</div>
				</div>

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
