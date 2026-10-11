"use client";

import { ArrowDownToLineIcon, ArrowUpFromLineIcon, CommandIcon, HashIcon, LockIcon, LogOutIcon, PlusIcon, SunMoonIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { type KeyboardEvent, useEffect, useMemo, useState } from "react";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { useVaultContext } from "@/hooks/use-vault";
import { cn } from "@/lib/utils";
import { copyText } from "../clipboard";
import { DialogHeading, POPUP } from "../kit/dialog";
import { Keycap } from "../kit/keycap";
import { PAGES } from "../nav";
import { logout } from "../session";

// Plain matching beats cmdk's fuzzy scorer here: long key names otherwise match almost any query.
const match = (value: string, search: string, keywords?: string[]) => {
	const query = search.trim().toLowerCase();
	if (!query) return 1;
	const text = [value, ...(keywords ?? [])].join(" ").toLowerCase();
	if (text.startsWith(query)) return 1;
	if (text.includes(` ${query}`)) return 0.8;
	return text.includes(query) ? 0.5 : 0;
};

interface PaletteProps {
	onOpenChange: (open: boolean) => void;
	onTheme: () => void;
	open: boolean;
}

export function CommandPalette({ onOpenChange, onTheme, open }: PaletteProps) {
	const router = useRouter();
	const { lock, vaultData } = useVaultContext();
	const [value, setValue] = useState("");

	useEffect(() => {
		if (open) setValue("");
	}, [open]);
	const keys = useMemo(() => vaultData?.keys ?? [], [vaultData]);
	// Names are unique (validateName), so the name is the cmdk value; fingerprints stay out of the fuzzy match.
	const ids = useMemo(() => new Map(keys.map((key) => [key.name.toLowerCase(), key.id])), [keys]);

	const run = (action: () => void) => {
		onOpenChange(false);
		action();
	};
	const go = (href: string) => run(() => router.push(href));

	const onKeyDown = (event: KeyboardEvent) => {
		if (!((event.metaKey || event.ctrlKey) && event.key === "Enter")) return;
		const id = ids.get(value.toLowerCase());
		if (!id) return;
		event.preventDefault();
		go(`/dashboard?key=${encodeURIComponent(id)}`);
	};

	return (
		<Dialog onOpenChange={onOpenChange} open={open}>
			<DialogContent className={cn(POPUP, "top-[12vh] translate-y-0 pb-3 sm:max-w-[600px]")} showCloseButton={false}>
				<DialogHeading icon={CommandIcon}>Commands</DialogHeading>
				<Command filter={match} loop onKeyDown={onKeyDown} onValueChange={setValue} value={value} vimBindings={false}>
					<div className="px-5 pt-1 pb-2">
						<div className="field gap-2.5">
							<span aria-hidden className="font-mono text-primary">
								❯
							</span>
							<CommandInput className="font-mono text-[14px]" placeholder="Type a command or a key name" />
						</div>
					</div>
					<CommandList className="max-h-[min(420px,60vh)] px-3">
						<CommandEmpty>Nothing matches. Try a key name or an action like "lock".</CommandEmpty>
						<CommandGroup heading="Actions">
							<CommandItem onSelect={() => go("/dashboard?do=new")} value="New key">
								<PlusIcon />
								New key
								<Keycap className="ml-auto" combo="n" />
							</CommandItem>
							<CommandItem onSelect={() => go("/dashboard?do=import")} value="Import keys">
								<ArrowDownToLineIcon />
								Import keys
								<Keycap className="ml-auto" combo="i" />
							</CommandItem>
							<CommandItem onSelect={() => go("/dashboard?do=export")} value="Export keys">
								<ArrowUpFromLineIcon />
								Export keys
								<Keycap className="ml-auto" combo="e" />
							</CommandItem>
							<CommandItem onSelect={() => run(() => void lock())} value="Lock vault">
								<LockIcon />
								Lock this browser
								<Keycap className="ml-auto" combo="l" />
							</CommandItem>
							<CommandItem onSelect={() => run(onTheme)} value="Switch theme dark light">
								<SunMoonIcon />
								Switch theme
								<Keycap className="ml-auto" combo="t" />
							</CommandItem>
							<CommandItem onSelect={() => run(() => void logout())} value="Log out sign out">
								<LogOutIcon />
								Log out
							</CommandItem>
						</CommandGroup>
						<CommandGroup heading="Go to">
							{PAGES.map((page) => (
								<CommandItem key={page.href} keywords={page.keywords} onSelect={() => go(page.href)} value={page.label}>
									<page.icon />
									{page.label}
									<Keycap className="ml-auto" combo={page.key} />
								</CommandItem>
							))}
						</CommandGroup>
						{keys.length > 0 && (
							<CommandGroup heading="Keys">
								{keys.map((key) => (
									<CommandItem
										key={key.id}
										keywords={[key.type]}
										onSelect={() => run(() => void copyText(key.publicKey, `Copied the public key of ${key.name}`))}
										value={key.name}
									>
										<HashIcon />
										<span className="truncate font-mono">{key.name}</span>
										<span className="ml-auto shrink-0 text-[12px] text-faint">Copy public key</span>
									</CommandItem>
								))}
							</CommandGroup>
						)}
					</CommandList>
					<div className="palette-foot">
						<span>
							<Keycap combo="arrowup" />
							<Keycap combo="arrowdown" />
							Move
						</span>
						<span>
							<Keycap combo="enter" />
							Run
						</span>
						<span>
							<Keycap combo="mod+enter" />
							Open key
						</span>
						<span>
							<Keycap combo="esc" />
							Close
						</span>
					</div>
				</Command>
			</DialogContent>
		</Dialog>
	);
}
