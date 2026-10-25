package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	a := newApp()
	mygo.App.SetName("RedisCli")
	mygo.App.SetMenu(appMenu())
	mygo.App.WhenReady(func() {
		a.loadStore()
		a.startWatchdog()
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "RedisCli",
			Width:     1180,
			Height:    760,
			MinWidth:  880,
			MinHeight: 560,
			StateKey:  "main",
			Content:   ui.View(a.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
