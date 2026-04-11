import { proxyToAPI } from "@/lib/api-proxy";

export function GET() {
	return proxyToAPI("GET", "/api/v1/sync/status");
}
