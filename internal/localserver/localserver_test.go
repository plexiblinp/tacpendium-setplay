package localserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

const testPort = 18787

const testHeaders = `# 注記
/*
  X-Content-Type-Options: nosniff
  Content-Security-Policy: default-src 'self'
/wasm/*
  Cache-Control: no-cache
/*
  X-Test: a
/data/*
  x-test: b
`

func testSite() fstest.MapFS {
	return fstest.MapFS{
		"index.html":         {Data: []byte("<!doctype html>")},
		"_headers":           {Data: []byte(testHeaders)},
		"wasm/setplay.wasm":  {Data: []byte("\x00asm")},
		"wasm/wasm_exec.js":  {Data: []byte("//")},
		"data/manifest.json": {Data: []byte("{}")},
		"assets/x.unknown":   {Data: []byte("?")},
		".hidden":            {Data: []byte("secret")},
		"assets/.keep":       {Data: []byte("")},
	}
}

func get(t *testing.T, h http.Handler, method, target, host string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	req.Host = host
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	h, err := New(testSite(), testPort)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func TestServe_FilesAndContentTypes(t *testing.T) {
	h := newTestHandler(t)
	host := "127.0.0.1:18787"
	cases := []struct {
		path, ct, body string
	}{
		{"/", "text/html; charset=utf-8", "<!doctype html>"},
		{"/index.html", "text/html; charset=utf-8", "<!doctype html>"},
		{"/wasm/setplay.wasm", "application/wasm", "\x00asm"},
		{"/wasm/wasm_exec.js", "text/javascript; charset=utf-8", "//"},
		{"/data/manifest.json?x=1", "application/json; charset=utf-8", "{}"},
		{"/assets/x.unknown", "application/octet-stream", "?"},
	}
	for _, c := range cases {
		rec := get(t, h, http.MethodGet, c.path, host)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != c.ct || rec.Body.String() != c.body {
			t.Errorf("%s: %d %q %q(%q を期待)", c.path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String(), c.ct)
		}
	}
}

func TestServe_HeadersFromRules(t *testing.T) {
	h := newTestHandler(t)
	rec := get(t, h, http.MethodGet, "/wasm/setplay.wasm", "localhost:18787")
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'self'" {
		t.Errorf("CSP = %q", got)
	}
	// 同じ名前は大文字小文字を区別せずに連結する(headers.mjs と同じ)。
	rec = get(t, h, http.MethodGet, "/data/manifest.json", "localhost:18787")
	if got := rec.Header().Get("X-Test"); got != "a, b" {
		t.Errorf("X-Test = %q(\"a, b\" を期待)", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("/data に /wasm の規則が当たった: %q", got)
	}
}

func TestServe_Refuses(t *testing.T) {
	h := newTestHandler(t)
	ok := "127.0.0.1:18787"
	cases := []struct {
		name, method, path, host string
		code                     int
	}{
		{"_headers は配らない", http.MethodGet, "/_headers", ok, http.StatusNotFound},
		{"ドットで始まる名前", http.MethodGet, "/.hidden", ok, http.StatusNotFound},
		{"ドットで始まる名前(下の階層)", http.MethodGet, "/assets/.keep", ok, http.StatusNotFound},
		{"無いファイル", http.MethodGet, "/nope.js", ok, http.StatusNotFound},
		{"ディレクトリ", http.MethodGet, "/wasm", ok, http.StatusNotFound},
		{"外へ出るパス", http.MethodGet, "/../index.html", ok, http.StatusOK}, // Clean で / の中に戻る
		{"POST", http.MethodPost, "/", ok, http.StatusMethodNotAllowed},
		{"別の名前(DNS rebinding)", http.MethodGet, "/", "evil.example:18787", http.StatusForbidden},
		{"別のポート", http.MethodGet, "/", "127.0.0.1:1", http.StatusForbidden},
		{"ポートなし", http.MethodGet, "/", "127.0.0.1", http.StatusForbidden},
		{"IPv6 の自分", http.MethodGet, "/", "[::1]:18787", http.StatusOK},
	}
	for _, c := range cases {
		rec := get(t, h, c.method, c.path, c.host)
		if rec.Code != c.code {
			t.Errorf("%s: %d(%d を期待)", c.name, rec.Code, c.code)
		}
	}
}

func TestServe_Head(t *testing.T) {
	h := newTestHandler(t)
	rec := get(t, h, http.MethodHead, "/wasm/setplay.wasm", "127.0.0.1:18787")
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 || rec.Header().Get("Content-Length") != "4" {
		t.Errorf("HEAD: %d body=%d Content-Length=%q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Length"))
	}
}

func TestNew_NoSite(t *testing.T) {
	// ビルドの手順を飛ばしたバイナリ(埋め込みがプレースホルダだけ)。
	_, err := New(fstest.MapFS{".gitkeep": {Data: nil}}, testPort)
	if !errors.Is(err, ErrNoSite) {
		t.Fatalf("err = %v(ErrNoSite を期待)", err)
	}
	_, err = New(fstest.MapFS{"index.html": {Data: nil}}, testPort)
	if err == nil || !strings.Contains(err.Error(), HeadersFile) {
		t.Fatalf("_headers が無いのに誤りにならない: %v", err)
	}
}

func TestParseHeaders_Errors(t *testing.T) {
	for _, text := range []string{
		"wasm/*\n  A: b\n",     // / で始まらない
		"  A: b\n",             // パスの前にヘッダ
		"/*\n  not a header\n", // 形の誤り
	} {
		if _, err := ParseHeaders(text); err == nil {
			t.Errorf("誤りにならない: %q", text)
		}
	}
}

// サイトの本物の _headers(web/public/_headers)を同じ規則で読めること。読めなければローカル版は起動しない。
func TestParseHeaders_RealFile(t *testing.T) {
	raw, err := os.ReadFile("../../web/public/_headers")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := ParseHeaders(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	find := func(hs []Header, name string) string {
		for _, h := range hs {
			if strings.EqualFold(h.Name, name) {
				return h.Value
			}
		}
		return ""
	}
	root := HeadersFor(rules, "/")
	if csp := find(root, "Content-Security-Policy"); !strings.Contains(csp, "'wasm-unsafe-eval'") || !strings.Contains(csp, "connect-src 'self'") {
		t.Errorf("/ の CSP: %q", csp)
	}
	if got := find(HeadersFor(rules, "/wasm/setplay.wasm"), "Cache-Control"); got != "no-cache" {
		t.Errorf("/wasm の Cache-Control: %q", got)
	}
}
