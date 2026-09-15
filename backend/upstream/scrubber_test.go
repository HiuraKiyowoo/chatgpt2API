package upstream

import "testing"

func TestEntityScrubber(t *testing.T) {
	E0, E1, E2 := "\ue200", "\ue201", "\ue202"
	cases := []struct {
		name   string
		deltas []string
		want   string
	}{
		{"polos", []string{"halo ", "dunia"}, "halo dunia"},
		{"sitasi utuh 1 delta", []string{"harga 6300" + E0 + "cite" + E2 + "turn0news0" + E1 + " hari ini"}, "harga 6300 hari ini"},
		{"marker 0search gaya E202-only", []string{"6300", E0 + "search" + E2 + "1"}, "6300"},
		{"lintas delta", []string{"ABC" + E0 + "cit", "e" + E2 + "x" + E1 + "DEF"}, "ABCDEF"},
		{"E202 pembuka tanpa E200", []string{"text " + E2 + "payload" + E1 + "more"}, "text more"},
		{"E201 nyasar dibuang", []string{"aa" + E1 + "bb"}, "aabb"},
		{"markup belum nutup sampai akhir", []string{"ok ", E0 + "tail"}, "ok "},
	}
	for _, c := range cases {
		s := &entityScrubber{}
		got := ""
		for _, d := range c.deltas {
			got += s.feed(d)
		}
		if got != c.want {
			t.Errorf("%s: want %q got %q", c.name, c.want, got)
		}
	}
}
