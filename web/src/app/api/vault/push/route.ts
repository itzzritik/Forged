import type { NextRequest } from "next/server";
import { proxyToAPI } from "@/lib/api-proxy";

export async function POST(request: NextRequest) {
	const body = await request.text();
	const deviceId = request.headers.get("x-device-id");
	return proxyToAPI("POST", "/api/v1/sync/push", body, deviceId ? { "X-Device-ID": deviceId } : {});
}
