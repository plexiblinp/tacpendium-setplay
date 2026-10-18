package main

import (
	"bytes"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestRun_Version(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-version"}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != version {
		t.Errorf("版: %q", out.String())
	}
}

func TestRun_BadFlag(t *testing.T) {
	if err := run([]string{"-no-such-flag"}, &bytes.Buffer{}); err == nil {
		t.Fatal("知らない引数で誤りにならない")
	}
}

// 既定のポートが使われていれば空いているポートにする。指定したポートなら誤りにする。
func TestListen_Fallback(t *testing.T) {
	busy, err := net.Listen("tcp", listenHost+":0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	if _, err := listen(port, false); err == nil {
		t.Error("使われているポートを指定したのに誤りにならない")
	}
	ln, err := listen(port, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if got := ln.Addr().(*net.TCPAddr); got.Port == port || !got.IP.IsLoopback() {
		t.Errorf("代わりの待ち受け: %v", got)
	}
}

// -port で指定したポートが使われていれば、黙って別のポートへ移らずに誤りで終わる(S8 レビュー高-3)。
// 既定値と同じ 8787 を指定したときも同じ(待ち受けは中身の検査より前なので、埋め込みの有無に関わらず決まる)。
func TestRun_ExplicitPortBusy(t *testing.T) {
	busy, err := net.Listen("tcp", listenHost+":0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port
	err = run([]string{"-open=false", "-port", strconv.Itoa(port)}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "待ち受けられない") {
		t.Fatalf("使われているポートを指定したのに誤りにならない: %v", err)
	}
}
