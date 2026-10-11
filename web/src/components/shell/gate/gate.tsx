"use client";

import { CircleAlertIcon, KeyRoundIcon, LockIcon } from "lucide-react";
import { useTheme } from "next-themes";
import { type FormEvent, type RefObject, useCallback, useEffect, useRef, useState } from "react";
import { useVaultContext } from "@/hooks/use-vault";
import { cn } from "@/lib/utils";
import { Action } from "../kit/action";
import { FieldLabel, PasswordField } from "../kit/field";
import { Panel } from "../kit/panel";
import { EASE, reducedMotion, replay, shake } from "../motion";
import { logout } from "../session";
import { Fire, HEAT } from "./fire";
import { Wordmark } from "./wordmark";

type Phase = "idle" | "busy" | "ok";

interface GateProps {
	email: string;
	leaving: boolean;
	onLeft: () => void;
	onReveal: () => void;
}

export function Gate({ email, leaving, onLeft, onReveal }: GateProps) {
	const { status } = useVaultContext();
	const { resolvedTheme } = useTheme();
	const stage = useRef<HTMLDivElement>(null);
	const flames = useRef<HTMLCanvasElement>(null);
	const embers = useRef<HTMLCanvasElement>(null);
	const stack = useRef<HTMLDivElement>(null);
	const fire = useRef<Fire | null>(null);
	const [phase, setPhase] = useState<Phase>("idle");

	useEffect(() => {
		if (!(stage.current && flames.current && embers.current)) return;
		const engine = new Fire(flames.current, embers.current, stage.current);
		fire.current = engine;
		engine.start();
		if (!reducedMotion()) {
			stage.current.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 220, easing: "ease-out" });
			stack.current?.animate(
				[
					{ opacity: 0, transform: "translateY(12px)" },
					{ opacity: 1, transform: "none" },
				],
				{ delay: 360, duration: 420, easing: EASE, fill: "backwards" }
			);
		}
		return () => {
			engine.destroy();
			fire.current = null;
		};
	}, []);

	useEffect(() => {
		fire.current?.theme(resolvedTheme !== "light");
	}, [resolvedTheme]);

	useEffect(() => {
		if (status === "no-vault" || status === "error") setPhase("idle");
	}, [status]);

	useEffect(() => {
		if (!leaving) return;
		setPhase("ok");
		fire.current?.flare();
		if (reducedMotion()) {
			onReveal();
			onLeft();
			return;
		}
		const timer = setTimeout(() => {
			stack.current?.animate(
				[
					{ opacity: 1, transform: "none" },
					{ opacity: 0, transform: "translateY(-10px) scale(0.985)" },
				],
				{ duration: 280, easing: EASE, fill: "forwards" }
			);
			const fade = stage.current?.animate([{ opacity: 1 }, { opacity: 0 }], { delay: 160, duration: 440, easing: "ease-in", fill: "forwards" });
			onReveal();
			if (fade) fade.onfinish = onLeft;
			else onLeft();
		}, 460);
		return () => clearTimeout(timer);
	}, [leaving, onLeft, onReveal]);

	const onBusy = useCallback(() => {
		setPhase("busy");
		fire.current?.setTarget(HEAT.busy);
	}, []);

	const onWrong = useCallback(() => {
		setPhase("idle");
		fire.current?.choke();
		replay(stage.current?.querySelector(".wordmark"), "dim");
	}, []);

	let card = <Unlock email={email} fire={fire} onBusy={onBusy} onWrong={onWrong} phase={phase} />;
	if (status === "no-vault") card = <NoVault />;
	if (status === "error") card = <Failure />;

	return (
		<div className="gate" data-phase={phase} ref={stage}>
			<div aria-hidden className="gate-halo" />
			<canvas aria-hidden className="gate-fire" ref={flames} />
			<canvas aria-hidden className="gate-sparks" ref={embers} />
			<div className="gate-stack" ref={stack}>
				<Wordmark size="gate" />
				{card}
				<p className="gate-hint">Locks again after 4 hours of inactivity</p>
			</div>
		</div>
	);
}

