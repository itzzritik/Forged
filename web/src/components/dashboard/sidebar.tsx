"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

interface SidebarProps {
  collapsed: boolean;
  user: { name: string; email: string };
}

const LockIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
    <path d="M7 11V7a5 5 0 0 1 10 0v4" />
  </svg>
);

const MonitorIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
    <line x1="8" y1="21" x2="16" y2="21" />
    <line x1="12" y1="17" x2="12" y2="21" />
  </svg>
);

const UserIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
    <circle cx="12" cy="7" r="4" />
  </svg>
);

const LogOutIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
    <polyline points="16 17 21 12 16 7" />
    <line x1="21" y1="12" x2="9" y2="12" />
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
        "flex flex-col h-screen bg-sidebar border-r border-sidebar-border transition-[width] duration-200 overflow-hidden shrink-0",
        collapsed ? "w-14" : "w-[220px]"
      )}
    >
      {/* Logo */}
      <div className={cn("flex items-center gap-3 px-3 py-4 border-b border-sidebar-border", collapsed && "justify-center")}>
        <div className="w-8 h-8 border border-sidebar-border flex items-center justify-center shrink-0">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ea580c" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
          </svg>
        </div>
        {!collapsed && (
          <span className="text-sm font-bold font-mono tracking-widest text-sidebar-foreground">
            FORGED
          </span>
        )}
      </div>

      {/* Nav */}
      <nav className="flex-1 py-4 space-y-1 px-2">
        {nav.map(({ href, label, icon: Icon }) => {
          const active = pathname === href;
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex items-center gap-3 px-2 py-2 text-sm font-mono transition-colors",
                collapsed && "justify-center",
                active
                  ? "border-l-2 border-primary bg-secondary text-sidebar-foreground pl-[6px]"
                  : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent"
              )}
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
      <div className={cn("flex items-center gap-3 px-3 py-4 border-t border-sidebar-border", collapsed && "justify-center")}>
        <div className="w-7 h-7 bg-[#ea580c] flex items-center justify-center text-[11px] font-bold font-mono text-black shrink-0">
          {initial}
        </div>
        {!collapsed && (
          <div className="flex-1 min-w-0">
            <p className="text-xs font-mono text-sidebar-foreground truncate">{user.name}</p>
            <p className="text-xs font-mono text-muted-foreground truncate">{user.email}</p>
          </div>
        )}
        {!collapsed && (
          <a
            href="/api/auth/logout"
            className="text-muted-foreground hover:text-sidebar-foreground transition-colors shrink-0"
            aria-label="Sign out"
          >
            <LogOutIcon />
          </a>
        )}
      </div>
    </aside>
  );
};
