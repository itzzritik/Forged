"use client";

import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Modal, ModalBody, ModalFooter } from "@/components/ui/modal";

const CONFIRM_TEXT = "DELETE";

interface BulkDeleteKeysModalProps {
	isDeleting?: boolean;
	keyNames: string[];
	onClose: () => void;
	onConfirm: () => void | Promise<void>;
	open: boolean;
}

export function BulkDeleteKeysModal({ open, keyNames, isDeleting, onClose, onConfirm }: BulkDeleteKeysModalProps) {
	const [value, setValue] = useState("");

	useEffect(() => {
		if (open) setValue("");
	}, [open]);

	const previewNames = useMemo(() => keyNames.slice(0, 5), [keyNames]);
	const canDelete = value === CONFIRM_TEXT && !isDeleting;

	return (
		<Modal closable onOpenChange={onClose} open={open} size="md" title="Bulk Delete">
			<ModalBody>
				<div className="space-y-4">
					<div className="space-y-2">
						<p className="font-semibold text-lg text-foreground">Delete {keyNames.length} SSH Keys</p>
						<p className="text-muted-foreground text-sm">This action cannot be undone. Type DELETE to confirm permanent removal.</p>
					</div>

					<div className="space-y-1 border border-border bg-background/60 px-3 py-2">
						{previewNames.map((name) => (
							<p className="truncate font-mono text-sm text-foreground" key={name}>
								{name}
							</p>
						))}
						{keyNames.length > previewNames.length && (
							<p className="pt-1 text-muted-foreground text-xs">and {keyNames.length - previewNames.length} more</p>
						)}
					</div>

					<div className="space-y-2">
						<label className="font-mono text-[11px] text-muted-foreground uppercase tracking-[0.12em]">Type DELETE to confirm</label>
						<Input autoComplete="off" onChange={(event) => setValue(event.target.value)} value={value} />
					</div>
				</div>
				<ModalFooter>
					<Button onClick={onClose} variant="ghost">
						Cancel
					</Button>
					<Button disabled={!canDelete} onClick={() => void onConfirm()} variant="destructive">
						Delete Keys
					</Button>
				</ModalFooter>
			</ModalBody>
		</Modal>
	);
}
