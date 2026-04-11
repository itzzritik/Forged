import { getSession, parseJWTPayload } from "@/lib/auth";

export default async function DashboardPage() {
  const token = await getSession();
  const payload = token ? parseJWTPayload(token) : null;
  const email = (payload?.email || payload?.sub || "") as string;

  return (
    <div className="min-h-screen bg-black text-white flex flex-col items-center justify-center px-6">
      <div className="w-full max-w-md space-y-8">
        <div className="text-center space-y-2">
          <h1 className="text-2xl font-bold tracking-tight font-mono">
            Dashboard
          </h1>
          {email && (
            <p className="text-sm text-[#a1a1aa] font-mono">{email}</p>
          )}
        </div>

        <div className="border border-[#27272a] bg-[#050505] p-6 space-y-4">
          <p className="text-sm text-[#a1a1aa] font-mono">
            Authenticated. Dashboard features coming soon.
          </p>
        </div>

        <div className="text-center">
          <a
            href="/api/auth/logout"
            className="text-xs text-[#a1a1aa] hover:text-white font-mono tracking-wider uppercase transition-colors"
          >
            Sign out
          </a>
        </div>
      </div>
    </div>
  );
}
