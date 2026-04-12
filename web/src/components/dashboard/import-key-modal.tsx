"use client";

import { useCallback, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useVaultContext } from "@/hooks/use-vault";
import { type ParsedSSHKey, parseSSHKeyFile } from "@/lib/ssh-key-parser";
import { computeFingerprintFromBlob, formatSSHPublicKeyFromBlob } from "@/lib/ssh-key-utils";
import { addKeyToVault, encryptNewItemKey, encryptPrivateKey } from "@/lib/vault-crypto";

interface ImportKeyModalProps {
	onClose: () => void;
}

const EXT_RE = /\.(pem|key|pub)$/i;
const ID_PREFIX_RE = /^id_/;
const NON_SLUG_RE = /[^a-z0-9_-]/gi;

function deriveNameFromFile(filename: string): string {
	return filename.replace(EXT_RE, "").replace(ID_PREFIX_RE, "").replace(NON_SLUG_RE, "-").toLowerCase();
}

export const ImportKeyModal = ({ onClose }: ImportKeyModalProps) => {
	const { vaultData, symmetricKeyRef, pushVault } = useVaultContext();
	const [parsedKey, setParsedKey] = useState<ParsedSSHKey | null>(null);
	const [fingerprint, setFingerprint] = useState("");
	const [name, setName] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [isLoading, setIsLoading] = useState(false);
	const [isDragOver, setIsDragOver] = useState(false);
	const fileInputRef = useRef<HTMLInputElement>(null);

	const handleFile = useCallback(async (file: File) => {
		setError(null);
		setParsedKey(null);

		try {
			const content = await file.text();
			const parsed = parseSSHKeyFile(content);
			const fp = await computeFingerprintFromBlob(parsed.publicKeyBlob);
			setParsedKey(parsed);
			setFingerprint(fp);
			setName(parsed.comment || deriveNameFromFile(file.name));
		} catch (err) {
			const msg = err instanceof Error ? err.message : "Failed to parse key";
			if (msg === "PASSPHRASE_PROTECTED") {
				setError("This key is passphrase-protected. Remove the passphrase first:\nssh-keygen -p -f <file>");
			} else if (msg === "PEM_LEGACY") {
				setError("Legacy PEM format. Convert to OpenSSH format:\nssh-keygen -p -f <file>");
			} else if (msg === "UNKNOWN_FORMAT") {
				setError("Unrecognized key format. Only OpenSSH private keys are supported.");
			} else {
				setError(msg);
			}
		}
	}, []);

	const handleDrop = useCallback(
		(e: React.DragEvent) => {
			e.preventDefault();
			setIsDragOver(false);
			const file = e.dataTransfer.files[0];
			if (file) handleFile(file);
		},
		[handleFile]
	);

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		const trimmed = name.trim();
		if (!(trimmed && parsedKey) || isLoading || !vaultData || !symmetricKeyRef.current) return;

		setIsLoading(true);
		setError(null);

		try {
			const symmetricKey = symmetricKeyRef.current;
			const { cipherKey, encryptedCipherKeyB64 } = await encryptNewItemKey(symmetricKey);
			const encryptedPrivateKeyB64 = await encryptPrivateKey(cipherKey, parsedKey.privateKeyBytes);

			const publicKeyStr = formatSSHPublicKeyFromBlob(parsedKey.publicKeyBlob, trimmed);
			const now = new Date().toISOString();

			const newKey = {
				id: crypto.randomUUID(),
				name: trimmed,
				type: parsedKey.type,
				public_key: publicKeyStr,
				fingerprint,
				comment: parsedKey.comment,
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

			const updated = addKeyToVault(vaultData, newKey);
			await pushVault(updated);
			onClose();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to import key");
		} finally {
			setIsLoading(false);
		}
	};

	return (
		<div className="fixed inset-0 z-50 flex items-center justify-center">
			<div aria-hidden className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
			<div className="relative z-10 w-full max-w-md border border-border bg-card p-6 font-mono shadow-2xl">
				<p className="mb-4 font-semibold text-lg">Import SSH Key</p>

				{parsedKey ? (
					<form className="flex flex-col gap-3" onSubmit={handleSubmit}>
						<div className="flex flex-col gap-1 border border-border bg-surface p-3 text-xs">
							<div className="flex justify-between">
								<span className="text-muted-foreground">Type</span>
								<span className="text-foreground">{parsedKey.type}</span>
							</div>
							<div className="flex justify-between">
								<span className="text-muted-foreground">Fingerprint</span>
								<span className="max-w-[240px] truncate text-foreground">{fingerprint}</span>
							</div>
							{parsedKey.comment && (
								<div className="flex justify-between">
									<span className="text-muted-foreground">Comment</span>
									<span className="text-foreground">{parsedKey.comment}</span>
								</div>
							)}
						</div>

						<Input autoFocus onChange={(e) => setName(e.target.value)} placeholder="Key name" value={name} />

						{error && <p className="text-destructive text-xs">{error}</p>}

						<div className="flex justify-end gap-2">
							<Button onClick={onClose} type="button" variant="outline">
								Cancel
							</Button>
							<Button disabled={isLoading || !name.trim()} type="submit">
								{isLoading ? "Importing..." : "Import"}
							</Button>
						</div>
					</form>
				) : (
					<div className="flex flex-col gap-3">
						<button
							className={`flex min-h-[120px] w-full cursor-pointer flex-col items-center justify-center gap-2 border border-dashed p-6 transition-colors ${
								isDragOver ? "border-primary bg-primary/5" : "border-border hover:border-muted-foreground"
							}`}
							onClick={() => fileInputRef.current?.click()}
							onDragLeave={() => setIsDragOver(false)}
							onDragOver={(e) => {
								e.preventDefault();
								setIsDragOver(true);
							}}
							onDrop={handleDrop}
							type="button"
						>
							<p className="text-muted-foreground text-sm">Drop a private key file here</p>
							<p className="text-muted-foreground text-xs">or click to browse</p>
						</button>
						<input
							accept=".pem,.key,*"
							className="hidden"
							onChange={(e) => {
								const file = e.target.files?.[0];
								if (file) handleFile(file);
							}}
							ref={fileInputRef}
							type="file"
						/>
						{error && <pre className="whitespace-pre-wrap text-destructive text-xs">{error}</pre>}
						<div className="flex justify-end">
							<Button onClick={onClose} type="button" variant="outline">
								Cancel
							</Button>
						</div>
					</div>
				)}
			</div>
		</div>
	);
};
