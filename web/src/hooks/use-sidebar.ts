"use client";

import { useCallback, useEffect, useState } from "react";

export const useSidebar = () => {
	const [collapsed, setCollapsed] = useState(false);

	useEffect(() => {
		const saved = localStorage.getItem("sidebar-collapsed");
		if (saved === "true") setCollapsed(true);
	}, []);

	const toggle = useCallback(() => {
		setCollapsed((prev) => {
			const next = !prev;
			localStorage.setItem("sidebar-collapsed", String(next));
			return next;
		});
	}, []);

	return { collapsed, toggle };
};
