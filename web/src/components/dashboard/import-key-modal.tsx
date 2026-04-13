"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import { badgeVariants } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Modal, ModalBody, ModalFooter } from "@/components/ui/modal";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useVaultContext } from "@/hooks/use-vault";
import { parseBitwarden } from "@/lib/importers/bitwarden";
import { parseForged } from "@/lib/importers/forged-format";
import { normalizeImportedName } from "@/lib/importers/name";
import { parse1Password } from "@/lib/importers/onepassword";
import type { ImportedKey } from "@/lib/importers/types";
import { parseSSHKeyFile, type ParsedSSHKey } from "@/lib/ssh-key-parser";
import { computeFingerprintFromBlob, formatSSHPublicKeyFromBlob } from "@/lib/ssh-key-utils";
import { cn } from "@/lib/utils";
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
	fingerprint: string;
	key: ImportedKey;
	parsed: ParsedSSHKey;
}

interface ReviewKey {
	alreadyInVault: boolean;
	checked: boolean;
	collapsedDuplicates: number;
	entry: PreparedImportKey;
}

const REVIEW_LIST_HEIGHT = 360;
const REVIEW_ROW_HEIGHT = 92;
const REVIEW_OVERSCAN = 6;

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

function deriveNameFromFile(filename: string): string {
	return normalizeImportedName(filename.replace(EXT_RE, "").replace(ID_PREFIX_RE, "").replace(/_/g, " "));
}

function formatParseError(msg: string): string {
	if (msg === "PASSPHRASE_PROTECTED") return "Key is passphrase-protected. Remove the passphrase first:\nssh-keygen -p -f <file>";
	if (msg === "UNKNOWN_FORMAT") return "Unrecognized key format. Only unencrypted OpenSSH or PEM private keys are supported.";
	return msg;
}

async function prepareImportedKeys(keys: ImportedKey[]): Promise<PreparedImportKey[]> {
	return Promise.all(
		keys.map(async (key) => {
			const parsed = await parseSSHKeyFile(key.privateKey);
			return {
				key,
				parsed,
				fingerprint: await computeFingerprintFromBlob(parsed.publicKeyBlob),
			};
		})
	);
}

function buildReviewKeys(entries: PreparedImportKey[], existingFingerprints: Set<string>): ReviewKey[] {
	const byFingerprint = new Map<string, ReviewKey>();
	const review: ReviewKey[] = [];

	for (const entry of entries) {
		const existing = byFingerprint.get(entry.fingerprint);
		if (existing) {
			existing.collapsedDuplicates++;
			continue;
		}

		const alreadyInVault = existingFingerprints.has(entry.fingerprint);
		const item: ReviewKey = {
			entry,
			checked: !alreadyInVault,
			alreadyInVault,
			collapsedDuplicates: 0,
		};
		byFingerprint.set(entry.fingerprint, item);
		review.push(item);
	}

	return review.sort((left, right) => Number(left.alreadyInVault) - Number(right.alreadyInVault));
}

function truncateMiddle(value: string, leading = 18, trailing = 10): string {
	if (value.length <= leading + trailing + 1) return value;
	return `${value.slice(0, leading)}...${value.slice(-trailing)}`;
}

function getUpgradeTooltip(parsed: ParsedSSHKey): string {
	if (parsed.sourceFormat === "pkcs8-pem") {
		return "PKCS#8 PEM detected. This key will be upgraded to the OpenSSH private key format";
	}
	if (parsed.sourceFormat === "legacy-pem") {
		return "Legacy PEM detected. This key will be upgraded to the OpenSSH private key format";
	}
	return "This key is already in the OpenSSH private key format";
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
	return [{ key: { name, privateKey: text }, parsed, fingerprint: await computeFingerprintFromBlob(parsed.publicKeyBlob) }];
}

