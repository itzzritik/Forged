import { type NextRequest, NextResponse } from "next/server";
import { clearSessionCookie, getSession } from "@/lib/auth";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export async function GET(request: NextRequest) {
	const session = await getSession();
	if (session?.refreshToken) {
		try {
			await fetch(`${API_URL}/api/v1/auth/logout`, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ refresh_token: session.refreshToken }),
				cache: "no-store",
			});
		} catch {
			// Best effort.
		}
	}
	await clearSessionCookie();
	const response = NextResponse.redirect(new URL("/", request.url));
	response.cookies.delete("forged_logged_in");
	return response;
}
