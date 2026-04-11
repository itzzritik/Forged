"use client";

import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import type { VaultKeyMetadata } from "@/lib/vault-crypto";

interface KeyTableProps {
	keys: VaultKeyMetadata[];
}

export const KeyTable = ({ keys }: KeyTableProps) => {
	if (keys.length === 0) {
		return (
			<div className="flex flex-col items-center justify-center gap-2 py-16 text-center">
				<p className="text-muted-foreground text-sm">No keys in vault</p>
				<p className="font-mono text-muted-foreground text-xs">
					Add keys via CLI: <span className="text-primary">forged add &lt;name&gt;</span>
				</p>
			</div>
		);
	}

	const handleCopy = async (key: VaultKeyMetadata) => {
		try {
			await navigator.clipboard.writeText(key.publicKey);
			toast.success("Public key copied to clipboard");
		} catch {
			toast.error("Failed to copy to clipboard");
		}
	};

	const handleExport = (key: VaultKeyMetadata) => {
		toast.info(`Export via CLI: forged export ${key.name}`);
	};

	return (
		<TooltipProvider>
			<Table>
				<TableHeader>
					<TableRow>
						<TableHead>Name</TableHead>
						<TableHead className="hidden sm:table-cell">Fingerprint</TableHead>
						<TableHead>Hosts</TableHead>
						<TableHead className="hidden sm:table-cell">Signing</TableHead>
						<TableHead>Actions</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					{keys.map((key) => (
						<TableRow key={key.id}>
							<TableCell>
								<span className="font-medium text-foreground">{key.name}</span>
								<div className="text-muted-foreground text-xs">{key.type}</div>
							</TableCell>
							<TableCell className="hidden sm:table-cell">
								<Tooltip>
									<TooltipTrigger className="block max-w-[180px] cursor-default truncate font-mono text-muted-foreground text-sm" render={<span />}>
										{key.fingerprint}
									</TooltipTrigger>
									<TooltipContent>
										<span className="font-mono">{key.fingerprint}</span>
									</TooltipContent>
								</Tooltip>
							</TableCell>
							<TableCell>
								<div className="flex flex-wrap gap-1">
									{key.hostRules.length > 0 ? (
										key.hostRules.map((rule, i) => (
											<Badge className="border border-primary/20 bg-primary/10 text-primary hover:bg-primary/10" key={i}>
												{rule.match}
											</Badge>
										))
									) : (
										<span className="text-muted-foreground text-xs">--</span>
									)}
								</div>
							</TableCell>
							<TableCell className="hidden sm:table-cell">
								{key.gitSigning ? (
									<span className="flex items-center gap-1.5 text-green-500 text-sm">
										<span className="size-1.5 shrink-0 rounded-full bg-green-500" />
										Active
									</span>
								) : (
									<span className="text-muted-foreground text-sm">Off</span>
								)}
							</TableCell>
							<TableCell>
								<div className="flex items-center gap-2">
									<Button onClick={() => handleCopy(key)} size="sm" variant="outline">
										Copy
									</Button>
									<Button onClick={() => handleExport(key)} size="sm" variant="ghost">
										Export
									</Button>
								</div>
							</TableCell>
						</TableRow>
					))}
				</TableBody>
			</Table>
		</TooltipProvider>
	);
};
