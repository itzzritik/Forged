import { type NextRequest, NextResponse } from "next/server";
import { setSessionCookie } from "@/lib/auth";

export async function GET(request: NextRequest) {
  const params = request.nextUrl.searchParams;
  const token = params.get("token");
  const callback = params.get("callback");

  if (!token) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  await setSessionCookie(token);

  if (callback) {
    try {
      const target = new URL(callback);
      target.searchParams.set("token", token);
      const userId = params.get("user_id");
      const email = params.get("email");
      if (userId) target.searchParams.set("user_id", userId);
      if (email) target.searchParams.set("email", email);
      return NextResponse.redirect(target.toString());
    } catch {
      return NextResponse.redirect(new URL("/dashboard", request.url));
    }
  }

  return NextResponse.redirect(new URL("/dashboard", request.url));
}
