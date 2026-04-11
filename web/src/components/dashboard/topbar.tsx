"use client";

import { usePathname } from "next/navigation";
import { useTheme } from "next-themes";
import { cn } from "@/lib/utils";

interface TopbarProps {
  collapsed: boolean;
  onToggle: () => void;
  onSearchOpen: () => void;
}

const HamburgerIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <line x1="3" y1="6" x2="21" y2="6" />
    <line x1="3" y1="12" x2="21" y2="12" />
    <line x1="3" y1="18" x2="21" y2="18" />
  </svg>
);

const SearchIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="11" cy="11" r="8" />
    <line x1="21" y1="21" x2="16.65" y2="16.65" />
  </svg>
);

const MoonIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
  </svg>
);

const SunIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="5" />
    <line x1="12" y1="1" x2="12" y2="3" />
    <line x1="12" y1="21" x2="12" y2="23" />
    <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
    <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
    <line x1="1" y1="12" x2="3" y2="12" />
    <line x1="21" y1="12" x2="23" y2="12" />
    <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
    <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
  </svg>
);

const SystemIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
    <line x1="8" y1="21" x2="16" y2="21" />
    <line x1="12" y1="17" x2="12" y2="21" />
  </svg>
);

const breadcrumbMap: Record<string, string> = {
  "/dashboard": "Dashboard / SSH Keys",
  "/dashboard/devices": "Dashboard / Devices",
  "/dashboard/account": "Dashboard / Account",
};

const themeOrder = ["dark", "light", "system"] as const;

export const Topbar = ({ collapsed, onToggle, onSearchOpen }: TopbarProps) => {
  const pathname = usePathname();
  const { theme, setTheme } = useTheme();

  const breadcrumb = breadcrumbMap[pathname] ?? "Dashboard";

  const cycleTheme = () => {
    const current = themeOrder.indexOf((theme ?? "dark") as typeof themeOrder[number]);
    const next = themeOrder[(current + 1) % themeOrder.length];
    setTheme(next);
  };

  const ThemeIcon = theme === "light" ? SunIcon : theme === "system" ? SystemIcon : MoonIcon;

  return (
    <header className="h-11 flex items-center px-3 gap-3 bg-sidebar border-b border-sidebar-border shrink-0">
      {/* Left */}
      <button
        onClick={onToggle}
        className="text-muted-foreground hover:text-sidebar-foreground transition-colors p-1 -ml-1"
        aria-label="Toggle sidebar"
      >
        <HamburgerIcon />
      </button>

      <span className="text-xs font-mono text-muted-foreground select-none">
        {breadcrumb}
      </span>

      {/* Right */}
      <div className="ml-auto flex items-center gap-2">
        <div
          role="button"
          onClick={onSearchOpen}
          className={cn(
            "flex items-center gap-2 h-7 px-2 border border-sidebar-border bg-background",
            "text-xs font-mono text-muted-foreground cursor-pointer",
            "hover:border-muted-foreground transition-colors select-none"
          )}
        >
          <SearchIcon />
          <span>Search...</span>
          <kbd className="ml-1 px-1 py-0.5 text-[10px] border border-sidebar-border bg-sidebar font-mono leading-none">
            ⌘K
          </kbd>
        </div>

        <button
          onClick={cycleTheme}
          className="text-muted-foreground hover:text-sidebar-foreground transition-colors p-1"
          aria-label="Toggle theme"
        >
          <ThemeIcon />
        </button>
      </div>
    </header>
  );
};
