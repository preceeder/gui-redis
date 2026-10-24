package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"redis/internal/format"
)

func (a *app) valuePane(c *ui.Context, t *ui.Theme) {
	pane := ui.Column(c).Fill().Padding(10).Gap(8)
	clearOnBlank(pane, a.clearMemberFocus)
	pane.Children(func() {
		switch {
		case !a.connected():
			ui.Column(c).Grow(1).Center().Children(func() {
				ui.Text(c, "未连接").TextColor(t.TextMuted)
			})
		case !a.inDB(), a.selected == "":
			a.serverInfoPane(c, t)
		case !a.tree.IsKey[a.selected]:
			ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
				ui.Text(c, "前缀 "+a.selected).Bold()
				ui.Textf(c, "共 %d 个 key", a.tree.DescendantKeys(a.selected)).TextColor(t.TextMuted)
			})
		case a.loadingVal && a.val.Key != a.selected:
			ui.Column(c).Grow(1).Center().Children(func() {
				ui.Spinner(c)
				ui.Text(c, "加载中…").TextColor(t.TextMuted)
			})
		default:
			a.valueHeader(c, t)
			if a.valErr != "" {
				ui.Text(c, a.valErr).FontSize(12).TextColor(ui.Hex("#ef4444"))
			}
			if a.val.TooLarge {
				ui.Text(c, fmt.Sprintf("值超过 %s 字节，已跳过加载", format.Size(format.MaxStringBytes))).
					TextColor(t.TextMuted)
				return
			}
			if a.val.Note != "" {
				ui.Text(c, a.val.Note).TextColor(t.TextMuted)
				return
			}
			if a.val.Truncated {
				ui.Text(c, fmt.Sprintf("仅显示前 %d / %d 项", shownCount(a.val.Type, a), a.val.Total)).
					FontSize(12).TextColor(t.TextMuted)
			}
			switch a.val.Type {
			case "string":
				a.stringEditor(c, t)
			case "hash":
				a.hashEditor(c, t)
			case "list":
				a.listEditor(c, t)
			case "set":
				a.setEditor(c, t)
			case "zset":
				a.zsetEditor(c, t)
			case "stream":
				a.streamEditor(c, t)
			default:
				ui.Text(c, "未知类型 "+a.val.Type).TextColor(t.TextMuted)
			}
		}
	})
}

func shownCount(typ string, a *app) int {
	switch typ {
	case "hash":
		return len(a.val.Hash)
	case "list":
		return len(a.val.List)
	case "set":
		return len(a.val.Set)
	case "zset":
		return len(a.val.ZSet)
	case "stream":
		return len(a.val.Stream)
	}
	return 0
}

func (a *app) valueHeader(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "复制 Key").Clicked() {
			a.copyText(a.val.Key, "已复制 "+a.val.Key)
			c.Toast("已复制 Key")
		}
		if ui.Button(c, "复制 Value").Clicked() {
			a.copyValue()
			c.Toast("已复制 Value")
		}
		if ui.Button(c, "复制命令").Clicked() {
			a.copyCommand()
			c.Toast("已复制命令")
		}
		if ui.Button(c, "TTL").Clicked() {
			a.showTTL = true
		}
		if ui.Button(c, "重命名").Clicked() {
			a.renameEdit = a.val.Key
			a.showRename = true
		}
		if ui.Button(c, "删除").Clicked() {
			a.askConfirm(confirmDeleteKey, a.val.Key)
		}
		if ui.Button(c, "刷新").Disabled(a.loadingVal).Clicked() {
			a.loadValue(a.val.Key)
		}
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Badge(c, a.val.Type)
		ui.Text(c, a.val.Key).Bold().Grow(1)
	})
	ui.Text(c, "TTL "+format.TTL(a.val.TTL)+" · 大小 "+format.Size(a.val.Len)+" · 内存 "+format.Bytes(a.val.Memory)).
		FontSize(12).TextColor(t.TextMuted)
	if a.val.Truncated {
		if ui.Button(c, "加载更多成员").Disabled(a.loadingVal).Clicked() {
			a.loadMoreValue()
		}
	}
}

