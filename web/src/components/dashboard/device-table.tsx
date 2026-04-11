"use client";

import { useEffect, useState } from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";

interface Device {
  id: string;
  name: string;
  platform: string;
  hostname: string;
  registered_at: string;
  last_seen_at: string;
  approved: boolean;
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
        {loading ? (
          Array.from({ length: 3 }).map((_, i) => (
            <TableRow key={i}>
              <TableCell>
                <div className="space-y-1">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-3 w-20" />
                </div>
              </TableCell>
              <TableCell><Skeleton className="h-4 w-40" /></TableCell>
              <TableCell><Skeleton className="h-4 w-20" /></TableCell>
              <TableCell><Skeleton className="h-5 w-16 rounded-full" /></TableCell>
            </TableRow>
          ))
        ) : devices.length === 0 ? (
          <TableRow>
            <TableCell colSpan={4} className="text-center py-12">
              <p className="text-sm font-medium">No devices registered</p>
              <p className="text-xs text-muted-foreground mt-1">
                Devices are registered when you run{" "}
                <code className="font-mono">`forged sync`</code> from a new machine.
              </p>
            </TableCell>
          </TableRow>
        ) : (
          devices.map((device) => (
            <TableRow key={device.id}>
              <TableCell>
                <p className="font-semibold text-sm">{device.name}</p>
                <p className="text-xs text-muted-foreground">{device.platform}</p>
              </TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">
                {device.hostname}
              </TableCell>
              <TableCell className="text-sm text-muted-foreground">
                {relativeTime(device.last_seen_at)}
              </TableCell>
              <TableCell>
                {device.approved ? (
                  <Badge className="bg-green-500/10 text-green-500 border-green-500/20 hover:bg-green-500/10">
                    Approved
                  </Badge>
                ) : (
                  <Badge className="bg-yellow-500/10 text-yellow-500 border-yellow-500/20 hover:bg-yellow-500/10">
                    Pending
                  </Badge>
                )}
              </TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  );
};
