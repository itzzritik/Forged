import { reducedMotion } from "../motion";

const LEVELS = 36;
const CELL = 4;
const FRAME_MS = 33;
const RAMPS = {
	dark: ["#000000", "#2B110A", "#4F1407", "#7E1C08", "#B32A0A", "#E2420E", "#FF6614", "#FF8A1F", "#FFAE33", "#FFCE6E", "#FFE9B3", "#FFF8E8"],
	light: ["#FFFFFF", "#FCE9D8", "#FAD0AE", "#F7B07F", "#F28C4F", "#EA6A2C", "#DB4F18", "#C63D10", "#AE320C"],
};
const EMBER_COLORS = { dark: ["#FFC23D", "#FF6614"], light: ["#C2410C", "#B45309"] };

export const HEAT = { busy: 1.1, choke: 0.3, flare: 2.4, idle: 0.85, surge: 2 };

interface Ember {
	amp: number;
	dec: number;
	life: number;
	phase: number;
	vy: number;
	x: number;
	y: number;
}

const toRgb = (hex: string) => [1, 3, 5].map((i) => Number.parseInt(hex.slice(i, i + 2), 16));

function buildRamp(stops: string[]) {
	const colors = stops.map(toRgb);
	return Array.from({ length: LEVELS + 1 }, (_, level) => {
		const at = (level / LEVELS) * (colors.length - 1);
		const low = Math.min(colors.length - 2, Math.floor(at));
		const t = at - low;
		return colors[low].map((c, i) => Math.round(c + (colors[low + 1][i] - c) * t));
	});
}

function sprite(color: string) {
	const canvas = document.createElement("canvas");
	canvas.width = 32;
	canvas.height = 32;
	const ctx = canvas.getContext("2d") as CanvasRenderingContext2D;
	const gradient = ctx.createRadialGradient(16, 16, 0, 16, 16, 16);
	gradient.addColorStop(0, color);
	gradient.addColorStop(0.22, color);
	gradient.addColorStop(1, `${color}00`);
	ctx.fillStyle = gradient;
	ctx.fillRect(0, 0, 32, 32);
	return canvas;
}

// The flame canvas is the simulation grid itself; CSS scales and blurs it so the web never shows pixels.
export class Fire {
	private readonly flames: HTMLCanvasElement;
	private readonly embersCanvas: HTMLCanvasElement;
	private readonly host: HTMLElement;
	private buf = new Float32Array(0);
	private src = new Float32Array(0);
	private image: ImageData | null = null;
	private ramp: number[][] = [];
	private sprites: HTMLCanvasElement[] = [];
	private embers: Ember[] = [];
	private width = 0;
	private height = 0;
	private decay = 0;
	private heat = 0.1;
	private target = HEAT.idle;
	private boost = 0;
	private dark = true;
	private frame = 0;
	private pending = 0;
	private last = 0;
	private shownHeat = "";
	private timer: ReturnType<typeof setTimeout> | undefined;

	constructor(flames: HTMLCanvasElement, embers: HTMLCanvasElement, host: HTMLElement) {
		this.flames = flames;
		this.embersCanvas = embers;
		this.host = host;
		this.theme(true);
		this.resize();
		window.addEventListener("resize", this.onResize);
	}

	start() {
		if (reducedMotion()) {
			this.heat = HEAT.idle;
			for (let i = 0; i < 80; i++) this.step();
			this.paint();
			return;
		}
		this.hold(HEAT.surge, 700);
		this.frame = requestAnimationFrame(this.loop);
	}

	destroy() {
		cancelAnimationFrame(this.frame);
		cancelAnimationFrame(this.pending);
		clearTimeout(this.timer);
		window.removeEventListener("resize", this.onResize);
	}

	theme(dark: boolean) {
		this.dark = dark;
		const scheme = dark ? "dark" : "light";
		this.ramp = buildRamp(RAMPS[scheme]);
		this.sprites = EMBER_COLORS[scheme].map(sprite);
		if (reducedMotion()) this.paint();
	}

	setTarget(heat: number) {
		clearTimeout(this.timer);
		this.target = heat;
	}

	kick() {
		this.boost = Math.min(0.5, this.boost + 0.14);
	}

	choke() {
		this.hold(HEAT.choke, 700);
	}

	flare() {
		this.setTarget(HEAT.flare);
		this.boost = 0.6;
	}

	private hold(heat: number, ms: number) {
		clearTimeout(this.timer);
		this.target = heat;
		this.timer = setTimeout(() => {
			this.target = HEAT.idle;
		}, ms);
	}

