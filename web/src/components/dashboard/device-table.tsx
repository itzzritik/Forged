"use client";

import { useEffect, useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { DataView, type DataViewColumn } from "./data-view";

interface Device {
	approved: boolean;
	hostname: string;
	id: string;
	last_seen_at: string;
	name: string;
	platform: string;
	registered_at: string;
}

const relativeTime = (dateStr: string): string => {
	const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
	if (seconds < 60) return "just now";
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	const days = Math.floor(hours / 24);
	return `${days}d ago`;
};

export const DeviceTable = () => {
	const [devices, setDevices] = useState<Device[]>([]);
	const [loading, setLoading] = useState(true);

	useEffect(() => {
		fetch("/api/vault/devices")
			.then((res) => res.json())
			.then((data) => setDevices(Array.isArray(data) ? data : []))
			.finally(() => setLoading(false));
	}, []);

	const columns = useMemo<DataViewColumn<Device>[]>(
		() => [
			{
				accessorKey: "name",
				header: "Device",
				cell: ({ row }) => (
					<div className="min-w-0">
						<p className="truncate font-semibold text-sm">{row.original.name}</p>
						<p className="truncate text-muted-foreground text-xs">{row.original.platform}</p>
					</div>
				),
				meta: {
					cellClassName: "min-w-[12rem]",
					headerClassName: "min-w-[12rem]",
					toggleable: false,
				},
			},
			{
				accessorKey: "hostname",
				header: "Hostname",
				cell: ({ row }) => <span className="font-mono text-muted-foreground text-xs">{row.original.hostname}</span>,
				meta: {
					cellClassName: "min-w-[14rem]",
					headerClassName: "min-w-[14rem]",
					responsive: "sm",
				},
			},
			{
				accessorKey: "last_seen_at",
				header: "Last Seen",
				cell: ({ row }) => <span className="text-muted-foreground text-sm">{relativeTime(row.original.last_seen_at)}</span>,
				meta: {
					cellClassName: "w-[8rem]",
					headerClassName: "w-[8rem]",
				},
			},
			{
				accessorKey: "approved",
				header: "Status",
				cell: ({ row }) =>
					row.original.approved ? (
						<Badge className="border-success/20 bg-success/10 text-success hover:bg-success/10">Approved</Badge>
					) : (
						<Badge className="border-warning/20 bg-warning/10 text-warning hover:bg-warning/10">Pending</Badge>
					),
				meta: {
					cellClassName: "w-[8rem]",
					headerClassName: "w-[8rem]",
				},
			},
		],
		[]
	);

	return (
		<DataView
			columns={columns}
			data={devices}
			emptyState={{
				title: "No devices registered",
				description: "Devices are registered when you run forged sync from a new machine.",
			}}
			entityLabel="devices"
			getRowId={(device) => device.id}
			getSearchText={(device) => [device.name, device.hostname, device.platform, device.approved ? "approved" : "pending", relativeTime(device.last_seen_at)].join(" ")}
			globalFilterPlaceholder="Search devices, hostnames, or platforms"
			initialSorting={[{ id: "name", desc: false }]}
			isLoading={loading}
			rowHeight={46}
		/>
	);
};
