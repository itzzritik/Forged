"use client";

import { useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { Input } from "@/components/ui/input";
import { useVaultContext } from "@/hooks/use-vault";
import { removeKeyFromVault, updateKeyInVault, type VaultKeyMetadata } from "@/lib/vault-crypto";
import { BulkDeleteKeysModal } from "./bulk-delete-keys-modal";
import { DataView, type DataViewAction, type DataViewColumn } from "./data-view";
import { DeleteKeyModal } from "./delete-key-modal";
import { HostRulesEditor } from "./host-rules-editor";

export const KeyTable = () => {
	const { deviceId, pushVault, status, vaultData } = useVaultContext();
	const keys = vaultData?.keys ?? [];

	const [editingId, setEditingId] = useState<string | null>(null);
	const [editName, setEditName] = useState("");
	const [hostRulesKey, setHostRulesKey] = useState<VaultKeyMetadata | null>(null);
	const [deleteKey, setDeleteKey] = useState<VaultKeyMetadata | null>(null);
	const [bulkDeleteKeys, setBulkDeleteKeys] = useState<VaultKeyMetadata[]>([]);
	const inputRef = useRef<HTMLInputElement>(null);

	const handleCopy = async (key: VaultKeyMetadata) => {
		try {
			await navigator.clipboard.writeText(key.publicKey);
			toast.success("Public key copied to clipboard");
		} catch {
			toast.error("Failed to copy to clipboard");
		}
	};

	const startRename = (key: VaultKeyMetadata) => {
		setEditingId(key.id);
		setEditName(key.name);
		setTimeout(() => inputRef.current?.focus(), 0);
	};

	const commitRename = async (key: VaultKeyMetadata) => {
		const trimmed = editName.trim();
		setEditingId(null);
		if (!trimmed || trimmed === key.name || !vaultData) return;
		try {
			const updated = updateKeyInVault(vaultData, key.id, { name: trimmed }, deviceId);
			await pushVault(updated);
			toast.success("Key renamed");
		} catch {
			toast.error("Failed to rename key");
		}
	};

	const confirmDeleteKey = async (key: VaultKeyMetadata) => {
		if (!vaultData) return;
		try {
			const updated = removeKeyFromVault(vaultData, key.id, deviceId);
			await pushVault(updated);
			setDeleteKey(null);
			toast.success("Key deleted");
		} catch {
			toast.error("Failed to delete key");
		}
	};

	const confirmBulkDelete = async () => {
		if (!vaultData || bulkDeleteKeys.length === 0) return;
		try {
			const updated = bulkDeleteKeys.reduce((current, key) => removeKeyFromVault(current, key.id, deviceId), vaultData);
			await pushVault(updated);
			setBulkDeleteKeys([]);
			toast.success(`${bulkDeleteKeys.length} key${bulkDeleteKeys.length === 1 ? "" : "s"} deleted`);
		} catch {
			toast.error("Failed to delete selected keys");
		}
	};

	const actions = useMemo(
		() =>
			(key: VaultKeyMetadata): DataViewAction<VaultKeyMetadata>[] => [
				{
					id: `copy-${key.id}`,
					label: "Copy Public Key",
					onClick: () => void handleCopy(key),
				},
				{
					id: `hosts-${key.id}`,
					label: "Edit Host Rules",
					onClick: () => setHostRulesKey(key),
				},
				{
					id: `delete-${key.id}`,
					label: "Delete",
					onClick: () => setDeleteKey(key),
					variant: "destructive",
				},
			],
		[]
	);

	const columns = useMemo<DataViewColumn<VaultKeyMetadata>[]>(
		() => [
			{
				accessorKey: "name",
				header: "Key",
				cell: ({ row }) => {
					const key = row.original;
					return (
						<div className="min-w-0">
							{editingId === key.id ? (
								<Input
									className="h-7 w-40 font-mono text-sm"
									onBlur={() => void commitRename(key)}
									onChange={(event) => setEditName(event.target.value)}
									onKeyDown={(event) => {
										if (event.key === "Enter") void commitRename(key);
										if (event.key === "Escape") setEditingId(null);
									}}
									ref={inputRef}
									value={editName}
								/>
							) : (
								<button className="cursor-pointer truncate font-medium text-foreground hover:text-primary" onClick={() => startRename(key)} type="button">
									{key.name}
								</button>
							)}
						</div>
					);
				},
				meta: {
					cellClassName: "min-w-[14rem]",
					headerClassName: "min-w-[14rem]",
					toggleable: false,
				},
			},
			{
				accessorKey: "type",
				header: "Type",
				cell: ({ row }) => <span className="block overflow-hidden text-ellipsis whitespace-nowrap font-mono text-muted-foreground text-xs">{row.original.type}</span>,
				meta: {
					cellClassName: "w-[10rem]",
					headerClassName: "w-[10rem]",
					responsive: "sm",
				},
			},
			{
				accessorKey: "fingerprint",
				header: "Fingerprint",
				cell: ({ row }) => <span className="block overflow-hidden text-ellipsis whitespace-nowrap font-mono text-muted-foreground text-xs">{row.original.fingerprint}</span>,
				meta: {
					cellClassName: "min-w-[20rem]",
					headerClassName: "min-w-[20rem]",
					responsive: "md",
				},
			},
		],
		[editName, editingId]
	);

	return (
		<>
			<DataView
				actions={actions}
				columns={columns}
				data={keys}
				emptyState={{
					title: "No keys in vault",
					description: "Generate a key or add one via CLI: forged add <name>",
				}}
				enableSelection
				entityLabel="keys"
				getRowId={(key) => key.id}
				getSearchText={(key) =>
					[key.name, key.type, key.fingerprint, key.comment, key.publicKey, key.hostRules.map((rule) => rule.match).join(" "), key.gitSigning ? "active signing" : "signing off"].join(" ")
				}
				globalFilterPlaceholder="Search keys, fingerprints, or hosts"
				initialSorting={[{ id: "name", desc: false }]}
				isLoading={status === "loading"}
				rowHeight={46}
				selectionToolbar={{
					label: (selectedRows) => `${selectedRows.length} key${selectedRows.length === 1 ? "" : "s"} selected`,
					onPrimaryAction: (selectedRows) => setBulkDeleteKeys(selectedRows),
					primaryActionLabel: (selectedRows) => `Delete ${selectedRows.length} Key${selectedRows.length === 1 ? "" : "s"}`,
				}}
			/>

			{hostRulesKey && (
				<HostRulesEditor hostRules={hostRulesKey.hostRules} keyId={hostRulesKey.id} keyName={hostRulesKey.name} onClose={() => setHostRulesKey(null)} />
			)}

			<DeleteKeyModal
				fingerprint={deleteKey?.fingerprint ?? ""}
				keyName={deleteKey?.name ?? ""}
				onClose={() => setDeleteKey(null)}
				onConfirm={() => (deleteKey ? confirmDeleteKey(deleteKey) : Promise.resolve())}
				open={Boolean(deleteKey)}
			/>

			<BulkDeleteKeysModal
				keyNames={bulkDeleteKeys.map((key) => key.name)}
				onClose={() => setBulkDeleteKeys([])}
				onConfirm={confirmBulkDelete}
				open={bulkDeleteKeys.length > 0}
			/>
		</>
	);
};
