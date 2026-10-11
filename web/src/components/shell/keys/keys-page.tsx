"use client";

import { HashIcon, PenLineIcon } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import { ContextMenuItem, ContextMenuSeparator } from "@/components/ui/context-menu";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { PHONE, useMedia, WIDE } from "@/hooks/use-media";
import { useNow } from "@/hooks/use-now";
import { useShortcuts } from "@/hooks/use-shortcuts";
import { useVaultContext } from "@/hooks/use-vault";
import { ago, compareText, middle, plural } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { VaultKeyMetadata } from "@/lib/vault-crypto";
import { copyText } from "../clipboard";
import { useHints } from "../frame/frame";
import type { Hint } from "../frame/key-bar";
import { Action } from "../kit/action";
import { Count } from "../kit/count";
import { type Column, DataTable } from "../kit/data-table";
import { FilterPrompt } from "../kit/filter-prompt";
import { Panel } from "../kit/panel";
import { say } from "../message";
import { burn, sparks } from "../motion";
import { useSync } from "../sync";
import { deleteKeys } from "./actions";
import { DeleteDialog } from "./delete-dialog";
import { ExportDialog } from "./export-dialog";
import { ImportDialog } from "./import-dialog";
import { inspectorIcon, KeyInspector, RenameField } from "./key-inspector";
import { NewKeyDialog } from "./new-key-dialog";

type KeysDialog = "new" | "import" | "export" | "delete" | null;

const NAME_SORT = { desc: false, id: "name" };
const FRESH_MS = 2400;
const rowElement = (id: string) => document.querySelector<HTMLElement>(`[data-row-id="${CSS.escape(id)}"]`);
const keyId = (key: VaultKeyMetadata) => key.id;
const usedAt = (key: VaultKeyMetadata) => (key.lastUsedAt ? Date.parse(key.lastUsedAt) : 0) || 0;
const searchText = (key: VaultKeyMetadata) => [key.name, key.type, key.fingerprint, key.comment, key.gitSigning ? "signing" : ""].join(" ").toLowerCase();

function keyColumns(now: number): Column<VaultKeyMetadata>[] {
	return [
		{
			cell: (key) => (
				<>
					<span>{key.name}</span>
					{key.gitSigning && (
						<span className="ml-1.5 inline-flex items-center gap-1 text-[11px] text-warning">
							<PenLineIcon aria-hidden className="size-3" />
							signing
						</span>
					)}
					<small className="block font-sans text-[12px] text-muted-foreground md:hidden">
						{key.type}, {ago(key.lastUsedAt, now) ? `used ${ago(key.lastUsedAt, now)}` : "never used"}
					</small>
				</>
			),
			header: "Name",
			id: "name",
			sort: (a, b) => compareText(a.name, b.name),
			width: "minmax(0,1.1fr)",
		},
		{
			cell: (key) => <span className="text-muted-foreground">{key.type}</span>,
			header: "Type",
			id: "type",
			sort: (a, b) => compareText(a.type, b.type),
			wide: true,
			width: "80px",
		},
		{ cell: (key) => <span className="text-faint">{middle(key.fingerprint)}</span>, header: "Fingerprint", id: "fingerprint", wide: true, width: "minmax(0,1.5fr)" },
		{
			align: "end",
			cell: (key) => {
				const used = ago(key.lastUsedAt, now);
				return <span className={used ? "text-muted-foreground" : "text-faint"}>{used ?? "never"}</span>;
			},
			header: "Last used",
			id: "used",
			sort: (a, b) => usedAt(b) - usedAt(a),
			wide: true,
			width: "110px",
		},
	];
}

