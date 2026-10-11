import { CircleUserIcon, HashIcon, type LucideIcon, MonitorSmartphoneIcon } from "lucide-react";

interface Page {
	href: string;
	icon: LucideIcon;
	key: "1" | "2" | "3";
	keywords: string[];
	label: string;
}

// Tabs, the palette and the 1 2 3 shortcuts all read from this list.
export const PAGES: Page[] = [
	{ href: "/dashboard", icon: HashIcon, key: "1", keywords: ["keys", "ssh", "home"], label: "Keys" },
	{ href: "/dashboard/devices", icon: MonitorSmartphoneIcon, key: "2", keywords: ["devices", "machines", "computers", "cli"], label: "Devices" },
	{ href: "/dashboard/account", icon: CircleUserIcon, key: "3", keywords: ["account", "password", "profile", "settings"], label: "Account" },
];

export const pageIndex = (pathname: string) =>
	Math.max(
		0,
		PAGES.findLastIndex((page) => pathname === page.href || pathname.startsWith(`${page.href}/`))
	);
