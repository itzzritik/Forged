interface RawVaultMetadata {
	created_at?: string;
	device_id?: string;
	device_name?: string;
}

interface RawTombstone {
	deleted_at: string;
	deleted_by_device?: string;
	key_id: string;
}

interface RawVaultKey {
	comment?: string;
	created_at?: string;
	device_origin?: string;
	encrypted_cipher_key?: string;
	encrypted_private_key?: string;
	fingerprint?: string;
	git_signing?: boolean;
	id: string;
	last_used_at?: string | null;
	name?: string;
	public_key?: string;
	tags?: string[];
	type?: string;
	updated_at?: string;
	version?: number;
	[key: string]: unknown;
}

interface RawVaultDocument {
	key_generation?: number;
	keys?: RawVaultKey[];
	metadata?: RawVaultMetadata;
	tombstones?: RawTombstone[];
	version_vector?: Record<string, number>;
	[key: string]: unknown;
}

const TOMBSTONE_TTL_MS = 90 * 24 * 60 * 60 * 1000;

export function mergeThreeWayRaw(baseRaw: string, localRaw: string, remoteRaw: string, localDeviceId: string, remoteDeviceId: string): string {
	const merged = mergeThreeWayDocument(parseDocument(baseRaw), parseDocument(localRaw), parseDocument(remoteRaw), localDeviceId, remoteDeviceId);
	return JSON.stringify(merged);
}

function mergeThreeWayDocument(base: RawVaultDocument, local: RawVaultDocument, remote: RawVaultDocument, localDeviceId: string, remoteDeviceId: string): RawVaultDocument {
	const merged: RawVaultDocument = {
		key_generation: maxNumber(base.key_generation ?? 1, local.key_generation ?? 1, remote.key_generation ?? 1),
		metadata: mergeMetadata(base.metadata, local.metadata, remote.metadata),
		tombstones: mergeTombstones(base.tombstones, local.tombstones, remote.tombstones),
		version_vector: mergeVersionVectors(base.version_vector, local.version_vector, remote.version_vector),
		keys: [],
	};

	const tombstoneSet = new Set((merged.tombstones ?? []).map((tombstone) => tombstone.key_id));
	const baseKeys = indexKeys(base.keys);
	const localKeys = indexKeys(local.keys);
	const remoteKeys = indexKeys(remote.keys);
	const ids = new Set([...baseKeys.keys(), ...localKeys.keys(), ...remoteKeys.keys()]);

	for (const id of ids) {
		if (tombstoneSet.has(id)) {
			continue;
		}

		const baseKey = baseKeys.get(id);
		const localKey = localKeys.get(id);
		const remoteKey = remoteKeys.get(id);

		if (localKey && remoteKey) {
			merged.keys?.push(mergeKey(baseKey, localKey, remoteKey, localDeviceId, remoteDeviceId));
			continue;
		}
		if (localKey) {
			merged.keys?.push(cloneKey(localKey));
			continue;
		}
		if (remoteKey) {
			merged.keys?.push(cloneKey(remoteKey));
		}
	}

	merged.keys = enforceGitSigningInvariant(merged.keys ?? [], localDeviceId, remoteDeviceId);
	merged.keys.sort((left, right) => compareKeyNames(left, right));

	return merged;
}

function mergeMetadata(base?: RawVaultMetadata, local?: RawVaultMetadata, remote?: RawVaultMetadata): RawVaultMetadata {
	return {
		created_at: earliestTime(base?.created_at, local?.created_at, remote?.created_at),
		device_id: firstNonEmpty(local?.device_id, remote?.device_id, base?.device_id),
		device_name: firstNonEmpty(local?.device_name, remote?.device_name, base?.device_name),
	};
}

function mergeVersionVectors(...vectors: Array<Record<string, number> | undefined>): Record<string, number> {
	const merged: Record<string, number> = {};
	for (const vector of vectors) {
		if (!vector) {
			continue;
		}
		for (const [deviceId, value] of Object.entries(vector)) {
			merged[deviceId] = Math.max(merged[deviceId] ?? 0, value);
		}
	}
	return merged;
}

function mergeTombstones(...tombstoneSets: Array<RawTombstone[] | undefined>): RawTombstone[] {
	const cutoff = Date.now() - TOMBSTONE_TTL_MS;
	const seen = new Map<string, RawTombstone>();

	for (const tombstones of tombstoneSets) {
		for (const tombstone of tombstones ?? []) {
			const deletedAt = Date.parse(tombstone.deleted_at);
			if (Number.isNaN(deletedAt) || deletedAt < cutoff) {
				continue;
			}
			const existing = seen.get(tombstone.key_id);
			if (!existing || deletedAt > Date.parse(existing.deleted_at)) {
				seen.set(tombstone.key_id, { ...tombstone });
			}
		}
	}

	return [...seen.values()].sort((left, right) => compareTimes(left.deleted_at, right.deleted_at) || left.key_id.localeCompare(right.key_id));
}

