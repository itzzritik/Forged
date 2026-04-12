"use client";

import { useCallback, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { useVaultContext } from "@/hooks/use-vault";
import { parseBitwarden } from "@/lib/importers/bitwarden";
import { parseForged } from "@/lib/importers/forged-format";
import { parse1Password } from "@/lib/importers/onepassword";
import type { ImportedKey } from "@/lib/importers/types";
import { parseSSHKeyFile, type ParsedSSHKey } from "@/lib/ssh-key-parser";
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

interface PreparedImportKey {
	key: ImportedKey;
	parsed: ParsedSSHKey;
}

interface ReviewKey {
	checked: boolean;
	entry: PreparedImportKey;
}

const SOURCES: SourceMeta[] = [
	{
		id: "1password",
		title: "1Password (.1pux, .csv)",
		subtitle: "Import from 1Password export. .1pux is recommended; .csv is best-effort.",
		instruction: "Select a 1Password export. .1pux is preferred. .csv is supported when rows contain embedded private keys.",
		accept: ".1pux,.csv,text/csv",
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
		subtitle: "Import a single OpenSSH, PKCS#8 PEM, or legacy PEM private key",
		instruction: "Select an unencrypted OpenSSH, PKCS#8 PEM, or legacy PEM private key file (e.g. id_ed25519, id_rsa, github_rsa.pem)",
		accept: "*",
	},
];

const EXT_RE = /\.(pem|key|pub)$/i;
const ID_PREFIX_RE = /^id_/;
const NON_SLUG_RE = /[^a-z0-9_-]/gi;

function deriveNameFromFile(filename: string): string {
	return filename.replace(EXT_RE, "").replace(ID_PREFIX_RE, "").replace(NON_SLUG_RE, "-").toLowerCase() || "imported";
}

function formatParseError(msg: string): string {
	if (msg === "PASSPHRASE_PROTECTED") return "Key is passphrase-protected. Remove the passphrase first:\nssh-keygen -p -f <file>";
	if (msg === "UNKNOWN_FORMAT") return "Unrecognized key format. Only unencrypted OpenSSH or PEM private keys are supported.";
	return msg;
}

function importFormatLabel(parsed: ParsedSSHKey): string {
	switch (parsed.sourceFormat) {
		case "openssh":
			return "OpenSSH";
		case "pkcs8-pem":
			return "PKCS#8 PEM -> OpenSSH";
		case "legacy-pem":
			return "Legacy PEM -> OpenSSH";
	}
}

async function prepareImportedKeys(keys: ImportedKey[]): Promise<PreparedImportKey[]> {
	return Promise.all(
		keys.map(async (key) => ({
			key,
			parsed: await parseSSHKeyFile(key.privateKey),
		}))
	);
}

async function parseSource(source: Source, file: File): Promise<PreparedImportKey[]> {
	if (source === "1password") {
		const buffer = await file.arrayBuffer();
		return prepareImportedKeys(parse1Password(new Uint8Array(buffer)));
	}
	if (source === "bitwarden") {
		return prepareImportedKeys(parseBitwarden(await file.text()));
	}
	if (source === "forged") {
		return prepareImportedKeys(parseForged(await file.text()));
	}
	// ssh
	const text = await file.text();
	const parsed = await parseSSHKeyFile(text);
	const name = parsed.comment || deriveNameFromFile(file.name);
	return [{ key: { name, privateKey: text }, parsed }];
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
				const entries = await parseSource(source, file);
				if (entries.length === 0) {
					setError("No SSH keys found in the selected file.");
					return;
				}
				setReviewKeys(entries.map((entry) => ({ entry, checked: true })));
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
	const convertedCount = reviewKeys.filter((r) => r.checked && r.entry.parsed.convertedToOpenSSH).length;
	const pkcs8ConvertedCount = reviewKeys.filter((r) => r.checked && r.entry.parsed.sourceFormat === "pkcs8-pem").length;
	const legacyConvertedCount = reviewKeys.filter((r) => r.checked && r.entry.parsed.sourceFormat === "legacy-pem").length;
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

			for (const { entry, checked } of reviewKeys) {
				if (!checked) continue;

				const { key, parsed } = entry;
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
								<label className="flex cursor-pointer items-center gap-3 border border-border p-2 hover:bg-muted/20" key={`${r.entry.key.name}-${idx}`}>
									<input checked={r.checked} className="accent-primary" onChange={() => toggleOne(idx)} type="checkbox" />
									<div className="flex min-w-0 flex-1 flex-col gap-0.5">
										<span className="truncate text-sm">{r.entry.key.name}</span>
										<div className="flex flex-wrap items-center gap-2 text-xs">
											<span className="text-muted-foreground">{r.entry.parsed.type}</span>
											<span className={r.entry.parsed.convertedToOpenSSH ? "text-amber-500" : "text-muted-foreground"}>{importFormatLabel(r.entry.parsed)}</span>
										</div>
									</div>
								</label>
							))}
						</div>
						{convertedCount > 0 && (
							<div className="border border-amber-500/30 bg-amber-500/10 p-3 text-amber-200 text-xs">
								<p>
									{pkcs8ConvertedCount > 0 && legacyConvertedCount > 0
										? `${convertedCount} PEM keys will be converted to the latest OpenSSH private key format. This includes ${pkcs8ConvertedCount} PKCS#8 PEM key${pkcs8ConvertedCount === 1 ? "" : "s"} and ${legacyConvertedCount} legacy PEM key${legacyConvertedCount === 1 ? "" : "s"}.`
										: pkcs8ConvertedCount > 0
											? `${pkcs8ConvertedCount} PKCS#8 PEM key${pkcs8ConvertedCount === 1 ? "" : "s"} will be converted to the latest OpenSSH private key format.`
											: `${legacyConvertedCount} legacy PEM key${legacyConvertedCount === 1 ? "" : "s"} will be converted to the latest OpenSSH private key format.`}
								</p>
								<p className="mt-1 text-amber-100/90">The keypair stays the same, so existing GitHub/server setups continue to work.</p>
							</div>
						)}
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
