"use client";

import { ArrowDownToLineIcon } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useVaultContext } from "@/hooks/use-vault";
import { middle, plural } from "@/lib/format";
import { Action } from "../kit/action";
import { type Column, DataTable } from "../kit/data-table";
import { ShellDialog } from "../kit/dialog";
import { say } from "../message";
import { useSync } from "../sync";
import { addImported, type Candidate, importError, parseImport, SOURCES, type Source } from "./import";

type Step = "source" | "file" | "review";

const STEPS: { id: Step; label: string }[] = [
	{ id: "source", label: "Source" },
	{ id: "file", label: "File" },
	{ id: "review", label: "Review" },
];

const fingerprintOf = (candidate: Candidate) => candidate.fingerprint;

const COLUMNS: Column<Candidate>[] = [
	{ cell: (c) => <span className={c.duplicate ? "text-faint" : undefined}>{c.name}</span>, header: "Name", id: "name", width: "minmax(0,1fr)" },
	{ cell: (c) => <span className="text-muted-foreground">{c.parsed.type}</span>, header: "Type", id: "type", wide: true, width: "70px" },
	{
		align: "end",
		cell: (c) => <span className="text-faint">{c.duplicate ? "already in vault" : middle(c.fingerprint, 11, 4)}</span>,
		header: "Fingerprint",
		id: "fp",
		width: "130px",
	},
];

const wipe = (list: Candidate[]) => {
	for (const candidate of list) candidate.parsed.privateKeyBytes.fill(0);
};

const stepState = (index: number, at: number) => {
	if (index === at) return "on";
	return index < at ? "done" : undefined;
};

interface ImportDialogProps {
	onImported: (ids: string[]) => void;
	onOpenChange: (open: boolean) => void;
	open: boolean;
}