func (a *app) serverInfoPane(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Grow(1).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "服务信息").Bold()
			ui.Spacer(c)
			if ui.Button(c, "CLI").Clicked() {
				a.showCLI = !a.showCLI
			}
			if ui.Button(c, "慢日志").Disabled(a.client == nil || !a.client.Ready()).Clicked() {
				a.refreshSlowlog()
			}
			if ui.Button(c, "刷新").Disabled(a.client == nil || !a.client.Ready()).Clicked() {
				a.refreshServerInfo()
				a.refreshDatabases()
				a.refreshSlowlog()
			}
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(14).Children(func() {
				a.serverInfoBody(c, t)
			})
		})
	})
}

func (a *app) serverInfoBody(c *ui.Context, t *ui.Theme) {
	type kv struct{ k, v string }
	type block struct {
		title string
		rows  []kv
	}
	blocks := []block{}
	appendBlock := func(title string, rows []kv) {
		clean := make([]kv, 0, len(rows))
		for _, r := range rows {
			if r.v != "" {
				clean = append(clean, r)
			}
		}
		if len(clean) > 0 {
			blocks = append(blocks, block{title, clean})
		}
	}

	connRows := []kv{}
	if conn, ok := a.connByID(a.connectedID); ok {
		connRows = append(connRows, kv{"名称", conn.Name}, kv{"地址", fmt.Sprintf("%s:%d", conn.Host, conn.Port)})
	}
	if a.inDB() {
		connRows = append(connRows, kv{"当前库", fmt.Sprintf("db%d", a.activeDB)}, kv{"Keys", format.Size(a.info.DBSize)})
	}
	appendBlock("连接", connRows)
	if len(a.info.Sections) == 0 {
		appendBlock("服务器", []kv{
			{"版本", a.info.Version},
			{"角色", a.info.Role},
			{"内存", a.info.Memory},
			{"客户端", a.info.Clients},
			{"系统", a.info.OS},
			{"运行", a.info.Uptime},
		})
	}
	for _, sec := range a.info.Sections {
		rows := make([]kv, 0, len(sec.Fields))
		for _, f := range sec.Fields {
			rows = append(rows, kv{f.Label, f.Value})
		}
		appendBlock(sec.Title, rows)
	}
	if !a.inDB() && len(a.databases) > 0 {
		rows := make([]kv, 0, len(a.databases))
		for _, db := range a.databases {
			detail := format.Size(db.Keys) + " keys"
			if db.Expires > 0 {
				detail += fmt.Sprintf(" · %s 过期", format.Size(db.Expires))
			}
			rows = append(rows, kv{fmt.Sprintf("db%d", db.Index), detail})
		}
		appendBlock("数据库", rows)
	}

	drawBlock := func(b block) {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, b.title).Bold()
			for _, r := range b.rows {
				ui.Row(c).Gap(12).Children(func() {
					ui.Text(c, r.k).Width(88).TextColor(t.TextMuted)
					ui.Text(c, r.v).Grow(1)
				})
			}
		})
	}

	ui.Grid(c).Columns(2).Gap(16).AlignItems(ui.Start).Children(func() {
		for _, b := range blocks {
			drawBlock(b)
		}
	})
	if len(a.slowlog) > 0 {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "慢日志").Bold()
			for _, e := range a.slowlog {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, e.Time.Format("15:04:05")).Width(64).TextColor(t.TextMuted)
					ui.Text(c, e.Duration.String()).Width(80).TextColor(t.TextMuted)
					ui.Text(c, e.Command).Grow(1).SingleLine()
				})
			}
		})
	}
	if a.inDB() && a.selected == "" {
		ui.Text(c, "选择一个 key 查看内容").FontSize(12).TextColor(t.TextMuted)
	}
}

func (a *app) stringEditor(c *ui.Context, t *ui.Theme) {
	mode := a.stringMode
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Segmented(c, &mode, "文本", "Hex", "JSON")
		if mode != a.stringMode {
			a.setStringMode(mode)
		}
		ui.Spacer(c)
		if ui.PrimaryButton(c, "保存").Clicked() {
			a.saveString()
		}
	})
	ui.TextArea(c, &a.stringEdit).Grow(1).Font("monospace")
}

