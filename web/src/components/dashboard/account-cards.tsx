"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { Separator } from "@/components/ui/separator";

interface AccountData {
	email: string;
	name: string;
	provider: string;
}

export const AccountCards = () => {
	const router = useRouter();
	const [account, setAccount] = useState<AccountData | null>(null);
	const [deleteOpen, setDeleteOpen] = useState(false);
	const [deleteInput, setDeleteInput] = useState("");
	const [deleting, setDeleting] = useState(false);

	useEffect(() => {
		fetch("/api/vault/account")
			.then((r) => r.json())
			.then((data) => setAccount(data))
			.catch(() => {});
	}, []);

	const handleDelete = async () => {
		if (deleteInput !== "DELETE") return;
		setDeleting(true);
		try {
			await fetch("/api/vault/account-delete", { method: "POST" });
			router.push("/");
		} finally {
			setDeleting(false);
		}
	};

	const initial = account?.name?.[0]?.toUpperCase() ?? "?";

	return (
		<div className="flex max-w-2xl flex-col gap-4">
			{/* Profile Card */}
			<div className="rounded-lg border border-border bg-card p-6">
				<div className="flex items-center gap-4">
					<div className="flex size-16 shrink-0 items-center justify-center rounded-full bg-orange-500 font-semibold text-2xl text-white">{initial}</div>
					<div className="min-w-0">
						<p className="truncate font-semibold text-lg">{account?.name ?? "--"}</p>
						<p className="truncate text-muted-foreground text-sm">{account?.email ?? "--"}</p>
						{account?.provider && (
							<Badge className="mt-1.5 capitalize" variant="outline">
								{account.provider}
							</Badge>
						)}
					</div>
				</div>
			</div>

			{/* Security Card */}
			<div className="rounded-lg border border-border bg-card p-6">
				<h2 className="mb-4 font-semibold text-sm">Master Password</h2>
				<div className="flex flex-col gap-3">
					<p className="text-muted-foreground text-sm">Change your master password via CLI:</p>
					<code className="inline-block w-fit rounded bg-muted px-3 py-1.5 font-mono text-foreground text-sm">forged change-password</code>
					<Separator />
					<p className="text-muted-foreground text-sm">Vault Timeout: 4 hours of inactivity</p>
				</div>
			</div>

			{/* Danger Zone Card */}
			<div className="rounded-lg border border-destructive/50 bg-card p-6">
				<h2 className="mb-1 font-semibold text-destructive text-sm">Delete Account</h2>
				<p className="mb-4 text-muted-foreground text-sm">Permanently delete your account and all data</p>
				<Button onClick={() => setDeleteOpen(true)} size="sm" variant="destructive">
					Delete Account
				</Button>
			</div>

			<Modal closable={true} onOpenChange={setDeleteOpen} open={deleteOpen} title="Account // Delete">
				<div className="flex flex-col gap-4 p-5">
					<p className="text-muted-foreground text-sm">
						This action is irreversible. Type <span className="font-mono text-foreground">DELETE</span> to confirm.
					</p>
					<Input autoFocus onChange={(e) => setDeleteInput(e.target.value)} placeholder="DELETE" value={deleteInput} />
					<Button disabled={deleteInput !== "DELETE" || deleting} onClick={handleDelete} variant="destructive">
						{deleting ? "Deleting..." : "Delete Account"}
					</Button>
				</div>
			</Modal>
		</div>
	);
};
