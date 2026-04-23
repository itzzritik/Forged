import { type NextRequest, NextResponse } from "next/server";
import { sessionFromQuery, setSessionCookieOnResponse } from "@/lib/auth";

export async function GET(request: NextRequest) {
	const params = request.nextUrl.searchParams;
	const code = params.get("code");
	const session = sessionFromQuery(params);

	if (!session) {
		return NextResponse.redirect(new URL("/login", request.url));
	}

	const target = code ? new URL("/auth/success", request.url) : new URL("/dashboard", request.url);
	const response = NextResponse.redirect(target);
	await setSessionCookieOnResponse(response, session);
	response.cookies.set("forged_logged_in", "1", {
		httpOnly: false,
		secure: process.env.NODE_ENV === "production",
		sameSite: "lax",
		path: "/",
		maxAge: 60 * 60 * 24 * 30,
	});
	return response;
}
