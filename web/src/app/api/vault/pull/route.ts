import { proxyToAPI } from "@/lib/api-proxy";

export async function GET() {
	return proxyToAPI("GET", "/api/v1/sync/pull");
}
