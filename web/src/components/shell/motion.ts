export const EASE = "cubic-bezier(0.2, 0.8, 0.2, 1)";

const BURN_MS = 680;
const BURN_EASE = "cubic-bezier(0.5, 0, 0.65, 1)";
const BURN_CAP = 12;
const SPARK_COLORS = { dark: ["255,230,160", "255,170,50", "240,90,30"], light: ["194,65,12", "180,83,9", "191,38,24"] };

interface Spark {
	dec: number;
	life: number;
	vx: number;
	vy: number;
	x: number;
	y: number;
}

let layer: HTMLCanvasElement | null = null;
let live: Spark[] = [];
let frame = 0;

export function reducedMotion() {
	return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

export function replay(el: Element | null | undefined, className: string) {
	if (!el) return;
	el.classList.remove(className);
	void (el as HTMLElement).offsetWidth;
	el.classList.add(className);
}

function surface() {
	if (!layer) {
		layer = document.createElement("canvas");
		layer.className = "spark-layer";
		layer.setAttribute("aria-hidden", "true");
		document.body.append(layer);
	}
	const dpr = window.devicePixelRatio || 1;
	const ctx = layer.getContext("2d") as CanvasRenderingContext2D;
	if (layer.width !== Math.round(innerWidth * dpr) || layer.height !== Math.round(innerHeight * dpr)) {
		layer.width = Math.round(innerWidth * dpr);
		layer.height = Math.round(innerHeight * dpr);
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
	}
	return ctx;
}

function tick() {
	const ctx = surface();
	const dark = document.documentElement.classList.contains("dark");
	const [hot, warm, cool] = SPARK_COLORS[dark ? "dark" : "light"];
	ctx.clearRect(0, 0, innerWidth, innerHeight);
	ctx.globalCompositeOperation = dark ? "lighter" : "source-over";
	ctx.lineCap = "round";
	ctx.lineWidth = 1.3;
	live = live.filter((s) => {
		s.vy += 0.16;
		s.vx *= 0.985;
		s.x += s.vx;
		s.y += s.vy;
		s.life -= s.dec;
		const tone = s.life > 0.6 ? hot : s.life > 0.3 ? warm : cool;
		ctx.strokeStyle = `rgba(${tone},${Math.max(0, s.life)})`;
		ctx.beginPath();
		ctx.moveTo(s.x - s.vx * 1.6, s.y - s.vy * 1.6);
		ctx.lineTo(s.x, s.y);
		ctx.stroke();
		return s.life > 0;
	});
	frame = live.length ? requestAnimationFrame(tick) : 0;
}

export function sparks(from: Element, count: number, { fall = false, spread = 1 } = {}) {
	if (reducedMotion()) return;
	const box = from.getBoundingClientRect();
	for (let i = 0; i < count; i++) {
		live.push({
			dec: 1 / (16 + Math.random() * 22),
			life: 1,
			vx: (Math.random() - 0.5) * 3 * spread,
			vy: fall ? -1 - Math.random() * 2.4 : -(1.5 + Math.random() * 3),
			x: box.left + Math.random() * box.width,
			y: fall ? box.bottom - box.height * 0.18 : box.top + box.height / 2,
		});
	}
	if (!frame) frame = requestAnimationFrame(tick);
}

export function shake(el: Element | null | undefined) {
	if (!el || reducedMotion()) return;
	el.animate(
		[
			{ transform: "none" },
			{ transform: "translateX(-9px)" },
			{ transform: "translateX(7px)" },
			{ transform: "translateX(-5px)" },
			{ transform: "translateX(3px)" },
			{ transform: "none" },
		],
		{ duration: 380, easing: "ease-out" }
	);
}

export function burn(rows: HTMLElement[]) {
	if (reducedMotion() || rows.length === 0) return { cancel: () => undefined, finished: Promise.resolve() };
	const timers: ReturnType<typeof setTimeout>[] = [];
	const runs = rows.slice(0, BURN_CAP).map((row, index) => {
		const delay = index * 60;
		const width = row.offsetWidth;
		const edge = document.createElement("i");
		edge.className = "burn-edge";
		edge.setAttribute("aria-hidden", "true");
		edge.style.height = `${row.offsetHeight}px`;
		edge.style.transform = row.style.transform;
		row.after(edge);
		const clip = row.animate([{ clipPath: "inset(0 0 0 0)" }, { clipPath: "inset(0 0 0 100%)" }], { delay, duration: BURN_MS, easing: BURN_EASE, fill: "forwards" });
		const move = edge.animate(
			[
				{ opacity: 1, translate: "0 0" },
				{ offset: 0.92, opacity: 1, translate: `${width}px 0` },
				{ opacity: 0, translate: `${width}px 0` },
			],
			{ delay, duration: BURN_MS, easing: BURN_EASE, fill: "both" }
		);
		timers.push(
			setTimeout(() => {
				const spray = setInterval(() => sparks(edge, 2, { spread: 0.6 }), 60);
				timers.push(setTimeout(() => clearInterval(spray), BURN_MS));
				timers.push(spray);
			}, delay)
		);
		return { clip, edge, move };
	});
	const clear = () => {
		for (const timer of timers) clearTimeout(timer);
		for (const run of runs) run.edge.remove();
	};
	return {
		cancel: () => {
			clear();
			for (const run of runs) {
				run.clip.cancel();
				run.move.cancel();
			}
		},
		finished: Promise.all(runs.map((run) => run.move.finished)).then(clear, clear),
	};
}
