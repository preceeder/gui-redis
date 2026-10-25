package main

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestStripEditRoleAccelerators(t *testing.T) {
	m := mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleEditMenu}})
	if pasteAccel(m.Items()) == "" {
		t.Fatal("edit menu should start with a paste shortcut")
	}
	stripEditRoleAccelerators(m.Items())
	if got := pasteAccel(m.Items()); got != "" {
		t.Fatalf("paste accelerator %q, want empty", got)
	}
}

func pasteAccel(items []*mygo.MenuItem) string {
	for _, it := range items {
		if it.Role == mygo.RolePaste {
			return it.Accelerator
		}
		if s := pasteAccel(it.Submenu); s != "" || roleIn(it.Submenu, mygo.RolePaste) {
			return s
		}
	}
	return ""
}

func roleIn(items []*mygo.MenuItem, role mygo.MenuRole) bool {
	for _, it := range items {
		if it.Role == role || roleIn(it.Submenu, role) {
			return true
		}
	}
	return false
}
