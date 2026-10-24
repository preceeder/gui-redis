package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const fileName = "connections.json"

// Connection is a saved standalone Redis endpoint, including secrets.
type Connection struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	DB       int    `json:"db"`
	LastDB   *int   `json:"lastDB,omitempty"`

	SSH           bool   `json:"ssh,omitempty"`
	SSHHost       string `json:"sshHost,omitempty"`
	SSHPort       int    `json:"sshPort,omitempty"`
	SSHUser       string `json:"sshUser,omitempty"`
	SSHAuth       string `json:"sshAuth,omitempty"` // password | key
	SSHPassword   string `json:"sshPassword,omitempty"`
	SSHKeyPath    string `json:"sshKeyPath,omitempty"`
	SSHKey        string `json:"sshKey,omitempty"`
	SSHPassphrase string `json:"sshPassphrase,omitempty"`

	Proxy         bool   `json:"proxy,omitempty"`
	ProxyType     string `json:"proxyType,omitempty"` // socks5 | http
	ProxyHost     string `json:"proxyHost,omitempty"`
	ProxyPort     int    `json:"proxyPort,omitempty"`
	ProxyUser     string `json:"proxyUser,omitempty"`
	ProxyPassword string `json:"proxyPassword,omitempty"`
}

// File is the on-disk app settings.
type File struct {
	Version     int          `json:"version"`
	LastConnID  string       `json:"lastConnID"`
	Separator   string       `json:"separator"`
	Connections []Connection `json:"connections"`
}

// Path returns connections.json under dir.
func Path(dir string) string {
	return filepath.Join(dir, fileName)
}

// Load reads settings. A missing file returns an empty File and no error.
func Load(dir string) (File, error) {
	b, err := os.ReadFile(Path(dir))
	if errors.Is(err, os.ErrNotExist) {
		return File{Version: 1, Separator: ":"}, nil
	}
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, err
	}
	if f.Version == 0 {
		f.Version = 1
	}
	if f.Separator == "" {
		f.Separator = ":"
	}
	return f, nil
}

// Save writes settings, creating dir if needed.
func Save(dir string, f File) error {
	if f.Version == 0 {
		f.Version = 1
	}
	if f.Separator == "" {
		f.Separator = ":"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(dir), b, 0o600)
}

// DefaultLocal is the first-run connection.
func DefaultLocal() Connection {
	return Connection{
		ID:   "local",
		Name: "Local",
		Host: "127.0.0.1",
		Port: 6379,
	}
}