func (a *app) streamEditor(c *ui.Context, t *ui.Theme) {
	a.addBar(c, t, "添加条目", func() {
		ui.TextInput(c, &a.newField).Placeholder("field").Width(140)
		if ui.TextInput(c, &a.newValue).Placeholder("value").Grow(1).Submitted() {
			a.addStreamEntry()
		}
		if ui.PrimaryButton(c, "添加").Clicked() {
			a.addStreamEntry()
		}
	})
	list := ui.List(c, nil, len(a.val.Stream), func(i int) {
		e := a.val.Stream[i]
		ui.Row(c.Key(e.ID)).Gap(8).Padding(4, 6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, e.ID).Width(180).SingleLine()
			var parts []string
			for _, f := range e.Fields {
				parts = append(parts, f.Name+"="+f.Value)
			}
			ui.Text(c, strings.Join(parts, " ")).Grow(1).SingleLine()
			if ui.Button(c.Key("xdel-"+e.ID), "删除").Clicked() {
				a.deleteStreamEntry(e.ID)
			}
		})
	}).Grow(1).Border(1, t.Border).Radius(8)
	clearOnBlank(list, a.clearMemberFocus)
	list.Children(func() {
		if len(a.val.Stream) == 0 {
			ui.Text(c, "空 Stream").TextColor(t.TextMuted).Padding(12)
		}
	})
}

func (a *app) hashEditor(c *ui.Context, t *ui.Theme) {
	a.addBar(c, t, "添加 field", func() {
		ui.TextInput(c, &a.newField).Placeholder("field").Width(140)
		if ui.TextInput(c, &a.newValue).Placeholder("value").Grow(1).Submitted() {
			a.addHashField()
		}
		if ui.PrimaryButton(c, "添加").Clicked() {
			a.addHashField()
		}
	})
	a.syncMemberEditsIfNeeded()
	a.hashList.Key = func(i int) any { return i }
	a.hashList.Label = func(i int) string { return a.val.Hash[i].Name }
	a.hashList.Selected = &a.hashRow
	list := ui.List(c, &a.hashList, len(a.val.Hash), func(i int) {
		f := &a.val.Hash[i]
		orig := f.Name
		if i < len(a.hashOrig) {
			orig = a.hashOrig[i]
		}
		ui.Row(c.Key(i)).Gap(8).Padding(4, 6).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &f.Name).Width(140).Placeholder("field")
			ui.TextInput(c, &f.Value).Grow(1).Placeholder("value")
			if ui.Button(c.Key("hsave-"+orig), "保存").Clicked() {
				a.saveHashRow(i)
			}
			if ui.Button(c.Key("hdel-"+orig), "删除").Clicked() {
				a.deleteHashField(orig)
			}
		})
	}).Grow(1).Border(1, t.Border).Radius(8)
	clearOnBlank(list, a.clearMemberFocus)
	list.Children(func() {
		if len(a.val.Hash) == 0 {
			ui.Text(c, "没有 field").TextColor(t.TextMuted).Padding(12)
		}
	})
}

func (a *app) listEditor(c *ui.Context, t *ui.Theme) {
	a.addBar(c, t, "添加元素", func() {
		if ui.TextInput(c, &a.newValue).Placeholder("value").Grow(1).Submitted() {
			a.pushList(false)
		}
		if ui.Button(c, "左侧添加").Clicked() {
			a.pushList(true)
		}
		if ui.PrimaryButton(c, "右侧添加").Clicked() {
			a.pushList(false)
		}
		if ui.Button(c, "上方插入").Disabled(len(a.val.List) == 0).Clicked() {
			a.insertList(true)
		}
		if ui.Button(c, "下方插入").Disabled(len(a.val.List) == 0).Clicked() {
			a.insertList(false)
		}
	})
	a.listList.Key = func(i int) any { return i }
	a.listList.Label = func(i int) string { return a.val.List[i] }
	a.listList.Selected = &a.listRow
	list := ui.List(c, &a.listList, len(a.val.List), func(i int) {
		ui.Row(c.Key(i)).Gap(8).Padding(4, 6).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "%d", i).Width(36).TextColor(t.TextMuted)
			ui.TextInput(c, &a.val.List[i]).Grow(1)
			if ui.Button(c.Key(fmt.Sprintf("lsave-%d", i)), "保存").Clicked() {
				a.setListItem(int64(i), a.val.List[i])
			}
			if ui.Button(c.Key(fmt.Sprintf("ldel-%d", i)), "删除").Clicked() {
				a.deleteListItem(int64(i))
			}
		})
	}).Grow(1).Border(1, t.Border).Radius(8)
	clearOnBlank(list, a.clearMemberFocus)
	list.Children(func() {
		if len(a.val.List) == 0 {
			ui.Text(c, "空列表").TextColor(t.TextMuted).Padding(12)
		}
	})
}

