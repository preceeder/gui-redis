package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"redis/internal/cliparse"
	"redis/internal/format"
	"redis/internal/redisx"
)

var typeFilters = []string{"全部", "string", "hash", "list", "set", "zset", "stream"}
var keyTypes = []string{"string", "hash", "list", "set", "zset", "stream"}

func (a *app) addKeyMenu(m *ui.Menu) {
	if m.Item("添加 Key").Chosen() {
		a.openAddKey()
	}
	if m.Item("导入").Chosen() {
		a.importCurrentDB()
	}
	if m.Item("同步").Chosen() {
		a.openSync(a.connectedID, a.activeDB, 0, "", nil)
	}
}

func (a *app) keysPane(c *ui.Context, t *ui.Theme) {
	if !a.connected() {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Text(c, "双击左侧连接").TextColor(t.TextMuted)
		})
		return
	}
	if !a.inDB() {
		ui.Column(c).Fill().Center().Children(func() {
			ui.Text(c, "双击左侧数据库").TextColor(t.TextMuted)
		})
		return
	}
	pane := ui.Column(c).Fill().Padding(10).Gap(8)
	pane.ContextMenu(a.addKeyMenu)
	clearOnBlank(pane, a.clearKeyFocus)
	pane.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("db%d", a.activeDB)).Bold()
			ui.Badge(c, format.Size(int64(len(a.visibleKeys()))))
			ui.Spacer(c)
			if n := len(a.chosenKeyNames()); n > 1 {
				if ui.Button(c, fmt.Sprintf("删除 %d", n)).Clicked() {
					a.askDeleteKeys(a.chosenKeyNames())
				}
			}
			if ui.Button(c.Key("export-keys"), "导出").Clicked() {
				a.exportFromKeysPane()
			}
			if ui.Button(c.Key("import-keys"), "导入").Clicked() {
				a.importCurrentDB()
			}
			if ui.Button(c.Key("sync-keys"), "同步").Clicked() {
				if names := a.chosenKeyNames(); len(names) > 0 {
					a.openSync(a.connectedID, a.activeDB, 2, "", names)
				} else {
					a.openSync(a.connectedID, a.activeDB, 0, "", nil)
				}
			}
			ui.Segmented(c, &a.keyMode, "树", "列表")
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.SearchField(c, &a.keyPattern).Label("搜索 Key").Grow(1).Submitted() {
				a.refreshKeys(true)
			}
			prev := a.typeFilter
			ui.Select(c, &a.typeFilter, typeFilters).Width(110)
			if a.typeFilter != prev {
				a.rebuildTree()
			}
			ui.Text(c, "分隔符").FontSize(12).TextColor(t.TextMuted)
			prevSep := a.sep
			ui.TextInput(c, &a.sep).Width(36)
			if a.sep != prevSep {
				a.setSeparator(a.sep)
			}
		})
		if a.keyMode == 0 {
			a.keysTree(c, t)
		} else {
			a.keysTable(c, t)
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, matchHint(a.keyPattern)).FontSize(12).TextColor(t.TextMuted).Grow(1)
			if ui.Button(c, "加载更多").Disabled(!a.hasMore || a.loadingKeys).Clicked() {
				a.refreshKeys(false)
			}
		})
	})
}

func matchHint(q string) string {
	return "SCAN MATCH " + cliparse.ScanMatch(q)
}

