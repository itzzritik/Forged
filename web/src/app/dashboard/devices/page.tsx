"use client";

import { DeviceTable } from "@/components/dashboard/device-table";

const DevicesPage = () => {
  return (
    <div className="p-6 lg:p-8">
      <div className="mb-6">
        <h1 className="text-xl font-semibold tracking-tight">Devices</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage devices connected to your vault
        </p>
      </div>
      <DeviceTable />
    </div>
  );
};

export default DevicesPage;
