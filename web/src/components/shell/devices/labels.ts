export interface Device {
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
const PLATFORMS: Record<string, string> = { darwin: "macOS", linux: "Linux", windows: "Windows" };
const ARCHES: Record<string, string> = { "386": "x86", amd64: "x64", arm64: "ARM64" };

export const archLabel = ({ arch, platform }: Device) => {
	if (platform === "darwin" && arch) return arch === "arm64" ? "Apple Silicon" : "Intel";
	return ARCHES[arch] ?? arch;
};

export const systemLabel = (device: Device) => device.os_version || PLATFORMS[device.platform] || device.platform;

export const isActive = (device: Device, now: number) => now - Date.parse(device.last_seen_at) < ACTIVE_WINDOW_MS;

const isOlder = (version: string, latest: string) => {
	const a = version.split(".").map(Number);
	const b = latest.split(".").map(Number);
	for (let i = 0; i < Math.max(a.length, b.length); i++) {
		if ((a[i] ?? 0) !== (b[i] ?? 0)) return (a[i] ?? 0) < (b[i] ?? 0);
	}
	return false;
};

// Versions before 0.1.40 don't report themselves, so a missing one is always outdated.
export const needsUpdate = (version: string, latest: string | null): latest is string =>
	Boolean(latest) && version !== "dev" && (!version || isOlder(version, latest as string));
