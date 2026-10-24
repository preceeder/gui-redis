package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"redis/internal/cliparse"
	"redis/internal/redisx"
)

func (a *app) openSync(connID string, db, mode int, prefix string, keys []string) {
	if connID == "" || db < 0 || !a.isLive(connID) {
		return
	}
	a.syncSrcConn = connID
	a.syncSrcDB = db
	a.syncMode = mode
	a.syncPrefix = prefix
	a.syncKeys = strings.Join(keys, "\n")
	a.syncErr = ""
	a.syncDestConn, a.syncDestDB = a.defaultSyncDest(connID, db)
	a.showSync = true
}

func (a *app) defaultSyncDest(srcConn string, srcDB int) (string, string) {
	destConn := srcConn
	for _, c := range a.connections {
		if c.ID != srcConn {
			destConn = c.ID
			break
		}
	}
	labels := a.syncDBLabels(destConn)
	destDB := "db0"
	if destConn == srcConn {
		alt := fmt.Sprintf("db%d", srcDB+1)
		if containsString(labels, alt) {
			destDB = alt
		} else if srcDB != 0 && containsString(labels, "db0") {
			destDB = "db0"
		} else if len(labels) > 0 {
			destDB = labels[0]
		}
	} else if len(labels) > 0 {
		destDB = labels[0]
	}
	return destConn, destDB
}

func (a *app) connSelectLabels() []string {
	out := make([]string, 0, len(a.connections))
	seen := map[string]int{}
	for _, c := range a.connections {
		seen[c.Name]++
	}
	for _, c := range a.connections {
		label := c.Name
		if seen[c.Name] > 1 {
			label = fmt.Sprintf("%s (%s:%d)", c.Name, c.Host, c.Port)
		}
		out = append(out, label)
	}
	return out
}

func (a *app) connLabel(id string) string {
	c, ok := a.connByID(id)
	if !ok {
		return id
	}
	seen := 0
	for _, x := range a.connections {
		if x.Name == c.Name {
			seen++
		}
	}
	if seen > 1 {
		return fmt.Sprintf("%s (%s:%d)", c.Name, c.Host, c.Port)
	}
	return c.Name
}

func (a *app) connIDForLabel(label string) string {
	for _, c := range a.connections {
		if a.connLabel(c.ID) == label {
			return c.ID
		}
	}
	return ""
}

func (a *app) syncDBLabels(connID string) []string {
	dbs := a.databasesOf(connID)
	if len(dbs) == 0 {
		out := make([]string, 16)
		for i := range out {
			out[i] = fmt.Sprintf("db%d", i)
		}
		return out
	}
	out := make([]string, 0, len(dbs))
	for _, d := range dbs {
		out = append(out, fmt.Sprintf("db%d", d.Index))
	}
	return out
}

func parseDBLabel(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "db") {
		s = s[2:]
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 0
}

func (a *app) validateSync() string {
	if a.syncSrcConn == "" || !a.isLive(a.syncSrcConn) {
		return "请先连接源数据库"
	}
	destID := a.syncDestConn
	if destID == "" {
		return "请选择目标连接"
	}
	if _, ok := a.connByID(destID); !ok {
		return "目标连接不存在"
	}
	destDB, ok := parseDBLabel(a.syncDestDB)
	if !ok {
		return "目标数据库无效"
	}
	if destID == a.syncSrcConn && destDB == a.syncSrcDB {
		return "目标和源不能是同一个库"
	}
	switch a.syncMode {
	case 1:
		if strings.TrimSpace(a.syncPrefix) == "" {
			return "请填写前缀"
		}
	case 2:
		if len(redisx.ParseKeyList(a.syncKeys)) == 0 {
			return "请填写要同步的 key，一行一个"
		}
	}
	return ""
}

func (a *app) startSync() {
	if err := a.validateSync(); err != "" {
		a.syncErr = err
		return
	}
	srcConn, srcDB := a.syncSrcConn, a.syncSrcDB
	mode, prefix, listed := a.syncMode, strings.TrimSpace(a.syncPrefix), a.syncKeys
	destConn, destDB := a.syncDestConn, a.syncSrcDB
	if n, ok := parseDBLabel(a.syncDestDB); ok {
		destDB = n
	}
	a.showSync = false
	a.syncErr = ""
	go a.runSync(srcConn, srcDB, destConn, destDB, mode, prefix, listed)
}

