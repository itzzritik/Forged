"use client";

import { useCallback, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { useVaultContext } from "@/hooks/use-vault";
import { parseBitwarden } from "@/lib/importers/bitwarden";
import { parseForged } from "@/lib/importers/forged-format";
import { parse1Password } from "@/lib/importers/onepassword";
import type { ImportedKey } from "@/lib/importers/types";
import { parseSSHKeyFile } from "@/lib/ssh-key-parser";
import { computeFingerprintFromBlob, formatSSHPublicKeyFromBlob } from "@/lib/ssh-key-utils";
import { addKeyToVault, encryptNewItemKey, encryptPrivateKey } from "@/lib/vault-crypto";

interface ImportKeyModalProps {
	onClose: () => void;
}

type Source = "1password" | "bitwarden" | "forged" | "ssh";
type Step = 1 | 2 | 3;

interface SourceMeta {
	accept: string;
	id: Source;
	instruction: string;
	subtitle: string;
	title: string;
}

interface ReviewKey {
	checked: boolean;
	key: ImportedKey;
}

const SOURCES: SourceMeta[] = [
	{
		id: "1password",
		title: "1Password (.1pux)",
		subtitle: "Export from 1Password Settings > Export",
		instruction: "Export your keys from 1Password: Settings > Export > 1Password (.1pux)",
		accept: ".1pux",
	},
	{
		id: "bitwarden",
		title: "Bitwarden (.json)",
		subtitle: "Export from Bitwarden Settings > Export vault",
		instruction: "Export your vault from Bitwarden: Settings > Export vault > JSON format (unencrypted)",
		accept: ".json",
	},
	{
		id: "forged",
		title: "Forged backup (.json)",
		subtitle: "Import from a previous Forged export",
		instruction: "Select a Forged export file (.json) from a previous backup",
		accept: ".json",
	},
	{
		id: "ssh",
		title: "SSH key file",
		subtitle: "Import a single OpenSSH private key",
		instruction: "Select an OpenSSH private key file (e.g. id_ed25519)",
		accept: "*",
	},
];

function detectKeyType(privateKey: string): string {
	if (privateKey.includes("ssh-ed25519") || privateKey.includes("ed25519")) return "ed25519";
	if (privateKey.includes("ssh-rsa") || privateKey.includes("RSA")) return "rsa";
	if (privateKey.includes("ecdsa")) return "ecdsa";
	return "unknown";
}

const EXT_RE = /\.(pem|key|pub)$/i;
const ID_PREFIX_RE = /^id_/;
const NON_SLUG_RE = /[^a-z0-9_-]/gi;

function deriveNameFromFile(filename: string): string {
	return filename.replace(EXT_RE, "").replace(ID_PREFIX_RE, "").replace(NON_SLUG_RE, "-").toLowerCase() || "imported";
}

function formatParseError(msg: string): string {
	if (msg === "PASSPHRASE_PROTECTED") return "Key is passphrase-protected. Remove the passphrase first:\nssh-keygen -p -f <file>";
	if (msg === "PEM_LEGACY") return "Legacy PEM format. Convert to OpenSSH:\nssh-keygen -p -f <file>";
	if (msg === "UNKNOWN_FORMAT") return "Unrecognized key format. Only OpenSSH private keys are supported.";
	return msg;
}

async function parseSource(source: Source, file: File): Promise<ImportedKey[]> {
	if (source === "1password") {
		const buffer = await file.arrayBuffer();
		return parse1Password(new Uint8Array(buffer));
	}
	if (source === "bitwarden") {
		return parseBitwarden(await file.text());
	}
	if (source === "forged") {
		return parseForged(await file.text());
	}
	// ssh
	const text = await file.text();
	const parsed = parseSSHKeyFile(text);
	const name = parsed.comment || deriveNameFromFile(file.name);
	return [{ name, privateKey: text }];
}

export const ImportKeyModal = ({ onClose }: ImportKeyModalProps) => {
	const { deviceId, vaultData, symmetricKeyRef, pushVault } = useVaultContext();
	const [step, setStep] = useState<Step>(1);
	const [source, setSource] = useState<Source | null>(null);
	const [reviewKeys, setReviewKeys] = useState<ReviewKey[]>([]);
	const [error, setError] = useState<string | null>(null);
	const [isDragOver, setIsDragOver] = useState(false);
	const [isLoading, setIsLoading] = useState(false);
	const fileInputRef = useRef<HTMLInputElement>(null);

	const sourceMeta = SOURCES.find((s) => s.id === source) ?? null;

	const parseFile = useCallback(
		async (file: File) => {
			if (!source) return;
			setError(null);
			try {
				const keys = await parseSource(source, file);
				if (keys.length === 0) {
					setError("No SSH keys found in the selected file.");
					return;
				}
				setReviewKeys(keys.map((k) => ({ key: k, checked: true })));
				setStep(3);
			} catch (err) {
				const msg = err instanceof Error ? err.message : "Failed to parse file";
				setError(formatParseError(msg));
			}
		},
		[source]
	);

	const handleDrop = useCallback(
		(e: React.DragEvent) => {
			e.preventDefault();
			setIsDragOver(false);
			const file = e.dataTransfer.files[0];
			if (file) parseFile(file);
		},
		[parseFile]
	);

	const checkedCount = reviewKeys.filter((r) => r.checked).length;
	const allChecked = checkedCount === reviewKeys.length;

	const toggleAll = () => {
		setReviewKeys((prev) => prev.map((r) => ({ ...r, checked: !allChecked })));
	};

	const toggleOne = (idx: number) => {
		setReviewKeys((prev) => prev.map((r, i) => (i === idx ? { ...r, checked: !r.checked } : r)));
	};

	const handleImport = async () => {
		if (isLoading || !vaultData || !symmetricKeyRef.current) return;
		setIsLoading(true);
		setError(null);

		try {
			const symmetricKey = symmetricKeyRef.current;
			let current = vaultData;

			for (const { key, checked } of reviewKeys) {
				if (!checked) continue;

				const parsed = parseSSHKeyFile(key.privateKey);
				const fingerprint = await computeFingerprintFromBlob(parsed.publicKeyBlob);
				const publicKeyStr = formatSSHPublicKeyFromBlob(parsed.publicKeyBlob, key.name);
				const { cipherKey, encryptedCipherKeyB64 } = await encryptNewItemKey(symmetricKey);
				const encryptedPrivateKeyB64 = await encryptPrivateKey(cipherKey, parsed.privateKeyBytes);

				const now = new Date().toISOString();
				current = addKeyToVault(current, {
					id: crypto.randomUUID(),
					name: key.name,
					type: parsed.type,
					public_key: publicKeyStr,
					fingerprint,
					comment: parsed.comment,
					created_at: now,
					updated_at: now,
					host_rules: [],
					git_signing: false,
					tags: [],
					version: 1,
					device_origin: "web",
					encrypted_cipher_key: encryptedCipherKeyB64,
					encrypted_private_key: encryptedPrivateKeyB64,
				}, deviceId);
			}

			await pushVault(current);
			onClose();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to import keys");
		} finally {
			setIsLoading(false);
		}
	};

	const keyCount = reviewKeys.length;
	const keyLabel = (n: number) => (n === 1 ? "Key" : "Keys");

	return (
		<div className="fixed inset-0 z-50 flex items-center justify-center">
			<div aria-hidden className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
			<div className="relative z-10 w-full max-w-md border border-border bg-card p-6 font-mono shadow-2xl">
				<p className="mb-4 font-semibold text-lg">Import SSH Keys</p>

				{step === 1 && (
					<div className="flex flex-col gap-3">
						{SOURCES.map((s) => (
							<button
								className="flex w-full flex-col gap-0.5 border border-border p-3 text-left transition-colors hover:border-muted-foreground hover:bg-muted/20"
								key={s.id}
								onClick={() => {
									setSource(s.id);
									setError(null);
									setStep(2);
								}}
								type="button"
							>
								<span className="text-sm">{s.title}</span>
								<span className="text-muted-foreground text-xs">{s.subtitle}</span>
							</button>
						))}
						<div className="flex justify-end pt-1">
							<Button onClick={onClose} type="button" variant="outline">
								Cancel
							</Button>
						</div>
					</div>
				)}

				{step === 2 && sourceMeta && (
					<div className="flex flex-col gap-3">
						<p className="text-muted-foreground text-xs">{sourceMeta.instruction}</p>
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
							<p className="text-muted-foreground text-sm">Drop file here or click to browse</p>
						</button>
						<input
							accept={sourceMeta.accept}
							className="hidden"
							onChange={(e) => {
								const file = e.target.files?.[0];
								if (file) parseFile(file);
								e.target.value = "";
							}}
							ref={fileInputRef}
							type="file"
						/>
						{error && <pre className="whitespace-pre-wrap text-destructive text-xs">{error}</pre>}
						<div className="flex justify-between">
							<Button
								onClick={() => {
									setError(null);
									setStep(1);
								}}
								type="button"
								variant="outline"
							>
								Back
							</Button>
							<Button onClick={onClose} type="button" variant="outline">
								Cancel
							</Button>
						</div>
					</div>
				)}

				{step === 3 && (
					<div className="flex flex-col gap-3">
						<div className="flex items-center justify-between">
							<p className="text-muted-foreground text-xs">
								{keyCount} {keyLabel(keyCount).toLowerCase()} found
							</p>
							<button className="text-muted-foreground text-xs underline-offset-2 hover:underline" onClick={toggleAll} type="button">
								{allChecked ? "Deselect all" : "Select all"}
							</button>
						</div>
						<div className="flex max-h-64 flex-col gap-1 overflow-y-auto">
							{reviewKeys.map((r, idx) => (
								<label className="flex cursor-pointer items-center gap-3 border border-border p-2 hover:bg-muted/20" key={`${r.key.name}-${idx}`}>
									<input checked={r.checked} className="accent-primary" onChange={() => toggleOne(idx)} type="checkbox" />
									<div className="flex min-w-0 flex-1 flex-col gap-0.5">
										<span className="truncate text-sm">{r.key.name}</span>
										<span className="text-muted-foreground text-xs">{detectKeyType(r.key.privateKey)}</span>
									</div>
								</label>
							))}
						</div>
						{error && <p className="text-destructive text-xs">{error}</p>}
						<div className="flex items-center justify-between">
							<Button
								onClick={() => {
									setError(null);
									setReviewKeys([]);
									setStep(2);
								}}
								type="button"
								variant="outline"
							>
								Back
							</Button>
							<div className="flex gap-2">
								<Button onClick={onClose} type="button" variant="outline">
									Cancel
								</Button>
								<Button disabled={checkedCount === 0 || isLoading} onClick={handleImport} type="button">
									{isLoading ? "Importing..." : `Import ${checkedCount} ${keyLabel(checkedCount)}`}
								</Button>
							</div>
						</div>
					</div>
				)}
			</div>
		</div>
	);
};
