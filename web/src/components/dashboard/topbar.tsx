"use client";

import { usePathname } from "next/navigation";
import { useTheme } from "next-themes";
import { cn } from "@/lib/utils";

interface TopbarProps {
	collapsed: boolean;
	onSearchOpen: () => void;
	onToggle: () => void;
}

const HamburgerIcon = () => (
	<svg fill="none" height="16" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="16">
		<line x1="3" x2="21" y1="6" y2="6" />
		<line x1="3" x2="21" y1="12" y2="12" />
		<line x1="3" x2="21" y1="18" y2="18" />
	</svg>
);

const SearchIcon = () => (
	<svg fill="none" height="14" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="14">
		<circle cx="11" cy="11" r="8" />
		<line x1="21" x2="16.65" y1="21" y2="16.65" />
	</svg>
);

const MoonIcon = () => (
	<svg fill="none" height="15" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="15">
		<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
	</svg>
);

const SunIcon = () => (
	<svg fill="none" height="15" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="15">
		<circle cx="12" cy="12" r="5" />
		<line x1="12" x2="12" y1="1" y2="3" />
		<line x1="12" x2="12" y1="21" y2="23" />
		<line x1="4.22" x2="5.64" y1="4.22" y2="5.64" />
		<line x1="18.36" x2="19.78" y1="18.36" y2="19.78" />
		<line x1="1" x2="3" y1="12" y2="12" />
		<line x1="21" x2="23" y1="12" y2="12" />
		<line x1="4.22" x2="5.64" y1="19.78" y2="18.36" />
		<line x1="18.36" x2="19.78" y1="5.64" y2="4.22" />
	</svg>
);

const SystemIcon = () => (
	<svg fill="none" height="15" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" viewBox="0 0 24 24" width="15">
		<rect height="14" rx="2" ry="2" width="20" x="2" y="3" />
		<line x1="8" x2="16" y1="21" y2="21" />
		<line x1="12" x2="12" y1="17" y2="21" />
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
		const current = themeOrder.indexOf((theme ?? "dark") as (typeof themeOrder)[number]);
		const next = themeOrder[(current + 1) % themeOrder.length];
		setTheme(next);
	};

	const ThemeIcon = theme === "light" ? SunIcon : theme === "system" ? SystemIcon : MoonIcon;

	return (
		<header className="flex h-11 shrink-0 items-center gap-3 border-sidebar-border border-b bg-sidebar px-3">
			{/* Left */}
			<button aria-label="Toggle sidebar" className="-ml-1 p-1 text-muted-foreground transition-colors hover:text-sidebar-foreground" onClick={onToggle}>
				<HamburgerIcon />
			</button>

			<span className="select-none font-mono text-muted-foreground text-xs">{breadcrumb}</span>

			{/* Right */}
			<div className="ml-auto flex items-center gap-2">
				<div
					className={cn(
						"flex h-7 items-center gap-2 border border-sidebar-border bg-background px-2",
						"cursor-pointer font-mono text-muted-foreground text-xs",
						"select-none transition-colors hover:border-muted-foreground"
					)}
					onClick={onSearchOpen}
					role="button"
				>
					<SearchIcon />
					<span className="hidden sm:inline">Search...</span>
					<kbd className="ml-1 hidden border border-sidebar-border bg-sidebar px-1 py-0.5 font-mono text-[10px] leading-none sm:inline">⌘K</kbd>
				</div>

				<button aria-label="Toggle theme" className="p-1 text-muted-foreground transition-colors hover:text-sidebar-foreground" onClick={cycleTheme}>
					<ThemeIcon />
				</button>
			</div>
		</header>
	);
};
