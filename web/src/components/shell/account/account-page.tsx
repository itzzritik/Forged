"use client";

import { CircleUserIcon, LockIcon, TriangleAlertIcon } from "lucide-react";
import { useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { useResource } from "@/hooks/use-resource";
import { useVaultContext } from "@/hooks/use-vault";
import { Action } from "../kit/action";
import { Panel } from "../kit/panel";
import { DeleteAccountDialog } from "./delete-account-dialog";
import { PasswordDialog } from "./password-dialog";

interface Account {
	email: string;
	name?: string;
	provider: string;
}

const PROVIDERS: Record<string, string> = { github: "GitHub", gitlab: "GitLab", google: "Google" };

export function AccountPage() {
	const account = useResource<Account>("/api/vault/account");
	const { lock } = useVaultContext();
	const [dialog, setDialog] = useState<"password" | "delete" | null>(null);
	const close = (open: boolean) => {
		if (!open) setDialog(null);
	};

	let profile = (
		<div aria-busy className="grid gap-2 px-[18px] pt-1 pb-3">
			<Skeleton className="h-5 w-48" />
			<Skeleton className="h-5 w-64" />
			<Skeleton className="h-5 w-32" />
		</div>
	);
	if (account.error) {
		profile = (
			<div className="grid justify-items-start gap-3 px-[18px] pt-1 pb-3">
				<p>Couldn't load your profile. {account.error}</p>
				<Action onClick={account.retry} tone="primary">
					Retry
				</Action>
			</div>
		);
	}
	if (account.data) {
		const { email, name, provider } = account.data;
		profile = (
			<dl className="inspector-kv">
				<dt>Name</dt>
				<dd>{name || "Not set"}</dd>
				<dt>Email</dt>
				<dd className="truncate">{email}</dd>
				<dt>Sign-in</dt>
				<dd>{PROVIDERS[provider] ?? provider}</dd>
			</dl>
		);
	}

	return (
		<div className="account-layout">
			<Panel icon={CircleUserIcon} title="Profile">
				{profile}
			</Panel>
			<Panel delay={80} icon={LockIcon} title="Security">
				<div className="setting">
					<p>
						Master password
						<small>Your other devices ask for the new one at their next unlock.</small>
					</p>
					<Action onClick={() => setDialog("password")}>Change</Action>
				</div>
				<div className="setting">
					<p>
						Lock this browser
						<small>Also locks after 4 hours of inactivity.</small>
					</p>
					<Action combo="l" onClick={() => void lock()}>
						Lock now
					</Action>
				</div>
			</Panel>
			<Panel delay={160} icon={TriangleAlertIcon} title="Danger zone" tone="danger">
				<div className="setting">
					<p>
						Delete account
						<small>Removes your account and encrypted vault from the server. This can't be undone.</small>
					</p>
					<Action onClick={() => setDialog("delete")} tone="danger">
						Delete account
					</Action>
				</div>
			</Panel>
			<PasswordDialog onOpenChange={close} open={dialog === "password"} />
			<DeleteAccountDialog onOpenChange={close} open={dialog === "delete"} />
		</div>
	);
}