interface UnlockProps {
	email: string;
	fire: RefObject<Fire | null>;
	onBusy: () => void;
	onWrong: () => void;
	phase: Phase;
}

function Unlock({ email, fire, onBusy, onWrong, phase }: UnlockProps) {
	const { error, unlock } = useVaultContext();
	const [password, setPassword] = useState("");
	const [message, setMessage] = useState<string | null>(null);
	const form = useRef<HTMLFormElement>(null);
	const input = useRef<HTMLInputElement>(null);
	const tried = useRef(false);

	useEffect(() => {
		input.current?.focus();
	}, []);

	useEffect(() => {
		if (!(tried.current && error === "Wrong password")) return;
		setMessage("That password didn't unlock your vault. Check it and try again.");
		shake(form.current);
		onWrong();
		input.current?.focus();
	}, [error, onWrong]);

	const submit = (event: FormEvent) => {
		event.preventDefault();
		if (phase !== "idle" || !password) return;
		tried.current = true;
		setMessage(null);
		onBusy();
		const attempt = password;
		setPassword("");
		void unlock(attempt);
	};

	let label = "Unlock";
	if (phase === "busy") label = "Unlocking";
	if (phase === "ok") label = "Unlocked";

	return (
		<form className="w-full" onSubmit={submit} ref={form}>
			<Panel aside={email} className="gate-card" icon={LockIcon} title="Unlock">
				<i aria-hidden className="sweep gate-sweep" data-on={phase === "busy" ? "" : undefined} />
				<div className="px-6 pt-1 pb-2">
					<p className="mb-[18px] text-[13.5px] text-muted-foreground">Enter your master password. It never leaves this browser.</p>
					<FieldLabel className="mb-[7px]" htmlFor="unlock-password">
						Master password
					</FieldLabel>
					<PasswordField
						id="unlock-password"
						invalid={Boolean(message)}
						onValueChange={(value) => {
							setPassword(value);
							fire.current?.kick();
							if (message) setMessage(null);
						}}
						readOnly={phase !== "idle"}
						ref={input}
						value={password}
					/>
					<p aria-live="polite" className={cn("mt-[9px] min-h-5 text-[12.5px]", message ? "text-destructive" : "text-muted-foreground")}>
						{message ?? (phase === "busy" ? "Deriving your key on this device" : "")}
					</p>
					<div className="mt-2.5 flex items-center justify-between border-rule border-t pt-3.5">
						<Action onClick={() => void logout()} tone="quiet" type="button">
							Log out
						</Action>
						<Action combo="enter" disabled={phase !== "idle"} tone="primary" type="submit">
							{label}
						</Action>
					</div>
				</div>
			</Panel>
		</form>
	);
}

function NoVault() {
	return (
		<Panel className="gate-card" icon={KeyRoundIcon} title="No vault yet">
			<div className="px-6 pt-1 pb-2">
				<p className="text-[13.5px] text-muted-foreground">Open Forged on a linked device to create your vault, then come back here.</p>
				<div className="mt-4 flex justify-end border-rule border-t pt-3.5">
					<Action onClick={() => void logout()} tone="outline">
						Log out
					</Action>
				</div>
			</div>
		</Panel>
	);
}

function Failure() {
	const { error } = useVaultContext();
	return (
		<Panel className="gate-card" icon={CircleAlertIcon} title="Can't open your vault" tone="danger">
			<div className="px-6 pt-1 pb-2">
				<p className="text-[13.5px] text-muted-foreground">{error ?? "Couldn't reach the server."}</p>
				<div className="mt-4 flex items-center justify-between border-rule border-t pt-3.5">
					<Action onClick={() => void logout()} tone="quiet">
						Log out
					</Action>
					<Action onClick={() => window.location.reload()} tone="primary">
						Retry
					</Action>
				</div>
			</div>
		</Panel>
	);
}
