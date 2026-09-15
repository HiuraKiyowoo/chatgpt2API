package api

import (
	"chatgpt2api-go/upstream"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ---- mode "mini deep research" (gpt-5-research) ----
//
// Deep Research native = fitur paid (terbukti: endpoint job 404 & slug
// 'deep-research' cuma fallback model biasa di akun free). Emulasinya 3 fase,
// semua jalan di jalur yang sama kayak chat biasa:
//
//	fase 1 PLAN       : model pecah pertanyaan jadi max 3 sub-pertanyaan
//	fase 2 RESEARCH   : tiap sub-pertanyaan dijawab dengan web_search ON
//	fase 3 SINTESIS   : jawaban asli + temuan -> laporan final (di-stream)
//
// Kalkulasi kuota: 1 + n + 1 request (maks 5). Fallback aman: kalau plan
// gagal diparse, sub-pertanyaan = pertanyaan asli.

const researchMaxSubq = 3

// researchEngine bikin stream final. Signature cocok buat dipakai handler:
// balikin channel Part yang selesai setelah sintesis.
func (h *Handler) runResearch(cred upstream.Credential, slug string, turns []upstream.ChatTurn, feat upstream.ChatFeatures) (<-chan upstream.Part, error) {
	if len(turns) == 0 {
		return nil, fmt.Errorf("research: history kosong")
	}
	last := turns[len(turns)-1]
	q := last.Content
	hist := turns[:len(turns)-1]

	out := make(chan upstream.Part, 32)
	go func() {
		defer close(out)

		// ---- fase 1: plan ----
		planTurns := append(cloneTurns(hist),
			upstream.ChatTurn{Role: "user", Content: researchPlanPrompt(q)})
		plan, err := h.collectOnce(cred, slug, planTurns, upstream.ChatFeatures{Reasoning: "high"})
		if err != nil {
			sendErr(out, err)
			return
		}
		subqs := parseSubQuestions(plan, q)

		// ---- fase 2: riset per sub-pertanyaan ----
		type finding struct{ q, a string }
		findings := make([]finding, 0, len(subqs))
		for _, sq := range subqs {
			resTurns := []upstream.ChatTurn{{Role: "user", Content: sq}}
			ans, err := h.collectOnce(cred, slug, resTurns, upstream.ChatFeatures{WebSearch: true})
			if err != nil {
				sendErr(out, err)
				return
			}
			findings = append(findings, finding{sq, strings.TrimSpace(ans)})
		}

		// ---- fase 3: sintesis (di-stream) ----
		var b strings.Builder
		b.WriteString("Pertanyaan asli: ")
		b.WriteString(q)
		b.WriteString("\n\nTemuan riset:\n")
		for i, f := range findings {
			fmt.Fprintf(&b, "\n%d) %s\n%s\n", i+1, f.q, truncateRunes(f.a, 3000))
		}
		b.WriteString("\nTulis jawaban final atas pertanyaan asli pakai temuan di atas. " +
			"Kalau ada angka/fakta, sebut sumbernya singkat dalam kurung. Bahasa mengikuti pertanyaan asli.")
		synTurns := append(cloneTurns(hist), upstream.ChatTurn{Role: "user", Content: b.String()})
		feat2 := feat
		feat2.Research = false // cegah rekursi
		feat2.WebSearch = true
		ch, err := h.streamAnyRoute(cred, slug, synTurns, feat2)
		if err != nil {
			sendErr(out, err)
			return
		}
		for p := range ch {
			out <- p
		}
	}()
	return out, nil
}

func sendErr(ch chan<- upstream.Part, err error) { ch <- upstream.Part{Err: err} }

// collectOnce manggil satu request non-stream dan kumpulin teksnya.
func (h *Handler) collectOnce(cred upstream.Credential, slug string, turns []upstream.ChatTurn, feat upstream.ChatFeatures) (string, error) {
	ch, err := h.streamAnyRoute(cred, slug, turns, feat)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for p := range ch {
		if p.Err != nil {
			return b.String(), p.Err
		}
		b.WriteString(p.Text)
	}
	return b.String(), nil
}

func cloneTurns(t []upstream.ChatTurn) []upstream.ChatTurn {
	c := make([]upstream.ChatTurn, len(t))
	copy(c, t)
	return c
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func researchPlanPrompt(q string) string {
	return "Kamu perencana riset. Pecah pertanyaan berikut jadi maksimal " +
		strconv.Itoa(researchMaxSubq) + " sub-pertanyaan pencarian yang kalau dijawab " +
		"semuanya cukup buat menjawab pertanyaan asli. Aturan: satu baris per sub-pertanyaan, " +
		"format '1. ...', TANPA penjelasan lain.\n\nPertanyaan: " + q
}

var reSubq = regexp.MustCompile(`(?m)^\s*\d+[.)]\s*(.+)$`)

// parseSubQuestions ambil daftar bernomor; gagal total -> fallback pertanyaan asli.
func parseSubQuestions(plan, orig string) []string {
	ms := reSubq.FindAllStringSubmatch(plan, -1)
	var out []string
	for _, m := range ms {
		s := strings.TrimSpace(m[1])
		if s != "" && len([]rune(s)) > 4 {
			out = append(out, s)
		}
		if len(out) >= researchMaxSubq {
			break
		}
	}
	if len(out) == 0 {
		out = []string{orig}
	}
	return out
}
