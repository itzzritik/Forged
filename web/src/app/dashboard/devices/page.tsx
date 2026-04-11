"use client";

import { motion } from "framer-motion";
import { DeviceTable } from "@/components/dashboard/device-table";

const DevicesPage = () => {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.2 }}
      className="p-6 lg:p-8"
    >
      <div className="mb-6">
        <h1 className="text-xl font-semibold tracking-tight">Devices</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage devices connected to your vault
        </p>
      </div>
      <DeviceTable />
    </motion.div>
  );
};

export default DevicesPage;
