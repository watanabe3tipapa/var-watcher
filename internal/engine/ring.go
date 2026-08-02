package engine

// Ring は上限付きログリングバッファ。TUI の再描画やログ保持に使う。
type Ring struct {
	lines []string
	max   int
}

func NewRing(max int) *Ring {
	if max <= 0 {
		max = 2000
	}
	return &Ring{max: max}
}

func (r *Ring) Add(s string) {
	r.lines = append(r.lines, s)
	if len(r.lines) > r.max {
		r.lines = r.lines[len(r.lines)-r.max:]
	}
}

func (r *Ring) All() []string {
	return append([]string(nil), r.lines...)
}
