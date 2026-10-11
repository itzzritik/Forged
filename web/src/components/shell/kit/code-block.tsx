"use client";

import { type CSSProperties, type ReactNode, useRef } from "react";
import { copyText } from "../clipboard";

interface CodeBlockProps {
	action?: ReactNode;
	children?: ReactNode;
	copy?: string;
	label: string;
	style?: CSSProperties;
	value?: string;
}

export function CopyButton({ message, text }: { message: string; text: string }) {
	const button = useRef<HTMLButtonElement>(null);
	return (
		<button className="text-foreground transition-colors hover:text-primary" onClick={() => void copyText(text, message, button.current)} ref={button} type="button">
			copy
		</button>
	);
}

export function CodeBlock({ action, children, copy, label, style, value }: CodeBlockProps) {
	return (
		<div className="inspector-section" style={style}>
			<h3 className="mb-1.5 flex items-center justify-between font-medium font-mono text-[12px] text-muted-foreground">
				{label}
				{action ?? (value && copy && <CopyButton message={copy} text={value} />)}
			</h3>
			<div className="code-block">{children ?? value}</div>
		</div>
	);
}
