package redisx

import (
	"net"
	"testing"
	"time"
)

func TestSSHAuthRequiresSecret(t *testing.T) {
	auths, err := sshAuth(SSHConfig{})
	if err != nil || len(auths) != 0 {
		t.Fatalf("%v %d", err, len(auths))
	}
}

func TestSSHConnIgnoresDeadline(t *testing.T) {
	c := &sshConn{Conn: discardConn{}}
	if err := c.SetDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetReadDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetWriteDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
}

type discardConn struct{ net.Conn }

func TestExpandHome(t *testing.T) {
	if got := expandHome("/tmp/id_rsa"); got != "/tmp/id_rsa" {
		t.Fatal(got)
	}
}

func TestSSHAddrDefaultPort(t *testing.T) {
	if got := (SSHConfig{Host: "bastion"}).addr(); got != "bastion:22" {
		t.Fatal(got)
	}
	if got := (SSHConfig{Host: "bastion", Port: 2222}).addr(); got != "bastion:2222" {
		t.Fatal(got)
	}
}
