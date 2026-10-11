import { redirect } from "next/navigation";
import { Shell } from "@/components/shell/shell";
import { getSession, refreshExpired } from "@/lib/auth";

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
	const session = await getSession();
	if (!session || refreshExpired(session)) redirect("/login");
	return <Shell email={session.user.email}>{children}</Shell>;
}
