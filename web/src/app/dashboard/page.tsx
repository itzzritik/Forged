"use client";

import { motion } from "framer-motion";
import { useState } from "react";
import { GenerateKeyModal } from "@/components/dashboard/generate-key-modal";
import { KeyTable } from "@/components/dashboard/key-table";
import { Button } from "@/components/ui/button";

const SSHKeysPage = () => {
	const [showGenerate, setShowGenerate] = useState(false);

	return (
		<motion.div animate={{ opacity: 1, y: 0 }} className="p-6 lg:p-8" initial={{ opacity: 0, y: 8 }} transition={{ duration: 0.2 }}>
			<div className="mb-6 flex items-center justify-between">
				<div>
					<h1 className="font-semibold text-xl tracking-tight">SSH Keys</h1>
					<p className="mt-1 text-muted-foreground text-sm">Manage your SSH keys and host mappings</p>
				</div>
				<Button onClick={() => setShowGenerate(true)}>Generate Key</Button>
			</div>
			<KeyTable />
			{showGenerate && <GenerateKeyModal onClose={() => setShowGenerate(false)} />}
		</motion.div>
	);
};

export default SSHKeysPage;
