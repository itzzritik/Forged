"use client";

import { type ReactNode, useCallback, useEffect, useState } from "react";
import { useVault, VaultContext } from "@/hooks/use-vault";
import { Frame, FrameSkeleton } from "./frame/frame";
import { Gate } from "./gate/gate";
import { PasswordPromptProvider } from "./kit/password-prompt";
import { SyncProvider } from "./sync";

export function Shell({ children, email }: { children: ReactNode; email: string }) {
	const vault = useVault();
	const gated = vault.status === "locked" || vault.status === "no-vault" || vault.status === "error";
	const [gateUp, setGateUp] = useState(false);
	const [revealed, setRevealed] = useState(false);
	const onReveal = useCallback(() => setRevealed(true), []);
	const onLeft = useCallback(() => setGateUp(false), []);

	useEffect(() => {
		if (!gated) return;
		setGateUp(true);
		setRevealed(false);
	}, [gated]);

	// A valid cached key goes loading -> unlocked and never mounts the gate.
	const showFrame = !gated && (!gateUp || revealed);

	return (
		<VaultContext.Provider value={vault}>
			<SyncProvider>
				<PasswordPromptProvider>
					<div className="bg-background font-sans text-foreground" data-shell="">
						{showFrame && <Frame>{vault.status === "unlocked" ? children : <FrameSkeleton />}</Frame>}
						{(gated || gateUp) && <Gate email={email} leaving={!gated} onLeft={onLeft} onReveal={onReveal} />}
					</div>
				</PasswordPromptProvider>
			</SyncProvider>
		</VaultContext.Provider>
	);
}
