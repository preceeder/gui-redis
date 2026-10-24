package main

import (
	"fmt"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"redis/internal/format"
)

func (a *app) newConnMenu(m *ui.Menu) {
	if m.Item("新建连接").Chosen() {
		a.openNewConn()
	}
}

func (a *app) connPane(c *ui.Context, t *ui.Theme) {
	pane := ui.Column(c).Fill().Background(t.Surface).Padding(10).Gap(8)
	pane.ContextMenu(a.newConnMenu)
	clearOnBlank(pane, a.clearNavFocus)
	pane.Children(func() {
		if len(a.connections) == 0 {
			empty := ui.Column(c).Grow(1).Center().Gap(8)
			empty.ContextMenu(a.newConnMenu)
			empty.Children(func() {
				ui.Text(c, "还没有连接").TextColor(t.TextMuted)
				if ui.PrimaryButton(c.Key("sidebar-new-conn"), "新建连接").Clicked() {
					a.openNewConn()
				}
			})
			return
		}
		a.connOutline.List.Selected = &a.navRow
		tree := ui.Outline(c, &a.connOutline, a.navRoots(), a.navChildren, func(id string) {
			if connID, db, ok := parseDBNode(id); ok {
				a.dbTreeRow(c, t, connID, db)
				return
			}
			a.connTreeRow(c, t, id)
		}).Grow(1).Padding(4)
		if tree.Clicked() {
			a.clearNavFocus()
		} else if a.navRow >= 0 && a.navRow < a.connOutline.Rows() {
			a.selectedNav = a.connOutline.Item(a.navRow)
		}
		if tree.Submitted() {
			a.submitNav()
		}
	})
}

func (a *app) connTreeRow(c *ui.Context, t *ui.Theme, id string) {
	conn, ok := a.connByID(id)
	if !ok {
		ui.Text(c, id)
		return
	}
	color := t.TextMuted
	if a.isLive(conn.ID) {
		color = ui.Hex("#22c55e")
	}
	row := ui.Row(c.Key(conn.ID)).Gap(8).AlignItems(ui.Center).Grow(1).FillHeight()
	row.Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(color)
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, conn.Name).SingleLine().Bold()
			addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
			if conn.SSH {
				addr += " · SSH"
			}
			if conn.Proxy {
				addr += " · 代理"
			}
			ui.Text(c, addr).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
	})
	row.ContextMenu(func(m *ui.Menu) { a.fillNavMenu(m, conn.ID) })
}

func (a *app) fillNavMenu(m *ui.Menu, id string) {
	if connID, db, ok := parseDBNode(id); ok {
		a.fillDBMenu(m, connID, db)
		return
	}
	conn, ok := a.connByID(id)
	if !ok {
		return
	}
	if m.Item("连接").Disabled(a.connecting).Chosen() {
		a.selectedNav = conn.ID
		a.connectSelected()
	}
	if m.Item("断开").Disabled(a.connectedID != conn.ID).Chosen() {
		a.disconnect()
	}
	if m.Item("刷新").Disabled(a.connectedID != conn.ID || a.loadingKeys || a.loadingDBs).Chosen() {
		a.selectedNav = conn.ID
		if a.inDB() {
			a.refreshKeys(true)
		} else {
			a.refreshDatabases()
		}
	}
	if m.Item("编辑").Chosen() {
		a.selectedNav = conn.ID
		a.openEditConn()
	}
	m.Separator()
	if m.Item("删除").Chosen() {
		a.selectedNav = conn.ID
		a.askConfirm(confirmDeleteConn, conn.Name)
	}
}

func (a *app) dbTreeRow(c *ui.Context, t *ui.Theme, connID string, db int) {
	keys := int64(0)
	for _, item := range a.databasesOf(connID) {
		if item.Index == db {
			keys = item.Keys
			break
		}
	}
	active := a.connectedID == connID && a.activeDB == db
	row := ui.Row(c.Key(dbNodeID(connID, db))).Gap(8).AlignItems(ui.Center).Grow(1).FillHeight()
	row.Children(func() {
		label := ui.Text(c, fmt.Sprintf("db%d", db)).SingleLine().Grow(1)
		if active {
			label.Bold()
		}
		ui.Text(c, format.Size(keys)).FontSize(11).TextColor(t.TextMuted)
	})
	row.ContextMenu(func(m *ui.Menu) { a.fillDBMenu(m, connID, db) })
}

func (a *app) fillDBMenu(m *ui.Menu, connID string, db int) {
	if m.Item("打开").Disabled(a.connecting || !a.isLive(connID)).Chosen() {
		a.selectedNav = dbNodeID(connID, db)
		a.openConnDatabase(connID, db)
	}
	if m.Item("同步").Disabled(!a.isLive(connID)).Chosen() {
		a.openSync(connID, db, 0, "", nil)
	}
	if m.Item("导入").Disabled(!a.isLive(connID)).Chosen() {
		a.importDatabase(connID, db)
	}
	if m.Item("导出").Disabled(!a.isLive(connID)).Chosen() {
		a.exportDatabase(connID, db)
	}
}

func (a *app) submitNav() {
	id := a.selectedNav
	if connID, db, ok := parseDBNode(id); ok {
		a.openConnDatabase(connID, db)
		return
	}
	if id != "" {
		a.connectSelected()
	}
}

