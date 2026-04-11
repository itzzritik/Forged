"use client";

import { motion } from "framer-motion";
import { AccountCards } from "@/components/dashboard/account-cards";

const AccountPage = () => {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.2 }}
      className="p-6 lg:p-8"
    >
      <div className="mb-6">
        <h1 className="text-xl font-semibold tracking-tight">Account</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage your account settings
        </p>
      </div>
      <AccountCards />
    </motion.div>
  );
};

export default AccountPage;
