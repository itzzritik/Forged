import { type NextRequest, NextResponse } from "next/server";
import { COOKIE_NAME, decrypt, parseJWTPayload } from "@/lib/auth";

export async function proxy(request: NextRequest) {
  const cookie = request.cookies.get(COOKIE_NAME);
  if (!cookie?.value) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  const token = await decrypt(cookie.value);
  if (!token) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  const payload = parseJWTPayload(token);
  const exp = payload?.exp;
  if (typeof exp === "number" && exp * 1000 < Date.now()) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*"],
};
