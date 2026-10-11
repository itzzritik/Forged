"use client";

import { useCallback, useEffect, useState } from "react";

interface Resource<T> {
	data: T | null;
	error: string | null;
}

const cache = new Map<string, unknown>();
const inflight = new Map<string, Promise<unknown>>();

function request<T>(url: string): Promise<T> {
	const running = inflight.get(url);
	if (running) return running as Promise<T>;
	const next = fetch(url)
		.then(async (response) => {
			if (response.status === 401) window.location.assign("/login");
			if (!response.ok) throw new Error(`The server answered ${response.status}. Try again in a moment.`);
			const data = (await response.json()) as T;
			cache.set(url, data);
			return data;
		})
		.finally(() => inflight.delete(url));
	inflight.set(url, next);
	return next;
}

export function prefetch(...urls: string[]) {
	for (const url of urls) request(url).catch(() => undefined);
}

// Cached data shows at once and refreshes in the background, so revisiting a page never flashes a skeleton.
export function useResource<T>(url: string) {
	const [state, setState] = useState<Resource<T>>(() => ({ data: (cache.get(url) as T | undefined) ?? null, error: null }));

	const load = useCallback(() => {
		let live = true;
		setState((current) => ({ ...current, error: null }));
		request<T>(url)
			.then((data) => {
				if (live) setState({ data, error: null });
			})
			.catch((error: unknown) => {
				if (!live) return;
				const message = error instanceof Error ? error.message : "Couldn't reach the server.";
				setState((current) => ({ data: current.data, error: current.data ? null : message }));
			});
		return () => {
			live = false;
		};
	}, [url]);

	useEffect(() => load(), [load]);

	return { ...state, loading: state.data === null && state.error === null, retry: load };
}
