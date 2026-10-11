"use client";

import { KeyRoundIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useVaultContext } from "@/hooks/use-vault";
import { Action } from "../kit/action";
import { ShellDialog } from "../kit/dialog";
import { FieldLabel, PasswordField } from "../kit/field";
import { say } from "../message";

const MIN_LENGTH = 8;

export function PasswordDialog({ onOpenChange, open }: { onOpenChange: (open: boolean) => void; open: boolean }) {
	const { rekey } = useVaultContext();
	const [current, setCurrent] = useState("");
	const [next, setNext] = useState("");
	const [again, setAgain] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [busy, setBusy] = useState(false);
	const first = useRef<HTMLInputElement>(null);

	useEffect(() => {
		if (open) return;
		setCurrent("");
		setNext("");
		setAgain("");
		setError(null);
	}, [open]);

	const submit = async () => {
		if (busy) return;
		if (next.length < MIN_LENGTH) {
			setError(`Use at least ${MIN_LENGTH} characters for the new password.`);
			return;
		}
		if (next !== again) {
			setError("The new passwords don't match.");
			return;
		}
		if (next === current) {
			setError("Pick a password different from the current one.");
			return;
		}
		setBusy(true);
		setError(null);
		try {
			await rekey(current, next);
			onOpenChange(false);
			say("Password changed. Your other devices ask for the new one at their next unlock.");
		} catch (cause) {
			const message = cause instanceof Error ? cause.message : "";
			setError(message === "WRONG_PASSWORD" ? "Your current password is not right." : message || "Couldn't change the password. Nothing changed.");
		} finally {
			setBusy(false);
		}
	};

	return (
		<ShellDialog
			actions={
				<Action combo="enter" disabled={busy} tone="primary" type="submit">
					{busy ? "Changing" : "Change password"}
				</Action>
			}
			description="Your vault is re-wrapped on this device. The server never sees either password."
			error={error}
			icon={KeyRoundIcon}
			initialFocus={first}
			onOpenChange={onOpenChange}
			onSubmit={() => void submit()}
			open={open}
			title="Change master password"
		>
			<FieldLabel htmlFor="password-current">Current password</FieldLabel>
			<PasswordField id="password-current" onValueChange={setCurrent} readOnly={busy} ref={first} value={current} />
			<FieldLabel htmlFor="password-next">New password</FieldLabel>
			<PasswordField autoComplete="new-password" id="password-next" onValueChange={setNext} readOnly={busy} value={next} />
			<FieldLabel htmlFor="password-again">Confirm new password</FieldLabel>
			<PasswordField autoComplete="new-password" id="password-again" onValueChange={setAgain} readOnly={busy} value={again} />
		</ShellDialog>
	);
}
