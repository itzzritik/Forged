import type { NextRequest } from "next/server";
import { proxyToAPI } from "@/lib/api-proxy";

export function GET(request: NextRequest) {
	const deviceId = request.headers.get("x-device-id");
	return proxyToAPI("GET", "/api/v1/sync/pull", undefined, deviceId ? { "X-Device-ID": deviceId } : {});
}
