"use client";

import { AccountCards } from "@/components/dashboard/account-cards";

const AccountPage = () => {
  return (
    <div className="p-6 lg:p-8">
      <div className="mb-6">
        <h1 className="text-xl font-semibold tracking-tight">Account</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage your account settings
        </p>
      </div>
      <AccountCards />
    </div>
  );
};

export default AccountPage;
