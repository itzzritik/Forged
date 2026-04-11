import { cookies } from "next/headers";
import { type NextRequest, NextResponse } from "next/server";
import { clearSessionCookie } from "@/lib/auth";

export async function GET(request: NextRequest) {
	await clearSessionCookie();
	const cookieStore = await cookies();
	cookieStore.delete("forged_logged_in");
	return NextResponse.redirect(new URL("/", request.url));
}
