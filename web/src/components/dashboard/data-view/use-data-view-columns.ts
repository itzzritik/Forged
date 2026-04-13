"use client";

import { useCallback, useEffect, useState } from "react";

const STORAGE_KEY = "forged.dataview.columns";

type DataViewStore = Record<string, Record<string, boolean>>;

export function useDataViewColumns(pageName: string, initialValue: Record<string, boolean>) {
	const [store, setStore] = useState<DataViewStore>({});
	const [isReady, setIsReady] = useState(false);

	useEffect(() => {
		try {
			const raw = window.localStorage.getItem(STORAGE_KEY);
			setStore(raw ? (JSON.parse(raw) as DataViewStore) : {});
		} catch {
			setStore({});
		}
		setIsReady(true);
	}, []);

	const cols = store[pageName] ?? initialValue;

	const setCols = useCallback(
		(next: Record<string, boolean>) => {
			setStore((prev) => {
				const updated = { ...prev, [pageName]: next };
				window.localStorage.setItem(STORAGE_KEY, JSON.stringify(updated));
				return updated;
			});
		},
		[pageName]
	);

	const reset = useCallback(() => {
		setStore((prev) => {
			const { [pageName]: _removed, ...rest } = prev;
			window.localStorage.setItem(STORAGE_KEY, JSON.stringify(rest));
			return rest;
		});
	}, [pageName]);

	return { cols, setCols, reset, isReady };
}
