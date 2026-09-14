package api

import (
	"chatgpt2api-go/upstream"
	"strings"
)

// streamAnyRoute: cobain jalur ANDROID dulu (bebas challenge Cloudflare),
// fallback ke jalur web kalau android gagal — termasuk 403 "unusual activity".
// Return error HANYA kalau dua-duanya gagal.
func (h *Handler) streamAnyRoute(cred upstream.Credential, model string, turns []upstream.ChatTurn) (<-chan upstream.Part, error) {
	parts, errA := h.App.Upstream.StreamAndroid(cred, model, turns, "root")
	if errA == nil {
		return parts, nil
	}
	// android gagal. 401 = token expired, sama aja di jalur web -> langsung.
	// 403 android masih layak dicoba web (kebalikannya yang terbukti: web 403, android jalan).
	if ue, ok := errA.(*upstream.UpstreamError); ok && ue.Status == 401 {
		return nil, errA
	}
	partsW, errW := h.App.Upstream.Stream(cred, model, turns, "root")
	if errW == nil {
		return partsW, nil
	}
	// kedua jalur mati —utamakannya error web (mock & produksi: status web
	// yang menentukan cooldown/invalidasi akun), android cuma di pesan log.
	if ueW, ok := errW.(*upstream.UpstreamError); ok {
		return nil, &routeError{android: errA, web: ueW}
	}
	// web pun error transport; pakai android kalau web malah sukses-status
	return nil, &routeError{android: errA, web: errW}
}

type routeError struct{ android, web error }

func (e *routeError) Error() string {
	var b strings.Builder
	b.WriteString("semua jalur upstream gagal: android=")
	b.WriteString(safeErr(e.android))
	b.WriteString(" web=")
	b.WriteString(safeErr(e.web))
	return b.String()
}

// Unwrap supaya errors.As di writeUpstreamErr/cooldownFor nemu UpstreamError web.
func (e *routeError) Unwrap() error {
	if e.web != nil {
		return e.web
	}
	return e.android
}

func safeErr(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}
