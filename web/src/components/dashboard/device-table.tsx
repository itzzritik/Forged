"use client";

import { LaptopIcon, type LucideIcon, MonitorIcon, ServerIcon } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { type DataViewColumn, DataViewTable } from "./data-view";

interface Device {
	approved: boolean;
	arch: string;
	cli_version: string;
	hostname?: string;
	id: string;
	last_seen_at: string;
	name: string;
	os_version: string;
	platform: string;
	registered_at: string;
}

const ACTIVE_WINDOW_MS = 5 * 60_000;

const PLATFORMS: Record<string, { icon: LucideIcon; label: string }> = {
	darwin: { icon: LaptopIcon, label: "macOS" },
	linux: { icon: ServerIcon, label: "Linux" },
	windows: { icon: MonitorIcon, label: "Windows" },
};

const ARCHES: Record<string, string> = { amd64: "x64", arm64: "ARM64", "386": "x86" };

const archLabel = ({ platform, arch }: Device) => {
	if (platform === "darwin" && arch) return arch === "arm64" ? "Apple Silicon" : "Intel";
	return ARCHES[arch] ?? arch;
};

const systemLabel = (device: Device) => device.os_version || PLATFORMS[device.platform]?.label || device.platform;

const isActive = (device: Device) => Date.now() - new Date(device.last_seen_at).getTime() < ACTIVE_WINDOW_MS;

const isOlder = (version: string, latest: string) => {
	const a = version.split(".").map(Number);
	const b = latest.split(".").map(Number);
	for (let i = 0; i < Math.max(a.length, b.length); i++) {
		if ((a[i] ?? 0) !== (b[i] ?? 0)) return (a[i] ?? 0) < (b[i] ?? 0);
	}
	return false;
};

// Versions before 0.1.40 don't report themselves, so a missing one is always outdated.
const needsUpdate = (version: string, latest: string | null) => !!latest && version !== "dev" && (!version || isOlder(version, latest));

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

const formatDate = (dateStr: string) => new Date(dateStr).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });

export const DeviceTable = () => {
	const [devices, setDevices] = useState<Device[]>([]);
	const [latest, setLatest] = useState<string | null>(null);
	const [loading, setLoading] = useState(true);

	useEffect(() => {
		fetch("/api/vault/devices")
			.then((res) => res.json())
			.then((data) => setDevices(data.devices ?? []))
			.finally(() => setLoading(false));
		fetch("/api/cli-version")
			.then((res) => res.json())
			.then((data) => setLatest(data.version))
			.catch(() => setLatest(null));
	}, []);

	const columns = useMemo<DataViewColumn<Device>[]>(
		() => [
			{
				accessorKey: "name",
				header: "Device",
				cell: ({ row }) => {
					const { name, hostname, platform } = row.original;
					const Icon = PLATFORMS[platform]?.icon ?? MonitorIcon;
					return (
						<div className="flex min-w-0 items-center gap-3">
							<span className="flex size-8 shrink-0 items-center justify-center rounded-md border bg-muted/40 text-muted-foreground">
								<Icon className="size-4" />
							</span>
							<div className="min-w-0">
								<p className="truncate font-semibold text-sm">{name}</p>
								{hostname && hostname !== name && <p className="truncate font-mono text-muted-foreground text-xs">{hostname}</p>}
							</div>
						</div>
					);
				},
				meta: {
					cellClassName: "min-w-[14rem]",
					headerClassName: "min-w-[14rem]",
					toggleable: false,
				},
			},
			{
				id: "system",
				accessorFn: systemLabel,
				header: "System",
				cell: ({ row }) => (
					<div className="min-w-0">
						<p className="truncate text-sm">{systemLabel(row.original)}</p>
						<p className="truncate text-muted-foreground text-xs">{archLabel(row.original)}</p>
					</div>
				),
				meta: {
					cellClassName: "min-w-[10rem]",
					headerClassName: "min-w-[10rem]",
					responsive: "sm",
				},
			},
			{
				accessorKey: "cli_version",
				header: "Forged",
				cell: ({ row }) => {
					const version = row.original.cli_version;
					return (
						<div className="min-w-0">
							<p className="truncate font-mono text-sm">{version || "Unknown"}</p>
							{needsUpdate(version, latest) && <p className="truncate text-warning text-xs">Update available</p>}
						</div>
					);
				},
				meta: {
					cellClassName: "min-w-[8rem]",
					headerClassName: "min-w-[8rem]",
					responsive: "lg",
				},
			},
			{
				accessorKey: "last_seen_at",
				header: "Status",
				cell: ({ row }) => {
					const active = isActive(row.original);
					return (
						<div className="flex flex-col items-start gap-1">
							<Badge
								className={active ? "border-success/20 bg-success/10 text-success hover:bg-success/10" : undefined}
								variant={active ? "default" : "outline"}
							>
								<span className={active ? "size-1.5 rounded-full bg-success" : "size-1.5 rounded-full bg-muted-foreground"} />
								{active ? "Active" : "Idle"}
							</Badge>
							<span className="text-muted-foreground text-xs">{relativeTime(row.original.last_seen_at)}</span>
						</div>
					);
				},
				meta: {
					cellClassName: "w-[8rem]",
					headerClassName: "w-[8rem]",
				},
			},
			{
				accessorKey: "registered_at",
				header: "First Seen",
				cell: ({ row }) => <span className="text-muted-foreground text-sm">{formatDate(row.original.registered_at)}</span>,
				meta: {
					cellClassName: "w-[8rem]",
					headerClassName: "w-[8rem]",
					responsive: "xl",
				},
			},
		],
		[latest]
	);

	return (
		<DataViewTable
			columns={columns}
			data={devices}
			emptyState={{
				title: "No devices yet",
				description: "A machine appears here after it syncs your vault with Forged.",
			}}
			entityLabel="devices"
			getRowId={(device) => device.id}
			getSearchText={(device) =>
				[device.name, device.hostname, systemLabel(device), archLabel(device), device.cli_version, isActive(device) ? "active" : "idle"].join(" ")
			}
			globalFilterPlaceholder="Search devices, systems, or versions"
			initialSorting={[{ id: "name", desc: false }]}
			isLoading={loading}
			rowHeight={56}
		/>
	);
};
