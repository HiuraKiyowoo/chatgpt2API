package upstream

// entityScrubber membuang markup entitas ChatGPT dari stream delta.
// Markup: pembuka U+E200 atau U+E202, penutup U+E201 (mis. sitasi web
// "...cite...turn0news0" & "0search1"). Marker bisa nyebrang antar-delta
// -> state machine. E201 nyasar di luar markup ikut dibuang.
type entityScrubber struct {
	in  bool   // sedang di dalam markup
	buf string // tail parsial (byte UTF-8 pembuka terpotong)
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
		if idx := idxStr(delta, entEnd); idx >= 0 {
			delta = delta[idx+len(entEnd):]
			s.in = false
		} else {
			return ""
		}
	}
	combined := s.buf + delta
	s.buf = ""
	for {
		// cari pembuka markup mana pun (E200 atau E202)
		i0, i2 := idxStr(combined, entStart), idxStr(combined, entMid)
		pos := -1
		switch {
		case i0 >= 0 && (i2 < 0 || i0 <= i2):
			pos = i0
		case i2 >= 0:
			pos = i2
		}
		if pos < 0 {
			break
		}
		if end := idxStr(combined[pos:], entEnd); end >= 0 {
			// markup utuh -> buang pos..end, lanjut
			combined = combined[:pos] + combined[pos+end+len(entEnd):]
			continue
		}
		// pembuka tanpa penutup -> mode skip sampai delta berikutnya
		out += combined[:pos]
		combined = ""
		s.in = true
		break
	}
	// E201 nyasar di luar markup: buang
	for {
		i := idxStr(combined, entEnd)
		if i < 0 {
			break
		}
		combined = combined[:i] + combined[i+len(entEnd):]
	}
	// tahan tail yang mungkin byte UTF-8 pembuka terpotong (E200/E202/E201:
	// EF B8 80 / 81 / 82 — prefiks 1-2 byte)
	if n := len(combined); n > 0 {
		for k := 1; k < 3 && k <= n; k++ {
			tail := combined[n-k:]
			if tail == "\xef" || tail == "\xb8" || tail == "\xef"+"\xb8" {
				s.buf = tail
				combined = combined[:n-k]
				break
			}
		}
	}
	return out + combined
}

func idxStr(hay, ndl string) int {
	for i := 0; i+len(ndl) <= len(hay); i++ {
		if hay[i:i+len(ndl)] == ndl {
			return i
		}
	}
	return -1
}
