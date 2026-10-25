package main

import (
	"runtime"

	"github.com/egoist/mygo"
)

func appMenu() *mygo.Menu {
	m := mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "文件", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
	})
	if runtime.GOOS == "windows" {
		// MyGo's text field already handles Ctrl+V/C/X/Z. The Edit menu
		// accelerators also fire on Windows, so paste/undo run twice.
		stripEditRoleAccelerators(m.Items())
	}
	return m
}

func stripEditRoleAccelerators(items []*mygo.MenuItem) {
	for _, it := range items {
		switch it.Role {
		case mygo.RoleUndo, mygo.RoleRedo, mygo.RoleCut, mygo.RoleCopy,
			mygo.RolePaste, mygo.RolePasteAndMatchStyle, mygo.RoleSelectAll:
			it.SetAccelerator("")
		}
		stripEditRoleAccelerators(it.Submenu)
	}
}
