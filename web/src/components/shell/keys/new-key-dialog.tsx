"use client";

import { PlusIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useVaultContext } from "@/hooks/use-vault";
import { Action } from "../kit/action";
import { ShellDialog } from "../kit/dialog";
import { FieldLabel, TextField } from "../kit/field";
import { say } from "../message";
import { useSync } from "../sync";
import { createKey, NAME_LIMIT, validateName } from "./actions";

interface NewKeyDialogProps {
	onCreated: (id: string) => void;
	onOpenChange: (open: boolean) => void;
	open: boolean;
}

export function NewKeyDialog({ onCreated, onOpenChange, open }: NewKeyDialogProps) {
	const { deviceId, symmetricKeyRef, vaultData } = useVaultContext();
	const { commit } = useSync();
	const [name, setName] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [busy, setBusy] = useState(false);
	const input = useRef<HTMLInputElement>(null);

	useEffect(() => {
		if (!open) return;
		setName("");
		setError(null);
	}, [open]);

	const submit = async () => {
		const trimmed = name.trim();
		const problem = validateName(trimmed, vaultData?.keys ?? []);
		if (problem) {
			setError(problem);
			return;
		}
		const key = symmetricKeyRef.current;
		if (!(vaultData && key) || busy) return;
		setBusy(true);
		setError(null);
		try {
			const { id, next } = await createKey(vaultData, key, trimmed, deviceId);
			if (!(await commit(next, { pin: false }))) {
				setError("Couldn't sync the new key. Nothing changed. Try again.");
				return;
			}
			onOpenChange(false);
			onCreated(id);
			say(`Created ${trimmed}`);
		} catch {
			setError("Couldn't create the key. Try again.");
		} finally {
			setBusy(false);
		}
	};

	return (
		<ShellDialog
			actions={
				<Action combo="enter" disabled={busy} tone="primary" type="submit">
					{busy ? "Creating" : "Create key"}
				</Action>
			}
			aside="ed25519"
			error={error}
			icon={PlusIcon}
			initialFocus={input}
			onOpenChange={onOpenChange}
			onSubmit={() => void submit()}
			open={open}
			title="New key"
		>
			<FieldLabel htmlFor="new-key-name">Name</FieldLabel>
			<TextField
				id="new-key-name"
				invalid={Boolean(error)}
				maxLength={NAME_LIMIT + 1}
				onValueChange={(value) => {
					setName(value);
					setError(null);
				}}
				placeholder="github-work"
				readOnly={busy}
				ref={input}
				value={name}
			/>
			<p className="text-[12.5px] text-muted-foreground">Created on this device, then synced to your others.</p>
		</ShellDialog>
	);
}
