"use client";

import { useMemo, useState } from "react";
import { toast } from "sonner";
import { useVaultContext } from "@/hooks/use-vault";
import { getVaultKeyDetails, removeKeyFromVault, updateKeyInVault, type VaultKeyMetadata } from "@/lib/vault-crypto";
import { BulkDeleteKeysModal } from "./bulk-delete-keys-modal";
import { DataView, type DataViewAction, type DataViewColumn } from "./data-view";
import { DeleteKeyModal } from "./delete-key-modal";
import { KeyDetailsModal } from "./key-details-modal";

export const KeyTable = () => {
	const { deviceId, pushVault, status, vaultData, symmetricKeyRef } = useVaultContext();
	const keys = vaultData?.keys ?? [];

	const [activeKeyId, setActiveKeyId] = useState<string | null>(null);
	const [keyModalMode, setKeyModalMode] = useState<"view" | "edit">("view");
	const [isSavingName, setIsSavingName] = useState(false);
	const [deleteKey, setDeleteKey] = useState<VaultKeyMetadata | null>(null);
	const [bulkDeleteKeys, setBulkDeleteKeys] = useState<VaultKeyMetadata[]>([]);

	const openKeyModal = (keyId: string, mode: "view" | "edit" = "view") => {
		setActiveKeyId(keyId);
		setKeyModalMode(mode);
	};

	const closeKeyModal = () => {
		setActiveKeyId(null);
		setKeyModalMode("view");
	};

	const activeKeyDetails = useMemo(() => {
		if (!vaultData || !activeKeyId) return null;
		return getVaultKeyDetails(vaultData, activeKeyId);
	}, [activeKeyId, vaultData]);

	const handleCopy = async (key: VaultKeyMetadata) => {
		try {
			await navigator.clipboard.writeText(key.publicKey);
			toast.success("Public key copied to clipboard");
		} catch {
			toast.error("Failed to copy to clipboard");
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

	const handleSaveKeyDetails = async (updates: { hostRules: VaultKeyMetadata["hostRules"]; name: string }) => {
		if (!vaultData || !activeKeyId) return;
		const trimmed = updates.name.trim();
		if (!trimmed) return;

		const current = vaultData.keys.find((key) => key.id === activeKeyId);
		if (
			!current ||
			(current.name === trimmed && JSON.stringify(current.hostRules ?? []) === JSON.stringify(updates.hostRules ?? []))
		) {
			setKeyModalMode("view");
			return;
		}

		try {
			setIsSavingName(true);
			const updated = updateKeyInVault(vaultData, activeKeyId, { host_rules: updates.hostRules, name: trimmed }, deviceId);
			await pushVault(updated);
			toast.success("Key updated");
			setKeyModalMode("view");
		} catch {
			toast.error("Failed to update key");
		} finally {
			setIsSavingName(false);
		}
	};

	const actions = useMemo(
		() =>
			(key: VaultKeyMetadata): DataViewAction<VaultKeyMetadata>[] => [
				{
					id: `edit-${key.id}`,
					label: "Edit",
					onClick: () => openKeyModal(key.id, "edit"),
				},
				{
					id: `copy-${key.id}`,
					label: "Copy Public Key",
					onClick: () => void handleCopy(key),
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
							<span className="block truncate font-medium text-foreground">{key.name}</span>
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
		[]
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
				onRowClick={(key) => openKeyModal(key.id, "view")}
				rowHeight={46}
				selectionToolbar={{
					label: (selectedRows) => `${selectedRows.length} key${selectedRows.length === 1 ? "" : "s"} selected`,
					onPrimaryAction: (selectedRows) => setBulkDeleteKeys(selectedRows),
					primaryActionLabel: (selectedRows) => `Delete ${selectedRows.length} Key${selectedRows.length === 1 ? "" : "s"}`,
				}}
			/>

			<KeyDetailsModal
				isSaving={isSavingName}
				keyDetails={activeKeyDetails}
				mode={keyModalMode}
				onClose={closeKeyModal}
				onModeChange={setKeyModalMode}
				onSaveChanges={handleSaveKeyDetails}
				open={Boolean(activeKeyId && activeKeyDetails)}
				symmetricKey={symmetricKeyRef.current}
			/>

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
