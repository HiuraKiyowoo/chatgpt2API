package api

import "testing"

func TestParseSubQuestions(t *testing.T) {
	plan := "Oke, ini rencananya:\n1. Berapa harga BBCA penutupan terakhir?\n2. Bagaimana kinerja fundamental emiten perbankan besar 2026?\n3. Apa sentimen pasar minggu ini terhadap sektor finansial?\n"
	subs := parseSubQuestions(plan, "pertanyaan asli")
	if len(subs) != 3 {
		t.Fatalf("want 3 got %d: %v", len(subs), subs)
	}
	if subs[0] != "Berapa harga BBCA penutupan terakhir?" {
		t.Errorf("sub1 salah: %q", subs[0])
	}
	if bad := parseSubQuestions("maaf gak bisa", "q asli"); len(bad) != 1 || bad[0] != "q asli" {
		t.Errorf("fallback salah: %v", bad)
	}
	many := "1. kenapa a\n2. kenapa b\n3. kenapa c\n4. kenapa d\n5. kenapa e"
	if got := parseSubQuestions(many, "x"); len(got) != researchMaxSubq {
		t.Errorf("cap salah: %v", got)
	}
}

func TestChatFeaturesResearch(t *testing.T) {
	slug, f := parseChatFeatures("gpt-5-research")
	if !f.Research || !f.WebSearch {
		t.Fatalf("research harus aktifkan web juga: %+v", f)
	}
	if slug != "gpt-5" {
		t.Errorf("slug want gpt-5 got %s", slug)
	}
	if !aliasValid("gpt-5-research") {
		t.Error("gpt-5-research harus valid")
	}
	slug2, f2 := parseChatFeatures("gpt-5-web-research")
	if !f2.Research || !f2.WebSearch || slug2 != "gpt-5" {
		t.Errorf("kombinasi web+research salah: %s %+v", slug2, f2)
	}
}
