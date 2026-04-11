"use client";

import { motion } from "framer-motion";
import { useVault } from "@/hooks/use-vault";
import { KeyTable } from "@/components/dashboard/key-table";

const SSHKeysPage = () => {
  const { vaultData } = useVault();

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.2 }}
      className="p-6 lg:p-8"
    >
      <div className="mb-6">
        <h1 className="text-xl font-semibold tracking-tight">SSH Keys</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage your SSH keys and host mappings
        </p>
      </div>
      <KeyTable keys={vaultData?.keys ?? []} />
    </motion.div>
  );
};

export default SSHKeysPage;
