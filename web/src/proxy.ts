import { type NextRequest, NextResponse } from "next/server";
import { COOKIE_NAME, decodeSessionCookie, refreshExpired } from "@/lib/auth";

function hasValidSessionCookie(value: string | undefined): Promise<boolean> {
	if (!value) return Promise.resolve(false);
	return decodeSessionCookie(value).then((session) => !!session && !refreshExpired(session));
}

export async function proxy(request: NextRequest) {
	const cookie = request.cookies.get(COOKIE_NAME);
	const isAuthenticated = await hasValidSessionCookie(cookie?.value);
	const path = request.nextUrl.pathname;

	// Redirect authenticated users away from /login (unless CLI flow with callback)
	if (path === "/login" && isAuthenticated && !request.nextUrl.searchParams.has("code")) {
		return NextResponse.redirect(new URL("/dashboard", request.url));
	}

	// Protect /dashboard routes
	if ((path.startsWith("/dashboard") || path === "/auth/success") && !isAuthenticated) {
		return NextResponse.redirect(new URL("/login", request.url));
	}

	return NextResponse.next();
}

export const config = {
	matcher: ["/dashboard/:path*", "/login", "/auth/success"],
};
