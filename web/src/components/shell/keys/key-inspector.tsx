"use client";

import { HashIcon, PenLineIcon } from "lucide-react";
import { type CSSProperties, useCallback, useEffect, useRef, useState } from "react";
import { useNow } from "@/hooks/use-now";
import { useVaultContext } from "@/hooks/use-vault";
import { ago, formatDate } from "@/lib/format";
import { decryptVaultKeyPrivateKey, getVaultKeyDetails, updateKeyInVault, type VaultKeyMetadata } from "@/lib/vault-crypto";
import { copyText } from "../clipboard";
import { Action } from "../kit/action";
import { CodeBlock, CopyButton } from "../kit/code-block";
import { usePasswordPrompt } from "../kit/password-prompt";
import { say } from "../message";
import { useSync } from "../sync";
import { NAME_LIMIT, validateName } from "./actions";

const SECRET_MS = 60_000;
const DOTS = "•".repeat(35);
const line = (j: number) => ({ "--j": j }) as CSSProperties;

export const inspectorIcon = (item: VaultKeyMetadata) => (item.gitSigning ? PenLineIcon : HashIcon);

interface InspectorProps {
	item: VaultKeyMetadata;
	onDelete: () => void;
	onRenaming: (on: boolean) => void;
}

// Keyed by item.id by the parent, so switching keys drops any revealed secret.
export function KeyInspector({ item, onDelete, onRenaming }: InspectorProps) {
	const now = useNow(60_000);
	const used = ago(item.lastUsedAt, now);
	return (
		<div className="inspector">
			<dl className="inspector-kv" style={line(0)}>
				<dt>Signing</dt>
				<dd>{item.gitSigning ? "Signs your Git commits" : "Not used for signing"}</dd>
				<dt>Last used</dt>
				<dd>{used ?? "Never used"}</dd>
				<dt>Created</dt>
				<dd>{formatDate(item.createdAt)}</dd>
				{item.comment && (
					<>
						<dt>Comment</dt>
						<dd className="truncate font-mono">{item.comment}</dd>
					</>
				)}
			</dl>
			<CodeBlock copy={`Copied the fingerprint of ${item.name}`} label="Fingerprint" style={line(1)} value={item.fingerprint} />
			<CodeBlock copy={`Copied the public key of ${item.name}`} label="Public key" style={line(2)} value={item.publicKey} />
			<PrivateKey item={item} style={line(3)} />
			<div className="inspector-actions" style={line(4)}>
				<CopyPublic item={item} />
				<Action combo="r" onClick={() => onRenaming(true)}>
					Rename
				</Action>
				<Action combo="d" onClick={onDelete} tone="danger">
					Delete
				</Action>
			</div>
		</div>
	);
}

function CopyPublic({ item }: { item: VaultKeyMetadata }) {
	const button = useRef<HTMLButtonElement>(null);
	const [copied, setCopied] = useState(false);
	useEffect(() => {
		if (!copied) return;
		const timer = setTimeout(() => setCopied(false), 1600);
		return () => clearTimeout(timer);
	}, [copied]);
	const copy = async () => {
		const from = button.current?.querySelector(".keycap") ?? button.current;
		setCopied(await copyText(item.publicKey, `Copied the public key of ${item.name}`, from));
	};
	return (
		<Action combo="c" data-copied={copied ? "" : undefined} data-copy-public="" onClick={() => void copy()} ref={button} tone="primary">
			<span className="roll">
				<span>Copy public</span>
				<span aria-hidden>Copied</span>
			</span>
		</Action>
	);
}

function PrivateKey({ item, style }: { item: VaultKeyMetadata; style: CSSProperties }) {
	const { symmetricKeyRef, vaultData } = useVaultContext();
	const confirm = usePasswordPrompt();
	const [secret, setSecret] = useState<string | null>(null);
	const [busy, setBusy] = useState(false);
	const hide = useCallback(() => setSecret(null), []);

	const reveal = async () => {
		if (busy || !(await confirm(`Enter your master password to reveal the private key of ${item.name}.`))) return;
		const details = vaultData ? getVaultKeyDetails(vaultData, item.id) : null;
		const key = symmetricKeyRef.current;
		if (!(details && key)) return;
		setBusy(true);
		try {
			setSecret(await decryptVaultKeyPrivateKey(details, key));
		} catch {
			say("Couldn't decrypt this key. Lock and unlock, then try again.", "danger");
		} finally {
			setBusy(false);
		}
	};

	if (secret) return <Secret item={item} onHide={hide} style={style} value={secret} />;
	return (
		<CodeBlock
			action={
				<button className="text-foreground transition-colors hover:text-primary" disabled={busy} onClick={() => void reveal()} type="button">
					{busy ? "decrypting" : "reveal"}
				</button>
			}
			label="Private key"
			style={style}
		>
			<span className="block truncate text-faint">{DOTS}</span>
		</CodeBlock>
	);
}

function Secret({ item, onHide, style, value }: { item: VaultKeyMetadata; onHide: () => void; style: CSSProperties; value: string }) {
	const [deadline] = useState(() => Date.now() + SECRET_MS);
	const now = useNow(1000);
	// Wall clock, not ticks: a throttled background tab must not keep the key visible longer.
	const left = Math.max(0, Math.ceil((deadline - now) / 1000));
	useEffect(() => {
		if (left === 0) onHide();
	}, [left, onHide]);
	return (
		<CodeBlock
			action={
				<span className="flex items-center gap-3">
					<span className="text-faint tabular-nums">hides in {left}s</span>
					<CopyButton message={`Copied the private key of ${item.name}`} text={value} />
					<button className="text-foreground transition-colors hover:text-primary" onClick={onHide} type="button">
						hide
					</button>
				</span>
			}
			label="Private key"
			style={style}
		>
			<pre className="max-h-44 overflow-auto whitespace-pre-wrap break-all">{value}</pre>
		</CodeBlock>
	);
}

export function RenameField({ item, onDone }: { item: VaultKeyMetadata; onDone: () => void }) {
	const { deviceId, vaultData } = useVaultContext();
	const { commit } = useSync();
	const [name, setName] = useState(item.name);
	const [saving, setSaving] = useState(false);
	const input = useRef<HTMLInputElement>(null);

	useEffect(() => {
		input.current?.select();
	}, []);

	const save = async () => {
		const trimmed = name.trim();
		if (trimmed === item.name) {
			onDone();
			return;
		}
		const problem = validateName(trimmed, vaultData?.keys ?? [], item.id);
		if (problem) {
			say(problem, "danger");
			return;
		}
		if (!vaultData) return;
		setSaving(true);
		const ok = await commit(updateKeyInVault(vaultData, item.id, { name: trimmed }, deviceId));
		setSaving(false);
		if (!ok) return;
		say(`Renamed to ${trimmed}`);
		onDone();
	};

	return (
		<input
			aria-label="Key name"
			className="rename-input"
			maxLength={NAME_LIMIT + 1}
			onBlur={() => {
				if (!saving) onDone();
			}}
			onChange={(event) => setName(event.target.value)}
			onKeyDown={(event) => {
				if (event.key === "Enter") {
					event.preventDefault();
					void save();
				}
				if (event.key === "Escape") {
					event.preventDefault();
					onDone();
				}
			}}
			readOnly={saving}
			ref={input}
			spellCheck={false}
			value={name}
		/>
	);
}
