const DEVICE_ID_KEY = "forged-browser-device-id";

export function getBrowserDeviceId(): string {
	if (typeof window === "undefined") {
		return "forged-browser";
	}

	const existing = window.localStorage.getItem(DEVICE_ID_KEY);
	if (existing) {
		return existing;
	}

	const created = crypto.randomUUID();
	window.localStorage.setItem(DEVICE_ID_KEY, created);
	return created;
}
