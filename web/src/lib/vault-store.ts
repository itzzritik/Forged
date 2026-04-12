const DB_NAME = "forged-vault";
const STORE_NAME = "keys";
const KEY_ID = "sync-key";
const TIMEOUT_MS = 4 * 60 * 60 * 1000; // 4 hours
const HAS_KEY_FLAG = "forged-has-key";

interface StoredEntry {
	cryptoKey: CryptoKey;
	id: string;
	lastActivity: number;
}

export function hasCachedKeySync(): boolean {
	try {
		return localStorage.getItem(HAS_KEY_FLAG) === "1";
	} catch {
		return false;
	}
}

let memoryFallback: StoredEntry | null = null;
let idbAvailable = true;

function openDB(): Promise<IDBDatabase> {
	return new Promise((resolve, reject) => {
		const req = indexedDB.open(DB_NAME, 1);
		req.onupgradeneeded = () => {
			req.result.createObjectStore(STORE_NAME, { keyPath: "id" });
		};
		req.onsuccess = () => resolve(req.result);
		req.onerror = () => reject(req.error);
	});
}

export async function storeSyncKey(cryptoKey: CryptoKey): Promise<void> {
	const entry: StoredEntry = {
		id: KEY_ID,
		cryptoKey,
		lastActivity: Date.now(),
	};

	try {
		localStorage.setItem(HAS_KEY_FLAG, "1");
	} catch {
		/* localStorage unavailable */
	}

	if (!idbAvailable) {
		memoryFallback = entry;
		return;
	}

	try {
		const db = await openDB();
		const tx = db.transaction(STORE_NAME, "readwrite");
		tx.objectStore(STORE_NAME).put(entry);
		await new Promise<void>((resolve, reject) => {
			tx.oncomplete = () => resolve();
			tx.onerror = () => reject(tx.error);
		});
		db.close();
	} catch {
		idbAvailable = false;
		memoryFallback = entry;
		console.warn("IndexedDB unavailable, using in-memory fallback");
	}
}

export async function getSyncKey(): Promise<StoredEntry | null> {
	if (!idbAvailable) {
		if (!memoryFallback) return null;
		if (Date.now() - memoryFallback.lastActivity > TIMEOUT_MS) {
			memoryFallback = null;
			return null;
		}
		return memoryFallback;
	}

	try {
		const db = await openDB();
		const tx = db.transaction(STORE_NAME, "readonly");
		const req = tx.objectStore(STORE_NAME).get(KEY_ID);

		return new Promise((resolve) => {
			req.onsuccess = () => {
				db.close();
				const entry = req.result as StoredEntry | undefined;
				if (!entry) return resolve(null);
				if (Date.now() - entry.lastActivity > TIMEOUT_MS) {
					clearSyncKey();
					return resolve(null);
				}
				resolve(entry);
			};
			req.onerror = () => {
				db.close();
				resolve(null);
			};
		});
	} catch {
		return null;
	}
}

export async function touchActivity(): Promise<void> {
	if (!idbAvailable) {
		if (memoryFallback) memoryFallback.lastActivity = Date.now();
		return;
	}

	try {
		const db = await openDB();
		const tx = db.transaction(STORE_NAME, "readwrite");
		const store = tx.objectStore(STORE_NAME);
		const req = store.get(KEY_ID);

		req.onsuccess = () => {
			const entry = req.result as StoredEntry | undefined;
			if (entry) {
				entry.lastActivity = Date.now();
				store.put(entry);
			}
			db.close();
		};
		req.onerror = () => db.close();
	} catch {
		/* IndexedDB operation failed - activity not updated */
	}
}

export async function clearSyncKey(): Promise<void> {
	memoryFallback = null;
	try {
		localStorage.removeItem(HAS_KEY_FLAG);
	} catch {
		/* localStorage unavailable */
	}

	if (!idbAvailable) return;

	try {
		const db = await openDB();
		const tx = db.transaction(STORE_NAME, "readwrite");
		tx.objectStore(STORE_NAME).delete(KEY_ID);
		db.close();
	} catch {
		/* IndexedDB operation failed - key not cleared from IDB */
	}
}