func (a *app) keysTree(c *ui.Context, t *ui.Theme) {
	a.keysOutline.List.Selected = &a.keyRow
	a.keysOutline.List.Selection = &a.keySel
	a.keysOutline.List.RoundRows = true
	a.keysOutline.List.CellMenu = func(m *ui.Menu, row, _ int) {
		if row < 0 || row >= a.keysOutline.Rows() {
			return
		}
		a.fillNodeMenu(m, a.keysOutline.Item(row))
	}
	list := ui.OutlineTable(c, &a.keysOutline, []ui.TableColumn{{Title: "Key", MinWidth: 80}}, a.tree.Roots, a.tree.ChildrenOf, func(id string, _ int) {
		label := a.tree.Label[id]
		if label == "" {
			label = id
		}
		ui.Row(c.Key(id)).Gap(8).AlignItems(ui.Center).Grow(1).FillHeight().Children(func() {
			ui.Text(c, label).SingleLine().Grow(1)
			if info, ok := a.keyByName[id]; ok {
				ui.Text(c, info.Type).FontSize(11).TextColor(t.TextMuted).Width(52)
			} else {
				ui.Text(c, format.Size(int64(a.tree.DescendantKeys(id)))).
					FontSize(11).TextColor(t.TextMuted)
			}
		})
	}).Grow(1).Padding(4).Border(1, t.Border).Radius(8)
	clearOnBlank(list, a.clearKeyFocus)
	list.Children(func() {
		if len(a.tree.Roots) == 0 && !a.loadingKeys {
			empty := ui.Column(c).Padding(12).Gap(8)
			empty.ContextMenu(a.addKeyMenu)
			empty.Children(func() {
				ui.Text(c, "没有匹配的 key").TextColor(t.TextMuted)
				if ui.Button(c.Key("empty-add-key"), "添加 Key").Clicked() {
					a.openAddKey()
				}
				if ui.Button(c.Key("empty-import"), "导入").Clicked() {
					a.importCurrentDB()
				}
			})
		}
	})
	if a.keyRow >= 0 && a.keyRow < a.keysOutline.Rows() {
		a.selectNode(a.keysOutline.Item(a.keyRow))
	}
	if list.Submitted() && a.tree.IsKey[a.selected] {
		a.loadValue(a.selected)
	}
}

func (a *app) keysTable(c *ui.Context, t *ui.Theme) {
	rows := slices.Clone(a.visibleKeys())
	slices.SortStableFunc(rows, func(x, y redisx.KeyInfo) int {
		var r int
		switch a.keySort.Column {
		case "Type":
			r = cmp.Compare(x.Type, y.Type)
		case "TTL":
			r = cmp.Compare(x.TTL, y.TTL)
		case "Size":
			r = cmp.Compare(x.Len, y.Len)
		case "Mem":
			r = cmp.Compare(x.Memory, y.Memory)
		default:
			r = cmp.Compare(strings.ToLower(x.Key), strings.ToLower(y.Key))
		}
		if a.keySort.Descending {
			r = -r
		}
		return r
	})
	a.keysList.Key = func(i int) any { return rows[i].Key }
	a.keysList.Label = func(i int) string { return rows[i].Key }
	a.keysList.Selected = &a.keyRow
	a.keysList.Selection = &a.keySel
	a.keysList.Sort = &a.keySort
	a.keysList.CellMenu = func(m *ui.Menu, row, _ int) {
		if row < 0 || row >= len(rows) {
			return
		}
		a.fillNodeMenu(m, rows[row].Key)
	}
	cols := []ui.TableColumn{
		{Title: "Key", Sortable: true, MinWidth: 140},
		{Title: "Type", Width: 72, Sortable: true},
		{Title: "TTL", Width: 72, Sortable: true},
		{Title: "Size", Width: 56, Align: ui.End, Sortable: true},
		{Title: "Mem", Width: 64, Align: ui.End, Sortable: true},
	}
	table := ui.Table(c, &a.keysList, cols, len(rows), func(r, col int) {
		k := rows[r]
		switch col {
		case 0:
			ui.Text(c, k.Key).SingleLine()
		case 1:
			ui.Text(c, k.Type)
		case 2:
			ui.Text(c, format.TTL(k.TTL))
		case 3:
			ui.Text(c, format.Size(k.Len))
		case 4:
			ui.Text(c, format.Bytes(k.Memory))
		}
	}).Grow(1).Border(1, t.Border).Radius(8).Clip()
	clearOnBlank(table, a.clearKeyFocus)
	if a.keyRow >= 0 && a.keyRow < len(rows) {
		a.selectNode(rows[a.keyRow].Key)
	}
	if table.Submitted() && a.keyRow >= 0 && a.keyRow < len(rows) {
		a.loadValue(rows[a.keyRow].Key)
	}
}

