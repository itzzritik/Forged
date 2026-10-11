"use client";

import { TriangleAlertIcon } from "lucide-react";
import { plural } from "@/lib/format";
import { Action } from "../kit/action";
import { ShellDialog } from "../kit/dialog";

const SHOWN = 5;

interface DeleteDialogProps {
	names: string[];
	onConfirm: () => void;
	onOpenChange: (open: boolean) => void;
	open: boolean;
}

export function DeleteDialog({ names, onConfirm, onOpenChange, open }: DeleteDialogProps) {
	const many = names.length > 1;
	const rest = names.length - SHOWN;
	const what = many ? plural(names.length, "key") : (names[0] ?? "this key");
	return (
		<ShellDialog
			actions={
				<Action combo="d" tone="fill" type="submit">
					{many ? `Delete ${what}` : "Delete key"}
				</Action>
			}
			description={`${many ? "They're" : "It's"} removed from every device at their next sync. This can't be undone.`}
			focusCancel
			heading={`Delete ${what}?`}
			icon={TriangleAlertIcon}
			keys={{ d: onConfirm }}
			onOpenChange={onOpenChange}
			onSubmit={onConfirm}
			open={open}
			title={many ? "Delete keys" : "Delete key"}
			tone="danger"
		>
			{many && (
				<p className="font-mono text-[12.5px] text-muted-foreground">
					{names.slice(0, SHOWN).join(", ")}
					{rest > 0 ? ` and ${rest} more` : ""}
				</p>
			)}
		</ShellDialog>
	);
}
