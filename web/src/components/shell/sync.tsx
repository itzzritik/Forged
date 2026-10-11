"use client";

import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { useVaultContext } from "@/hooks/use-vault";
import type { VaultData } from "@/lib/vault-crypto";
import { pin, say, unpin } from "./message";

interface Sync {
	commit: (next: VaultData, options?: { pin?: boolean }) => Promise<boolean>;
	syncing: boolean;
}

const SyncContext = createContext<Sync | null>(null);

export function useSync(): Sync {
	const sync = useContext(SyncContext);
	if (!sync) throw new Error("useSync must be used inside SyncProvider");
	return sync;
}

export function SyncProvider({ children }: { children: ReactNode }) {
	const { pushVault, status } = useVaultContext();
	const [syncing, setSyncing] = useState(false);
	const pushLatest = useRef(pushVault);

	useEffect(() => {
		pushLatest.current = pushVault;
	});

	useEffect(() => {
		if (status === "unlocked") return;
		// A failed change must not be retried after a lock: the server may have moved on.
		setSyncing(false);
		unpin();
	}, [status]);

	const commit = useCallback(async function push(next: VaultData, { pin: offerRetry = true } = {}): Promise<boolean> {
		setSyncing(true);
		try {
			await pushLatest.current(next);
			unpin();
			return true;
		} catch (error) {
			if (error instanceof Error && error.message === "401") {
				window.location.assign("/login");
				return false;
			}
			// Dialogs with their own retry opt out, so a stored push can't land behind them.
			if (offerRetry) {
				pin("Sync failed. Nothing changed.", {
					key: "s",
					label: "Retry",
					run: () =>
						void push(next).then((ok) => {
							if (ok) say("Synced. Your change is saved.");
						}),
				});
			}
			return false;
		} finally {
			setSyncing(false);
		}
	}, []);

	const value = useMemo(() => ({ commit, syncing }), [commit, syncing]);
	return <SyncContext.Provider value={value}>{children}</SyncContext.Provider>;
}
