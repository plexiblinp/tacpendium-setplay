package main

import (
	"bytes"
	"net"
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
