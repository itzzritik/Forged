"use client";

import { motion } from "framer-motion";
import { useState } from "react";
import { ExportModal } from "@/components/dashboard/export-modal";
import { GenerateKeyModal } from "@/components/dashboard/generate-key-modal";
import { ImportKeyModal } from "@/components/dashboard/import-key-modal";
import { KeyTable } from "@/components/dashboard/key-table";
import { Button } from "@/components/ui/button";

const SSHKeysPage = () => {
	const [showGenerate, setShowGenerate] = useState(false);
	const [showImport, setShowImport] = useState(false);
	const [showExport, setShowExport] = useState(false);

	return (
		<motion.div animate={{ opacity: 1, y: 0 }} className="p-6 lg:p-8" initial={{ opacity: 0, y: 8 }} transition={{ duration: 0.2 }}>
			<div className="mb-6 flex items-center justify-between">
				<div>
					<h1 className="font-semibold text-xl tracking-tight">SSH Keys</h1>
					<p className="mt-1 text-muted-foreground text-sm">Manage your SSH keys and host mappings</p>
				</div>
				<div className="flex gap-2">
					<Button onClick={() => setShowExport(true)} variant="ghost">
						Export
					</Button>
					<Button onClick={() => setShowImport(true)} variant="outline">
						Import
					</Button>
					<Button onClick={() => setShowGenerate(true)}>Generate Key</Button>
				</div>
			</div>
			<KeyTable />
			{showGenerate && <GenerateKeyModal onClose={() => setShowGenerate(false)} />}
			{showImport && <ImportKeyModal onClose={() => setShowImport(false)} />}
			{showExport && <ExportModal onClose={() => setShowExport(false)} />}
		</motion.div>
	);
};

export default SSHKeysPage;
