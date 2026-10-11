"use client";

import { ArrowUpFromLineIcon } from "lucide-react";
import { useRef, useState } from "react";
import { useVaultContext } from "@/hooks/use-vault";
import { plural } from "@/lib/format";
import { Action } from "../kit/action";
import { ShellDialog } from "../kit/dialog";
import { usePasswordPrompt } from "../kit/password-prompt";
import { say } from "../message";
import { exportFileName, exportKeys } from "./actions";

interface ExportDialogProps {
	ids: string[];
	onOpenChange: (open: boolean) => void;
	open: boolean;
}

export function ExportDialog({ ids, onOpenChange, open }: ExportDialogProps) {
	const { symmetricKeyRef, vaultData } = useVaultContext();
	const confirm = usePasswordPrompt();
	const download = useRef<HTMLButtonElement>(null);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const count = ids.length || (vaultData?.keys.length ?? 0);
	const file = exportFileName();

	const submit = async () => {
		const key = symmetricKeyRef.current;
		if (!(vaultData && key) || busy || count === 0) return;
		if (!(await confirm(`Enter your master password to export ${plural(count, "key")}.`))) return;
		setBusy(true);
		setError(null);
		try {
			const exported = await exportKeys(vaultData.raw, ids.length ? new Set(ids) : null, key, file);
			onOpenChange(false);
			say(`Exported ${plural(exported, "key")} to ${file}. It holds plain-text private keys.`, "warn");
		} catch {
			setError("Couldn't decrypt the keys for export. Nothing was saved.");
		} finally {
			setBusy(false);
		}
	};

	return (
		<ShellDialog
			actions={
				<Action combo="enter" disabled={busy || count === 0} ref={download} tone="primary" type="submit">
					{busy ? "Exporting" : "Download file"}
				</Action>
			}
			aside={`${count.toLocaleString()} selected`}
			error={error}
			heading={`Export ${plural(count, "key")}`}
			icon={ArrowUpFromLineIcon}
			initialFocus={download}
			onOpenChange={onOpenChange}
			onSubmit={() => void submit()}
			open={open}
			title="Export keys"
		>
			<div className="warn-box">
				<b aria-hidden>!</b>
				<span>This file holds your private keys in plain text. Keep it somewhere safe and delete it when you're done.</span>
			</div>
			<p className="font-mono text-[12.5px]">{file}</p>
		</ShellDialog>
	);
}
