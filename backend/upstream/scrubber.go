package upstream

// entityScrubber membuang markup entitas ChatGPT dari stream delta.
// Format: <U+E200>...<U+E201> (payload <U+E202>JSON</> di dalamnya), bisa
// nyebrang antar-delta -> state machine per-karakter dengan buffer tail.
type entityScrubber struct {
	in  bool   // sedang di dalam markup
	buf string // tail yang mungkin awal markup (ditahan)
}

const (
	entStart = "\ue200"
	entMid   = "\ue202"
	entEnd   = "\ue201"
)

// feed memproses satu delta, mengembalikan teks bersih (markup dibuang).
func (s *entityScrubber) feed(delta string) string {
	out := ""
	if s.in {
		if idx := indexOf(delta, entEnd); idx >= 0 {
			delta = delta[idx+len(entEnd):]
			s.in = false
		} else {
			return ""
		}
	}
	combined := s.buf + delta
	s.buf = ""
	for {
		idx := indexOf(combined, entStart)
		if idx < 0 {
			break
		}
		if end := indexOf(combined[idx:], entEnd); end >= 0 {
			combined = combined[:idx] + combined[idx+end+len(entEnd):]
			continue
		}
		// start tanpa end -> masuk markup, tahan sisanya
		out += combined[:idx]
		combined = ""
		s.in = true
		break
	}
	// tail yang mungkin awalan entStart sebagian (mis. byte UTF-8 terpotong):
	// entStart = 3 byte EF B8 80. Kalau combined berakhir dengan prefiksnya,
	// tahan.
	if n := len(combined); n > 0 {
		for k := 1; k < len(entStart) && k <= n; k++ {
			if combined[n-k:] == entStart[:k] {
				s.buf = combined[n-k:]
				combined = combined[:n-k]
				break
			}
		}
	}
	return out + combined
}

func indexOf(hay, ndl string) int {
	for i := 0; i+len(ndl) <= len(hay); i++ {
		if hay[i:i+len(ndl)] == ndl {
			return i
		}
	}
	return -1
}