export function ImportDialog({ onImported, onOpenChange, open }: ImportDialogProps) {
	const { deviceId, symmetricKeyRef, vaultData } = useVaultContext();
	const { commit } = useSync();
	const [step, setStep] = useState<Step>("source");
	const [source, setSource] = useState<Source>("ssh");
	const [candidates, setCandidates] = useState<Candidate[]>([]);
	const [picked, setPicked] = useState<Set<string>>(() => new Set());
	const [failed, setFailed] = useState(0);
	const [error, setError] = useState<string | null>(null);
	const [progress, setProgress] = useState<number | null>(null);
	const [dragging, setDragging] = useState(false);
	const live = useRef<Candidate[]>([]);
	// Bumped on close and Back so a parse that finishes late is dropped and wiped.
	const generation = useRef(0);
	const fileInput = useRef<HTMLInputElement>(null);
	const existing = useMemo(() => new Set((vaultData?.keys ?? []).map((key) => key.fingerprint)), [vaultData]);
	const current = SOURCES.find((item) => item.id === source) ?? SOURCES[0];
	const at = STEPS.findIndex((item) => item.id === step);
	const dupes = candidates.filter((c) => c.duplicate).length;
	const encrypting = progress !== null;

	const replace = (next: Candidate[]) => {
		wipe(live.current);
		live.current = next;
		setCandidates(next);
	};

	// Parsed private keys must not outlive the dialog.
	useEffect(() => {
		if (open) return;
		generation.current += 1;
		wipe(live.current);
		live.current = [];
		setCandidates([]);
		setPicked(new Set());
		setStep("source");
		setError(null);
		setFailed(0);
		setProgress(null);
	}, [open]);

	useEffect(() => () => wipe(live.current), []);

	const read = async (file: File | undefined) => {
		if (!file) return;
		setError(null);
		const mine = ++generation.current;
		try {
			const result = await parseImport(source, file, existing);
			if (mine !== generation.current) {
				wipe(result.candidates);
				return;
			}
			replace(result.candidates);
			setFailed(result.failed);
			setPicked(new Set(result.candidates.filter((c) => !c.duplicate).map(fingerprintOf)));
			setStep("review");
		} catch (cause) {
			if (mine === generation.current) setError(importError(cause));
		}
	};

	const submit = async () => {
		if (step === "source") {
			setStep("file");
			return;
		}
		if (step === "file") return;
		const key = symmetricKeyRef.current;
		const picks = candidates.filter((c) => picked.has(c.fingerprint));
		if (!(vaultData && key) || encrypting) return;
		if (picks.length === 0) {
			setError("Pick at least one key to import.");
			return;
		}
		setProgress(0);
		setError(null);
		try {
			const { ids, next } = await addImported(vaultData, key, picks, deviceId, setProgress);
			if (!(await commit(next, { pin: false }))) {
				setError("Couldn't sync. Nothing was imported. Try again.");
				return;
			}
			const left = candidates.length - picks.length;
			onOpenChange(false);
			onImported(ids);
			say([`Imported ${plural(ids.length, "key")}`, left ? `left out ${left}` : "", failed ? `${failed} couldn't be read` : ""].filter(Boolean).join(", "));
		} catch {
			setError("Couldn't encrypt the keys. Nothing was imported.");
		} finally {
			setProgress(null);
		}
	};

	// Closing mid-encryption would wipe bytes addImported is still reading and sync zeroed keys.
	const close = (next: boolean) => {
		if (next || !encrypting) onOpenChange(next);
	};

	const back = () => {
		generation.current += 1;
		if (step === "review") {
			replace([]);
			setStep("file");
		} else setStep("source");
		setError(null);
	};

	let actions = (
		<Action combo="enter" tone="primary" type="submit">
			Continue
		</Action>
	);
	if (step === "file") {
		actions = (
			<Action onClick={() => fileInput.current?.click()} tone="primary">
				Browse
			</Action>
		);
	}
	if (step === "review") {
		actions = (
			<Action combo="enter" disabled={encrypting} tone="primary" type="submit">
				{encrypting ? `Encrypting ${progress} of ${picked.size}` : `Import ${plural(picked.size, "key")}`}
			</Action>
		);
	}

	return (
		<ShellDialog
			actions={actions}
			aside={step === "source" ? undefined : current.title}
			cancelLabel={step === "source" ? "Cancel" : "Back"}
			className={step === "review" ? "sm:max-w-[560px]" : undefined}
			error={error}
			icon={ArrowDownToLineIcon}
			onCancel={step === "source" || encrypting ? undefined : back}
			onOpenChange={close}
			onSubmit={() => void submit()}
			open={open}
			title="Import keys"
		>
			<div className="dialog-steps">
				{STEPS.map((item, index) => (
					<span data-state={stepState(index, at)} key={item.id}>
						{index + 1} {item.label}
					</span>
				))}
			</div>
			{step === "source" && (
				<fieldset className="grid gap-1.5">
					<legend className="sr-only">Import from</legend>
					{SOURCES.map((item) => (
						<label className="source-option" key={item.id}>
							<input checked={item.id === source} className="sr-only" name="import-source" onChange={() => setSource(item.id)} type="radio" />
							<span className="font-medium font-sans text-[13.5px] text-foreground">{item.title}</span>
							<span className="text-[12.5px] text-muted-foreground">{item.hint}</span>
						</label>
					))}
				</fieldset>
			)}
			{step === "file" && (
				<>
					<button
						className="drop-zone"
						data-dragging={dragging ? "" : undefined}
						onClick={() => fileInput.current?.click()}
						onDragLeave={() => setDragging(false)}
						onDragOver={(event) => {
							event.preventDefault();
							setDragging(true);
						}}
						onDrop={(event) => {
							event.preventDefault();
							setDragging(false);
							void read(event.dataTransfer.files[0]);
						}}
						type="button"
					>
						<ArrowDownToLineIcon aria-hidden className="size-5 text-primary" />
						<span className="font-medium font-sans text-[13.5px]">Drop the file here or browse</span>
						<span className="text-[12.5px] text-muted-foreground">{current.hint}</span>
					</button>
					<input
						accept={current.accept === "*" ? undefined : current.accept}
						aria-label={`Choose a ${current.title} file`}
						className="sr-only"
						onChange={(event) => {
							void read(event.target.files?.[0]);
							event.target.value = "";
						}}
						ref={fileInput}
						tabIndex={-1}
						type="file"
					/>
				</>
			)}
			{step === "review" && (
				<>
					<p className="text-[13.5px] text-muted-foreground">
						Found {plural(candidates.length, "key")}.{dupes > 0 && ` ${dupes} already in your vault ${dupes === 1 ? "stays" : "stay"} unchecked.`}
						{failed > 0 && ` ${failed} couldn't be read.`}
					</p>
					<div className="flex h-[260px] flex-col">
						<DataTable
							columns={COLUMNS}
							getId={fingerprintOf}
							keyboard={false}
							label="Keys to import"
							onSelectedChange={setPicked}
							rows={candidates}
							selected={picked}
							selectMode="always"
						/>
					</div>
				</>
			)}
		</ShellDialog>
	);
}
