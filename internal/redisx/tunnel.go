package redisx

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConfig is an optional SSH tunnel in front of Redis.
type SSHConfig struct {
	Enabled    bool
	Host       string
	Port       int
	User       string
	Password   string
	KeyPath    string
	KeyPEM     string
	Passphrase string
}

func (s SSHConfig) addr() string {
	port := s.Port
	if port <= 0 {
		port = 22
	}
	return net.JoinHostPort(s.Host, strconv.Itoa(port))
}

func dialSSH(ctx context.Context, cfg SSHConfig, dial dialFunc) (*ssh.Client, error) {
	if cfg.Host == "" || cfg.User == "" {
		return nil, fmt.Errorf("SSH 主机和用户不能为空")
	}
	auths, err := sshAuth(cfg)
	if err != nil {
		return nil, err
	}
	if len(auths) == 0 {
		return nil, fmt.Errorf("请填写 SSH 密码或私钥")
	}
	config := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         8 * time.Second,
	}
	if dial == nil {
		dial = netDialer()
	}
	raw, err := dial(ctx, "tcp", cfg.addr())
	if err != nil {
		return nil, fmt.Errorf("SSH 连接失败: %w", err)
	}
	cc, chans, reqs, err := ssh.NewClientConn(raw, cfg.addr(), config)
	if err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("SSH 握手失败: %w", err)
	}
	return ssh.NewClient(cc, chans, reqs), nil
}

func dialSSHTunnel(ctx context.Context, cli *ssh.Client, network, addr string) (net.Conn, error) {
	type dialed struct {
		conn net.Conn
		err  error
	}
	done := make(chan dialed, 1)
	go func() {
		conn, err := cli.Dial(network, addr)
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
		return nil, fmt.Errorf("SSH 隧道超时: %w", ctx.Err())
	case r := <-done:
		if r.err != nil {
			return nil, fmt.Errorf("SSH 隧道拨号失败: %w", r.err)
		}
		return &sshConn{Conn: r.conn}, nil
	}
}

// sshConn wraps an SSH channel. x/crypto/ssh tcpChan rejects SetDeadline.
type sshConn struct{ net.Conn }

func (c *sshConn) SetDeadline(time.Time) error      { return nil }
func (c *sshConn) SetReadDeadline(time.Time) error  { return nil }
func (c *sshConn) SetWriteDeadline(time.Time) error { return nil }

func sshAuth(cfg SSHConfig) ([]ssh.AuthMethod, error) {
	var auths []ssh.AuthMethod
	if strings.TrimSpace(cfg.KeyPEM) != "" || strings.TrimSpace(cfg.KeyPath) != "" {
		signer, err := loadSSHSigner(cfg)
		if err != nil {
			return nil, err
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		auths = append(auths, ssh.Password(cfg.Password))
	}
	return auths, nil
}

func loadSSHSigner(cfg SSHConfig) (ssh.Signer, error) {
	var b []byte
	if pem := strings.TrimSpace(cfg.KeyPEM); pem != "" {
		b = []byte(pem)
	} else {
		raw, err := os.ReadFile(expandHome(cfg.KeyPath))
		if err != nil {
			return nil, fmt.Errorf("读取 SSH 私钥失败: %w", err)
		}
		b = raw
	}
	if cfg.Passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(b, []byte(cfg.Passphrase))
		if err != nil {
			return nil, fmt.Errorf("解析 SSH 私钥失败: %w", err)
		}
		return signer, nil
	}
	signer, err := ssh.ParsePrivateKey(b)
	if err != nil {
		return nil, fmt.Errorf("解析 SSH 私钥失败: %w", err)
	}
	return signer, nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
