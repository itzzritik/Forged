"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useVaultContext } from "@/hooks/use-vault";
import { updateKeyInVault } from "@/lib/vault-crypto";

interface HostRulesEditorProps {
	hostRules: Array<{ match: string; type: string }>;
	keyId: string;
	keyName: string;
	onClose: () => void;
}

function detectType(pattern: string): string {
	if (pattern.includes("/")) return "cidr";
	if (pattern.includes("*")) return "wildcard";
	return "exact";
}

export const HostRulesEditor = ({ hostRules, keyId, keyName, onClose }: HostRulesEditorProps) => {
	const { vaultData, pushVault } = useVaultContext();
	const [rules, setRules] = useState(hostRules);
	const [input, setInput] = useState("");
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const addRule = () => {
		const trimmed = input.trim();
		if (!trimmed) return;
		if (rules.some((r) => r.match === trimmed)) {
			setInput("");
			return;
		}
		setRules((prev) => [...prev, { match: trimmed, type: detectType(trimmed) }]);
		setInput("");
	};

	const removeRule = (match: string) => {
		setRules((prev) => prev.filter((r) => r.match !== match));
	};

	const handleSave = async () => {
		if (!vaultData) return;
		setIsLoading(true);
		setError(null);
		try {
			const updated = updateKeyInVault(vaultData, keyId, { host_rules: rules });
			await pushVault(updated);
			onClose();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to save");
		} finally {
			setIsLoading(false);
		}
	};

	return (
		<div className="fixed inset-0 z-50 flex items-center justify-center">
			<div aria-hidden className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
			<div className="relative z-10 w-full max-w-md border border-border bg-card p-6 font-mono shadow-2xl">
				<p className="mb-1 font-semibold text-lg">Host Rules</p>
				<p className="mb-4 text-muted-foreground text-xs">{keyName}</p>

				<div className="mb-3 flex min-h-10 flex-wrap gap-1.5">
					{rules.length === 0 ? (
						<span className="text-muted-foreground text-xs">No host rules</span>
					) : (
						rules.map((rule) => (
							<Badge className="border border-primary/20 bg-primary/10 text-primary hover:bg-primary/10" key={rule.match}>
								{rule.match}
								<button className="ml-1.5 opacity-60 hover:opacity-100" onClick={() => removeRule(rule.match)} type="button">
									x
								</button>
							</Badge>
						))
					)}
				</div>

				<div className="mb-4 flex gap-2">
					<Input
						className="flex-1"
						onChange={(e) => setInput(e.target.value)}
						onKeyDown={(e) => {
							if (e.key === "Enter") {
								e.preventDefault();
								addRule();
							}
						}}
						placeholder="e.g. *.github.com, 10.0.0.0/8"
						value={input}
					/>
					<Button onClick={addRule} type="button" variant="outline">
						Add
					</Button>
				</div>

				{error && <p className="mb-3 text-destructive text-xs">{error}</p>}

				<div className="flex justify-end gap-2">
					<Button onClick={onClose} type="button" variant="outline">
						Cancel
					</Button>
					<Button disabled={isLoading} onClick={handleSave} type="button">
						{isLoading ? "Saving..." : "Save"}
					</Button>
				</div>
			</div>
		</div>
	);
};
