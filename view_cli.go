package main

import (
	"github.com/egoist/mygo/ui"

	"redis/internal/cliparse"
)

func (a *app) cliPane(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Padding(8, 12).Gap(6).Background(t.Surface).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "CLI").Bold()
			ui.Text(c, "输入时提示语法 · Enter 执行 · 危险命令会二次确认").FontSize(12).TextColor(t.TextMuted).Grow(1)
			if ui.Button(c, "关闭").Clicked() {
				a.showCLI = false
			}
		})
		ui.Scroll(c).Grow(1).Padding(8).Border(1, t.Border).Radius(8).Background(t.Background).Children(func() {
			if a.cliOutput == "" {
				ui.Text(c, "输入命令可补全，例如 GET user:1 或 HSET user:1 name Ada").FontSize(12).TextColor(t.TextMuted)
			} else {
				ui.Text(c, a.cliOutput).Font("monospace").FontSize(12).Selectable()
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			in := ui.Autocomplete(c, &a.cliInput, cliparse.Suggestions()).
				Placeholder("GET key").Grow(1).Font("monospace")
			if in.Submitted() {
				a.submitCLI()
			}
			if ui.Button(c, "↑").Disabled(len(a.cliHist) == 0).Clicked() {
				a.cliHistory(-1)
			}
			if ui.Button(c, "↓").Disabled(len(a.cliHist) == 0).Clicked() {
				a.cliHistory(1)
			}
			if ui.PrimaryButton(c, "执行").Disabled(!a.connected()).Clicked() {
				a.submitCLI()
			}
		})
		if h, ok := cliparse.Lookup(a.cliInput); ok {
			ui.Text(c, cliparse.FormatHint(h)).FontSize(12).TextColor(t.TextMuted).SingleLine()
		} else {
			ui.Text(c, "常用：GET / SET / HSET / LRANGE / SADD / ZADD / SCAN / TTL").
				FontSize(12).TextColor(t.TextMuted).SingleLine()
		}
	})
}
