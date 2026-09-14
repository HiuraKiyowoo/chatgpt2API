package upstream

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestConsumeSSEPatchRealDump pakai SSE android BETULAN yang direkam dari
// android.chat.openai.com (live, akun free) — bukan fixture karangan.
func TestConsumeSSEPatchRealDump(t *testing.T) {
	path := os.Getenv("CGT2_SSE_DUMP")
	if path == "" {
		t.Skip("set CGT2_SSE_DUMP=/path/to/android_sse_dump.txt (dihasilkan probe_android_dump.py)")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	ch := make(chan Part, 64)
	go consumeSSEPatch(bufio.NewReader(f), ch)

	var text strings.Builder
	var last Part
	for p := range ch {
		if p.Err != nil {
			t.Fatalf("parser error: %v", p.Err)
		}
		if p.Done {
			last = p
			break
		}
		text.WriteString(p.Text)
	}
	got := text.String()
	t.Logf("delta tergabung: %q conv=%s mess=%s model=%s finish=%s usage=%d/%d",
		got, last.ConvID, last.MessID, last.Model, last.Finish, last.UsageIn, last.UsageOut)
	if !strings.Contains(got, "PONG") {
		t.Fatalf("delta harusnya memuat PONG, dapat %q", got)
	}
	if !last.Done {
		t.Fatal("harus diakhiri Part{Done:true}")
	}
}

// TestConsumeSSEPatchSynthetic: bentuk patch event yang mungkin muncul,
// termasuk varian implisit (v string tanpa o/p) dan batch.
func TestConsumeSSEPatchSynthetic(t *testing.T) {
	lines := []interface{}{
		"v1",
		map[string]interface{}{"type": "resume_conversation_token", "token": "x"},
		map[string]interface{}{"p": "", "o": "add", "v": map[string]interface{}{
			"message": map[string]interface{}{"id": "u1", "author": map[string]string{"role": "user"},
				"content": map[string]interface{}{"parts": []string{"hi"}}}},
		},
		map[string]interface{}{"p": "/message/content/parts/0", "o": "append", "v": "HA"},
		map[string]interface{}{"p": "/message/content/parts/0", "o": "append", "v": "LO"},
		map[string]interface{}{"p": "", "o": "patch", "v": []interface{}{
			map[string]interface{}{"p": "/message/content/parts/0", "o": "append", "v": "!"},
			map[string]interface{}{"p": "/message/status", "o": "replace", "v": "finished_successfully"},
		}},
		map[string]interface{}{"p": "", "o": "add", "v": map[string]interface{}{
			"message": map[string]interface{}{
				"id": "a1", "conversation_id": "c1", "model_slug": "gpt-5",
				"author":  map[string]string{"role": "assistant"},
				"status":  "finished_successfully",
				"content": map[string]interface{}{"parts": []string{"HALO!"}},
				"metadata": map[string]interface{}{
					"finish_details": map[string]string{"type": "stop"},
					"usage":          map[string]int{"prompt_tokens": 1, "completion_tokens": 2},
				},
			}}},
		map[string]interface{}{"type": "message_stream_complete", "conversation_id": "c1"},
	}
	var buf strings.Builder
	for _, l := range lines {
		b, _ := json.Marshal(l)
		buf.WriteString("data: " + string(b) + "\n\n")
	}
	buf.WriteString("data: [DONE]\n\n")

	ch := make(chan Part, 32)
	go consumeSSEPatch(strings.NewReader(buf.String()), ch)
	var text strings.Builder
	var last Part
	for p := range ch {
		if p.Err != nil {
			t.Fatalf("err: %v", p.Err)
		}
		if p.Done {
			last = p
			break
		}
		text.WriteString(p.Text)
	}
	if text.String() != "HALO!" {
		t.Fatalf("delta want HALO! got %q", text.String())
	}
	if last.ConvID != "c1" || last.MessID != "a1" || last.Model != "gpt-5" {
		t.Fatalf("meta final salah: %+v", last)
	}
	if last.Finish != "stop" || last.UsageIn != 1 || last.UsageOut != 2 {
		t.Fatalf("usage/finish salah: %+v", last)
	}
}
