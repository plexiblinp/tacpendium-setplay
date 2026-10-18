package main

import (
	"bytes"
	"flag"
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

// -port を指定したら、既定値と同じ 8787 でも空いているポートへ移らない(S8 レビュー高-3・再レビュー高-A)。
func TestPortFallback(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{"-open=false"}, true},
		{[]string{"-port", "8787"}, false},
		{[]string{"-port=8787"}, false},
		{[]string{"-port", "0"}, false},
		{[]string{"-port", "9000"}, false},
	}
	for _, c := range cases {
		fl := flag.NewFlagSet("t", flag.ContinueOnError)
		fl.Int("port", defaultPort, "")
		fl.Bool("open", true, "")
		if err := fl.Parse(c.args); err != nil {
			t.Fatal(err)
		}
		if got := portFallback(fl); got != c.want {
			t.Errorf("%v: portFallback = %v(%v を期待)", c.args, got, c.want)
		}
	}
}
