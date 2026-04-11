import type { NextRequest } from "next/server";
import { proxyToAPI } from "@/lib/api-proxy";

export const POST = (_request: NextRequest) => proxyToAPI("POST", "/api/v1/account/delete");
