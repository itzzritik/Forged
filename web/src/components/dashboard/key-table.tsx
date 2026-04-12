"use client";

import { useRef, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { useVaultContext } from "@/hooks/use-vault";
import { removeKeyFromVault, updateKeyInVault, type VaultKeyMetadata } from "@/lib/vault-crypto";
import { HostRulesEditor } from "./host-rules-editor";

export const KeyTable = () => {
	const { deviceId, vaultData, pushVault } = useVaultContext();
	const keys = vaultData?.keys ?? [];

	const [editingId, setEditingId] = useState<string | null>(null);
	const [editName, setEditName] = useState("");
	const [hostRulesKey, setHostRulesKey] = useState<VaultKeyMetadata | null>(null);
	const [pendingDeleteId, setPendingDeleteId] = useState<string | null>(null);
	const inputRef = useRef<HTMLInputElement>(null);

	if (keys.length === 0) {
		return (
			<div className="flex flex-col items-center justify-center gap-2 py-16 text-center">
				<p className="text-muted-foreground text-sm">No keys in vault</p>
				<p className="font-mono text-muted-foreground text-xs">
					Generate a key or add one via CLI: <span className="text-primary">forged add &lt;name&gt;</span>
				</p>
			</div>
		);
	}

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

	const handleDelete = async (key: VaultKeyMetadata) => {
		if (pendingDeleteId !== key.id) {
			setPendingDeleteId(key.id);
			return;
		}
		setPendingDeleteId(null);
		if (!vaultData) return;
		try {
			const updated = removeKeyFromVault(vaultData, key.id, deviceId);
			await pushVault(updated);
			toast.success("Key deleted");
		} catch {
			toast.error("Failed to delete key");
		}
	};

	return (
		<>
			<TooltipProvider>
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead className="hidden sm:table-cell">Fingerprint</TableHead>
							<TableHead>Hosts</TableHead>
							<TableHead className="hidden sm:table-cell">Signing</TableHead>
							<TableHead>Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{keys.map((key) => (
							<TableRow key={key.id}>
								<TableCell>
									{editingId === key.id ? (
										<Input
											className="h-7 w-36 font-mono text-sm"
											onBlur={() => commitRename(key)}
											onChange={(e) => setEditName(e.target.value)}
											onKeyDown={(e) => {
												if (e.key === "Enter") commitRename(key);
												if (e.key === "Escape") setEditingId(null);
											}}
											ref={inputRef}
											value={editName}
										/>
									) : (
										<>
											<button
												className="cursor-pointer font-medium text-foreground hover:text-primary"
												onClick={() => startRename(key)}
												title="Click to rename"
												type="button"
											>
												{key.name}
											</button>
											<div className="text-muted-foreground text-xs">{key.type}</div>
										</>
									)}
								</TableCell>
								<TableCell className="hidden sm:table-cell">
									<Tooltip>
										<TooltipTrigger className="block max-w-45 cursor-default truncate font-mono text-muted-foreground text-sm" render={<span />}>
											{key.fingerprint}
										</TooltipTrigger>
										<TooltipContent>
											<span className="font-mono">{key.fingerprint}</span>
										</TooltipContent>
									</Tooltip>
								</TableCell>
								<TableCell>
									<div className="flex flex-wrap gap-1">
										{key.hostRules.length > 0 ? (
											key.hostRules.map((rule, i) => (
												<Badge className="border border-primary/20 bg-primary/10 text-primary hover:bg-primary/10" key={i}>
													{rule.match}
												</Badge>
											))
										) : (
											<span className="text-muted-foreground text-xs">--</span>
										)}
									</div>
								</TableCell>
								<TableCell className="hidden sm:table-cell">
									{key.gitSigning ? (
										<span className="flex items-center gap-1.5 text-sm text-success">
											<span className="size-1.5 shrink-0 rounded-full bg-success" />
											Active
										</span>
									) : (
										<span className="text-muted-foreground text-sm">Off</span>
									)}
								</TableCell>
								<TableCell>
									<div className="flex items-center gap-2">
										<Button onClick={() => handleCopy(key)} size="sm" variant="outline">
											Copy
										</Button>
										<Button onClick={() => setHostRulesKey(key)} size="sm" variant="ghost">
											Hosts
										</Button>
										<Button
											onBlur={() => setPendingDeleteId(null)}
											onClick={() => handleDelete(key)}
											size="sm"
											variant={pendingDeleteId === key.id ? "destructive" : "ghost"}
										>
											{pendingDeleteId === key.id ? "Confirm?" : "Delete"}
										</Button>
									</div>
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			</TooltipProvider>
			{hostRulesKey && (
				<HostRulesEditor hostRules={hostRulesKey.hostRules} keyId={hostRulesKey.id} keyName={hostRulesKey.name} onClose={() => setHostRulesKey(null)} />
			)}
		</>
	);
};
