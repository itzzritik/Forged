"use client";

import { Sidebar } from "@/components/dashboard/sidebar";
import { Topbar } from "@/components/dashboard/topbar";
import { VaultUnlock } from "@/components/dashboard/vault-unlock";
import { CommandPalette } from "@/components/dashboard/command-palette";
import { Skeleton } from "@/components/ui/skeleton";
import { useVault } from "@/hooks/use-vault";
import { useSidebar } from "@/hooks/use-sidebar";
import { useCommandPalette } from "@/hooks/use-command-palette";

interface DashboardShellProps {
  user: { name: string; email: string };
  children: React.ReactNode;
}

export const DashboardShell = ({ user, children }: DashboardShellProps) => {
  const vault = useVault();
  const sidebar = useSidebar();
  const commandPalette = useCommandPalette();

  return (
    <div className="flex h-screen bg-background">
      <Sidebar user={user} collapsed={sidebar.collapsed} />
      <div className="flex-1 flex flex-col min-w-0">
        <Topbar
          collapsed={sidebar.collapsed}
          onToggle={sidebar.toggle}
          onSearchOpen={commandPalette.setOpen.bind(null, true)}
        />
        <main className="flex-1 overflow-auto">
          {vault.status === "loading" && <LoadingSkeleton />}
          {vault.status === "no-vault" && <NoVault />}
          {vault.status === "error" && <VaultError message={vault.error} />}
          {vault.status === "unlocked" && children}
        </main>
      </div>
      {vault.status === "locked" && (
        <VaultUnlock
          onUnlock={vault.unlock}
          error={vault.error}
          attemptsRemaining={vault.attemptsRemaining}
          lockedUntil={vault.lockedUntil}
        />
      )}
      <CommandPalette
        open={commandPalette.open}
        onOpenChange={commandPalette.setOpen}
        keys={vault.vaultData?.keys ?? []}
      />
    </div>
  );
};

const LoadingSkeleton = () => (
  <div className="p-6 space-y-4">
    <Skeleton className="h-8 w-48" />
    <Skeleton className="h-4 w-full max-w-md" />
    <Skeleton className="h-4 w-full max-w-sm" />
    <div className="pt-4 space-y-3">
      <Skeleton className="h-12 w-full" />
      <Skeleton className="h-12 w-full" />
      <Skeleton className="h-12 w-full" />
    </div>
  </div>
);

const NoVault = () => (
  <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-6">
    <p className="text-sm font-mono text-foreground">No vault synced yet.</p>
    <p className="text-xs font-mono text-muted-foreground">
      Run <code className="text-primary">forged sync</code> from your CLI to push your vault to the cloud.
    </p>
  </div>
);

const VaultError = ({ message }: { message: string | null }) => (
  <div className="flex flex-col items-center justify-center h-full gap-4 text-center px-6">
    <p className="text-sm font-mono text-destructive">
      {message || "An error occurred loading your vault."}
    </p>
    <button
      onClick={() => window.location.reload()}
      className="text-xs font-mono text-muted-foreground hover:text-foreground transition-colors underline underline-offset-4"
    >
      Retry
    </button>
  </div>
);
