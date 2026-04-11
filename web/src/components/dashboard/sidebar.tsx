"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

interface SidebarProps {
	collapsed: boolean;
	user: { name: string; email: string };
}

const LockIcon = () => (
	<svg fill="none" height="16" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="16">
		<rect height="11" rx="2" ry="2" width="18" x="3" y="11" />
		<path d="M7 11V7a5 5 0 0 1 10 0v4" />
	</svg>
);

const MonitorIcon = () => (
	<svg fill="none" height="16" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="16">
		<rect height="14" rx="2" ry="2" width="20" x="2" y="3" />
		<line x1="8" x2="16" y1="21" y2="21" />
		<line x1="12" x2="12" y1="17" y2="21" />
	</svg>
);

const UserIcon = () => (
	<svg fill="none" height="16" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="16">
		<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
		<circle cx="12" cy="7" r="4" />
	</svg>
);

const LogOutIcon = () => (
	<svg fill="none" height="16" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="16">
		<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
		<polyline points="16 17 21 12 16 7" />
		<line x1="21" x2="9" y1="12" y2="12" />
	</svg>
);

const nav = [
	{ href: "/dashboard", label: "SSH Keys", icon: LockIcon },
	{ href: "/dashboard/devices", label: "Devices", icon: MonitorIcon },
	{ href: "/dashboard/account", label: "Account", icon: UserIcon },
];

export const Sidebar = ({ collapsed, user }: SidebarProps) => {
	const pathname = usePathname();
	const initial = user.name ? user.name[0].toUpperCase() : "?";

	return (
		<aside
			className={cn(
				"flex h-screen shrink-0 flex-col overflow-hidden border-sidebar-border border-r bg-sidebar transition-[width] duration-200",
				collapsed ? "w-14" : "w-[220px]"
			)}
		>
			{/* Logo */}
			<div className={cn("flex items-center gap-3 border-sidebar-border border-b px-3 py-4", collapsed && "justify-center")}>
				<div className="flex h-8 w-8 shrink-0 items-center justify-center border border-sidebar-border">
					<svg fill="none" height="16" stroke="#ea580c" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="16">
						<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
					</svg>
				</div>
				{!collapsed && <span className="font-bold font-mono text-sidebar-foreground text-sm tracking-widest">FORGED</span>}
			</div>

			{/* Nav */}
			<nav className="flex-1 space-y-1 px-2 py-4">
				{nav.map(({ href, label, icon: Icon }) => {
					const active = pathname === href;
					return (
						<Link
							className={cn(
								"flex items-center gap-3 px-2 py-2 font-mono text-sm transition-colors",
								collapsed && "justify-center",
								active
									? "border-primary border-l-2 bg-secondary pl-[6px] text-sidebar-foreground"
									: "text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-foreground"
							)}
							href={href}
							key={href}
						>
							<span className="shrink-0">
								<Icon />
							</span>
							{!collapsed && <span>{label}</span>}
						</Link>
					);
				})}
			</nav>

			{/* User */}
			<div className={cn("flex items-center gap-3 border-sidebar-border border-t px-3 py-4", collapsed && "justify-center")}>
				<div className="flex h-7 w-7 shrink-0 items-center justify-center bg-[#ea580c] font-bold font-mono text-[11px] text-black">{initial}</div>
				{!collapsed && (
					<div className="min-w-0 flex-1">
						<p className="truncate font-mono text-sidebar-foreground text-xs">{user.name}</p>
						<p className="truncate font-mono text-muted-foreground text-xs">{user.email}</p>
					</div>
				)}
				{!collapsed && (
					<button
						aria-label="Sign out"
						className="shrink-0 cursor-pointer text-muted-foreground transition-colors hover:text-sidebar-foreground"
						onClick={async () => {
							const { clearSyncKey } = await import("@/lib/vault-store");
							await clearSyncKey();
							window.location.href = "/api/auth/logout";
						}}
					>
						<LogOutIcon />
					</button>
				)}
			</div>
		</aside>
	);
};
