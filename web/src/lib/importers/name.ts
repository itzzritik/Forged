export const DEFAULT_IMPORTED_NAME = "Imported";

const CONTROL_RE = /[\u0000-\u001f\u007f]+/g;
const SPACE_RE = /\s+/g;

export function normalizeImportedName(name: string): string {
	const normalized = name.replace(CONTROL_RE, " ").trim().replace(SPACE_RE, " ");
	return normalized || DEFAULT_IMPORTED_NAME;
}

export function fallbackImportedName(ordinal: number): string {
	if (ordinal <= 0) {
		return DEFAULT_IMPORTED_NAME;
	}
	return `${DEFAULT_IMPORTED_NAME} ${ordinal}`;
}
