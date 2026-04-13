"use client";

import { Button } from "@/components/ui/button";
import { Modal, ModalBody, ModalFooter } from "@/components/ui/modal";

interface DeleteKeyModalProps {
	fingerprint: string;
	isDeleting?: boolean;
	keyName: string;
	onClose: () => void;
	onConfirm: () => void | Promise<void>;
	open: boolean;
}

export function DeleteKeyModal({ open, keyName, fingerprint, isDeleting, onClose, onConfirm }: DeleteKeyModalProps) {
	return (
		<Modal closable onOpenChange={onClose} open={open} size="sm" title="Delete Key">
			<ModalBody>
				<div className="space-y-3">
					<p className="font-semibold text-lg text-foreground">Delete SSH Key</p>
					<p className="text-muted-foreground text-sm">This will permanently remove the selected key from your vault.</p>
					<div className="space-y-1 border border-border bg-background/60 px-3 py-2">
						<p className="font-medium text-sm text-foreground">{keyName}</p>
						<p className="truncate font-mono text-muted-foreground text-xs">{fingerprint}</p>
					</div>
				</div>
				<ModalFooter>
					<Button onClick={onClose} variant="ghost">
						Cancel
					</Button>
					<Button disabled={isDeleting} onClick={() => void onConfirm()} variant="destructive">
						Delete Key
					</Button>
				</ModalFooter>
			</ModalBody>
		</Modal>
	);
}