	private readonly onResize = () => {
		cancelAnimationFrame(this.pending);
		this.pending = requestAnimationFrame(this.resize);
	};

	private readonly resize = () => {
		const width = Math.max(1, Math.ceil(this.flames.clientWidth / CELL));
		const height = Math.max(1, Math.ceil(this.flames.clientHeight / CELL));
		if (width !== this.width || height !== this.height) {
			this.width = width;
			this.height = height;
			this.flames.width = width;
			this.flames.height = height;
			this.image = (this.flames.getContext("2d") as CanvasRenderingContext2D).createImageData(width, height);
			this.buf = new Float32Array(width * height);
			this.src = new Float32Array(width).fill(LEVELS);
			this.decay = (2 * LEVELS) / (0.72 * height);
			for (let i = 0; i < 80; i++) this.step();
		}
		const dpr = window.devicePixelRatio || 1;
		this.embersCanvas.width = Math.round(this.embersCanvas.clientWidth * dpr);
		this.embersCanvas.height = Math.round(this.embersCanvas.clientHeight * dpr);
		(this.embersCanvas.getContext("2d") as CanvasRenderingContext2D).setTransform(dpr, 0, 0, dpr, 0, 0);
		if (reducedMotion()) this.paint();
	};

	private readonly loop = (now: number) => {
		this.frame = requestAnimationFrame(this.loop);
		if (now - this.last < FRAME_MS) return;
		this.last = now;
		this.boost *= 0.9;
		this.heat += (this.target + this.boost - this.heat) * 0.1;
		this.step();
		this.paint();
		this.drift();
		const heat = Math.min(1, Math.max(0, (this.heat - 0.45) / 1.4)).toFixed(3);
		if (heat === this.shownHeat) return;
		this.shownHeat = heat;
		this.host.style.setProperty("--heat", heat);
	};

	private step() {
		const { buf, height: h, src, width: w } = this;
		const floor = 20 * this.heat;
		for (let x = 0; x < w; x++) src[x] = Math.min(LEVELS, Math.max(floor, src[x] + (Math.random() - 0.5) * 8));
		for (let x = 0; x < w; x++) buf[(h - 1) * w + x] = (src[Math.max(0, x - 1)] + 2 * src[x] + src[Math.min(w - 1, x + 1)]) / 4;
		const decay = this.decay / this.heat;
		for (let y = 1; y < h; y++) {
			for (let x = 0; x < w; x++) {
				const value = buf[y * w + x];
				const nx = Math.min(w - 1, Math.max(0, x - Math.floor(Math.random() * 3) + 1));
				const loss = Math.random() * decay;
				buf[(y - 1) * w + nx] = value > loss ? value - loss : 0;
			}
		}
	}

	private paint() {
		if (!this.image) return;
		const data = this.image.data;
		for (let i = 0; i < this.buf.length; i++) {
			const level = Math.min(LEVELS, Math.floor(this.buf[i]));
			const [r, g, b] = this.ramp[level];
			data[i * 4] = r;
			data[i * 4 + 1] = g;
			data[i * 4 + 2] = b;
			data[i * 4 + 3] = level ? 255 : 0;
		}
		(this.flames.getContext("2d") as CanvasRenderingContext2D).putImageData(this.image, 0, 0);
	}

	private drift() {
		const ctx = this.embersCanvas.getContext("2d") as CanvasRenderingContext2D;
		const w = this.embersCanvas.clientWidth;
		const h = this.embersCanvas.clientHeight;
		const fireHeight = this.flames.clientHeight - 30;
		ctx.clearRect(0, 0, w, h);
		if (Math.random() < (w / 1600) * this.heat) {
			this.embers.push({
				amp: 0.3 + Math.random() * 0.6,
				dec: 1 / (30 * (1.6 + Math.random() * 2)),
				life: 1,
				phase: Math.random() * 6.3,
				vy: 1.2 + Math.random() * 2.2,
				x: Math.random() * w,
				y: h - fireHeight * (0.25 + Math.random() * 0.4),
			});
		}
		ctx.globalCompositeOperation = this.dark ? "lighter" : "source-over";
		this.embers = this.embers.filter((ember) => {
			ember.y -= ember.vy;
			ember.phase += 0.09;
			ember.x += Math.sin(ember.phase) * ember.amp;
			ember.life -= ember.dec;
			const size = 2 + ember.life * 5;
			ctx.globalAlpha = Math.max(0, ember.life);
			ctx.drawImage(this.sprites[ember.life > 0.55 ? 0 : 1], ember.x - size / 2, ember.y - size / 2, size, size);
			return ember.life > 0;
		});
		ctx.globalAlpha = 1;
	}
}
