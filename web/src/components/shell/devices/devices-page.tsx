"use client";

import { CircleArrowUpIcon, MonitorSmartphoneIcon } from "lucide-react";
import { useDeferredValue, useMemo, useRef, useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useNow } from "@/hooks/use-now";
import { useResource } from "@/hooks/use-resource";
import { useShortcuts } from "@/hooks/use-shortcuts";
import { ago, compareText, formatDate } from "@/lib/format";
import { useHints } from "../frame/frame";
import { Action } from "../kit/action";
import { Count } from "../kit/count";
import { type Column, DataTable } from "../kit/data-table";
import { FilterPrompt } from "../kit/filter-prompt";
import { Panel } from "../kit/panel";
import { archLabel, type Device, isActive, needsUpdate, systemLabel } from "./labels";

const SKELETON_ROWS = [0, 1, 2, 3];
const NAME_SORT = { desc: false, id: "name" };
const deviceKey = (device: Device) => device.id;
const seenAt = (device: Device) => Date.parse(device.last_seen_at) || 0;
const seen = (device: Device, now: number) => (isActive(device, now) ? "Active now" : (ago(device.last_seen_at, now) ?? "never"));
const system = (device: Device) => [systemLabel(device), archLabel(device)].filter(Boolean).join(", ");

function Version({ device, latest }: { device: Device; latest: string | null }) {
	return (
		<span className="inline-flex items-center gap-1.5">
			<span className={device.cli_version ? "text-muted-foreground" : "text-faint"}>{device.cli_version || "Unknown"}</span>
			{needsUpdate(device.cli_version, latest) && (
				<Tooltip>
					<TooltipTrigger aria-label={`Update available: ${latest}`} className="flex cursor-default text-warning" render={<button type="button" />}>
						<CircleArrowUpIcon className="size-3.5" />
					</TooltipTrigger>
					<TooltipContent>Update available: {latest}. Run npm i -g forgedssh@latest</TooltipContent>
				</Tooltip>
			)}
		</span>
	);
}

function deviceColumns(now: number, latest: string | null): Column<Device>[] {
	return [
		{
			cell: (device) => (
				<>
					<span>{device.name}</span>
					{device.hostname && device.hostname !== device.name && <span className="ml-2 text-faint">{device.hostname}</span>}
					<small className="block font-sans text-[12px] text-muted-foreground md:hidden">
						{system(device)}, {device.cli_version || "unknown version"}, {seen(device, now)}
					</small>
				</>
			),
			header: "Device",
			id: "name",
			sort: (a, b) => compareText(a.name, b.name),
			width: "minmax(0,1.3fr)",
		},
		{
			cell: (device) => <span className="text-muted-foreground">{system(device)}</span>,
			header: "System",
			id: "system",
			sort: (a, b) => compareText(system(a), system(b)),
			wide: true,
			width: "minmax(0,1fr)",
		},
		{ cell: (device) => <Version device={device} latest={latest} />, header: "Forged", id: "version", wide: true, width: "120px" },
		{
			cell: (device) => <span className={isActive(device, now) ? "text-success" : "text-muted-foreground"}>{seen(device, now)}</span>,
			header: "Last seen",
			id: "seen",
			sort: (a, b) => seenAt(b) - seenAt(a),
			wide: true,
			width: "120px",
		},
		{
			align: "end",
			cell: (device) => <span className="text-faint">{formatDate(device.registered_at)}</span>,
			header: "First seen",
			id: "first",
			wide: true,
			width: "140px",
		},
	];
}

export function DevicesPage() {
	const devices = useResource<{ devices: Device[] }>("/api/vault/devices");
	const cli = useResource<{ version: string | null }>("/api/cli-version");
	const now = useNow(60_000);
	const latest = cli.data?.version ?? null;
	const list = useMemo(() => devices.data?.devices ?? [], [devices.data]);
	const columns = useMemo(() => deviceColumns(now, latest), [now, latest]);
	const [query, setQuery] = useState("");
	const deferred = useDeferredValue(query.trim().toLowerCase());
	const filter = useRef<HTMLInputElement>(null);
	const rows = useMemo(
		() => (deferred ? list.filter((device) => [device.name, device.hostname, system(device), device.cli_version].join(" ").toLowerCase().includes(deferred)) : list),
		[list, deferred]
	);

	useShortcuts({ "/": () => filter.current?.focus(), esc: () => setQuery(""), ...(devices.error ? { r: devices.retry } : {}) });
	useHints(devices.error ? [{ combo: "r", label: "Retry" }] : [{ combo: "/", label: "Filter" }]);

	let body = (
		<>
			<FilterPrompt label="Filter devices" onChange={setQuery} placeholder="filter by name, system or version" ref={filter} value={query} />
			<DataTable
				columns={columns}
				defaultSort={NAME_SORT}
				empty={
					<p className="px-[18px] py-10 font-sans text-muted-foreground">
						{list.length ? "No devices match the filter." : "Devices show up here after they sync."}
					</p>
				}
				getId={deviceKey}
				keyboard={false}
				label="Devices"
				rows={rows}
			/>
		</>
	);
	if (devices.loading) {
		body = (
			<div aria-busy className="flex flex-col gap-2 px-4 pt-2">
				{SKELETON_ROWS.map((row) => (
					<Skeleton className="h-7" key={row} />
				))}
			</div>
		);
	}
	if (devices.error) {
		body = (
			<div className="grid justify-items-start gap-3 px-[18px] py-8 font-sans">
				<p>Couldn't load your devices. {devices.error}</p>
				<Action combo="r" onClick={devices.retry} tone="primary">
					Retry
				</Action>
			</div>
		);
	}

	return (
		<Panel
			aside={devices.data ? <Count value={list.length} word="device" /> : undefined}
			className="flex min-h-0 flex-1 flex-col"
			icon={MonitorSmartphoneIcon}
			title="Devices"
		>
			{body}
		</Panel>
	);
}
