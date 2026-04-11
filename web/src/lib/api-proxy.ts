import { NextResponse } from "next/server";
import { getSession } from "@/lib/auth";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export async function proxyToAPI(
  method: string,
  path: string,
  body?: string,
): Promise<NextResponse> {
  const token = await getSession();
  if (!token) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const headers: Record<string, string> = {
    Authorization: `Bearer ${token}`,
  };
  if (body) headers["Content-Type"] = "application/json";

  const resp = await fetch(`${API_URL}${path}`, {
    method,
    headers,
    body: body || undefined,
  });

  const data = await resp.text();
  return new NextResponse(data, {
    status: resp.status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "no-store",
    },
  });
}
