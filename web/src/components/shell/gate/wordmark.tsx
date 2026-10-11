"use client";

import { type CSSProperties, useEffect, useRef } from "react";
import { sparks } from "../motion";

const LETTERS = ["f", "o", "r", "g", "e", "d"];

export function Wordmark({ size }: { size: "gate" | "bar" }) {
	const ref = useRef<HTMLSpanElement>(null);
	useEffect(() => {
		if (size !== "gate") return;
		const timer = setTimeout(() => {
			if (ref.current) sparks(ref.current, 14, { fall: true });
		}, 40);
		return () => clearTimeout(timer);
	}, [size]);
	return (
		<span aria-label="forged" className="wordmark" data-size={size} ref={ref} role="img">
			{LETTERS.map((letter, index) => (
				<span aria-hidden key={letter} style={{ "--i": index } as CSSProperties}>
					{letter}
				</span>
			))}
		</span>
	);
}
