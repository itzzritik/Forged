"use client";

import { ChevronDownIcon, ChevronRightIcon, CopyIcon, EyeIcon, EyeOffIcon, PencilIcon, PlusIcon, XIcon } from "lucide-react";
import { type CSSProperties, useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Modal, ModalBody, ModalFooter } from "@/components/ui/modal";
import { cn } from "@/lib/utils";
import { decryptVaultKeyPrivateKey, type VaultKeyDetails } from "@/lib/vault-crypto";

type KeyDetailsMode = "view" | "edit";
type DraftHostRule = VaultKeyDetails["hostRules"][number];

interface KeyDetailsModalProps {
	isSaving?: boolean;
	keyDetails: VaultKeyDetails | null;
	mode: KeyDetailsMode;
	onClose: () => void;
	onModeChange: (mode: KeyDetailsMode) => void;
	onSaveChanges: (updates: { hostRules: DraftHostRule[]; name: string }) => Promise<void> | void;
	open: boolean;
	symmetricKey: CryptoKey | null;
}

interface TimelineEvent {
	id: string;
	subtitle?: string;
	title: string;
}

const keyTypeBadgeStyle = {
	backgroundColor: "color-mix(in srgb, var(--warning) 12%, transparent)",
	borderColor: "color-mix(in srgb, var(--warning) 28%, var(--border) 72%)",
	color: "color-mix(in srgb, var(--warning) 82%, white 18%)",
} satisfies CSSProperties;

function formatTimestamp(value: string, style: "summary" | "detail" = "summary") {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return "Unknown";

	return new Intl.DateTimeFormat(undefined, {
		weekday: style === "detail" ? "long" : undefined,
		month: "short",
		day: "numeric",
		year: "numeric",
		hour: "numeric",
		minute: "2-digit",
	}).format(date);
}

function sameInstant(a?: string, b?: string) {
	if (!a || !b) return false;
	const left = new Date(a).getTime();
	const right = new Date(b).getTime();
	return !Number.isNaN(left) && left === right;
}

function buildTimelineEvents(keyDetails: VaultKeyDetails): TimelineEvent[] {
	const events: TimelineEvent[] = [];

	if (keyDetails.updatedAt && !sameInstant(keyDetails.updatedAt, keyDetails.createdAt)) {
		events.push({
			id: "updated",
			title: `Last edited ${formatTimestamp(keyDetails.updatedAt, "detail")}`,
			subtitle: keyDetails.version ? `Current item version ${keyDetails.version}` : undefined,
		});
	}

	events.push({
		id: "created",
		title: `Added ${formatTimestamp(keyDetails.createdAt, "detail")}`,
	});

	return events;
}

function detectHostRuleType(pattern: string): string {
	if (pattern.includes("/")) return "cidr";
	if (pattern.includes("*")) return "wildcard";
	return "exact";
}

function IconButton({
	icon,
	label,
	onClick,
	variant = "outline",
	disabled,
}: {
	disabled?: boolean;
	icon: React.ReactNode;
	label: string;
	onClick: () => void;
	variant?: "ghost" | "outline";
}) {
	return (
		<Button aria-label={label} disabled={disabled} onClick={onClick} size="icon-sm" title={label} type="button" variant={variant}>
			{icon}
		</Button>
	);
}

