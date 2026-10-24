package redisx

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// ProxyConfig is an optional HTTP or SOCKS5 proxy in front of Redis (or SSH).
type ProxyConfig struct {
	Enabled  bool
	Type     string // socks5 | http
	Host     string
	Port     int
	Username string
	Password string
}

func (p ProxyConfig) addr() string {
	port := p.Port
	if port <= 0 {
		port = 7890
	}
	return net.JoinHostPort(p.Host, strconv.Itoa(port))
}

func (p ProxyConfig) kind() string {
	if strings.EqualFold(p.Type, "http") {
		return "http"
	}
	return "socks5"
}

func netDialer() dialFunc {
	d := net.Dialer{Timeout: 8 * time.Second}
	return d.DialContext
}

func (p ProxyConfig) dialer() (dialFunc, error) {
	if !p.Enabled {
		return netDialer(), nil
	}
	if strings.TrimSpace(p.Host) == "" {
		return nil, fmt.Errorf("代理主机不能为空")
	}
	if p.kind() == "http" {
		return p.httpDialer(), nil
	}
	return p.socksDialer()
}

func (p ProxyConfig) socksDialer() (dialFunc, error) {
	var auth *proxy.Auth
	if p.Username != "" || p.Password != "" {
		auth = &proxy.Auth{User: p.Username, Password: p.Password}
	}
	d, err := proxy.SOCKS5("tcp", p.addr(), auth, &net.Dialer{Timeout: 8 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 代理无效: %w", err)
	}
	if cd, ok := d.(proxy.ContextDialer); ok {
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := cd.DialContext(ctx, network, addr)
			if err != nil {
				return nil, fmt.Errorf("SOCKS5 代理失败: %w", err)
			}
			return conn, nil
		}, nil
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		type dialed struct {
			conn net.Conn
			err  error
		}
		done := make(chan dialed, 1)
		go func() {
			conn, err := d.Dial(network, addr)
			done <- dialed{conn, err}
		}()
		select {
		case <-ctx.Done():
			go func() {
				r := <-done
				if r.conn != nil {
					_ = r.conn.Close()
				}
			}()
			return nil, fmt.Errorf("SOCKS5 代理超时: %w", ctx.Err())
		case r := <-done:
			if r.err != nil {
				return nil, fmt.Errorf("SOCKS5 代理失败: %w", r.err)
			}
			return r.conn, nil
		}
	}, nil
}

func (p ProxyConfig) httpDialer() dialFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialHTTPProxy(ctx, p, addr)
	}
}

func dialHTTPProxy(ctx context.Context, p ProxyConfig, target string) (net.Conn, error) {
	d := net.Dialer{Timeout: 8 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", p.addr())
	if err != nil {
		return nil, fmt.Errorf("HTTP 代理连接失败: %w", err)
	}
	var req strings.Builder
	fmt.Fprintf(&req, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", target, target)
	if p.Username != "" || p.Password != "" {
		token := base64.StdEncoding.EncodeToString([]byte(p.Username + ":" + p.Password))
		fmt.Fprintf(&req, "Proxy-Authorization: Basic %s\r\n", token)
	}
	req.WriteString("\r\n")
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := io.WriteString(conn, req.String()); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP 代理请求失败: %w", err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP 代理响应无效: %w", err)
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP 代理拒绝: %s", resp.Status)
	}
	_ = conn.SetDeadline(time.Time{})
	if n := br.Buffered(); n > 0 {
		extra, err := br.Peek(n)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("HTTP 代理响应无效: %w", err)
		}
		return &prefixConn{Conn: conn, prefix: extra}, nil
	}
	return conn, nil
}

type prefixConn struct {
	net.Conn
	prefix []byte
}

func (c *prefixConn) Read(p []byte) (int, error) {
	if len(c.prefix) > 0 {
		n := copy(p, c.prefix)
		c.prefix = c.prefix[n:]
		return n, nil
	}
	return c.Conn.Read(p)
}