func (a *app) fillNodeMenu(m *ui.Menu, id string) {
	if id == "" {
		return
	}
	if a.tree.IsKey[id] {
		if m.Item("复制 Key").Chosen() {
			a.copyText(id, "已复制 "+id)
		}
		if len(a.tree.Kids[id]) > 0 {
			if m.Item("导出前缀").Chosen() {
				a.exportPrefix(id)
			}
		}
		if m.Item("导出").Chosen() {
			a.exportKeys([]string{id})
		}
		if m.Item("同步").Chosen() {
			if names := a.chosenKeyNames(); len(names) > 1 {
				a.openSync(a.connectedID, a.activeDB, 2, "", names)
			} else {
				a.openSync(a.connectedID, a.activeDB, 2, "", []string{id})
			}
		}
		if m.Item("重命名").Chosen() {
			a.selected = id
			a.renameEdit = id
			a.showRename = true
		}
		if m.Item("设置 TTL").Chosen() {
			a.selected = id
			a.showTTL = true
		}
		if m.Item("刷新").Chosen() {
			a.loadValue(id)
		}
		m.Separator()
		if m.Item("删除").Chosen() {
			a.selected = id
			if names := a.chosenKeyNames(); len(names) > 1 {
				a.askDeleteKeys(names)
			} else {
				a.askConfirm(confirmDeleteKey, id)
			}
		}
		return
	}
	if m.Item("复制前缀").Chosen() {
		a.copyText(id, "已复制 "+id)
	}
	if m.Item("导出").Chosen() {
		a.exportPrefix(id)
	}
	if m.Item("同步").Chosen() {
		a.openSync(a.connectedID, a.activeDB, 1, id, nil)
	}
	if m.Item("刷新").Chosen() {
		a.refreshKeys(true)
	}
	m.Separator()
	if m.Item("删除").Chosen() {
		a.askConfirm(confirmDeletePrefix, id)
	}
}

func (a *app) addKeyDialog(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &a.showAddKey, func() {
		ui.Column(c).Width(420).Gap(12).Children(func() {
			ui.Text(c, "添加 Key").Bold()
			ui.Form(c, func() {
				ui.Field(c, "Key", func() {
					ui.TextInput(c, &a.newKeyName).Placeholder("user:1").AutoFocus()
				})
				ui.Field(c, "类型", func() {
					ui.Select(c, &a.newKeyType, keyTypes)
				})
				switch a.newKeyType {
				case "hash", "stream":
					ui.Field(c, "Field", func() {
						ui.TextInput(c, &a.newKeyField).Placeholder("name")
					})
					ui.Field(c, "值", func() {
						ui.TextInput(c, &a.newKeyValue).Placeholder("value")
					})
				case "set":
					ui.Field(c, "Member", func() {
						ui.TextInput(c, &a.newKeyMember).Placeholder("member")
					})
				case "zset":
					ui.Field(c, "Score", func() {
						ui.TextInput(c, &a.newKeyScore).Placeholder("0")
					})
					ui.Field(c, "Member", func() {
						ui.TextInput(c, &a.newKeyMember).Placeholder("member")
					})
				default:
					ui.Field(c, "值", func() {
						ui.TextInput(c, &a.newKeyValue).Placeholder("value")
					})
				}
				ui.Field(c, "TTL", func() {
					ui.TextInput(c, &a.newKeyTTL).Placeholder("秒，空则永久")
				})
			})
			if a.newKeyErr != "" {
				ui.Text(c, a.newKeyErr).FontSize(12).TextColor(ui.Hex("#ef4444"))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.showAddKey = false
				}
				if ui.PrimaryButton(c, "创建").Clicked() {
					a.saveAddKey()
				}
			})
		})
	})
}
