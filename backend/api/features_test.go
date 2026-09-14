package api

import "testing"

func TestParseChatFeatures(t *testing.T) {
	cases := []struct {
		in, wantSlug string
		wantWeb      bool
		wantReason   string
	}{
		{"gpt-5", "gpt-5", false, ""},
		{"gpt-5-web", "gpt-5", true, ""},
		{"gpt-5-thinking", "gpt-5-thinking", false, "high"},
		{"gpt-5-web-thinking", "gpt-5-thinking", true, "high"},
		{"gpt-4o-mini-web", "gpt-4o-mini", true, ""},
	}
	for _, c := range cases {
		slug, f := parseChatFeatures(c.in)
		if slug != c.wantSlug {
			t.Errorf("slug %s: want %s got %s", c.in, c.wantSlug, slug)
		}
		if f.WebSearch != c.wantWeb {
			t.Errorf("%s: web want %v got %v", c.in, c.wantWeb, f.WebSearch)
		}
		if f.Reasoning != c.wantReason {
			t.Errorf("%s: reasoning want %q got %q", c.in, c.wantReason, f.Reasoning)
		}
	}
	if !aliasValid("gpt-5-web") || !aliasValid("gpt-5-thinking") || aliasValid("gpt-9-bogus") {
		t.Fatal("aliasValid suffix salah")
	}
}
