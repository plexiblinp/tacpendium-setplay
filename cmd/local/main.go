// Command local はローカル版(HANDOVER D10・§5)の単一バイナリ。KA の上限 100 を外した画面を手元で開く。
//
// フロントのローカル版ビルド(データと WASM を含む)を埋め込み、127.0.0.1 だけで配る。計算はブラウザの中の
// WASM が行う(サーバは計算しない・外へ通信しない)。起動するとブラウザで画面を開く。Ctrl+C で終わる。
//
// 埋め込む中身は scripts/local/build.sh が site/ に置く(リポジトリにはプレースホルダだけ)。
// 中身の無いバイナリは起動時に理由を出して終わる。
//
//	tacpendium-setplay-local [-port 8787] [-open=false] [-version]
//
// -port を指定しなければ 8787 で待ち受け、使われていれば空いているポートにする。指定したポートが使えなければ誤りで終わる。
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/plexiblinp/tacpendium-setplay/internal/localserver"
)

//go:embed all:site
var embedded embed.FS

// version はビルドの版(scripts/local/build.sh が -ldflags -X で入れる)。
var version = "dev"

const (
	// defaultPort は既定の待ち受けのポート。使われていれば空いているポートにする。
	defaultPort = 8787
	// listenHost は待ち受けの名前。ほかの端末からは読めないようにする。
	listenHost = "127.0.0.1"
	// readHeaderTimeout ・shutdownTimeout はサーバの時間の上限。
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	fl := flag.NewFlagSet("tacpendium-setplay-local", flag.ContinueOnError)
	port := fl.Int("port", defaultPort, "待ち受けのポート(0 なら空いているポート。指定しなければ 8787 で、使われていれば空いているポートにする)")
	open := fl.Bool("open", true, "起動したらブラウザで開く")
	showVersion := fl.Bool("version", false, "版を出して終わる")
	if err := fl.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(out, version)
		return nil
	}
	site, err := fs.Sub(embedded, "site")
	if err != nil {
		return fmt.Errorf("埋め込みの中身: %w", err)
	}
	// 空いているポートへ移るのは -port を指定しなかったときだけ(指定したポートが使えなければ誤り)。
	explicit := false
	fl.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "port" })
	ln, err := listen(*port, !explicit)
	if err != nil {
		return err
	}
	actual := ln.Addr().(*net.TCPAddr).Port
	h, err := localserver.New(site, actual)
	if err != nil {
		ln.Close()
		return err
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: readHeaderTimeout}
	url := fmt.Sprintf("http://%s:%d/", listenHost, actual)
	fmt.Fprintf(out, "Tacpendium-Setplay (local) %s\n%s を開いてください / Open %s\nCtrl+C で終了 / Press Ctrl+C to quit\n", version, url, url)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	if *open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(out, "ブラウザを開けませんでした(上の URL を開いてください): %v\n", err)
		}
	}
	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("配信: %w", err)
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return fmt.Errorf("終了: %w", err)
	}
	return nil
}

// listen は 127.0.0.1:port で待ち受ける。fallback なら、使われていたときに空いているポートにする。
func listen(port int, fallback bool) (net.Listener, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenHost, port))
	if err == nil || !fallback {
		if err != nil {
			return nil, fmt.Errorf("ポート %d で待ち受けられない: %w", port, err)
		}
		return ln, nil
	}
	ln, err = net.Listen("tcp", listenHost+":0")
	if err != nil {
		return nil, fmt.Errorf("待ち受けられない: %w", err)
	}
	return ln, nil
}

// openBrowser は OS の既定のブラウザで url を開く(待たない)。
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
