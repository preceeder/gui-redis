package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"redis/internal/format"
)

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Split(c, &a.sidebarW, func() {
			a.connPane(c, t)
		}, func() {
			ui.SplitPercent(c, &a.keysPct, func() {
				a.keysPane(c, t)
			}, func() {
				if a.showCLI {
					ui.SplitVerticalPercent(c, &a.editorPct, func() {
						a.valuePane(c, t)
					}, func() {
						a.cliPane(c, t)
					}).Fill()
				} else {
					a.valuePane(c, t)
				}
			}).Fill()
		}).Grow(1).AlignSelf(ui.Stretch)
		ui.Divider(c)
		a.statusBar(c, t)
	})
	a.connDialog(c, t)
	a.addKeyDialog(c, t)
	a.renameDialog(c, t)
	a.ttlDialog(c, t)
	a.confirmDialog(c)
	a.exportProgress(c, t)
	a.exportDoneDialog(c)
	a.importProgress(c, t)
	a.importDoneDialog(c)
	a.syncDialog(c, t)
	a.syncProgress(c, t)
	a.syncDoneDialog(c)
}

func (a *app) statusBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Padding(6, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		if a.connecting || a.loadingKeys || a.loadingVal || a.loadingDBs || a.syncing {
			ui.Spinner(c)
		}
		dot := t.TextMuted
		switch {
		case a.connected():
			dot = ui.Hex("#22c55e")
		case a.connecting:
			dot = t.Accent
		case strings.HasPrefix(a.status, "连接失败"):
			dot = ui.Hex("#ef4444")
		}
		ui.Box(c).Size(8, 8).Radius(4).Background(dot)
		ui.Text(c, a.status).FontSize(12).SingleLine().Grow(1)
		if a.exporting || a.importing || a.syncing {
			done, total, label := a.exportDone, a.exportTotal, "导出"
			if a.importing {
				done, total, label = a.importDone, a.importTotal, "导入"
			}
			if a.syncing {
				done, total, label = a.syncDone, a.syncTotal, "同步"
			}
			if total > 0 {
				ui.Progress(c, float64(done)/float64(total)).Width(140).Label(label)
			} else {
				ui.Progress(c, -1).Width(140).Label(label)
			}
		}
		if a.val.Key != "" {
			ui.Text(c, a.val.Type+" · TTL "+format.TTL(a.val.TTL)+" · "+format.Size(a.val.Len)).
				FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (a *app) confirmDialog(c *ui.Context) {
	if a.confirm == confirmNone && !a.confirmOpen {
		return
	}
	title, msg, action := "确认", "", "确定"
	switch a.confirm {
	case confirmDeleteKey:
		title, msg, action = "删除 Key", "确定删除 “"+a.confirmID+"”？不可撤销。", "删除"
	case confirmDeleteKeys:
		n := strings.Count(a.confirmID, "\n") + 1
		title, msg, action = "批量删除", fmt.Sprintf("确定删除 %d 个 key？不可撤销。", n), "删除"
	case confirmDeletePrefix:
		n := a.tree.DescendantKeys(a.confirmID)
		title, msg, action = "删除前缀", fmt.Sprintf("确定删除前缀 “%s” 下的全部 key（当前 %d 个）？不可撤销。", a.confirmID, n), "删除"
	case confirmDeleteConn:
		title, msg, action = "删除连接", "确定删除连接 “"+a.confirmID+"”？", "删除"
	case confirmDangerCLI:
		title, msg, action = "危险命令", "即将执行：\n"+a.confirmID+"\n\n此命令可能影响线上数据。", "执行"
	default:
		return
	}
	switch ui.AlertDialog(c, &a.confirmOpen, title, msg, "取消", action) {
	case 1:
		switch a.confirm {
		case confirmDeleteKey, confirmDeleteKeys:
			a.deleteSelectedKey()
		case confirmDeletePrefix:
			a.deletePrefix(a.confirmID)
		case confirmDeleteConn:
			a.deleteSelectedConn()
		case confirmDangerCLI:
			a.execCLI(a.pendingCLI, a.confirmID)
		}
		a.confirm = confirmNone
	case 0, ui.AlertDismissed:
		a.confirm = confirmNone
		a.pendingCLI = nil
	}
}

func (a *app) renameDialog(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &a.showRename, func() {
		ui.Column(c).Width(360).Gap(12).Children(func() {
			ui.Text(c, "重命名 Key").Bold()
			ui.Text(c, a.selected).FontSize(12).TextColor(t.TextMuted)
			if ui.TextInput(c, &a.renameEdit).Placeholder("新名称").AutoFocus().Submitted() {
				a.renameSelected()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.showRename = false
				}
				if ui.PrimaryButton(c, "重命名").Clicked() {
					a.renameSelected()
				}
			})
		})
	})
}

