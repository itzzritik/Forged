"use client";

import { ShieldCheckIcon } from "lucide-react";
import { createContext, type ReactNode, useCallback, useContext, useRef, useState } from "react";
import { useVaultContext } from "@/hooks/use-vault";
import { shake } from "../motion";
import { Action } from "./action";
import { ShellDialog } from "./dialog";
import { PasswordField } from "./field";

type Confirm = (reason: string) => Promise<boolean>;

const PromptContext = createContext<Confirm | null>(null);

export function usePasswordPrompt(): Confirm {
	const confirm = useContext(PromptContext);
	if (!confirm) throw new Error("usePasswordPrompt must be used inside PasswordPromptProvider");
	return confirm;
}

export function PasswordPromptProvider({ children }: { children: ReactNode }) {
	const { verifyPassword } = useVaultContext();
	const [reason, setReason] = useState<string | null>(null);
	const [password, setPassword] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [busy, setBusy] = useState(false);
	const resolver = useRef<((ok: boolean) => void) | null>(null);
	const checking = useRef(false);
	const input = useRef<HTMLInputElement>(null);
	const popup = useRef<HTMLDivElement>(null);

	const confirm = useCallback<Confirm>((next) => {
		// The crypto worker runs one derivation at a time; never start a second prompt over a running check.
		if (checking.current) return Promise.resolve(false);
		resolver.current?.(false);
		setReason(next);
		setPassword("");
		setError(null);
		return new Promise((resolve) => {
			resolver.current = resolve;
		});
	}, []);

	const finish = (ok: boolean) => {
		resolver.current?.(ok);
		resolver.current = null;
		setReason(null);
		setPassword("");
		setError(null);
		setBusy(false);
	};

	const submit = async () => {
		if (busy || !password) return;
		const mine = resolver.current;
		checking.current = true;
		setBusy(true);
		setError(null);
		try {
			const ok = await verifyPassword(password);
			checking.current = false;
			// A result for a prompt that is gone must never answer the next one.
			if (resolver.current !== mine) return;
			if (ok) {
				finish(true);
				return;
			}
			setError("That password is not right.");
			setPassword("");
			shake(popup.current);
			input.current?.focus();
		} catch {
			checking.current = false;
			setError("Couldn't check the password. Try again.");
		}
		setBusy(false);
	};

	return (
		<PromptContext.Provider value={confirm}>
			{children}
			<ShellDialog
				actions={
					<Action combo="enter" disabled={busy} tone="primary" type="submit">
						{busy ? "Checking" : "Confirm"}
					</Action>
				}
				description={reason}
				error={error}
				icon={ShieldCheckIcon}
				initialFocus={input}
				onOpenChange={(open) => {
					if (!(open || busy)) finish(false);
				}}
				onSubmit={() => void submit()}
				open={reason !== null}
				popupRef={popup}
				title="Confirm it's you"
			>
				<PasswordField aria-label="Master password" invalid={Boolean(error)} onValueChange={setPassword} readOnly={busy} ref={input} value={password} />
			</ShellDialog>
		</PromptContext.Provider>
	);
}