export function KeysPage() {
	const router = useRouter();
	const params = useSearchParams();
	const { deviceId, vaultData } = useVaultContext();
	const { commit } = useSync();
	const wide = useMedia(WIDE, true);
	const phone = useMedia(PHONE);
	const now = useNow(60_000);
	const keys = useMemo(() => vaultData?.keys ?? [], [vaultData]);
	const columns = useMemo(() => keyColumns(now), [now]);
	const [query, setQuery] = useState("");
	const deferred = useDeferredValue(query.trim().toLowerCase());
	const [leaving, setLeaving] = useState<VaultKeyMetadata[]>([]);
	const [activeId, setActiveId] = useState<string | null>(null);
	const [sheetOpen, setSheetOpen] = useState(false);
	const [selected, setSelected] = useState<Set<string>>(() => new Set());
	const [dialog, setDialog] = useState<KeysDialog>(null);
	const [targets, setTargets] = useState<string[]>([]);
	const [renaming, setRenaming] = useState(false);
	const [fresh, setFresh] = useState<ReadonlySet<string>>(() => new Set());
	const filter = useRef<HTMLInputElement>(null);

	const search = useMemo(() => keys.map(searchText), [keys]);
	const matched = useMemo(() => (deferred ? keys.filter((_, index) => search[index].includes(deferred)) : keys), [keys, search, deferred]);
	// Deleted rows stay on screen until their burn finishes, even if the push lands first.
	const visible = useMemo(
		() => (leaving.length ? [...matched, ...leaving.filter((ghost) => !keys.some((key) => key.id === ghost.id))] : matched),
		[matched, leaving, keys]
	);
	const active = keys.find((key) => key.id === activeId) ?? null;
	const targetNames = useMemo(() => {
		const ids = new Set(targets);
		return keys.filter((key) => ids.has(key.id)).map((key) => key.name);
	}, [keys, targets]);

	useEffect(() => {
		if (!wide || keys.length === 0 || keys.some((key) => key.id === activeId)) return;
		setActiveId(keys.reduce((first, key) => (compareText(key.name, first.name) < 0 ? key : first)).id);
	}, [wide, keys, activeId]);

	// The sheet only exists below the wide breakpoint; never let it pop open when the window shrinks.
	useEffect(() => {
		if (wide) setSheetOpen(false);
	}, [wide]);

	useEffect(() => {
		setSelected((current) => {
			const ids = new Set(keys.map(keyId));
			const next = new Set([...current].filter((id) => ids.has(id)));
			return next.size === current.size ? current : next;
		});
	}, [keys]);

	useEffect(() => {
		const id = params.get("key");
		const intent = params.get("do");
		if (!(id || intent)) return;
		if (id) {
			setActiveId(id);
			setSheetOpen(!window.matchMedia(WIDE).matches);
		}
		if (intent === "new" || intent === "import" || intent === "export") {
			setTargets([]);
			setDialog(intent);
		}
		router.replace("/dashboard", { scroll: false });
	}, [params, router]);

	useEffect(() => {
		if (fresh.size === 0) return;
		const spark = setTimeout(() => {
			for (const id of [...fresh].slice(0, 6)) {
				const row = rowElement(id);
				if (row) sparks(row, 16, { spread: 0.6 });
			}
		}, 150);
		const clear = setTimeout(() => setFresh(new Set()), FRESH_MS);
		return () => {
			clearTimeout(spark);
			clearTimeout(clear);
		};
	}, [fresh]);

	const choose = (id: string) => {
		setActiveId(id);
		setRenaming(false);
	};
	const open = (id: string) => {
		choose(id);
		if (!wide) setSheetOpen(true);
	};
	const ask = (kind: Exclude<KeysDialog, null>, ids: string[] = []) => {
		setTargets(ids);
		setDialog(kind);
	};
	const closeDialog = (next: boolean) => {
		if (!next) setDialog(null);
	};
	const selection = () => {
		if (selected.size) return [...selected];
		return activeId ? [activeId] : [];
	};
	const copyKey = (key: VaultKeyMetadata) => void copyText(key.publicKey, `Copied the public key of ${key.name}`, rowElement(key.id));
	const copyActive = () => {
		const button = document.querySelector<HTMLButtonElement>("[data-copy-public]");
		if (button) button.click();
		else if (active) copyKey(active);
	};
	const startRename = (id: string) => {
		open(id);
		setRenaming(true);
	};
	const created = (ids: string[]) => {
		setFresh(new Set(ids));
		const [first] = ids;
		if (first) open(first);
	};

	const remove = async () => {
		const ids = targets;
		const chosen = new Set(ids);
		setDialog(null);
		if (!vaultData || ids.length === 0) return;
		const gone = keys.filter((key) => chosen.has(key.id));
		setLeaving(gone);
		const effect = burn(ids.map(rowElement).filter((row): row is HTMLElement => row !== null));
		const pushed = commit(deleteKeys(vaultData, ids, deviceId)).then((pushedOk) => {
			if (!pushedOk) effect.cancel();
			return pushedOk;
		});
		const [ok] = await Promise.all([pushed, effect.finished]);
		setLeaving([]);
		if (!ok) return;
		setSelected(new Set());
		if (activeId && chosen.has(activeId)) setActiveId(null);
		say(
			gone.length === 1
				? `Deleted ${gone[0].name}. Your other devices drop it at their next sync.`
				: `Deleted ${plural(gone.length, "key")}. Your other devices drop them at their next sync.`,
			"warn"
		);
	};

	useShortcuts({
		"/": () => filter.current?.focus(),
		c: copyActive,
		d: () => {
			const ids = selection();
			if (ids.length) ask("delete", ids);
		},
		e: () => ask("export", selection()),
		esc: () => {
			if (selected.size) setSelected(new Set());
			else if (query) setQuery("");
		},
		i: () => ask("import"),
		n: () => ask("new"),
		r: () => {
			if (active) startRename(active.id);
		},
	});

	const count = selected.size;
	const hints: Hint[] = count
		? [
				{ combo: "d", label: `Delete ${count.toLocaleString()}` },
				{ combo: "e", label: `Export ${count.toLocaleString()}` },
				{ combo: "esc", label: "Clear selection" },
			]
		: [
				{ combo: "n", label: "New key", mobile: true, primary: true },
				{ combo: "i", label: "Import", mobile: true },
				{ combo: "e", label: "Export" },
				{ combo: "/", label: "Filter" },
				...(wide ? [] : [{ combo: "enter", label: "Open" }]),
			];
	useHints(hints);

	const menu = (id: string) => {
		const key = keys.find((item) => item.id === id);
		if (!key) return null;
		return (
			<>
				<ContextMenuItem onClick={() => copyKey(key)}>Copy public key</ContextMenuItem>
				<ContextMenuItem onClick={() => startRename(id)}>Rename</ContextMenuItem>
				<ContextMenuItem
					onClick={() =>
						setSelected((current) => {
							const next = new Set(current);
							if (next.has(id)) next.delete(id);
							else next.add(id);
							return next;
						})
					}
				>
					{selected.has(id) ? "Deselect" : "Select"}
				</ContextMenuItem>
				<ContextMenuItem onClick={() => ask("export", [id])}>Export</ContextMenuItem>
				<ContextMenuSeparator />
				<ContextMenuItem onClick={() => ask("delete", [id])} variant="destructive">
					Delete
				</ContextMenuItem>
			</>
		);
	};

	const empty =
		keys.length === 0 ? (
			<div className="grid justify-items-center gap-2 px-6 py-16 text-center font-sans">
				<p className="font-medium text-[15px]">No keys yet</p>
				<p className="text-muted-foreground">Create a key here or bring the ones you already have.</p>
				<div className="mt-2 flex gap-2">
					<Action combo="n" onClick={() => ask("new")} tone="primary">
						New key
					</Action>
					<Action combo="i" onClick={() => ask("import")}>
						Import
					</Action>
				</div>
			</div>
		) : (
			<p className="px-[18px] py-10 font-sans text-muted-foreground">No keys match "{query.trim()}". Press esc to clear the filter.</p>
		);

	const inspector = active && (
		<Panel
			aside={active.type}
			className="flex min-h-0 flex-col"
			delay={110}
			icon={inspectorIcon(active)}
			title={
				renaming ? (
					<RenameField item={active} onDone={() => setRenaming(false)} />
				) : (
					<span className="title-roll" key={active.id}>
						{active.name}
					</span>
				)
			}
		>
			<KeyInspector item={active} key={active.id} onDelete={() => ask("delete", [active.id])} onRenaming={setRenaming} />
		</Panel>
	);

	return (
		<div className="keys-layout" data-solo={wide && active ? undefined : ""}>
			<Panel
				aside={matched.length === keys.length ? <Count value={keys.length} word="key" /> : `${matched.length.toLocaleString()} of ${plural(keys.length, "key")}`}
				className="flex min-h-0 flex-col"
				icon={HashIcon}
				title="SSH keys"
			>
				<FilterPrompt label="Filter keys" onChange={setQuery} placeholder="filter by name, type or fingerprint" ref={filter} value={query} />
				<DataTable
					activeId={activeId}
					columns={columns}
					defaultSort={NAME_SORT}
					empty={empty}
					getId={keyId}
					label="SSH keys"
					menu={menu}
					onActiveChange={choose}
					onOpen={open}
					onSelectedChange={setSelected}
					rowData={(key) => ({ "data-fresh": fresh.has(key.id) ? "" : undefined })}
					rows={visible}
					selected={selected}
				/>
			</Panel>
			{wide && inspector}
			{!wide && (
				<Sheet onOpenChange={setSheetOpen} open={sheetOpen && Boolean(active)}>
					<SheetContent
						className={cn("gap-0 bg-background p-3", phone ? "max-h-[88dvh] rounded-t-2xl pt-7" : "w-[min(420px,100vw)] pt-6 sm:max-w-[420px]")}
						showCloseButton={false}
						side={phone ? "bottom" : "right"}
					>
						{phone && <i aria-hidden className="sheet-grab" />}
						<SheetTitle className="sr-only">{active?.name ?? "Key details"}</SheetTitle>
						{inspector}
					</SheetContent>
				</Sheet>
			)}
			<NewKeyDialog onCreated={(id) => created([id])} onOpenChange={closeDialog} open={dialog === "new"} />
			<ImportDialog onImported={created} onOpenChange={closeDialog} open={dialog === "import"} />
			<ExportDialog ids={targets} onOpenChange={closeDialog} open={dialog === "export"} />
			<DeleteDialog names={targetNames} onConfirm={() => void remove()} onOpenChange={closeDialog} open={dialog === "delete"} />
		</div>
	);
}
