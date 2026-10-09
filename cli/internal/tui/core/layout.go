package core

func pad(st *State) int {
	if st.Height >= 24 {
		return 1
	}
	return 0
}

func HeaderY(st *State) int { return pad(st) }

func BodySize(st *State) (w, h int) {
	return max(0, st.Width-4), max(0, st.Height-3-3*pad(st))
}

func BodyOrigin(st *State) (x, y int) { return 2, 2 + 2*pad(st) }
