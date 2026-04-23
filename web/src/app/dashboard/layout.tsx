import { redirect } from "next/navigation";
import { DashboardShell } from "@/components/dashboard/shell";
import { getSession, refreshExpired } from "@/lib/auth";

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
	const session = await getSession();
	if (!session || refreshExpired(session)) redirect("/login");
	const user = {
		name: session.user.name,
		email: session.user.email,
	};

	return <DashboardShell user={user}>{children}</DashboardShell>;
}
