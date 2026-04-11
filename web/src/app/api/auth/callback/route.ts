import { cookies } from "next/headers";
import { type NextRequest, NextResponse } from "next/server";
import { setSessionCookie } from "@/lib/auth";

export async function GET(request: NextRequest) {
	const params = request.nextUrl.searchParams;
	const token = params.get("token");
	const code = params.get("code");

	if (!token) {
		return NextResponse.redirect(new URL("/login", request.url));
	}

	await setSessionCookie(token);

	const cookieStore = await cookies();
	cookieStore.set("forged_logged_in", "1", {
		httpOnly: false,
		secure: process.env.NODE_ENV === "production",
		sameSite: "lax",
		path: "/",
		maxAge: 60 * 60 * 24 * 30,
	});

	if (code) {
		return NextResponse.redirect(new URL("/auth/success", request.url));
	}

	return NextResponse.redirect(new URL("/dashboard", request.url));
}
