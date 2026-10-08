package flame

import (
	"math"
	"math/rand/v2"
)

const levels = 36

type Fire struct {
	w, rows, W, H int
	p, out        []uint8
	src           []float64
	kd, heat      float64
	sparks        []spark
}

type spark struct{ x, y, vy, ph, life, dec float64 }

type Spark struct {
	X, Row int
	Pos    float64
	Life   float64
}

func New(w, rows int) *Fire {
	if w <= 0 || rows <= 0 {
		return nil
	}
	f := &Fire{w: w, rows: rows, W: w * 2, H: rows * 4, heat: 1}
	f.p = make([]uint8, f.W*f.H)
	f.out = make([]uint8, w*rows*2)
	f.src = make([]float64, f.W)
	for i := range f.src {
		f.src[i] = levels
	}
	f.kd = 1 + 2*levels/(0.8*float64(f.H))
	for i := 0; i < 60; i++ {
		f.Step()
	}
	return f
}

func (f *Fire) Size() (int, int) {
	if f == nil {
		return 0, 0
	}
	return f.w, f.rows
}

func (f *Fire) SetHeat(h float64) {
	if f != nil {
		f.heat = max(h, 0.05)
	}
}

func (f *Fire) Step() {
	if f == nil {
		return
	}
	W, H := f.W, f.H
	lo := 20.0 * f.heat
	for x := range f.src {
		f.src[x] = min(levels, max(lo, f.src[x]+(rand.Float64()-0.5)*8))
	}
	for x := 0; x < W; x++ {
		l, r := max(0, x-1), min(W-1, x+1)
		f.p[(H-1)*W+x] = uint8((f.src[l] + 2*f.src[x] + f.src[r]) / 4)
	}
	kd := f.kd / f.heat
	for y := 1; y < H; y++ {
		for x := 0; x < W; x++ {
			v := f.p[y*W+x]
			nx := min(W-1, max(0, x-rand.IntN(3)+1))
			d := uint8(min(255, rand.Float64()*kd))
			if v > d {
				f.p[(y-1)*W+nx] = v - d
			} else {
				f.p[(y-1)*W+nx] = 0
			}
		}
	}
	for y := 0; y < f.rows*2; y++ {
		for x := 0; x < f.w; x++ {
			i := 2*y*W + 2*x
			f.out[y*f.w+x] = uint8((int(f.p[i]) + int(f.p[i+1]) + int(f.p[i+W]) + int(f.p[i+W+1]) + 2) >> 2)
		}
	}
	f.stepSparks()
}

func (f *Fire) stepSparks() {
	const fps = 20.0
	if rand.Float64() < 0.55*float64(f.w)/100 {
		f.sparks = append(f.sparks, spark{x: rand.Float64() * float64(f.w), y: rand.Float64() * 1.5,
			vy: (3 + rand.Float64()*4) / fps, ph: rand.Float64() * 6.3, life: 1, dec: 1 / (fps * (1.4 + rand.Float64()*1.8))})
	}
	kept := f.sparks[:0]
	for _, s := range f.sparks {
		s.y += s.vy
		s.ph += 0.12
		s.x += math.Sin(s.ph) * 0.12
		s.life -= s.dec
		if s.life > 0 {
			kept = append(kept, s)
		}
	}
	f.sparks = kept
}

// Sparks returns spark cells in screen rows above top; y counts upward from the fire's top edge.
func (f *Fire) Sparks(top int) []Spark {
	if f == nil {
		return nil
	}
	out := make([]Spark, 0, len(f.sparks))
	for _, s := range f.sparks {
		ry := float64(top) - s.y
		row := int(ry)
		if row < 1 || row >= top {
			continue
		}
		out = append(out, Spark{X: int(s.x + 0.5), Row: row, Pos: ry - float64(row), Life: s.life})
	}
	return out
}