function mergeKey(base: RawVaultKey | undefined, local: RawVaultKey, remote: RawVaultKey, localDeviceId: string, remoteDeviceId: string): RawVaultKey {
	const winner = preferLocal(local, remote, localDeviceId, remoteDeviceId) ? local : remote;
	const merged = cloneKey(winner);

	merged.created_at = earliestTime(base?.created_at, local.created_at, remote.created_at);
	merged.device_origin = firstNonEmpty(merged.device_origin, local.device_origin, remote.device_origin, localDeviceId);
	merged.tags = mergeStringSets(base?.tags, local.tags, remote.tags);
	merged.version = maxNumber(base?.version ?? 0, local.version ?? 0, remote.version ?? 0);

	return merged;
}

function preferLocal(local: RawVaultKey, remote: RawVaultKey, localDeviceId: string, remoteDeviceId: string): boolean {
	const updatedComparison = compareTimes(local.updated_at, remote.updated_at);
	if (updatedComparison !== 0) {
		return updatedComparison > 0;
	}

	const localVersion = local.version ?? 0;
	const remoteVersion = remote.version ?? 0;
	if (localVersion !== remoteVersion) {
		return localVersion > remoteVersion;
	}

	return tieBreakId(local, localDeviceId) <= tieBreakId(remote, remoteDeviceId);
}

function mergeStringSets(baseValues: string[] | undefined, localValues: string[] | undefined, remoteValues: string[] | undefined): string[] {
	const base = new Set(baseValues ?? []);
	const local = new Set(localValues ?? []);
	const remote = new Set(remoteValues ?? []);
	const merged = new Set<string>();

	for (const value of new Set([...base, ...local, ...remote])) {
		const inBase = base.has(value);
		const inLocal = local.has(value);
		const inRemote = remote.has(value);

		if (inBase) {
			if (inLocal && inRemote) {
				merged.add(value);
			}
			continue;
		}
		if (inLocal || inRemote) {
			merged.add(value);
		}
	}

	return [...merged].sort((left, right) => left.localeCompare(right));
}

function enforceGitSigningInvariant(keys: RawVaultKey[], localDeviceId: string, remoteDeviceId: string): RawVaultKey[] {
	let winnerIndex = -1;

	for (const [index, key] of keys.entries()) {
		if (!key.git_signing) {
			continue;
		}
		if (winnerIndex < 0 || preferLocal(key, keys[winnerIndex], localDeviceId, remoteDeviceId)) {
			winnerIndex = index;
		}
	}

	if (winnerIndex < 0) {
		return keys;
	}

	return keys.map((key, index) => ({ ...key, git_signing: index === winnerIndex }));
}

function indexKeys(keys: RawVaultKey[] | undefined): Map<string, RawVaultKey> {
	return new Map((keys ?? []).filter((key) => Boolean(key.id)).map((key) => [key.id, cloneKey(key)]));
}

function compareKeyNames(left: RawVaultKey, right: RawVaultKey): number {
	return (left.name ?? "").localeCompare(right.name ?? "") || left.id.localeCompare(right.id);
}

function compareTimes(left: string | undefined, right: string | undefined): number {
	const leftTime = parseTimestamp(left);
	const rightTime = parseTimestamp(right);
	return leftTime - rightTime;
}

function parseTimestamp(value: string | undefined): number {
	if (!value) {
		return 0;
	}
	const parsed = Date.parse(value);
	return Number.isNaN(parsed) ? 0 : parsed;
}

function earliestTime(...values: Array<string | undefined>): string {
	let earliest = "";
	let earliestValue = 0;

	for (const value of values) {
		const parsed = parseTimestamp(value);
		if (parsed === 0) {
			continue;
		}
		if (!earliest || parsed < earliestValue) {
			earliest = value ?? "";
			earliestValue = parsed;
		}
	}

	return earliest;
}

function maxNumber(...values: number[]): number {
	return values.reduce((current, value) => Math.max(current, value), 0);
}

function firstNonEmpty(...values: Array<string | undefined>): string {
	for (const value of values) {
		if (value) {
			return value;
		}
	}
	return "";
}

function tieBreakId(key: RawVaultKey, fallback: string): string {
	return firstNonEmpty(key.device_origin, key.id, fallback);
}

function cloneKey(key: RawVaultKey): RawVaultKey {
	const clone: RawVaultKey = {
		...key,
		tags: [...(key.tags ?? [])],
	};
	delete clone.host_rules;
	return clone;
}

function parseDocument(raw: string): RawVaultDocument {
	const parsed = JSON.parse(raw) as RawVaultDocument;
	return {
		...parsed,
		keys: (parsed.keys ?? []).map((key) => cloneKey(key)),
		metadata: parsed.metadata ? { ...parsed.metadata } : {},
		tombstones: (parsed.tombstones ?? []).map((tombstone) => ({ ...tombstone })),
		version_vector: { ...(parsed.version_vector ?? {}) },
	};
}
