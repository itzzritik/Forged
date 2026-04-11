import { type NextRequest, NextResponse } from "next/server";
import { COOKIE_NAME, decrypt, parseJWTPayload } from "@/lib/auth";

function hasValidSession(token: string | null): boolean {
  if (!token) return false;
  const payload = parseJWTPayload(token);
  const exp = payload?.exp;
  return !(typeof exp === "number" && exp * 1000 < Date.now());
}

export async function proxy(request: NextRequest) {
  const cookie = request.cookies.get(COOKIE_NAME);
  const token = cookie?.value ? await decrypt(cookie.value) : null;
  const isAuthenticated = hasValidSession(token);
  const path = request.nextUrl.pathname;

  // Redirect authenticated users away from /login (unless CLI flow with callback)
  if (path === "/login" && isAuthenticated) {
    if (!request.nextUrl.searchParams.has("code")) {
      return NextResponse.redirect(new URL("/dashboard", request.url));
    }
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
