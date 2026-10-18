// Package localserver はローカル版(HANDOVER D10・§5)の静的配信を行う。
//
// 配るのはフロントのローカル版ビルド(web の `vite build --mode local`。データと WASM を含む)だけで、
// 計算はブラウザの中の WASM が行う(サーバは計算しない)。サイトと同じ `_headers`(Cloudflare の形式)を読み、
// 同じヘッダを付ける(CSP の下で WASM ・Worker が動くこと＝サイトと同じ条件で動かす)。
//
// 規則は web/scripts/headers.mjs と同じ: 行頭が空白でない行がパスの規則(* は任意の文字列)、続く字下げの行が
// "Name: value"、# で始まる行は注記。当たる規則はすべて足し合わせ、同じ名前のヘッダは ", " で連結する。
package localserver

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path"
	"regexp"
	"strings"
)

// HeadersFile は配信のヘッダの規則を書いたファイルの名前(dist の直下)。配らない。
const HeadersFile = "_headers"

// IndexFile は / で終わるパスに返すファイル。
const IndexFile = "index.html"

// contentTypes は拡張子ごとの Content-Type。mime パッケージは Windows では registry の値を使い、
// .js が text/plain になる環境がある(module の script と Worker が読めなくなる)ので、固定の表で返す。
// 表に無い拡張子は application/octet-stream(nosniff の下で推測させない)。
var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".mjs":  "text/javascript; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".json": "application/json; charset=utf-8",
	".wasm": "application/wasm",
	".svg":  "image/svg+xml",
	".txt":  "text/plain; charset=utf-8",
}

const defaultContentType = "application/octet-stream"

// Header は 1 つのヘッダ(名前は _headers に書いたとおり)。
type Header struct {
	Name  string
	Value string
}

// Rule は _headers の 1 つのパスの規則。
type Rule struct {
	Source  string
	pattern *regexp.Regexp
	Headers []Header
}

var headerLine = regexp.MustCompile(`^\s+([A-Za-z0-9-]+):\s*(.+)$`)

// ParseHeaders は _headers の本文を規則の列にする。形の誤りは行番号付きの誤りで返す。
func ParseHeaders(text string) ([]Rule, error) {
	var rules []Rule
	sc := bufio.NewScanner(strings.NewReader(text))
	for i := 1; sc.Scan(); i++ {
		line := strings.TrimRight(sc.Text(), " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			if !strings.HasPrefix(trimmed, "/") {
				return nil, fmt.Errorf("_headers %d 行: パスは / で始めること: %s", i, trimmed)
			}
			parts := strings.Split(trimmed, "*")
			for k, p := range parts {
				parts[k] = regexp.QuoteMeta(p)
			}
			rules = append(rules, Rule{Source: trimmed, pattern: regexp.MustCompile("^" + strings.Join(parts, ".*") + "$")})
			continue
		}
		if len(rules) == 0 {
			return nil, fmt.Errorf("_headers %d 行: パスの前にヘッダがある", i)
		}
		m := headerLine.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("_headers %d 行: \"Name: value\" の形でない: %s", i, line)
		}
		r := &rules[len(rules)-1]
		r.Headers = append(r.Headers, Header{Name: m[1], Value: m[2]})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("_headers の読み込み: %w", err)
	}
	return rules, nil
}

// HeadersFor はパスに当たるヘッダを返す(規則の順。同じ名前(大文字小文字を区別しない)は ", " で連結)。
func HeadersFor(rules []Rule, urlPath string) []Header {
	var out []Header
	index := map[string]int{}
	for _, r := range rules {
		if !r.pattern.MatchString(urlPath) {
			continue
		}
		for _, h := range r.Headers {
			key := strings.ToLower(h.Name)
			if k, ok := index[key]; ok {
				out[k].Value += ", " + h.Value
				continue
			}
			index[key] = len(out)
			out = append(out, h)
		}
	}
	return out
}

// ErrNoSite は埋め込んだ中身に index.html が無いとき(ビルドの手順を飛ばしたバイナリ)の誤り。
var ErrNoSite = errors.New("配る中身(index.html)がありません。scripts/local/build.sh でビルドしてください")

// Handler は site(dist の中身)を配る http.Handler。
type Handler struct {
	site  fs.FS
	rules []Rule
	port  string
}

// New は site を配る Handler を作る。site の直下に index.html と _headers が要る。
// port は待ち受けのポート(Host の検査に使う。DNS rebinding で別の名前から読まれないようにする)。
func New(site fs.FS, port int) (*Handler, error) {
	if _, err := fs.Stat(site, IndexFile); err != nil {
		return nil, ErrNoSite
	}
	raw, err := fs.ReadFile(site, HeadersFile)
	if err != nil {
		return nil, fmt.Errorf("%s を読めない: %w", HeadersFile, err)
	}
	rules, err := ParseHeaders(string(raw))
	if err != nil {
		return nil, err
	}
	return &Handler{site: site, rules: rules, port: fmt.Sprint(port)}, nil
}

// allowedHost は Host が自分の待ち受け(127.0.0.1 / localhost / [::1] と同じポート)か。
func (h *Handler) allowedHost(host string) bool {
	name, port, err := net.SplitHostPort(host)
	if err != nil || port != h.port {
		return false
	}
	return name == "127.0.0.1" || name == "localhost" || name == "::1"
}

// ServeHTTP は GET と HEAD だけを受け、site のファイルを返す。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.allowedHost(r.Host) {
		http.Error(w, "forbidden host", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name, ok := h.fileName(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	body, err := fs.ReadFile(h.site, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	for _, hd := range HeadersFor(h.rules, r.URL.Path) {
		w.Header().Set(hd.Name, hd.Value)
	}
	ct, ok := contentTypes[strings.ToLower(path.Ext(name))]
	if !ok {
		ct = defaultContentType
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Length", fmt.Sprint(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// fileName は URL のパスを site の中の名前にする。配らないもの(_headers ・ドットで始まる名前 ・ディレクトリ)は false。
func (h *Handler) fileName(urlPath string) (string, bool) {
	p := urlPath
	if strings.HasSuffix(p, "/") {
		p += IndexFile
	}
	name := strings.TrimPrefix(path.Clean("/"+p), "/")
	if !fs.ValidPath(name) || name == "." {
		return "", false
	}
	for _, seg := range strings.Split(name, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", false
		}
	}
	if name == HeadersFile {
		return "", false
	}
	st, err := fs.Stat(h.site, name)
	if err != nil || st.IsDir() {
		return "", false
	}
	return name, true
}
