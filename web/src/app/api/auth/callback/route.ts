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

  if (code) {
    return NextResponse.redirect(new URL("/auth/success", request.url));
  }

  return NextResponse.redirect(new URL("/dashboard", request.url));
}
