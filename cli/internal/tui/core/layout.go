package core

func BodySize(st *State) (w, h int) {
	h = st.Height - 3
	if st.Height >= 24 {
		h = st.Height - 5
	}
	return max(0, st.Width-4), max(0, h)
}

func BodyOrigin(st *State) (x, y int) {
	if st.Height >= 24 {
		return 2, 3
	}
	return 2, 2
}
