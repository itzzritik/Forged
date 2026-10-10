export async function GET() {
	const res = await fetch("https://registry.npmjs.org/@getforged/cli/latest", { next: { revalidate: 3600 } }).catch(() => null);
	const data = res?.ok ? await res.json() : null;
	return Response.json({ version: data?.version ?? null });
}
