import { proxyToAPI } from "@/lib/api-proxy";

export const GET = () => proxyToAPI("GET", "/api/v1/devices");