function DetailRow({
	action,
	dim = false,
	expanded,
	label,
	mono = false,
	value,
}: {
	action?: React.ReactNode;
	dim?: boolean;
	expanded?: React.ReactNode;
	label: string;
	mono?: boolean;
	value: React.ReactNode;
}) {
	return (
		<div className="overflow-hidden rounded-lg border border-key-details-border bg-key-details-surface shadow-[0_20px_50px_-42px_rgba(0,0,0,0.55)] animate-in fade-in-0 slide-in-from-bottom-2 duration-300">
			<div className="grid min-h-[60px] grid-cols-[112px_minmax(0,1fr)_auto] items-center gap-4 px-4 py-3 sm:min-h-[58px] sm:py-0">
				<p className="font-mono text-[11px] text-muted-foreground uppercase tracking-[0.12em]">{label}</p>
				<div
					className={cn(
						"min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-sm text-foreground",
						mono && "font-mono text-xs sm:text-sm",
						dim && "text-muted-foreground"
					)}
				>
					{value}
				</div>
				{action ? <div className="flex items-center gap-1.5">{action}</div> : <div className="h-7" />}
			</div>
			{expanded ? (
				<div className="border-key-details-border border-t bg-[linear-gradient(180deg,rgba(255,255,255,0.02),transparent_16%),repeating-linear-gradient(180deg,rgba(255,255,255,0.012)_0,rgba(255,255,255,0.012)_1px,transparent_1px,transparent_28px),var(--color-key-details-surface-strong)] px-4 py-4 sm:pl-[8.25rem]">
					{expanded}
				</div>
			) : null}
		</div>
	);
}

function TimelineSection({ events, open, onToggle }: { events: TimelineEvent[]; onToggle: () => void; open: boolean }) {
	const summary = events[0]?.title ?? "Added recently";

	return (
		<div className="pt-1">
			<button className="group flex w-full items-center gap-3 rounded-md px-1 py-1 text-left text-muted-foreground transition-colors hover:text-foreground" onClick={onToggle} type="button">
				{open ? (
					<ChevronDownIcon className="size-4 shrink-0 transition-transform group-hover:text-foreground" />
				) : (
					<ChevronRightIcon className="size-4 shrink-0 transition-transform group-hover:text-foreground" />
				)}
				<span className="text-sm leading-6 sm:text-[15px]">{summary}</span>
			</button>

			{open ? (
				<div className="ml-[0.95rem] mt-2 border-key-details-border border-l pl-4 animate-in fade-in-0 slide-in-from-bottom-2 duration-300">
					<div className="space-y-3 py-1">
						{events.map((event) => (
							<div className="space-y-0.5" key={event.id}>
								<p className="text-[12px] text-foreground/90 leading-5 sm:text-[13px]">{event.title}</p>
								{event.subtitle ? <p className="text-muted-foreground text-[11px] leading-4">{event.subtitle}</p> : null}
							</div>
						))}
					</div>
				</div>
			) : null}
		</div>
	);
}

function HostRulesSummary({ rules }: { rules: DraftHostRule[] }) {
	if (rules.length === 0) {
		return <span className="text-muted-foreground">No host rules</span>;
	}

	return <span className="block overflow-hidden text-ellipsis whitespace-nowrap">{rules.map((rule) => rule.match).join(", ")}</span>;
}

function HostRulesExpandedList({ rules }: { rules: DraftHostRule[] }) {
	return (
		<div className="space-y-2">
			{rules.length === 0 ? <p className="font-mono text-[12px] text-muted-foreground">No host rules</p> : null}
			{rules.map((rule) => (
				<div className="font-mono text-[12px] leading-6 text-foreground" key={`${rule.type}:${rule.match}`}>
					{rule.match}
				</div>
			))}
		</div>
	);
}

function HostRulesEditorSection({
	input,
	onAdd,
	onChangeInput,
	onRemove,
	rules,
}: {
	input: string;
	onAdd: () => void;
	onChangeInput: (value: string) => void;
	onRemove: (match: string) => void;
	rules: DraftHostRule[];
}) {
	return (
		<div className="space-y-4 animate-in fade-in-0 slide-in-from-bottom-2 duration-300">
			<div className="flex gap-2">
				<Input
					className="h-10 flex-1 border-key-details-border bg-key-details-surface font-mono text-sm"
					onChange={(event) => onChangeInput(event.target.value)}
					onKeyDown={(event) => {
						if (event.key === "Enter") {
							event.preventDefault();
							onAdd();
						}
					}}
					placeholder="e.g. github.com, *.github.com, 10.0.0.0/8"
					value={input}
				/>
				<Button className="h-10 shrink-0 px-4" onClick={onAdd} type="button" variant="outline">
					<PlusIcon className="size-4" />
					Add
				</Button>
			</div>

			<div className="min-h-[4.25rem] rounded-lg border border-key-details-border bg-key-details-surface px-3 py-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.02)]">
				<div className="flex min-h-8 flex-wrap gap-2">
					{rules.length === 0 ? (
						<div className="flex min-h-10 w-full items-center justify-center font-mono text-[11px] text-muted-foreground/80 tracking-[0.06em] uppercase">
							No host rules yet
						</div>
					) : (
						rules.map((rule) => (
							<Badge className="gap-2 border-key-details-border bg-key-details-surface px-2.5 py-1 text-foreground hover:bg-key-details-surface" key={`${rule.type}:${rule.match}`} variant="outline">
								<span className="max-w-[16rem] overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[11px]">{rule.match}</span>
								<button className="text-muted-foreground transition-colors hover:text-foreground" onClick={() => onRemove(rule.match)} type="button">
									<XIcon className="size-3" />
								</button>
							</Badge>
						))
					)}
				</div>
			</div>
		</div>
	);
}

