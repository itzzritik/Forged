import { clearSyncKey } from "@/lib/vault-store";

export async function logout() {
	await clearSyncKey();
	window.location.assign("/api/auth/logout");
}
