import { unzipSync } from "fflate";
import { DEFAULT_IMPORTED_NAME, fallbackImportedName, normalizeImportedName } from "./name";
import type { ImportedKey } from "./types";

const PRIVATE_KEY_RE = /-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z0-9 ]*PRIVATE KEY-----/;

export function parse1Password(data: Uint8Array): ImportedKey[] {
	if (looksLikeZipArchive(data)) {
		return parse1Password1PUX(data);
	}
	return parse1PasswordCSV(new TextDecoder().decode(data));
}

function parse1Password1PUX(data: Uint8Array): ImportedKey[] {
	const files = unzipSync(data);
	const exportDataBytes = files["export.data"];
	if (!exportDataBytes) throw new Error("export.data not found in 1pux archive");

	const exportData = JSON.parse(new TextDecoder().decode(exportDataBytes));
	const keys: ImportedKey[] = [];

	for (const account of exportData.accounts || []) {
		for (const vault of account.vaults || []) {
			for (const item of vault.items || []) {
				if (item.categoryUuid !== "114") continue;
				const privKey = extractSSHKey(item);
				if (!privKey) continue;
				keys.push({
					name: normalizeImportedName(item.overview?.title || DEFAULT_IMPORTED_NAME),
					privateKey: privKey,
				});
			}
		}
	}
	return keys;
}

function extractSSHKey(item: Record<string, unknown>): string | null {
	const details = item.details as Record<string, unknown> | undefined;
	if (!details) return null;

	for (const section of (details.sections as Record<string, unknown>[]) || []) {
		for (const field of (section.fields as Record<string, unknown>[]) || []) {
			const value = field.value;
			if (typeof value === "object" && value !== null) {
				const sshKey = (value as Record<string, Record<string, string>>).sshKey;
				if (sshKey?.privateKey) return extractPrivateKeyBlock(sshKey.privateKey);
			}
			if (typeof value === "string") {
				const privateKey = extractPrivateKeyBlock(value);
				if (privateKey) return privateKey;
			}
		}
	}
	return null;
}

function parse1PasswordCSV(text: string): ImportedKey[] {
	const rows = parseCSV(text);
	if (rows.length === 0) return [];

	const { hasHeader, titleIndex } = detectCSVHeader(rows[0]);
	const dataRows = hasHeader ? rows.slice(1) : rows;
	const keys: ImportedKey[] = [];

	for (const [index, row] of dataRows.entries()) {
		const privateKey = extractPrivateKeyFromRow(row);
		if (!privateKey) continue;
		keys.push({
			name: deriveCSVRowName(row, titleIndex, index + 1),
			privateKey,
		});
	}

	return keys;
}

function looksLikeZipArchive(data: Uint8Array): boolean {
	return data.length >= 4 && data[0] === 0x50 && data[1] === 0x4b &&
		((data[2] === 0x03 && data[3] === 0x04) || (data[2] === 0x05 && data[3] === 0x06) || (data[2] === 0x07 && data[3] === 0x08));
}

function parseCSV(text: string): string[][] {
	const rows: string[][] = [];
	let row: string[] = [];
	let field = "";
	let inQuotes = false;

	for (let i = 0; i < text.length; i++) {
		const ch = text[i];
		if (inQuotes) {
			if (ch === "\"") {
				if (text[i + 1] === "\"") {
					field += "\"";
					i++;
					continue;
				}
				inQuotes = false;
				continue;
			}
			field += ch;
			continue;
		}

		if (ch === "\"") {
			inQuotes = true;
			continue;
		}
		if (ch === ",") {
			row.push(field);
			field = "";
			continue;
		}
		if (ch === "\r") {
			if (text[i + 1] === "\n") i++;
			row.push(field);
			rows.push(row);
			row = [];
			field = "";
			continue;
		}
		if (ch === "\n") {
			row.push(field);
			rows.push(row);
			row = [];
			field = "";
			continue;
		}
		field += ch;
	}

	if (field !== "" || row.length > 0) {
		row.push(field);
		rows.push(row);
	}

	return rows.filter((parsedRow) => parsedRow.some((value) => value !== ""));
}

function detectCSVHeader(row: string[]): { hasHeader: boolean; titleIndex: number } {
	let hasHeader = false;
	let titleIndex = -1;

	for (const [index, value] of row.entries()) {
		switch (normalizeCSVHeader(value)) {
			case "title":
			case "name":
				hasHeader = true;
				titleIndex = index;
				break;
			case "website":
			case "url":
			case "username":
			case "password":
			case "notes":
			case "tags":
			case "favorite":
			case "archived":
			case "one-timepassword":
				hasHeader = true;
				break;
		}
	}

	return { hasHeader, titleIndex };
}

function normalizeCSVHeader(value: string): string {
	return value.toLowerCase().trim().replaceAll(" ", "").replaceAll("-", "").replaceAll("_", "");
}

function extractPrivateKeyFromRow(row: string[]): string | null {
	for (const value of row) {
		const privateKey = extractPrivateKeyBlock(value);
		if (privateKey) return privateKey;
	}
	return null;
}

function extractPrivateKeyBlock(value: string): string | null {
	const match = value.match(PRIVATE_KEY_RE);
	return match ? match[0].trim() : null;
}

function deriveCSVRowName(row: string[], titleIndex: number, ordinal: number): string {
	if (titleIndex >= 0 && titleIndex < row.length) {
		const title = normalizeImportedName(row[titleIndex]);
		if (title !== DEFAULT_IMPORTED_NAME) return title;
	}

	for (const value of row) {
		const trimmed = value.trim();
		if (trimmed === "" || trimmed.includes("PRIVATE KEY")) continue;
		const name = normalizeImportedName(trimmed);
		if (name !== DEFAULT_IMPORTED_NAME) return name;
	}

	return fallbackImportedName(ordinal);
}
