"use client";

import { motion } from "framer-motion";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { CommandPalette } from "@/components/dashboard/command-palette";
import { Sidebar } from "@/components/dashboard/sidebar";
import { Topbar } from "@/components/dashboard/topbar";
import { VaultUnlock } from "@/components/dashboard/vault-unlock";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { useCommandPalette } from "@/hooks/use-command-palette";
import { useSidebar } from "@/hooks/use-sidebar";
import { useVault, VaultContext } from "@/hooks/use-vault";

interface DashboardShellProps {
	children: React.ReactNode;
	user: { name: string; email: string };
}

export const DashboardShell = ({ user, children }: DashboardShellProps) => {
	const vault = useVault();
	const sidebar = useSidebar();
	const commandPalette = useCommandPalette();
	const [mobileOpen, setMobileOpen] = useState(false);
	const pathname = usePathname();
	const prefersTableLoading = pathname === "/dashboard" || pathname === "/dashboard/devices";

	return (
		<VaultContext.Provider value={vault}>
			<div className="flex h-screen bg-background">
				{/* Desktop sidebar */}
				<div className="hidden md:block">
					<Sidebar collapsed={sidebar.collapsed} user={user} />
				</div>

				{/* Mobile sidebar sheet */}
				<Sheet onOpenChange={setMobileOpen} open={mobileOpen}>
					<SheetContent className="w-55 p-0" showCloseButton={false} side="left">
						<Sidebar collapsed={false} user={user} />
					</SheetContent>
				</Sheet>

				<div className="flex min-w-0 flex-1 flex-col">
					<Topbar
						collapsed={sidebar.collapsed}
						onSearchOpen={commandPalette.setOpen.bind(null, true)}
						onToggle={() => {
							if (typeof window !== "undefined" && window.innerWidth < 768) {
								setMobileOpen((v) => !v);
							} else {
								sidebar.toggle();
							}
						}}
					/>
					<main className="flex-1 overflow-auto">
						{vault.status === "no-vault" && <NoVault />}
						{vault.status === "error" && <VaultError message={vault.error} />}
						{(vault.status === "unlocked" || (vault.status === "loading" && prefersTableLoading)) && (
							<motion.div animate={{ opacity: 1 }} initial={{ opacity: 0 }} transition={{ duration: 0.3 }}>
								{children}
							</motion.div>
						)}
						{vault.status === "loading" && !prefersTableLoading && <LoadingSkeleton />}
					</main>
				</div>
				{vault.status === "locked" && <VaultUnlock error={vault.error} onUnlock={vault.unlock} />}
				<CommandPalette keys={vault.vaultData?.keys ?? []} onOpenChange={commandPalette.setOpen} open={commandPalette.open} />
			</div>
		</VaultContext.Provider>
	);
};

const LoadingSkeleton = () => (
	<div className="space-y-4 p-6">
		<Skeleton className="h-8 w-48" />
		<Skeleton className="h-4 w-full max-w-md" />
		<Skeleton className="h-4 w-full max-w-sm" />
		<div className="space-y-3 pt-4">
			<Skeleton className="h-12 w-full" />
			<Skeleton className="h-12 w-full" />
			<Skeleton className="h-12 w-full" />
		</div>
	</div>
);

const NoVault = () => (
	<div className="flex h-full flex-col items-center justify-center gap-3 px-6 text-center">
		<p className="font-mono text-foreground text-sm">No vault synced yet.</p>
		<p className="font-mono text-muted-foreground text-xs">
			Run <code className="text-primary">forged sync</code> from your CLI to push your vault to the cloud.
		</p>
	</div>
);

const VaultError = ({ message }: { message: string | null }) => (
	<div className="flex h-full flex-col items-center justify-center gap-4 px-6 text-center">
		<p className="font-mono text-destructive text-sm">{message || "An error occurred loading your vault."}</p>
		<button
			className="font-mono text-muted-foreground text-xs underline underline-offset-4 transition-colors hover:text-foreground"
			onClick={() => window.location.reload()}
		>
			Retry
		</button>
	</div>
);
