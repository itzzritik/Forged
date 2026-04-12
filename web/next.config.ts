import type { NextConfig } from "next";

const isDevelopment = process.env.NODE_ENV !== "production";

const contentSecurityPolicy = [
	"default-src 'self'",
	["script-src", "'self'", "'unsafe-inline'", "'wasm-unsafe-eval'", ...(isDevelopment ? ["'unsafe-eval'"] : [])].join(" "),
	"connect-src 'self' https://forged-api.ritik.me",
	"style-src 'self' 'unsafe-inline'",
	"img-src 'self' data:",
	"frame-src 'none'",
	"object-src 'none'",
].join("; ");

const nextConfig: NextConfig = {
	async headers() {
		return [
			{
				source: "/(.*)",
				headers: [
					{
						key: "Content-Security-Policy",
						value: contentSecurityPolicy,
					},
				],
			},
		];
	},
};

export default nextConfig;
