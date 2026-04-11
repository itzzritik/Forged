"use client";

import { motion } from "framer-motion";
import { AccountCards } from "@/components/dashboard/account-cards";

const AccountPage = () => {
	return (
		<motion.div animate={{ opacity: 1, y: 0 }} className="p-6 lg:p-8" initial={{ opacity: 0, y: 8 }} transition={{ duration: 0.2 }}>
			<div className="mb-6">
				<h1 className="font-semibold text-xl tracking-tight">Account</h1>
				<p className="mt-1 text-muted-foreground text-sm">Manage your account settings</p>
			</div>
			<AccountCards />
		</motion.div>
	);
};

export default AccountPage;
