import { say } from "./message";
import { sparks } from "./motion";

export async function copyText(text: string, message: string, from?: Element | null) {
	try {
		await navigator.clipboard.writeText(text);
	} catch {
		say("Couldn't copy. The browser blocked clipboard access.", "danger");
		return false;
	}
	if (from) sparks(from, 14);
	say(message);
	return true;
}