func (a *app) runSync(srcConn string, srcDB int, destConn string, destDB, mode int, prefix, listed string) {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	defer cancel()
	a.beginSync()
	src, srcClose, err := a.clientFor(ctx, srcConn, srcDB)
	if err != nil {
		a.failSync("源连接失败: " + err.Error())
		return
	}
	defer srcClose()
	dest, destClose, err := a.clientFor(ctx, destConn, destDB)
	if err != nil {
		a.failSync("目标连接失败: " + err.Error())
		return
	}
	defer destClose()
	keys, err := a.resolveSyncKeys(ctx, src, mode, prefix, listed)
	if err != nil {
		a.failSync("列出 key 失败: " + err.Error())
		return
	}
	if len(keys) == 0 {
		a.failSync("没有可同步的 key")
		return
	}
	a.publish(func() {
		a.syncTotal = len(keys)
		a.status = fmt.Sprintf("正在同步 0 / %d", len(keys))
		a.setWindowProgress(0, len(keys))
	})
	last := time.Time{}
	res, err := src.SyncTo(ctx, dest, keys, func(done, total int) {
		now := time.Now()
		if done < total && !last.IsZero() && now.Sub(last) < 80*time.Millisecond {
			return
		}
		last = now
		a.publish(func() {
			a.syncDone = done
			a.syncTotal = total
			a.status = fmt.Sprintf("正在同步 %d / %d", done, total)
			a.setWindowProgress(done, total)
		})
	})
	if err != nil && res.OK == 0 {
		a.failSync("同步失败: " + err.Error())
		return
	}
	msg := importStatus(res)
	msg = strings.Replace(msg, "已导入", "已同步", 1)
	a.publish(func() {
		a.clearSyncProgress()
		a.syncResult = res
		a.showSyncDone = true
		a.status = msg
		a.refreshAfterWrite()
	})
}

func (a *app) resolveSyncKeys(ctx context.Context, cli *redisx.Client, mode int, prefix, listed string) ([]string, error) {
	switch mode {
	case 1:
		return a.scanPrefixKeys(ctx, cli, prefix)
	case 2:
		return redisx.ParseKeyList(listed), nil
	default:
		return cli.ScanAll(ctx, cliparse.ScanMatch(""))
	}
}

func (a *app) clientFor(ctx context.Context, connID string, db int) (*redisx.Client, func(), error) {
	if a.connectedID == connID && a.client != nil && a.client.Ready() && a.activeDB == db {
		return a.client, func() {}, nil
	}
	if l := a.parked[connID]; l != nil && l.client != nil && l.client.Ready() && l.activeDB == db {
		return l.client, func() {}, nil
	}
	c, ok := a.connByID(connID)
	if !ok {
		return nil, func() {}, fmt.Errorf("连接不存在")
	}
	cli, err := redisx.Dial(ctx, a.redisConfig(c, db))
	if err != nil {
		return nil, func() {}, err
	}
	return cli, func() { cli.Close() }, nil
}

func (a *app) beginSync() {
	a.publish(func() {
		a.syncing = true
		a.syncDone = 0
		a.syncTotal = 0
		a.showSyncDone = false
		a.status = "正在同步…"
		a.setWindowProgress(0, 0)
	})
}

func (a *app) failSync(msg string) {
	a.publish(func() {
		a.clearSyncProgress()
		a.status = msg
	})
}

func (a *app) clearSyncProgress() {
	a.syncing = false
	a.syncDone = 0
	a.syncTotal = 0
	if a.win != nil {
		a.win.SetProgressBar(mygo.ProgressBar{})
	}
}

func (a *app) syncSourceLabel() string {
	name := a.syncSrcConn
	if c, ok := a.connByID(a.syncSrcConn); ok {
		name = c.Name
	}
	return fmt.Sprintf("%s / db%d", name, a.syncSrcDB)
}
