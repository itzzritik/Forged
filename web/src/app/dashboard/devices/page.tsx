"use client";

import { motion } from "framer-motion";
import { DeviceTable } from "@/components/dashboard/device-table";

const DevicesPage = () => {
	return (
		<motion.div animate={{ opacity: 1, y: 0 }} className="p-6 lg:p-8" initial={{ opacity: 0, y: 8 }} transition={{ duration: 0.2 }}>
			<div className="mb-6">
				<h1 className="font-semibold text-xl tracking-tight">Devices</h1>
				<p className="mt-1 text-muted-foreground text-sm">Manage devices connected to your vault</p>
			</div>
			<DeviceTable />
		</motion.div>
	);
};

export default DevicesPage;
