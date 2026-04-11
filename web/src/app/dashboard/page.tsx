"use client";

import { motion } from "framer-motion";
import { KeyTable } from "@/components/dashboard/key-table";
import { useVault } from "@/hooks/use-vault";

const SSHKeysPage = () => {
	const { vaultData } = useVault();

	return (
		<motion.div animate={{ opacity: 1, y: 0 }} className="p-6 lg:p-8" initial={{ opacity: 0, y: 8 }} transition={{ duration: 0.2 }}>
			<div className="mb-6">
				<h1 className="font-semibold text-xl tracking-tight">SSH Keys</h1>
				<p className="mt-1 text-muted-foreground text-sm">Manage your SSH keys and host mappings</p>
			</div>
			<KeyTable keys={vaultData?.keys ?? []} />
		</motion.div>
	);
};

export default SSHKeysPage;
