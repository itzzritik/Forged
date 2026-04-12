"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Modal, ModalBody, ModalFooter } from "@/components/ui/modal";
import { useVaultContext } from "@/hooks/use-vault";
import { decryptItemKey, decryptPrivateKey } from "@/lib/vault-crypto";

interface ExportModalProps {
	onClose: () => void;
}

interface ExportItem {
	created_at: string;
	name: string;
	ssh_key: Record<string, unknown>;
	type: string;
	updated_at: string;
}

async function buildExportItems(raw: string, selected: Set<string>, symmetricKey: CryptoKey): Promise<ExportItem[]> {
	const parsed = JSON.parse(raw);
	const rawKeys = (parsed.keys || []) as Record<string, unknown>[];
	const items: ExportItem[] = [];

	for (const rawKey of rawKeys) {
		const id = rawKey.id as string;
		if (!selected.has(id)) continue;

		const encCK = rawKey.encrypted_cipher_key as string;
		const encPK = rawKey.encrypted_private_key as string;
		if (encCK == null || encPK == null || encCK === "" || encPK === "") continue;

		const cipherKey = await decryptItemKey(symmetricKey, encCK);
		const privateKeyBytes = await decryptPrivateKey(cipherKey, encPK);

		items.push({
			type: "ssh_key",
			name: rawKey.name as string,
			ssh_key: {
				private_key: new TextDecoder().decode(privateKeyBytes),
				public_key: rawKey.public_key || "",
				fingerprint: rawKey.fingerprint || "",
				key_type: rawKey.type || "",
				comment: rawKey.comment || "",
				host_rules: rawKey.host_rules || [],
				git_signing: Boolean(rawKey.git_signing),
			},
			created_at: (rawKey.created_at as string) || "",
			updated_at: (rawKey.updated_at as string) || "",
		});
	}
	return items;
}

function downloadForgedExport(items: ExportItem[]) {
	const exportData = {
		format: "forged-export",
		version: 1,
		exported_at: new Date().toISOString(),
		items,
	};

	const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: "application/json" });
	const url = URL.createObjectURL(blob);
	const a = document.createElement("a");
	a.href = url;
	a.download = `forged-export-${new Date().toISOString().slice(0, 10)}.json`;
	a.click();
	URL.revokeObjectURL(url);
}

function keyLabel(n: number): string {
	return n === 1 ? "Key" : "Keys";
}

export const ExportModal = ({ onClose }: ExportModalProps) => {
	const { vaultData, symmetricKeyRef } = useVaultContext();
	const [selected, setSelected] = useState<Set<string>>(() => {
		const ids = new Set<string>();
		for (const k of vaultData?.keys ?? []) ids.add(k.id);
		return ids;
	});
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const keys = vaultData?.keys ?? [];
	const allSelected = selected.size === keys.length;
	const selectedCount = selected.size;

	const toggleKey = (id: string) => {
		setSelected((prev) => {
			const next = new Set(prev);
			if (next.has(id)) next.delete(id);
			else next.add(id);
			return next;
		});
	};

	const toggleAll = () => {
		if (allSelected) {
			setSelected(new Set());
		} else {
			setSelected(new Set(keys.map((k) => k.id)));
		}
	};

	const handleExport = async () => {
		if (isLoading || selectedCount === 0 || !vaultData || !symmetricKeyRef.current) return;

		setIsLoading(true);
		setError(null);

		try {
			const items = await buildExportItems(vaultData.raw, selected, symmetricKeyRef.current);
			downloadForgedExport(items);
			onClose();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Export failed");
		} finally {
			setIsLoading(false);
		}
	};

	return (
		<Modal onOpenChange={(open) => !open && onClose()} open size="sm" title="Keys // Export">
			<ModalBody>
				<div className="space-y-1">
					<p className="font-semibold text-lg">Export SSH Keys</p>
					<p className="text-muted-foreground text-sm">Select the keys you want to include in this export</p>
				</div>

				<div className="flex items-center gap-2">
					<button className="text-primary text-xs hover:underline" onClick={toggleAll} type="button">
						{allSelected ? "Deselect all" : "Select all"}
					</button>
				</div>

				<div className="max-h-[240px] overflow-y-auto border border-border">
					{keys.map((key) => (
						<label className="flex cursor-pointer items-center gap-3 border-border border-b px-3 py-2 last:border-b-0 hover:bg-surface" key={key.id}>
							<input checked={selected.has(key.id)} className="accent-primary" onChange={() => toggleKey(key.id)} type="checkbox" />
							<div className="min-w-0 flex-1">
								<p className="truncate text-foreground text-sm">{key.name}</p>
								<p className="truncate text-muted-foreground text-xs">{key.type}</p>
							</div>
						</label>
					))}
				</div>

				{error && <p className="text-destructive text-xs">{error}</p>}

				<ModalFooter className="justify-end">
					<Button onClick={onClose} type="button" variant="outline">
						Cancel
					</Button>
					<Button disabled={isLoading || selectedCount === 0} onClick={handleExport} type="button">
						{isLoading ? "Exporting..." : `Export ${selectedCount} ${keyLabel(selectedCount)}`}
					</Button>
				</ModalFooter>
			</ModalBody>
		</Modal>
	);
};
