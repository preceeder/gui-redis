package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	f, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f.Separator != ":" || len(f.Connections) != 0 {
		t.Fatalf("%+v", f)
	}
}

func TestSaveRoundTripKeepsSecrets(t *testing.T) {
	dir := t.TempDir()
	in := File{
		LastConnID: "a1",
		Separator:  "/",
		Connections: []Connection{{
			ID: "a1", Name: "dev", Host: "10.0.0.1", Port: 6380, Username: "app", Password: "redis-secret", DB: 2,
			SSH: true, SSHHost: "bastion", SSHPort: 22, SSHUser: "ops", SSHAuth: "key",
			SSHKeyPath: "/tmp/id_rsa", SSHKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nxxx\n", SSHPassphrase: "phrase",
			Proxy: true, ProxyType: "socks5", ProxyHost: "127.0.0.1", ProxyPort: 7890, ProxyUser: "p", ProxyPassword: "proxy-secret",
		}},
	}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(Path(dir)) != "connections.json" {
		t.Fatal(Path(dir))
	}
	text := string(raw)
	for _, want := range []string{"redis-secret", "sshKey", "phrase", "/tmp/id_rsa", "proxy-secret", "7890"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.LastConnID != "a1" || out.Separator != "/" || len(out.Connections) != 1 {
		t.Fatalf("%+v", out)
	}
	got := out.Connections[0]
	if got.Host != "10.0.0.1" || got.Port != 6380 || got.Username != "app" || got.DB != 2 || got.Password != "redis-secret" {
		t.Fatalf("%+v", got)
	}
	if !got.SSH || got.SSHHost != "bastion" || got.SSHUser != "ops" || got.SSHKeyPath != "/tmp/id_rsa" || got.SSHKey == "" || got.SSHPassphrase != "phrase" {
		t.Fatalf("ssh %+v", got)
	}
	if !got.Proxy || got.ProxyType != "socks5" || got.ProxyHost != "127.0.0.1" || got.ProxyPort != 7890 || got.ProxyPassword != "proxy-secret" {
		t.Fatalf("proxy %+v", got)
	}
}
