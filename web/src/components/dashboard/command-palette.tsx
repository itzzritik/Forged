"use client";

import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { toast } from "sonner";
import { CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator, CommandShortcut } from "@/components/ui/command";
import type { VaultKeyMetadata } from "@/lib/vault-crypto";

interface CommandPaletteProps {
	keys: VaultKeyMetadata[];
	onOpenChange: (open: boolean) => void;
	open: boolean;
}

const KeyIcon = () => (
	<svg
		aria-label="Key"
		fill="none"
		height="14"
		role="img"
		stroke="currentColor"
		strokeLinecap="round"
		strokeLinejoin="round"
		strokeWidth="2"
		viewBox="0 0 24 24"
		width="14"
	>
		<circle cx="7.5" cy="15.5" r="5.5" />
		<path d="M21 2l-9.6 9.6" />
		<path d="M15.5 7.5l3 3L22 7l-3-3" />
	</svg>
);

const NavIcon = () => (
	<svg
		aria-label="Navigate"
		fill="none"
		height="14"
		role="img"
		stroke="currentColor"
		strokeLinecap="round"
		strokeLinejoin="round"
		strokeWidth="2"
		viewBox="0 0 24 24"
		width="14"
	>
		<polyline points="9 18 15 12 9 6" />
	</svg>
);

const ThemeIcon = () => (
	<svg
		aria-label="Theme"
		fill="none"
		height="14"
		role="img"
		stroke="currentColor"
		strokeLinecap="round"
		strokeLinejoin="round"
		strokeWidth="2"
		viewBox="0 0 24 24"
		width="14"
	>
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

const LockIcon = () => (
	<svg
		aria-label="Lock"
		fill="none"
		height="14"
		role="img"
		stroke="currentColor"
		strokeLinecap="round"
		strokeLinejoin="round"
		strokeWidth="2"
		viewBox="0 0 24 24"
		width="14"
	>
		<rect height="11" rx="2" ry="2" width="18" x="3" y="11" />
		<path d="M7 11V7a5 5 0 0 1 10 0v4" />
	</svg>
);

const CopyIcon = () => (
	<svg
		aria-label="Copy"
		fill="none"
		height="14"
		role="img"
		stroke="currentColor"
		strokeLinecap="round"
		strokeLinejoin="round"
		strokeWidth="2"
		viewBox="0 0 24 24"
		width="14"
	>
		<rect height="13" rx="2" ry="2" width="13" x="9" y="9" />
		<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
	</svg>
);

const navItems = [
	{ label: "SSH Keys", href: "/dashboard", shortcut: "G K" },
	{ label: "Devices", href: "/dashboard/devices", shortcut: "G D" },
	{ label: "Account", href: "/dashboard/account", shortcut: "G A" },
];

export const CommandPalette = ({ open, onOpenChange, keys }: CommandPaletteProps) => {
	const router = useRouter();
	const { theme, setTheme } = useTheme();

	const run = (fn: () => void) => {
		onOpenChange(false);
		fn();
	};

	const copyPublicKey = async (key: VaultKeyMetadata) => {
		await navigator.clipboard.writeText(key.publicKey);
		toast.success(`Copied public key for "${key.name}"`);
	};

	const toggleTheme = () => {
		let next: string;
		if (theme === "dark") next = "light";
		else if (theme === "light") next = "system";
		else next = "dark";
		setTheme(next);
		toast.success(`Theme set to ${next}`);
	};

	return (
		<CommandDialog className="max-w-[calc(100vw-2rem)] sm:max-w-lg" onOpenChange={onOpenChange} open={open}>
			<CommandInput placeholder="Search keys, actions, navigation..." />
			<CommandList>
				<CommandEmpty>No results found.</CommandEmpty>

				{keys.length > 0 && (
					<>
						<CommandGroup heading="SSH Keys">
							{keys.map((key) => {
								const hosts = key.hostRules.map((r) => r.match).join(", ");
								return (
									<CommandItem key={key.id} onSelect={() => run(() => copyPublicKey(key))} value={`key-${key.name}-${key.type}`}>
										<KeyIcon />
										<span className="flex-1 truncate">{key.name}</span>
										<span className="max-w-50 truncate text-muted-foreground text-xs">
											{key.type}
											{hosts ? ` · ${hosts}` : ""}
										</span>
										<CommandShortcut>Copy</CommandShortcut>
									</CommandItem>
								);
							})}
						</CommandGroup>
						<CommandSeparator />
					</>
				)}

				<CommandGroup heading="Navigation">
					{navItems.map((item) => (
						<CommandItem key={item.href} onSelect={() => run(() => router.push(item.href))} value={`nav-${item.label}`}>
							<NavIcon />
							<span>{item.label}</span>
							<CommandShortcut>{item.shortcut}</CommandShortcut>
						</CommandItem>
					))}
				</CommandGroup>

				<CommandSeparator />

				<CommandGroup heading="Actions">
					<CommandItem onSelect={() => run(toggleTheme)} value="action-toggle-theme">
						<ThemeIcon />
						<span>Toggle Theme</span>
						<CommandShortcut>T</CommandShortcut>
					</CommandItem>
					<CommandItem
						onSelect={() =>
							run(() => {
								window.dispatchEvent(new CustomEvent("vault:lock"));
							})
						}
						value="action-lock-vault"
					>
						<LockIcon />
						<span>Lock Vault</span>
					</CommandItem>
					{keys.length > 0 && (
						<CommandItem onSelect={() => run(() => copyPublicKey(keys[0]))} value="action-copy-public-key">
							<CopyIcon />
							<span>Copy Public Key</span>
							<CommandShortcut>C</CommandShortcut>
						</CommandItem>
					)}
				</CommandGroup>
			</CommandList>
		</CommandDialog>
	);
};
