import { NextResponse } from "next/server";
import { COOKIE_NAME, getAccessTokenForRequest, setSessionCookieOnResponse } from "@/lib/auth";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// A dead session must drop its cookies, or /login sends the browser straight back to the dashboard.
function signedOut(response: NextResponse) {
	response.cookies.delete(COOKIE_NAME);
	response.cookies.delete("forged_logged_in");
	return response;
}

export async function proxyToAPI(method: string, path: string, body?: string, extraHeaders: Record<string, string> = {}): Promise<NextResponse> {
	const { token, refreshed } = await getAccessTokenForRequest();
	if (!token) return signedOut(NextResponse.json({ error: "unauthorized" }, { status: 401 }));

	const headers: Record<string, string> = {
		Authorization: `Bearer ${token}`,
		...extraHeaders,
	};
	if (body) headers["Content-Type"] = "application/json";

	const resp = await fetch(`${API_URL}${path}`, {
		method,
		headers,
		body: body || undefined,
	});

	const data = await resp.text();
	const next = new NextResponse(data, {
		status: resp.status,
		headers: {
			"Content-Type": "application/json",
			"Cache-Control": "no-store",
		},
	});
	if (resp.status === 401) return signedOut(next);
	if (refreshed) await setSessionCookieOnResponse(next, refreshed);
	return next;
}
