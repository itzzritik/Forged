import { redirect } from "next/navigation";
import { DashboardShell } from "@/components/dashboard/shell";
import { getSession, parseJWTPayload } from "@/lib/auth";

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
	const token = await getSession();
	if (!token) redirect("/login");

	const payload = parseJWTPayload(token);
	const user = {
		name: (payload?.name || "") as string,
		email: (payload?.email || payload?.sub || "") as string,
	};

	return <DashboardShell user={user}>{children}</DashboardShell>;
}
