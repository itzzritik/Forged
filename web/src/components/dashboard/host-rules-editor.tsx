"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Modal, ModalBody, ModalFooter } from "@/components/ui/modal";
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
	const { deviceId, vaultData, pushVault } = useVaultContext();
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
			const updated = updateKeyInVault(vaultData, keyId, { host_rules: rules }, deviceId);
			await pushVault(updated);
			onClose();
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to save");
		} finally {
			setIsLoading(false);
		}
	};

	return (
		<Modal onOpenChange={(open) => !open && onClose()} open size="sm" title="Hosts // Edit">
			<ModalBody>
				<div className="space-y-1">
					<p className="font-semibold text-lg">Host Rules</p>
					<p className="text-muted-foreground text-sm">{keyName}</p>
				</div>

				<div className="flex min-h-10 flex-wrap gap-1.5">
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

				<div className="flex gap-2">
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

				{error && <p className="text-destructive text-xs">{error}</p>}

				<ModalFooter className="justify-end">
					<Button onClick={onClose} type="button" variant="outline">
						Cancel
					</Button>
					<Button disabled={isLoading} onClick={handleSave} type="button">
						{isLoading ? "Saving..." : "Save"}
					</Button>
				</ModalFooter>
			</ModalBody>
		</Modal>
	);
};
