package main

import (
	"errors"

	"github.com/egoist/mygo"
)

// Legacy keychain keys. New saves go to connections.json; these are only read once to migrate.
func secretKey(id string) string    { return "conn." + id }
func sshSecretKey(id string) string { return "conn." + id + ".ssh" }
func sshPhraseKey(id string) string { return "conn." + id + ".ssh.ph" }

func loadLegacySecret(key string) string {
	b, err := mygo.Secrets.Get(key)
	if err != nil {
		return ""
	}
	return string(b)
}

func clearKeychainSecrets(id string) {
	for _, key := range []string{secretKey(id), sshSecretKey(id), sshPhraseKey(id)} {
		if err := mygo.Secrets.Delete(key); err != nil && !errors.Is(err, mygo.ErrSecretNotFound) {
			continue
		}
	}
}

func (a *app) migrateSecretsToStore() bool {
	changed := false
	for i, c := range a.connections {
		if c.Password == "" {
			if p := loadLegacySecret(secretKey(c.ID)); p != "" {
				a.connections[i].Password = p
				changed = true
			}
		}
		if c.SSHPassword == "" {
			if p := loadLegacySecret(sshSecretKey(c.ID)); p != "" {
				a.connections[i].SSHPassword = p
				changed = true
			}
		}
		if c.SSHPassphrase == "" {
			if p := loadLegacySecret(sshPhraseKey(c.ID)); p != "" {
				a.connections[i].SSHPassphrase = p
				changed = true
			}
		}
		clearKeychainSecrets(c.ID)
	}
	return changed
}
