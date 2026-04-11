"use client";

import { useEffect, useMemo, useState } from "react";

export type DocsTOCGroup = {
	title: string;
	items: Array<{
		href: string;
		label: string;
	}>;
};

function getSectionId(href: string) {
	return decodeURIComponent(href.replace(/^#/, ""));
}

export function DocsTOC({ groups }: { groups: DocsTOCGroup[] }) {
	const sectionIds = useMemo(() => groups.flatMap((group) => group.items.map((item) => getSectionId(item.href))), [groups]);
	const [activeId, setActiveId] = useState(sectionIds[0] ?? "");

	useEffect(() => {
		if (!sectionIds.length) return;

		const sections = sectionIds.map((id) => document.getElementById(id)).filter((section): section is HTMLElement => section !== null);

		if (!sections.length) return;

		let frame = 0;
		const headerOffset = 160;

		const updateActive = () => {
			frame = 0;

			let nextActive = sections[0].id;

			for (const section of sections) {
				if (section.getBoundingClientRect().top <= headerOffset) {
					nextActive = section.id;
					continue;
				}

				break;
			}

			setActiveId((current) => (current === nextActive ? current : nextActive));
		};

		const requestUpdate = () => {
			if (frame) return;
			frame = window.requestAnimationFrame(updateActive);
		};

		requestUpdate();
		window.addEventListener("scroll", requestUpdate, { passive: true });
		window.addEventListener("resize", requestUpdate);
		window.addEventListener("hashchange", requestUpdate);

		return () => {
			if (frame) {
				window.cancelAnimationFrame(frame);
			}
			window.removeEventListener("scroll", requestUpdate);
			window.removeEventListener("resize", requestUpdate);
			window.removeEventListener("hashchange", requestUpdate);
		};
	}, [sectionIds]);

	return (
		<aside className="sticky top-32 hidden h-[calc(100vh-140px)] w-72 shrink-0 self-start overflow-y-auto pt-2 pr-12 lg:block">
			<div className="flex flex-col gap-12">
				{groups.map((group) => (
					<div key={group.title}>
						<div className="mb-6 border-[#27272a] border-b pb-2 font-bold font-mono text-[#ea580c] text-[10px] uppercase tracking-[0.2em]">{group.title}</div>
						<nav className="flex flex-col space-y-1">
							{group.items.map((item) => {
								const sectionId = getSectionId(item.href);
								const isActive = activeId === sectionId;

								return (
									<a
										aria-current={isActive ? "location" : undefined}
										className={`group -mx-3 flex items-center justify-between px-3 py-2 font-mono text-xs tracking-widest transition-colors ${
											isActive ? "bg-[#09090b] text-white" : "text-[#a1a1aa] hover:text-white"
										}`}
										href={item.href}
										key={item.href}
										onClick={() => setActiveId(sectionId)}
									>
										<span className="flex items-center gap-3 uppercase">
											<span className={`transition-colors ${isActive ? "text-[#ea580c]" : "text-[#3f3f46] group-hover:text-[#ea580c]"}`}>
												{">"}
											</span>
											{item.label}
										</span>
										<span className={`text-[#ea580c] transition-opacity ${isActive ? "opacity-100" : "opacity-0"}`}>●</span>
									</a>
								);
							})}
						</nav>
					</div>
				))}
			</div>
		</aside>
	);
}