export function KeyDetailsModal({ open, keyDetails, mode, onClose, onModeChange, onSaveChanges, symmetricKey, isSaving }: KeyDetailsModalProps) {
	const [draftName, setDraftName] = useState("");
	const [draftRules, setDraftRules] = useState<DraftHostRule[]>([]);
	const [hostRuleInput, setHostRuleInput] = useState("");
	const [revealedPrivateKey, setRevealedPrivateKey] = useState<string | null>(null);
	const [isRevealing, setIsRevealing] = useState(false);
	const [privateKeyError, setPrivateKeyError] = useState<string | null>(null);
	const [timelineOpen, setTimelineOpen] = useState(false);
	const [hostRulesOpen, setHostRulesOpen] = useState(false);

	useEffect(() => {
		setDraftName(keyDetails?.name ?? "");
		setDraftRules(keyDetails?.hostRules ?? []);
		setHostRuleInput("");
	}, [keyDetails?.id, keyDetails?.name, keyDetails?.hostRules, mode]);

	useEffect(() => {
		if (!open) {
			setRevealedPrivateKey(null);
			setPrivateKeyError(null);
			setIsRevealing(false);
			setTimelineOpen(false);
			setHostRulesOpen(false);
			setHostRuleInput("");
		}
	}, [open]);

	useEffect(() => {
		setRevealedPrivateKey(null);
		setPrivateKeyError(null);
		setIsRevealing(false);
		setTimelineOpen(false);
		setHostRulesOpen(false);
		setHostRuleInput("");
	}, [keyDetails?.id, mode]);

	const handleCopy = useCallback(async (value: string, successMessage: string) => {
		try {
			await navigator.clipboard.writeText(value);
			toast.success(successMessage);
		} catch {
			toast.error("Failed to copy value");
		}
	}, []);

	const handleRevealPrivateKey = useCallback(async () => {
		if (!keyDetails || !symmetricKey || isRevealing) return;
		try {
			setIsRevealing(true);
			setPrivateKeyError(null);
			const decrypted = await decryptVaultKeyPrivateKey(keyDetails, symmetricKey);
			setRevealedPrivateKey(decrypted);
		} catch (error) {
			setPrivateKeyError(error instanceof Error ? error.message : "Failed to reveal private key");
		} finally {
			setIsRevealing(false);
		}
	}, [isRevealing, keyDetails, symmetricKey]);

	const handleClose = useCallback(() => {
		setRevealedPrivateKey(null);
		setPrivateKeyError(null);
		setIsRevealing(false);
		setTimelineOpen(false);
		setHostRulesOpen(false);
		setHostRuleInput("");
		onClose();
	}, [onClose]);

	const handleSave = useCallback(async () => {
		const trimmed = draftName.trim();
		if (!trimmed) return;
		await onSaveChanges({ name: trimmed, hostRules: draftRules });
	}, [draftName, draftRules, onSaveChanges]);

	const handleAddHostRule = useCallback(() => {
		const trimmed = hostRuleInput.trim();
		if (!trimmed) return;
		if (draftRules.some((rule) => rule.match === trimmed)) {
			setHostRuleInput("");
			return;
		}
		setDraftRules((current) => [...current, { match: trimmed, type: detectHostRuleType(trimmed) }]);
		setHostRuleInput("");
	}, [draftRules, hostRuleInput]);

	const handleRemoveHostRule = useCallback((match: string) => {
		setDraftRules((current) => current.filter((rule) => rule.match !== match));
	}, []);

	const timelineEvents = useMemo(() => (keyDetails ? buildTimelineEvents(keyDetails) : []), [keyDetails]);
	const hostRulesSummary = useMemo(() => draftRules.map((rule) => rule.match).join(", "), [draftRules]);
	const shouldRevealHostRules = mode === "edit" || draftRules.length > 1 || hostRulesSummary.length > 52;

	if (!keyDetails) {
		return (
			<Modal closable onOpenChange={(next) => !next && handleClose()} open={open} size="xl" title="Keys // Details">
				<ModalBody className="gap-0 p-0">
					<div className="overflow-y-auto px-6 py-6" style={{ maxHeight: "calc(100vh - 6rem)" }}>
						<div className="rounded-lg border border-key-details-border bg-key-details-surface px-4 py-4">
							<p className="font-semibold text-lg text-foreground">Key not found</p>
							<p className="mt-2 text-muted-foreground text-sm">This key is no longer available in the current vault state.</p>
						</div>
					</div>
				</ModalBody>
			</Modal>
		);
	}

	return (
		<Modal
			className="max-h-[calc(100vh-2rem)]"
			closable
			onOpenChange={(next) => !next && handleClose()}
			open={open}
			size="xl"
			title={mode === "edit" ? "Keys // Edit" : "Keys // Details"}
		>
			<ModalBody className="gap-0 p-0">
				<div className="overflow-y-auto px-6 py-6" style={{ maxHeight: "calc(100vh - 6rem)" }}>
					{mode === "view" ? (
						<div className="space-y-4">
							<div className="rounded-lg border border-key-details-border bg-key-details-surface px-5 py-4 shadow-[0_24px_60px_-40px_rgba(0,0,0,0.55)] transition-colors duration-200 animate-in fade-in-0 slide-in-from-bottom-2">
								<div className="flex gap-4 sm:items-center sm:justify-between">
									<div className="min-w-0 flex-1">
										<div className="flex flex-wrap items-center gap-x-3 gap-y-2">
											<h2 className="min-w-0 truncate font-semibold text-2xl tracking-tight text-foreground sm:text-3xl">{keyDetails.name}</h2>
											<Badge className="h-6 rounded-md border px-2.5 font-mono text-[11px] tracking-[0.06em]" style={keyTypeBadgeStyle} variant="outline">
												{keyDetails.type}
											</Badge>
										</div>
										<p className="mt-2 break-all font-mono text-muted-foreground text-xs sm:text-sm">{keyDetails.fingerprint}</p>
									</div>
									<IconButton icon={<PencilIcon className="size-4" />} label="Edit key" onClick={() => onModeChange("edit")} />
								</div>
							</div>

							<DetailRow
								action={<IconButton icon={<CopyIcon className="size-4" />} label="Copy public key" onClick={() => void handleCopy(keyDetails.publicKey, "Public key copied to clipboard")} variant="ghost" />}
								label="Public key"
								mono
								value={keyDetails.publicKey}
							/>

							<DetailRow
								action={
									revealedPrivateKey ? (
										<>
											<IconButton
												icon={<CopyIcon className="size-4" />}
												label="Copy private key"
												onClick={() => void handleCopy(revealedPrivateKey, "Private key copied to clipboard")}
												variant="ghost"
											/>
											<IconButton icon={<EyeOffIcon className="size-4" />} label="Hide private key" onClick={() => setRevealedPrivateKey(null)} />
										</>
									) : (
										<IconButton
											disabled={!symmetricKey || isRevealing}
											icon={<EyeIcon className="size-4" />}
											label={isRevealing ? "Revealing private key" : "Reveal private key"}
											onClick={() => void handleRevealPrivateKey()}
										/>
									)
								}
								dim
								expanded={
									revealedPrivateKey ? (
										<pre className="overflow-x-auto whitespace-pre-wrap break-all font-mono text-[12px] leading-7 text-foreground">{revealedPrivateKey}</pre>
									) : null
								}
								label="Private key"
								mono
								value={<span className="tracking-[0.32em]">••••••••••••</span>}
							/>

							{privateKeyError ? <p className="-mt-2 text-destructive text-xs">{privateKeyError}</p> : null}

							<DetailRow
								action={
									shouldRevealHostRules ? (
										hostRulesOpen ? (
											<IconButton icon={<ChevronDownIcon className="size-4" />} label="Hide host rules" onClick={() => setHostRulesOpen(false)} />
										) : (
											<IconButton icon={<ChevronRightIcon className="size-4" />} label="Reveal host rules" onClick={() => setHostRulesOpen(true)} />
										)
									) : undefined
								}
								expanded={hostRulesOpen ? <HostRulesExpandedList rules={draftRules} /> : null}
								label="Host rules"
								mono
								value={<HostRulesSummary rules={draftRules} />}
							/>

							<TimelineSection events={timelineEvents} onToggle={() => setTimelineOpen((openState) => !openState)} open={timelineOpen} />
						</div>
					) : (
						<div className="space-y-5">
							<div className="rounded-lg border border-key-details-border bg-key-details-surface px-5 py-4 shadow-[0_24px_60px_-40px_rgba(0,0,0,0.55)] transition-colors duration-200 animate-in fade-in-0 slide-in-from-bottom-2">
								<div className="min-w-0">
									<div className="flex flex-wrap items-center gap-x-3 gap-y-2">
										<h2 className="min-w-0 truncate font-semibold text-2xl tracking-tight text-foreground sm:text-3xl">Edit {keyDetails.name}</h2>
										<Badge className="h-6 rounded-md border px-2.5 font-mono text-[11px] tracking-[0.06em]" style={keyTypeBadgeStyle} variant="outline">
											{keyDetails.type}
										</Badge>
									</div>
									<p className="mt-2 break-all font-mono text-muted-foreground text-xs sm:text-sm">{keyDetails.fingerprint}</p>
								</div>
							</div>

							<div className="rounded-lg border border-key-details-border bg-key-details-surface px-5 py-5 shadow-[0_24px_60px_-40px_rgba(0,0,0,0.55)] transition-colors duration-200 animate-in fade-in-0 slide-in-from-bottom-2">
								<div className="space-y-2">
									<label className="font-mono text-[11px] text-muted-foreground uppercase tracking-[0.12em]">Name</label>
									<Input
										autoFocus
										className="h-10 border-key-details-border bg-key-details-surface-strong font-mono text-sm"
										onChange={(event) => setDraftName(event.target.value)}
										onKeyDown={(event) => {
											if (event.key === "Enter" && !isSaving) {
												event.preventDefault();
												void handleSave();
											}
										}}
										value={draftName}
									/>
								</div>
							</div>

							<div className="rounded-lg border border-key-details-border bg-key-details-surface px-5 py-5 shadow-[0_24px_60px_-40px_rgba(0,0,0,0.55)] transition-colors duration-200 animate-in fade-in-0 slide-in-from-bottom-2">
								<div className="mb-3">
									<label className="font-mono text-[11px] text-muted-foreground uppercase tracking-[0.12em]">Host rules</label>
								</div>
								<HostRulesEditorSection
									input={hostRuleInput}
									onAdd={handleAddHostRule}
									onChangeInput={setHostRuleInput}
									onRemove={handleRemoveHostRule}
									rules={draftRules}
								/>
							</div>

							<ModalFooter className="justify-end">
								<Button onClick={() => onModeChange("view")} type="button" variant="ghost">
									Cancel
								</Button>
								<Button disabled={isSaving || draftName.trim().length === 0} onClick={() => void handleSave()} type="button">
									{isSaving ? "Saving..." : "Save"}
								</Button>
							</ModalFooter>
						</div>
					)}
				</div>
			</ModalBody>
		</Modal>
	);
}
