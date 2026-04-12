"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useVaultContext } from "@/hooks/use-vault";
import { exportPrivateKeyToOpenSSH } from "@/lib/ssh-key-parser";
import { computeFingerprint, formatSSHPublicKey } from "@/lib/ssh-key-utils";
import { addKeyToVault, encryptNewItemKey, encryptPrivateKey } from "@/lib/vault-crypto";

interface GenerateKeyModalProps {
	onClose: () => void;
}

export const GenerateKeyModal = ({ onClose }: GenerateKeyModalProps) => {
	const { deviceId, vaultData, symmetricKeyRef, pushVault } = useVaultContext();
	const [name, setName] = useState("");
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const handleGenerate = async (e: React.FormEvent) => {
		e.preventDefault();
		const trimmed = name.trim();
		if (!trimmed || isLoading || !vaultData || !symmetricKeyRef.current) return;

		setIsLoading(true);
		setError(null);

		try {
			const keyPair = (await crypto.subtle.generateKey("Ed25519", true, ["sign", "verify"])) as CryptoKeyPair;
			const publicKeyRaw = new Uint8Array(await crypto.subtle.exportKey("raw", keyPair.publicKey));
			const privateKeyBytes = await exportPrivateKeyToOpenSSH(keyPair.privateKey, "ed25519");

			const publicKeyStr = formatSSHPublicKey(publicKeyRaw, trimmed);
			const fingerprint = await computeFingerprint(publicKeyRaw);

			const symmetricKey = symmetricKeyRef.current;
			const { cipherKey, encryptedCipherKeyB64 } = await encryptNewItemKey(symmetricKey);
			const encryptedPrivateKeyB64 = await encryptPrivateKey(cipherKey, privateKeyBytes);
			privateKeyBytes.fill(0);

			const now = new Date().toISOString();
			const newKey = {
				id: crypto.randomUUID(),
				name: trimmed,
				type: "ed25519",
				public_key: publicKeyStr,
				fingerprint,
				comment: "",
				created_at: now,
				updated_at: now,
				host_rules: [],
				git_signing: false,
				tags: [],
				version: 1,
				device_origin: "web",
				encrypted_cipher_key: encryptedCipherKeyB64,
				encrypted_private_key: encryptedPrivateKeyB64,
			};

			const updated = addKeyToVault(vaultData, newKey, deviceId);
			await pushVault(updated);
			onClose();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to generate key");
		} finally {
			setIsLoading(false);
		}
	};

	return (
		<div className="fixed inset-0 z-50 flex items-center justify-center">
			<div aria-hidden className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
			<div className="relative z-10 w-full max-w-md border border-border bg-card p-6 font-mono shadow-2xl">
				<p className="mb-4 font-semibold text-lg">Generate SSH Key</p>
				<form className="flex flex-col gap-3" onSubmit={handleGenerate}>
					<Input autoFocus onChange={(e) => setName(e.target.value)} placeholder="Key name (e.g. github-personal)" value={name} />
					{error && <p className="text-destructive text-xs">{error}</p>}
					<div className="flex justify-end gap-2">
						<Button onClick={onClose} type="button" variant="outline">
							Cancel
						</Button>
						<Button disabled={isLoading || !name.trim()} type="submit">
							{isLoading ? "Generating..." : "Generate Ed25519"}
						</Button>
					</div>
				</form>
			</div>
		</div>
	);
};
