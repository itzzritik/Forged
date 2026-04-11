"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { GlitchButton } from "@/components/client";

export function AuthNavButton() {
	const [loggedIn, setLoggedIn] = useState(false);

	useEffect(() => {
		setLoggedIn(document.cookie.includes("forged_logged_in=1"));
	}, []);

	if (loggedIn) {
		return (
			<Link className="group flex items-center gap-2" href="/dashboard">
				<div className="flex h-7 w-7 items-center justify-center bg-[#ea580c] font-bold font-mono text-[11px] text-black">F</div>
			</Link>
		);
	}

	return (
		<GlitchButton className="h-8 px-5 text-[12px]" href="/login">
			Sign in
		</GlitchButton>
	);
}

export function AuthCTAButton() {
	const [loggedIn, setLoggedIn] = useState(false);

	useEffect(() => {
		setLoggedIn(document.cookie.includes("forged_logged_in=1"));
	}, []);

	return (
		<GlitchButton className="h-14 max-w-full px-12 text-sm" href={loggedIn ? "/dashboard" : "/login"}>
			{loggedIn ? "Dashboard" : "Start Syncing"}
		</GlitchButton>
	);
}
