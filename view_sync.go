package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"redis/internal/redisx"
)

func (a *app) syncDialog(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &a.showSync, func() {
		ui.Column(c).Width(460).Gap(12).Children(func() {
			ui.Text(c, "同步数据").Bold()
			ui.Form(c, func() {
				ui.Field(c, "源", func() {
					ui.Text(c, a.syncSourceLabel())
				})
				ui.Field(c, "范围", func() {
					ui.Segmented(c, &a.syncMode, "全部", "前缀", "指定 Key")
				})
				if a.syncMode == 1 {
					ui.Field(c, "前缀", func() {
						ui.TextInput(c, &a.syncPrefix).Placeholder("user")
					})
				}
				if a.syncMode == 2 {
					ui.Field(c, "Key", func() {
						ui.TextArea(c, &a.syncKeys).Placeholder("一行一个 key").Height(120)
					})
				}
				ui.Field(c, "目标连接", func() {
					labels := a.connSelectLabels()
					choice := a.connLabel(a.syncDestConn)
					prev := choice
					ui.Select(c, &choice, labels)
					if choice != prev {
						a.syncDestConn = a.connIDForLabel(choice)
						opts := a.syncDBLabels(a.syncDestConn)
						if !slices.Contains(opts, a.syncDestDB) && len(opts) > 0 {
							a.syncDestDB = opts[0]
						}
					}
				})
				ui.Field(c, "目标库", func() {
					ui.Select(c, &a.syncDestDB, a.syncDBLabels(a.syncDestConn))
				})
			})
			ui.Text(c, "同名 key 会被覆盖，并保留 TTL。").FontSize(12).TextColor(t.TextMuted)
			if a.syncErr != "" {
				ui.Text(c, a.syncErr).FontSize(12).TextColor(ui.Hex("#ef4444"))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.showSync = false
				}
				if ui.PrimaryButton(c, "开始同步").Clicked() {
					a.startSync()
				}
			})
		})
	})
}

func (a *app) syncProgress(c *ui.Context, t *ui.Theme) {
	if !a.syncing {
		return
	}
	if ui.Modal(c, &a.syncing, func() {
		ui.Column(c).Width(360).Gap(12).Children(func() {
			ui.Text(c, "正在同步").Bold()
			if a.syncTotal > 0 {
				ui.Progress(c, float64(a.syncDone)/float64(a.syncTotal)).Label("同步进度")
				ui.Text(c, fmt.Sprintf("%d / %d", a.syncDone, a.syncTotal)).
					FontSize(12).TextColor(t.TextMuted)
			} else {
				ui.Progress(c, -1).Label("正在列出")
				ui.Text(c, "正在列出 key…").FontSize(12).TextColor(t.TextMuted)
			}
		})
	}).Dismissed() {
		a.syncing = true
	}
}

func (a *app) syncDoneDialog(c *ui.Context) {
	if !a.showSyncDone {
		return
	}
	ui.AlertDialog(c, &a.showSyncDone, "同步完成", syncDoneMessage(a.syncResult), "完成")
}

func syncDoneMessage(res redisx.ImportResult) string {
	return strings.Replace(importDoneMessage(res), "已导入", "已同步", 1)
}