func (a *app) ttlDialog(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &a.showTTL, func() {
		ui.Column(c).Width(360).Gap(12).Children(func() {
			ui.Text(c, "设置 TTL").Bold()
			ui.Text(c, "单位秒，留空表示永久").FontSize(12).TextColor(t.TextMuted)
			if ui.TextInput(c, &a.ttlEdit).Placeholder("秒").AutoFocus().Submitted() {
				a.applyTTL()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.showTTL = false
				}
				if ui.PrimaryButton(c, "保存").Clicked() {
					a.applyTTL()
				}
			})
		})
	})
}

func (a *app) exportProgress(c *ui.Context, t *ui.Theme) {
	if !a.exporting {
		return
	}
	if ui.Modal(c, &a.exporting, func() {
		ui.Column(c).Width(360).Gap(12).Children(func() {
			ui.Text(c, "正在导出").Bold()
			if a.exportTotal > 0 {
				ui.Progress(c, float64(a.exportDone)/float64(a.exportTotal)).Label("导出进度")
				ui.Text(c, fmt.Sprintf("%d / %d", a.exportDone, a.exportTotal)).
					FontSize(12).TextColor(t.TextMuted)
			} else {
				ui.Progress(c, -1).Label("正在扫描")
				ui.Text(c, "正在扫描 key…").FontSize(12).TextColor(t.TextMuted)
			}
		})
	}).Dismissed() {
		a.exporting = true
	}
}

func (a *app) importProgress(c *ui.Context, t *ui.Theme) {
	if !a.importing {
		return
	}
	if ui.Modal(c, &a.importing, func() {
		ui.Column(c).Width(360).Gap(12).Children(func() {
			ui.Text(c, "正在导入").Bold()
			if a.importTotal > 0 {
				ui.Progress(c, float64(a.importDone)/float64(a.importTotal)).Label("导入进度")
				ui.Text(c, fmt.Sprintf("%d / %d", a.importDone, a.importTotal)).
					FontSize(12).TextColor(t.TextMuted)
			} else {
				ui.Progress(c, -1).Label("正在读取")
				ui.Text(c, "正在读取文件…").FontSize(12).TextColor(t.TextMuted)
			}
		})
	}).Dismissed() {
		a.importing = true
	}
}

func (a *app) importDoneDialog(c *ui.Context) {
	if !a.showImportDone {
		return
	}
	ui.AlertDialog(c, &a.showImportDone, "导入完成", importDoneMessage(a.importResult), "完成")
}

func (a *app) exportDoneDialog(c *ui.Context) {
	if !a.showExportDone {
		return
	}
	msg := fmt.Sprintf("已导出 %d 个 key。\n是否打开所在目录？", a.exportCount)
	if a.exportPath != "" {
		msg += "\n\n" + a.exportPath
	}
	switch ui.AlertDialog(c, &a.showExportDone, "导出完成", msg, "完成", "打开目录") {
	case 1:
		a.revealExport()
	}
}

func clearOnBlank(e ui.Element, clear func()) {
	if e.Clicked() {
		clear()
	}
}

