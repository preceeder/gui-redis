package redisx

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestProxyAddrDefaultPort(t *testing.T) {
	if got := (ProxyConfig{Host: "127.0.0.1"}).addr(); got != "127.0.0.1:7890" {
		t.Fatal(got)
	}
	if got := (ProxyConfig{Host: "127.0.0.1", Port: 1080}).addr(); got != "127.0.0.1:1080" {
		t.Fatal(got)
	}
}

func TestDialHTTPProxyCONNECT(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			got <- err.Error()
			return
		}
		defer c.Close()
		req, err := http.ReadRequest(bufio.NewReader(c))
		if err != nil {
			got <- err.Error()
			return
		}
		got <- req.Method + " " + req.Host
		_, _ = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
		_, _ = io.Copy(io.Discard, c)
	}()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	p := ProxyConfig{Enabled: true, Type: "http", Host: host, Port: mustPort(port)}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := dialHTTPProxy(ctx, p, "redis.example:6379")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	select {
	case req := <-got:
		if req != "CONNECT redis.example:6379" {
			t.Fatalf("req %q", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("proxy saw no request")
	}
}

func TestProxyDialerRequiresHost(t *testing.T) {
	if _, err := (ProxyConfig{Enabled: true}).dialer(); err == nil {
		t.Fatal("expected host error")
	}
}

func mustPort(s string) int {
	var n int
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}
