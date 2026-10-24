package main

import (
	"context"
	"fmt"
	"time"

	"redis/internal/redisx"
)

type live struct {
	client    *redisx.Client
	databases []redisx.Database
	activeDB  int
	info      redisx.Info
}

func (a *app) isLive(id string) bool {
	if id == "" {
		return false
	}
	if a.connectedID == id && a.client != nil {
		return true
	}
	return a.parked[id] != nil
}

func (a *app) databasesOf(id string) []redisx.Database {
	if a.connectedID == id {
		return a.databases
	}
	if l := a.parked[id]; l != nil {
		return l.databases
	}
	return nil
}

func (a *app) clearBrowse() {
	a.databases = nil
	a.activeDB = -1
	a.keys = nil
	a.hasMore = false
	a.scanCursor = 0
	a.selected = ""
	a.keyRow = -1
	a.val = redisx.Value{}
	a.stringEdit = ""
	a.info = redisx.Info{}
	a.showAddKey = false
	a.slowlog = nil
	a.keySel.Clear()
	a.rebuildTree()
}

func (a *app) parkFocus() {
	if a.client == nil || a.connectedID == "" {
		return
	}
	if a.parked == nil {
		a.parked = map[string]*live{}
	}
	a.parked[a.connectedID] = &live{
		client:    a.client,
		databases: a.databases,
		activeDB:  a.activeDB,
		info:      a.info,
	}
	a.client = nil
	a.connectedID = ""
	a.clearBrowse()
}

func (a *app) adoptConnection(id string) bool {
	if id == "" || a.connectedID == id {
		return a.connectedID == id
	}
	a.parkFocus()
	l := a.parked[id]
	if l == nil {
		return false
	}
	delete(a.parked, id)
	a.client = l.client
	a.connectedID = id
	a.databases = l.databases
	a.activeDB = l.activeDB
	a.info = l.info
	a.lastConnID = id
	a.connOutline.Open.Add(id)
	a.persist()
	return true
}

func (a *app) focusConnection(id string) {
	if !a.adoptConnection(id) {
		return
	}
	if a.activeDB >= 0 {
		a.refreshKeys(true)
		return
	}
	a.refreshServerInfo()
	a.refreshDatabases()
}

func (a *app) openConnDatabase(connID string, db int) {
	if !a.isLive(connID) {
		return
	}
	if a.connectedID != connID && !a.adoptConnection(connID) {
		return
	}
	a.openDatabase(db)
}

func (a *app) disconnectID(id string) {
	if id == "" {
		return
	}
	if a.connectedID == id {
		a.disconnect()
		return
	}
	if l := a.parked[id]; l != nil {
		if l.client != nil {
			l.client.Close()
		}
		delete(a.parked, id)
	}
}

func (a *app) startWatchdog() {
	go func() {
		tick := time.NewTicker(8 * time.Second)
		defer tick.Stop()
		for range tick.C {
			a.publish(func() { a.watchConnections() })
		}
	}()
}

func (a *app) watchConnections() {
	type pair struct {
		id  string
		cli *redisx.Client
	}
	var list []pair
	if a.client != nil && a.connectedID != "" {
		list = append(list, pair{a.connectedID, a.client})
	}
	for id, l := range a.parked {
		if l != nil && l.client != nil {
			list = append(list, pair{id, l.client})
		}
	}
	for _, p := range list {
		id, cli := p.id, p.cli
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := cli.Ping(ctx); err != nil {
				a.publish(func() { a.reconnect(id) })
			}
		}()
	}
}

func (a *app) reconnect(id string) {
	c, ok := a.connByID(id)
	if !ok || a.connecting {
		return
	}
	focused := a.connectedID == id
	go func() {
		ctx, cancel := timeout(a.ctx)
		defer cancel()
		cli, err := redisx.Dial(ctx, a.redisConfig(c, 0))
		if err != nil {
			a.publish(func() {
				if focused {
					a.status = "连接断开，正在重试: " + err.Error()
				}
			})
			return
		}
		snap, _ := cli.Snapshot(ctx)
		dbs, _ := cli.Databases(ctx)
		db := -1
		if focused {
			db = a.activeDB
		} else if l := a.parked[id]; l != nil {
			db = l.activeDB
		}
		if db < 0 && c.LastDB != nil {
			db = *c.LastDB
		}
		if db >= 0 {
			_ = cli.SwitchDB(ctx, db)
		}
		a.publish(func() {
			if focused && a.connectedID == id {
				if a.client != nil {
					a.client.Close()
				}
				a.client = cli
				a.databases = dbs
				a.info = snap
				a.activeDB = db
				a.status = fmt.Sprintf("已重连 %s", c.Name)
				if db >= 0 {
					a.refreshKeys(true)
				}
				return
			}
			if old := a.parked[id]; old != nil && old.client != nil {
				old.client.Close()
			}
			if a.parked == nil {
				a.parked = map[string]*live{}
			}
			a.parked[id] = &live{client: cli, databases: dbs, activeDB: db, info: snap}
		})
	}()
}
