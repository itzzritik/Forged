"use client";

import { CircleUserIcon, KeyRoundIcon, LockIcon, LogOutIcon, type LucideIcon, ShieldCheckIcon, TrashIcon, TriangleAlertIcon } from "lucide-react";
import Link from "next/link";
import { type ReactNode, useMemo, useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { useNow } from "@/hooks/use-now";
import { useResource } from "@/hooks/use-resource";
import { useShortcuts } from "@/hooks/use-shortcuts";
import { useVaultContext } from "@/hooks/use-vault";
import { ago } from "@/lib/format";
import { useHints } from "../frame/frame";
import { Action } from "../kit/action";
import { Count } from "../kit/count";
import { Panel } from "../kit/panel";
import { logout } from "../session";
import { DeleteAccountDialog } from "./delete-account-dialog";
import { PasswordDialog } from "./password-dialog";

interface Account {
	email: string;
	name?: string;
	provider: string;
}

const PROVIDERS: Record<string, string> = { github: "GitHub", gitlab: "GitLab", google: "Google" };

const WORD_BREAK = /[\s._-]+/;

const initials = (name: string) => {
	const words = name.split(WORD_BREAK).filter(Boolean);
	return [words[0], words.length > 1 ? words.at(-1) : undefined]
		.map((word) => word?.[0] ?? "")
		.join("")
		.toUpperCase();
};

function Setting({ action, icon: Icon, note, title }: { action: ReactNode; icon: LucideIcon; note: string; title: string }) {
	return (
		<div className="setting">
			<span aria-hidden className="setting-icon">
				<Icon />
			</span>
			<p>
				{title}
				<small>{note}</small>
			</p>
			{action}
		</div>
	);
}

function Stat({ children, href, label }: { children: ReactNode; href?: string; label: string }) {
	const body = (
		<>
			<span className="stat-value">{children}</span>
			<span className="stat-label">{label}</span>
		</>
	);
	return href ? (
		<Link className="stat" href={href}>
			{body}
		</Link>
	) : (
		<div className="stat">{body}</div>
	);
}

export function AccountPage() {
	const account = useResource<Account>("/api/vault/account");
	const devices = useResource<{ devices: unknown[] }>("/api/vault/devices");
	const { kdfParams, lock, vaultData, version } = useVaultContext();
	const now = useNow(60_000);
	const [dialog, setDialog] = useState<"password" | "delete" | null>(null);
	const close = (open: boolean) => {
		if (!open) setDialog(null);
	};
	const keys = vaultData?.keys ?? [];
	const lastUsed = useMemo(
		() => (vaultData?.keys ?? []).reduce((latest, key) => (Date.parse(key.lastUsedAt ?? "") > (Date.parse(latest) || 0) ? (key.lastUsedAt ?? latest) : latest), ""),
		[vaultData]
	);

	useShortcuts({ p: () => setDialog("password") });
	useHints([{ combo: "p", label: "Change password" }]);

	let identity = (
		<div aria-busy className="identity-who">
			<Skeleton className="size-[52px] rounded-xl" />
			<div className="grid gap-2">
				<Skeleton className="h-5 w-44" />
				<Skeleton className="h-4 w-56" />
			</div>
		</div>
	);
	if (account.error) {
		identity = (
			<div className="identity-who flex-wrap">
				<p className="text-muted-foreground">Couldn't load your profile. {account.error}</p>
				<Action onClick={account.retry} tone="primary">
					Retry
				</Action>
			</div>
		);
	}
	if (account.data) {
		const { email } = account.data;
		const name = account.data.name?.trim() || email.split("@")[0];
		identity = (
			<div className="identity-who">
				<span aria-hidden className="identity-tile">
					{initials(name)}
				</span>
				<div className="min-w-0">
					<p className="identity-name">{name}</p>
					<p className="identity-email">{email}</p>
				</div>
			</div>
		);
	}

	const provider = account.data?.provider;
	return (
		<div className="account-layout">
			<Panel aside={provider ? `Signed in with ${PROVIDERS[provider] ?? provider}` : undefined} className="account-wide" icon={CircleUserIcon} title="Profile">
				<div className="identity">
					{identity}
					<div className="stats">
						<Stat href="/dashboard" label="SSH keys">
							<Count value={keys.length} />
						</Stat>
						<Stat href="/dashboard/devices" label="Devices">
							{devices.data ? <Count value={devices.data.devices.length} /> : "-"}
						</Stat>
						<Stat label="Last SSH use">{ago(lastUsed, now) ?? "Never"}</Stat>
					</div>
				</div>
			</Panel>
			<Panel delay={80} icon={LockIcon} title="Security">
				<Setting
					action={
						<Action combo="p" onClick={() => setDialog("password")}>
							Change
						</Action>
					}
					icon={KeyRoundIcon}
					note="Your other devices ask for the new one at their next unlock."
					title="Master password"
				/>
				<Setting
					action={
						<Action combo="l" onClick={() => void lock()}>
							Lock now
						</Action>
					}
					icon={LockIcon}
					note="Also locks after 4 hours of inactivity."
					title="Lock this browser"
				/>
				<Setting
					action={<Action onClick={() => void logout()}>Log out</Action>}
					icon={LogOutIcon}
					note="Locks the vault and signs this browser out."
					title="Log out"
				/>
			</Panel>
			<Panel delay={120} icon={ShieldCheckIcon} title="Vault">
				<dl className="inspector-kv account-spec">
					<dt>Key derivation</dt>
					<dd>{kdfParams ? `Argon2id, ${Math.round(kdfParams.memory / 1024)} MiB, ${kdfParams.time} passes` : "Argon2id"}</dd>
					<dt>Encryption</dt>
					<dd>AES-256-GCM, one key per SSH key</dd>
					<dt>Decryption</dt>
					<dd>Only on your devices</dd>
					<dt>Master password</dt>
					<dd>Never leaves your devices</dd>
					<dt>Server stores</dt>
					<dd>Ciphertext only</dd>
					<dt>Revision</dt>
					<dd className="tabular-nums">{version.toLocaleString()}</dd>
				</dl>
			</Panel>
			<Panel className="account-wide" delay={160} icon={TriangleAlertIcon} title="Danger zone" tone="danger">
				<Setting
					action={
						<Action onClick={() => setDialog("delete")} tone="danger">
							Delete account
						</Action>
					}
					icon={TrashIcon}
					note="Removes your account and encrypted vault from the server. This can't be undone."
					title="Delete account"
				/>
			</Panel>
			<PasswordDialog onOpenChange={close} open={dialog === "password"} />
			<DeleteAccountDialog onOpenChange={close} open={dialog === "delete"} />
		</div>
	);
}
