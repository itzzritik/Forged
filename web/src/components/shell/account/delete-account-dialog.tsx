"use client";

import { TriangleAlertIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Action } from "../kit/action";
import { ShellDialog } from "../kit/dialog";
import { FieldLabel, TextField } from "../kit/field";
import { logout } from "../session";

const CONFIRM = "DELETE";

export function DeleteAccountDialog({ onOpenChange, open }: { onOpenChange: (open: boolean) => void; open: boolean }) {
	const [typed, setTyped] = useState("");
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const input = useRef<HTMLInputElement>(null);

	useEffect(() => {
		if (open) return;
		setTyped("");
		setError(null);
	}, [open]);

	const submit = async () => {
		if (typed !== CONFIRM || busy) return;
		setBusy(true);
		setError(null);
		try {
			const res = await fetch("/api/vault/account-delete", { method: "POST" });
			if (!res.ok) throw new Error(String(res.status));
			await logout();
		} catch {
			setError("Couldn't delete the account. Nothing changed. Try again.");
			setBusy(false);
		}
	};

	return (
		<ShellDialog
			actions={
				<Action disabled={typed !== CONFIRM || busy} tone="fill" type="submit">
					{busy ? "Deleting" : "Delete account"}
				</Action>
			}
			description="This removes your account and your encrypted vault from the server. This can't be undone."
			error={error}
			heading="Delete your Forged account?"
			icon={TriangleAlertIcon}
			initialFocus={input}
			onOpenChange={onOpenChange}
			onSubmit={() => void submit()}
			open={open}
			title="Delete account"
			tone="danger"
		>
			<FieldLabel htmlFor="delete-confirm">Type {CONFIRM} to confirm</FieldLabel>
			<TextField autoComplete="off" id="delete-confirm" onValueChange={setTyped} readOnly={busy} ref={input} value={typed} />
		</ShellDialog>
	);
}