func (a *app) setEditor(c *ui.Context, t *ui.Theme) {
	a.addBar(c, t, "添加 member", func() {
		if ui.TextInput(c, &a.newMember).Placeholder("member").Grow(1).Submitted() {
			a.addSetMember()
		}
		if ui.PrimaryButton(c, "添加").Clicked() {
			a.addSetMember()
		}
	})
	a.syncMemberEditsIfNeeded()
	a.setList.Key = func(i int) any { return i }
	a.setList.Label = func(i int) string { return a.val.Set[i] }
	a.setList.Selected = &a.setRow
	list := ui.List(c, &a.setList, len(a.val.Set), func(i int) {
		orig := a.val.Set[i]
		if i < len(a.setOrig) {
			orig = a.setOrig[i]
		}
		ui.Row(c.Key(i)).Gap(8).Padding(4, 6).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &a.val.Set[i]).Grow(1).Placeholder("member")
			if ui.Button(c.Key("ssave-"+orig), "保存").Clicked() {
				a.saveSetRow(i)
			}
			if ui.Button(c.Key("sdel-"+orig), "删除").Clicked() {
				a.deleteSetMember(orig)
			}
		})
	}).Grow(1).Border(1, t.Border).Radius(8)
	clearOnBlank(list, a.clearMemberFocus)
	list.Children(func() {
		if len(a.val.Set) == 0 {
			ui.Text(c, "空集合").TextColor(t.TextMuted).Padding(12)
		}
	})
}

func (a *app) zsetEditor(c *ui.Context, t *ui.Theme) {
	a.addBar(c, t, "添加 member", func() {
		ui.TextInput(c, &a.newScore).Placeholder("score").Width(80)
		if ui.TextInput(c, &a.newMember).Placeholder("member").Grow(1).Submitted() {
			a.addZMember()
		}
		if ui.PrimaryButton(c, "添加").Clicked() {
			a.addZMember()
		}
	})
	a.syncMemberEditsIfNeeded()
	a.zsetList.Key = func(i int) any { return i }
	a.zsetList.Label = func(i int) string { return a.val.ZSet[i].Member }
	a.zsetList.Selected = &a.zsetRow
	list := ui.List(c, &a.zsetList, len(a.val.ZSet), func(i int) {
		orig := a.val.ZSet[i].Member
		if i < len(a.zsetOrig) {
			orig = a.zsetOrig[i]
		}
		ui.Row(c.Key(i)).Gap(8).Padding(4, 6).AlignItems(ui.Center).Children(func() {
			if i < len(a.zsetScoreEdits) {
				ui.TextInput(c, &a.zsetScoreEdits[i]).Width(80).Placeholder("score")
			}
			ui.TextInput(c, &a.val.ZSet[i].Member).Grow(1).Placeholder("member")
			if ui.Button(c.Key("zsave-"+orig), "保存").Clicked() {
				a.saveZSetRow(i)
			}
			if ui.Button(c.Key("zdel-"+orig), "删除").Clicked() {
				a.deleteZMember(orig)
			}
		})
	}).Grow(1).Border(1, t.Border).Radius(8)
	clearOnBlank(list, a.clearMemberFocus)
	list.Children(func() {
		if len(a.val.ZSet) == 0 {
			ui.Text(c, "空有序集合").TextColor(t.TextMuted).Padding(12)
		}
	})
}

func (a *app) syncMemberEditsIfNeeded() {
	if len(a.hashOrig) != len(a.val.Hash) || len(a.setOrig) != len(a.val.Set) || len(a.zsetOrig) != len(a.val.ZSet) || len(a.zsetScoreEdits) != len(a.val.ZSet) {
		a.syncMemberEdits()
	}
}

func (a *app) addBar(c *ui.Context, t *ui.Theme, label string, fields func()) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
		fields()
	})
}