export const ImportKeyModal = ({ onClose }: ImportKeyModalProps) => {
	const { deviceId, vaultData, symmetricKeyRef, pushVault } = useVaultContext();
	const [step, setStep] = useState<Step>(1);
	const [source, setSource] = useState<Source | null>(null);
	const [reviewKeys, setReviewKeys] = useState<ReviewKey[]>([]);
	const [error, setError] = useState<string | null>(null);
	const [isDragOver, setIsDragOver] = useState(false);
	const [isLoading, setIsLoading] = useState(false);
	const [scrollTop, setScrollTop] = useState(0);
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
				const existingFingerprints = new Set((vaultData?.keys ?? []).map((key) => key.fingerprint));
				setReviewKeys(buildReviewKeys(entries, existingFingerprints));
				setScrollTop(0);
				setStep(3);
			} catch (err) {
				const msg = err instanceof Error ? err.message : "Failed to parse file";
				setError(formatParseError(msg));
			}
		},
		[source, vaultData]
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
	const convertedCount = reviewKeys.filter((r) => r.entry.parsed.convertedToOpenSSH).length;
	const hasVaultDuplicates = reviewKeys.some((r) => r.alreadyInVault);
	const collapsedDuplicateCount = reviewKeys.reduce((sum, reviewKey) => sum + reviewKey.collapsedDuplicates, 0);
	const allChecked = reviewKeys.length > 0 && checkedCount === reviewKeys.length;
	const duplicateSelected = reviewKeys.some((reviewKey) => reviewKey.alreadyInVault && reviewKey.checked);
	const keyCount = reviewKeys.length;
	const keyLabel = (n: number) => (n === 1 ? "Key" : "Keys");
	const primaryImportLabel =
		checkedCount === 0 ? "Import Keys" : checkedCount === keyCount ? (keyCount === 1 ? "Import Key" : "Import All Keys") : `Import ${checkedCount.toLocaleString()} ${keyLabel(checkedCount)}`;
	const bulkToggleLabel = hasVaultDuplicates
		? allChecked
			? "Deselect all"
			: duplicateSelected
				? "Select all"
				: "Select all unique"
		: allChecked
			? "Deselect all"
			: "Select all";
	const summaryLines = [
		hasVaultDuplicates ? `${reviewKeys.filter((reviewKey) => reviewKey.alreadyInVault).length.toLocaleString()} duplicates in vault` : null,
		convertedCount > 0 ? `${convertedCount.toLocaleString()} keys will upgrade to OpenSSH` : null,
		collapsedDuplicateCount > 0 ? `${collapsedDuplicateCount.toLocaleString()} duplicate entries in this import were consolidated` : null,
	].filter((value): value is string => Boolean(value));
	const startIndex = Math.max(0, Math.floor(scrollTop / REVIEW_ROW_HEIGHT) - REVIEW_OVERSCAN);
	const endIndex = Math.min(keyCount, Math.ceil((scrollTop + REVIEW_LIST_HEIGHT) / REVIEW_ROW_HEIGHT) + REVIEW_OVERSCAN);
	const visibleReviewKeys = useMemo(() => reviewKeys.slice(startIndex, endIndex), [endIndex, reviewKeys, startIndex]);
	const topSpacerHeight = startIndex * REVIEW_ROW_HEIGHT;
	const bottomSpacerHeight = Math.max(0, (keyCount - endIndex) * REVIEW_ROW_HEIGHT);

	const toggleAll = () => {
		setReviewKeys((prev) => {
			if (!hasVaultDuplicates) {
				const nextChecked = !prev.every((reviewKey) => reviewKey.checked);
				return prev.map((reviewKey) => ({ ...reviewKey, checked: nextChecked }));
			}

			const everyChecked = prev.every((reviewKey) => reviewKey.checked);
			const anyDuplicateSelected = prev.some((reviewKey) => reviewKey.alreadyInVault && reviewKey.checked);

			if (everyChecked) {
				return prev.map((reviewKey) => ({ ...reviewKey, checked: false }));
			}
			if (anyDuplicateSelected) {
				return prev.map((reviewKey) => ({ ...reviewKey, checked: true }));
			}
			return prev.map((reviewKey) => ({ ...reviewKey, checked: !reviewKey.alreadyInVault }));
		});
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

				const { fingerprint, key, parsed } = entry;
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

	return (
		<Modal onOpenChange={(open) => !open && onClose()} open size={step === 3 ? "lg" : "sm"} title="Keys // Import">
			<ModalBody className="gap-3">
				<div className="space-y-1">
					<p className="font-semibold text-lg">Import SSH Keys</p>
				</div>

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
						<ModalFooter className="justify-end pt-1">
							<Button onClick={onClose} type="button" variant="outline">
								Cancel
							</Button>
						</ModalFooter>
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
						<ModalFooter>
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
						</ModalFooter>
					</div>
				)}

				{step === 3 && (
					<div className="flex flex-col gap-3">
						<div className="flex items-center justify-between">
							<p className="text-muted-foreground text-xs">
								{keyCount.toLocaleString()} {keyLabel(keyCount).toLowerCase()} found
							</p>
							<button className="text-muted-foreground text-xs underline-offset-2 transition-colors hover:text-foreground hover:underline" onClick={toggleAll} type="button">
								{bulkToggleLabel}
							</button>
						</div>
						<div className="max-h-[360px] overflow-y-auto pr-1" onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}>
							<div className="flex flex-col gap-3" style={{ paddingTop: topSpacerHeight, paddingBottom: bottomSpacerHeight }}>
								{visibleReviewKeys.map((reviewKey, visibleIndex) => {
									const idx = startIndex + visibleIndex;
									const isChecked = reviewKey.checked;
									const duplicateTooltip = "This key already exists in this vault";
									const fingerprint = truncateMiddle(reviewKey.entry.fingerprint);

									return (
										<div
											className={cn(
												"flex min-h-[80px] items-center gap-3 border bg-surface px-4 py-3.5 shadow-sm transition-colors",
												isChecked ? "border-primary/25 bg-surface-hover" : "border-border text-muted-foreground",
												reviewKey.alreadyInVault && !isChecked && "border-info/20",
												!isChecked && "hover:border-border",
												isChecked && "hover:border-primary/35"
											)}
											key={`${reviewKey.entry.fingerprint}-${idx}`}
										>
											<input
												checked={isChecked}
												className="mt-1 size-4 shrink-0 accent-primary"
												onChange={() => toggleOne(idx)}
												type="checkbox"
											/>
											<div className="min-w-0 flex-1">
												<div className="flex items-center justify-between gap-3">
													<button className="min-w-0 flex-1 text-left" onClick={() => toggleOne(idx)} type="button">
														<div className={cn("truncate text-sm", isChecked ? "text-foreground" : "text-muted-foreground")}>{reviewKey.entry.key.name}</div>
														<div className="mt-1 truncate font-mono text-muted-foreground text-xs">{fingerprint}</div>
													</button>
													<div className="flex shrink-0 flex-wrap items-center justify-end gap-1.5 self-center">
														{reviewKey.alreadyInVault && (
															<Tooltip>
																<TooltipTrigger
																	className={cn(
																		badgeVariants({ variant: "outline" }),
																		"cursor-default border-info/25 bg-info/10 text-info hover:bg-info/10"
																	)}
																	render={<button type="button" />}
																>
																	Duplicate
																</TooltipTrigger>
																<TooltipContent>
																	<span>{duplicateTooltip}</span>
																</TooltipContent>
															</Tooltip>
														)}
														{reviewKey.entry.parsed.convertedToOpenSSH && (
															<Tooltip>
																<TooltipTrigger
																	className={cn(
																		badgeVariants({ variant: "outline" }),
																		"cursor-default border-warning/25 bg-warning/10 text-warning hover:bg-warning/10"
																	)}
																	render={<button type="button" />}
																>
																	Upgrade
																</TooltipTrigger>
																<TooltipContent>
																	<span>{getUpgradeTooltip(reviewKey.entry.parsed)}</span>
																</TooltipContent>
															</Tooltip>
														)}
													</div>
												</div>
											</div>
										</div>
									);
								})}
							</div>
						</div>
						{summaryLines.length > 0 && (
							<div className="mb-4 mt-2 border border-primary/15 bg-linear-to-b from-primary/8 to-surface/60 px-4 py-3 shadow-sm">
								<p className="mb-1.5 text-[11px] text-primary/80 uppercase tracking-[0.16em]">Import Summary</p>
								<div className="flex flex-col gap-0.5 text-muted-foreground text-xs leading-5">
									{summaryLines.map((line) => (
										<p key={line}>{line}</p>
									))}
								</div>
							</div>
						)}
						{error && <p className="text-destructive text-xs">{error}</p>}
						<ModalFooter>
							<Button
								onClick={() => {
									setError(null);
									setReviewKeys([]);
									setScrollTop(0);
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
									{isLoading ? "Importing..." : primaryImportLabel}
								</Button>
							</div>
						</ModalFooter>
					</div>
				)}
			</ModalBody>
		</Modal>
	);
};
