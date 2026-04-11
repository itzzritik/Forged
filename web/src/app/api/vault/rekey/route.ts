import type { NextRequest } from "next/server";
import { proxyToAPI } from "@/lib/api-proxy";

export async function POST(request: NextRequest) {
	const body = await request.text();
	return proxyToAPI("POST", "/api/v1/vault/rekey", body);
}