func (a *app) connFormFields(c *ui.Context) {
	ui.Form(c, func() {
		ui.Field(c, "名称", func() {
			ui.TextInput(c, &a.formName).Placeholder("Local")
		})
		ui.Field(c, "主机", func() {
			ui.TextInput(c, &a.formHost).Placeholder("127.0.0.1")
		})
		ui.Field(c, "端口", func() {
			ui.TextInput(c, &a.formPort).Placeholder("6379")
		})
		ui.Field(c, "用户名", func() {
			ui.TextInput(c, &a.formUser).Placeholder("可选")
		})
		ui.Field(c, "密码", func() {
			a.secretInput(c, "pass", "可选", &a.formPass, &a.formShowPass)
		})
		ui.Field(c, "隧道", func() {
			ui.Checkbox(c, &a.formSSH, "SSH 隧道")
		})
		if a.formSSH {
			ui.Field(c, "SSH 主机", func() {
				ui.TextInput(c, &a.formSSHHost).Placeholder("bastion.example.com")
			})
			ui.Field(c, "SSH 端口", func() {
				ui.TextInput(c, &a.formSSHPort).Placeholder("22")
			})
			ui.Field(c, "SSH 用户", func() {
				ui.TextInput(c, &a.formSSHUser).Placeholder("root")
			})
			ui.Field(c, "SSH 认证", func() {
				ui.Segmented(c, &a.formSSHAuth, "密码", "私钥")
			})
			if a.formSSHAuth == 1 {
				ui.Field(c, "私钥", func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.TextInput(c, &a.formSSHKey).Placeholder("~/.ssh/id_rsa").Grow(1)
						if ui.Button(c.Key("pick-ssh-key"), "选择").Clicked() {
							a.pickSSHKey()
						}
					})
				})
				ui.Field(c, "私钥口令", func() {
					a.secretInput(c, "ssh-phrase", "可选", &a.formSSHPhrase, &a.formShowSSHPhrase)
				})
			} else {
				ui.Field(c, "SSH 密码", func() {
					a.secretInput(c, "ssh-pass", "必填", &a.formSSHPass, &a.formShowSSHPass)
				})
			}
		}
		ui.Field(c, "代理", func() {
			ui.Checkbox(c, &a.formProxy, "本地代理")
		})
		if a.formProxy {
			ui.Field(c, "代理类型", func() {
				ui.Segmented(c, &a.formProxyType, "SOCKS5", "HTTP")
			})
			ui.Field(c, "代理主机", func() {
				ui.TextInput(c, &a.formProxyHost).Placeholder("127.0.0.1")
			})
			ui.Field(c, "代理端口", func() {
				ui.TextInput(c, &a.formProxyPort).Placeholder("7890")
			})
			ui.Field(c, "代理用户", func() {
				ui.TextInput(c, &a.formProxyUser).Placeholder("可选")
			})
			ui.Field(c, "代理密码", func() {
				a.secretInput(c, "proxy-pass", "可选", &a.formProxyPass, &a.formShowProxyPass)
			})
		}
	})
}

func (a *app) secretInput(c *ui.Context, id, placeholder string, value *string, show *bool) {
	ui.Row(c.Key("secret-"+id)).Grow(1).Children(func() {
		in := ui.TextInput(c, value).Placeholder(placeholder).Grow(1).PaddingEnd(28)
		if !*show {
			in.Password()
		}
		t := c.Theme()
		eye := ui.ButtonBase(c.Key("eye-" + id)).Absolute().Right(4).Top(0).Bottom(0).Width(24)
		if *show {
			eye.Label("隐藏密码").Tooltip("隐藏密码")
		} else {
			eye.Label("显示密码").Tooltip("显示密码")
		}
		color := t.TextMuted
		if eye.Hovered() {
			color = t.Text
		}
		eye.TextColor(color)
		eye.Children(func() {
			ic := iconEye
			if *show {
				ic = iconEyeOff
			}
			ui.Icon(c, ic).Size(16, 16)
		})
		if eye.Clicked() {
			*show = !*show
		}
	})
}

func iconSVG(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconEye    = iconSVG(`<path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/>`)
	iconEyeOff = iconSVG(`<path d="M9.88 9.88a3 3 0 1 0 4.24 4.24"/><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68"/><path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61"/><line x1="2" x2="22" y1="2" y2="22"/>`)
)

func (a *app) pickSSHKey() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win,
			Title:  "选择 SSH 私钥",
		})
		if err != nil || len(paths) == 0 {
			return
		}
		pem := ""
		if b, err := os.ReadFile(paths[0]); err == nil {
			pem = string(b)
		}
		a.publish(func() {
			a.formSSHKey = paths[0]
			a.formSSHKeyPEM = pem
		})
	}()
}

func (a *app) connDialog(c *ui.Context, t *ui.Theme) {
	title := "新建连接"
	if a.editingID != "" {
		title = "编辑连接"
	}
	ui.Modal(c, &a.showConnForm, func() {
		ui.Column(c).Width(460).Gap(12).Children(func() {
			ui.Text(c, title).Bold()
			ui.Scroll(c).MaxHeight(440).Children(func() {
				a.connFormFields(c)
			})
			if a.formError != "" {
				ui.Text(c, a.formError).FontSize(12).TextColor(ui.Hex("#ef4444"))
			} else if a.formHint != "" {
				ui.Text(c, a.formHint).FontSize(12).TextColor(ui.Hex("#22c55e"))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c.Key("test-conn"), "测试连接").Disabled(a.formTesting).Clicked() {
					a.testConnForm()
				}
				if ui.Button(c, "取消").Clicked() {
					a.showConnForm = false
				}
				if ui.PrimaryButton(c, "保存").Clicked() {
					if a.saveConnForm() && a.win != nil {
						c.Toast("已保存连接")
					}
				}
			})
		})
	})
}
