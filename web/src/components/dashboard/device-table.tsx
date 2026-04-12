"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

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

	return (
		<Table>
			<TableHeader>
				<TableRow>
					<TableHead>Device</TableHead>
					<TableHead>Hostname</TableHead>
					<TableHead>Last Seen</TableHead>
					<TableHead>Status</TableHead>
				</TableRow>
			</TableHeader>
			<TableBody>
				{loading &&
					Array.from({ length: 3 }).map((_, i) => (
						<TableRow key={i}>
							<TableCell>
								<div className="space-y-1">
									<Skeleton className="h-4 w-32" />
									<Skeleton className="h-3 w-20" />
								</div>
							</TableCell>
							<TableCell>
								<Skeleton className="h-4 w-40" />
							</TableCell>
							<TableCell>
								<Skeleton className="h-4 w-20" />
							</TableCell>
							<TableCell>
								<Skeleton className="h-5 w-16 rounded-full" />
							</TableCell>
						</TableRow>
					))}
				{!loading && devices.length === 0 && (
					<TableRow>
						<TableCell className="py-12 text-center" colSpan={4}>
							<p className="font-medium text-sm">No devices registered</p>
							<p className="mt-1 text-muted-foreground text-xs">
								Devices are registered when you run <code className="font-mono">`forged sync`</code> from a new machine.
							</p>
						</TableCell>
					</TableRow>
				)}
				{!loading &&
					devices.length > 0 &&
					devices.map((device) => (
						<TableRow key={device.id}>
							<TableCell>
								<p className="font-semibold text-sm">{device.name}</p>
								<p className="text-muted-foreground text-xs">{device.platform}</p>
							</TableCell>
							<TableCell className="font-mono text-muted-foreground text-xs">{device.hostname}</TableCell>
							<TableCell className="text-muted-foreground text-sm">{relativeTime(device.last_seen_at)}</TableCell>
							<TableCell>
								{device.approved ? (
									<Badge className="border-success/20 bg-success/10 text-success hover:bg-success/10">Approved</Badge>
								) : (
									<Badge className="border-yellow-500/20 bg-yellow-500/10 text-yellow-500 hover:bg-yellow-500/10">Pending</Badge>
								)}
							</TableCell>
						</TableRow>
					))}
			</TableBody>
		</Table>
	);
};
