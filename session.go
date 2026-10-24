package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"

	"redis/internal/cliparse"
	"redis/internal/format"
	"redis/internal/redisx"
	"redis/internal/store"
)

func timeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 15*time.Second)
}

func bump(g *atomic.Uint64) uint64 { return g.Add(1) }

func stale(g *atomic.Uint64, n uint64) bool { return g.Load() != n }

func (a *app) openNewConn() {
	a.editingID = ""
	a.formName = ""
	a.formHost = "127.0.0.1"
	a.formPort = "6379"
	a.formUser = ""
	a.formPass = ""
	a.formSSH = false
	a.formSSHHost = ""
	a.formSSHPort = "22"
	a.formSSHUser = ""
	a.formSSHAuth = 0
	a.formSSHPass = ""
	a.formSSHKey = ""
	a.formSSHKeyPEM = ""
	a.formSSHPhrase = ""
	a.formProxy = false
	a.formProxyType = 0
	a.formProxyHost = "127.0.0.1"
	a.formProxyPort = "7890"
	a.formProxyUser = ""
	a.formProxyPass = ""
	a.formShowPass = false
	a.formShowSSHPass = false
	a.formShowSSHPhrase = false
	a.formShowProxyPass = false
	a.formError = ""
	a.formHint = ""
	a.showConnForm = true
}

func (a *app) openEditConn() {
	c, ok := a.selectedConn()
	if !ok {
		return
	}
	a.editingID = c.ID
	a.formName = c.Name
	a.formHost = c.Host
	a.formPort = strconv.Itoa(c.Port)
	a.formUser = c.Username
	a.formPass = c.Password
	a.formSSH = c.SSH
	a.formSSHHost = c.SSHHost
	a.formSSHPort = strconv.Itoa(c.SSHPort)
	if c.SSHPort <= 0 {
		a.formSSHPort = "22"
	}
	a.formSSHUser = c.SSHUser
	a.formSSHAuth = 0
	if c.SSHAuth == "key" {
		a.formSSHAuth = 1
	}
	a.formSSHPass = c.SSHPassword
	a.formSSHKey = c.SSHKeyPath
	a.formSSHKeyPEM = c.SSHKey
	a.formSSHPhrase = c.SSHPassphrase
	a.formProxy = c.Proxy
	a.formProxyType = 0
	if c.ProxyType == "http" {
		a.formProxyType = 1
	}
	a.formProxyHost = c.ProxyHost
	if a.formProxyHost == "" {
		a.formProxyHost = "127.0.0.1"
	}
	a.formProxyPort = strconv.Itoa(c.ProxyPort)
	if c.ProxyPort <= 0 {
		a.formProxyPort = "7890"
	}
	a.formProxyUser = c.ProxyUser
	a.formProxyPass = c.ProxyPassword
	a.formShowPass = false
	a.formShowSSHPass = false
	a.formShowSSHPhrase = false
	a.formShowProxyPass = false
	a.formError = ""
	a.formHint = ""
	a.showConnForm = true
}

func (a *app) formConnection() (store.Connection, string) {
	name := strings.TrimSpace(a.formName)
	host := strings.TrimSpace(a.formHost)
	if name == "" {
		return store.Connection{}, "名称不能为空"
	}
	if host == "" {
		return store.Connection{}, "主机不能为空"
	}
	port, err := strconv.Atoi(strings.TrimSpace(a.formPort))
	if err != nil || port < 1 || port > 65535 {
		return store.Connection{}, "端口无效"
	}
	id := a.editingID
	if id == "" {
		id = newID()
	}
	c := store.Connection{
		ID:       id,
		Name:     name,
		Host:     host,
		Port:     port,
		Username: strings.TrimSpace(a.formUser),
		Password: a.formPass,
	}
	if a.formSSH {
		sshHost := strings.TrimSpace(a.formSSHHost)
		if sshHost == "" {
			return store.Connection{}, "SSH 主机不能为空"
		}
		sshPort, err := strconv.Atoi(strings.TrimSpace(a.formSSHPort))
		if a.formSSHPort == "" {
			sshPort, err = 22, nil
		}
		if err != nil || sshPort < 1 || sshPort > 65535 {
			return store.Connection{}, "SSH 端口无效"
		}
		sshUser := strings.TrimSpace(a.formSSHUser)
		if sshUser == "" {
			return store.Connection{}, "SSH 用户不能为空"
		}
		auth := "password"
		if a.formSSHAuth == 1 {
			auth = "key"
			path, pem, errMsg := a.resolveSSHKey()
			if errMsg != "" {
				return store.Connection{}, errMsg
			}
			c.SSHKeyPath = path
			c.SSHKey = pem
			c.SSHPassphrase = a.formSSHPhrase
		} else if strings.TrimSpace(a.formSSHPass) == "" {
			return store.Connection{}, "SSH 密码不能为空"
		} else {
			c.SSHPassword = a.formSSHPass
		}
		c.SSH = true
		c.SSHHost = sshHost
		c.SSHPort = sshPort
		c.SSHUser = sshUser
		c.SSHAuth = auth
	}
	if a.formProxy {
		proxyHost := strings.TrimSpace(a.formProxyHost)
		if proxyHost == "" {
			return store.Connection{}, "代理主机不能为空"
		}
		proxyPort, err := strconv.Atoi(strings.TrimSpace(a.formProxyPort))
		if a.formProxyPort == "" {
			proxyPort, err = 7890, nil
		}
		if err != nil || proxyPort < 1 || proxyPort > 65535 {
			return store.Connection{}, "代理端口无效"
		}
		kind := "socks5"
		if a.formProxyType == 1 {
			kind = "http"
		}
		c.Proxy = true
		c.ProxyType = kind
		c.ProxyHost = proxyHost
		c.ProxyPort = proxyPort
		c.ProxyUser = strings.TrimSpace(a.formProxyUser)
		c.ProxyPassword = a.formProxyPass
	}
	if old, ok := a.connByID(id); ok {
		c.LastDB = old.LastDB
	}
	return c, ""
}

func (a *app) resolveSSHKey() (path, pem, errMsg string) {
	path = strings.TrimSpace(a.formSSHKey)
	pem = strings.TrimSpace(a.formSSHKeyPEM)
	if path == "" && pem == "" {
		return "", "", "请选择 SSH 私钥"
	}
	if path != "" {
		if b, err := os.ReadFile(expandHomePath(path)); err == nil && len(b) > 0 {
			pem = string(b)
			a.formSSHKeyPEM = pem
		}
	}
	return path, pem, ""
}

func expandHomePath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func (a *app) redisConfig(c store.Connection, db int) redisx.Config {
	cfg := redisx.Config{
		Host: c.Host, Port: c.Port, Username: c.Username,
		Password: c.Password, DB: db,
	}
	if c.SSH {
		port := c.SSHPort
		if port <= 0 {
			port = 22
		}
		cfg.SSH = redisx.SSHConfig{
			Enabled: true,
			Host:    c.SSHHost,
			Port:    port,
			User:    c.SSHUser,
		}
		if c.SSHAuth == "key" {
			cfg.SSH.KeyPath = c.SSHKeyPath
			cfg.SSH.KeyPEM = c.SSHKey
			cfg.SSH.Passphrase = c.SSHPassphrase
		} else {
			cfg.SSH.Password = c.SSHPassword
		}
	}
	if c.Proxy {
		port := c.ProxyPort
		if port <= 0 {
			port = 7890
		}
		cfg.Proxy = redisx.ProxyConfig{
			Enabled:  true,
			Type:     c.ProxyType,
			Host:     c.ProxyHost,
			Port:     port,
			Username: c.ProxyUser,
			Password: c.ProxyPassword,
		}
	}
	return cfg
}

func (a *app) saveConnForm() bool {
	c, errMsg := a.formConnection()
	if errMsg != "" {
		a.formError = errMsg
		return false
	}
	replaced := false
	for i := range a.connections {
		if a.connections[i].ID == c.ID {
			a.connections[i] = c
			replaced = true
			break
		}
	}
	if !replaced {
		a.connections = append(a.connections, c)
	}
	a.selectedNav = c.ID
	a.formError = ""
	a.lastConnID = c.ID
	a.showConnForm = false
	a.persist()
	return true
}

func (a *app) deleteSelectedConn() {
	c, ok := a.selectedConn()
	if !ok {
		return
	}
	a.disconnectID(c.ID)
	clearKeychainSecrets(c.ID)
	out := a.connections[:0]
	for _, x := range a.connections {
		if x.ID != c.ID {
			out = append(out, x)
		}
	}
	a.connections = out
	a.selectedNav = ""
	if len(a.connections) > 0 {
		a.selectedNav = a.connections[0].ID
	}
	a.persist()
}

func (a *app) testConnForm() {
	c, errMsg := a.formConnection()
	if errMsg != "" {
		a.formError = errMsg
		return
	}
	a.formTesting = true
	a.formError = ""
	a.formHint = ""
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
		defer cancel()
		cli, err := redisx.Dial(ctx, a.redisConfig(c, 0))
		if err == nil {
			cli.Close()
		}
		a.publish(func() {
			a.formTesting = false
			if err != nil {
				a.formError = err.Error()
				a.formHint = ""
				return
			}
			a.formError = ""
			a.formHint = "连接成功"
		})
	}()
}

func (a *app) connectSelected() {
	c, ok := a.selectedConn()
	if !ok || a.connecting {
		return
	}
	if a.isLive(c.ID) {
		if a.connectedID == c.ID {
			a.showServerInfo()
			return
		}
		a.focusConnection(c.ID)
		return
	}
	prev := a.connectedID
	a.parkFocus()
	a.connecting = true
	a.status = "连接中…"
	n := bump(&a.connGen)
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		cli, err := redisx.Dial(ctx, a.redisConfig(c, 0))
		if err != nil {
			a.publish(func() {
				if stale(&a.connGen, n) {
					return
				}
				a.connecting = false
				a.status = "连接失败: " + err.Error()
				if prev != "" {
					a.focusConnection(prev)
				}
			})
			return
		}
		snap, _ := cli.Snapshot(ctx)
		dbs, dbErr := cli.Databases(ctx)
		a.publish(func() {
			if stale(&a.connGen, n) {
				cli.Close()
				return
			}
			a.client = cli
			a.connectedID = c.ID
			a.connecting = false
			a.info = snap
			a.activeDB = -1
			if dbErr != nil {
				a.status = "已连接，但无法列出数据库: " + dbErr.Error()
				a.databases = nil
			} else {
				a.databases = dbs
				a.connOutline.Open.Add(c.ID)
				a.status = fmt.Sprintf("已连接 %s (%s:%d)", c.Name, c.Host, c.Port)
			}
			a.lastConnID = c.ID
			a.persist()
			if c.LastDB != nil {
				a.openDatabase(*c.LastDB)
			}
		})
	}()
}

func (a *app) showServerInfo() {
	if !a.connected() {
		return
	}
	a.connOutline.Open.Add(a.connectedID)
	if a.inDB() {
		a.leaveDatabase()
	} else {
		a.refreshDatabases()
	}
	a.refreshServerInfo()
}

func (a *app) refreshServerInfo() {
	if !a.connected() || a.client == nil || !a.client.Ready() {
		return
	}
	cli := a.client
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		snap, err := cli.Snapshot(ctx)
		a.publish(func() {
			if a.client != cli {
				return
			}
			if err != nil {
				a.status = "读取服务信息失败: " + err.Error()
				return
			}
			a.info = snap
		})
	}()
}

func (a *app) disconnect() {
	bump(&a.connGen)
	bump(&a.keyGen)
	bump(&a.valGen)
	bump(&a.cliGen)
	if a.client != nil {
		a.client.Close()
		a.client = nil
	}
	a.connectedID = ""
	a.connecting = false
	a.clearBrowse()
	if len(a.parked) == 0 {
		if a.status == "" || strings.HasPrefix(a.status, "已连接") || strings.HasPrefix(a.status, "连接中") || strings.HasPrefix(a.status, "已重连") {
			a.status = "未连接"
		}
	} else {
		a.status = "已断开当前连接"
	}
}

func (a *app) refreshDatabases() {
	if !a.connected() || !a.client.Ready() {
		return
	}
	a.loadingDBs = true
	cli := a.client
	n := bump(&a.keyGen)
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		dbs, err := cli.Databases(ctx)
		a.publish(func() {
			if stale(&a.keyGen, n) || a.client != cli {
				return
			}
			a.loadingDBs = false
			if err != nil {
				a.status = "列出数据库失败: " + err.Error()
				return
			}
			a.databases = dbs
		})
	}()
}

func (a *app) openDatabase(index int) {
	if !a.connected() || !a.client.Ready() || a.connecting {
		return
	}
	cli := a.client
	n := bump(&a.connGen)
	a.loadingDBs = true
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		err := cli.SwitchDB(ctx, index)
		var snap redisx.Info
		if err == nil {
			snap, _ = cli.Snapshot(ctx)
		}
		a.publish(func() {
			if stale(&a.connGen, n) || a.client != cli {
				return
			}
			a.loadingDBs = false
			if err != nil {
				a.status = "打开数据库失败: " + err.Error()
				return
			}
			a.activeDB = index
			a.info = snap
			a.status = fmt.Sprintf("已打开 db%d", index)
			a.setLastDB(index)
			a.refreshKeys(true)
		})
	}()
}

func (a *app) leaveDatabase() {
	if !a.connected() {
		return
	}
	a.activeDB = -1
	a.keys = nil
	a.hasMore = false
	a.scanCursor = 0
	a.selected = ""
	a.keyRow = -1
	a.val = redisx.Value{}
	a.stringEdit = ""
	a.showAddKey = false
	a.rebuildTree()
	a.refreshDatabases()
	if c, ok := a.selectedConn(); ok {
		a.status = fmt.Sprintf("已连接 %s (%s:%d)", c.Name, c.Host, c.Port)
	} else {
		a.status = "已连接"
	}
}

func (a *app) refreshAfterWrite() {
	if a.inDB() {
		a.refreshKeys(true)
		return
	}
	if a.connected() {
		a.refreshDatabases()
	}
}

func (a *app) refreshKeys(reset bool) {
	if !a.inDB() {
		return
	}
	if reset {
		a.keys = nil
		a.scanCursor = 0
		a.hasMore = false
		a.selected = ""
		a.keyRow = -1
		a.val = redisx.Value{}
		a.stringEdit = ""
		a.rebuildTree()
	}
	a.loadingKeys = true
	cursor := a.scanCursor
	match := cliparse.ScanMatch(a.keyPattern)
	cli := a.client
	n := bump(&a.keyGen)
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		names, nxt, err := cli.Scan(ctx, cursor, match, format.ScanCount)
		var infos []redisx.KeyInfo
		if err == nil {
			infos, err = cli.KeyInfos(ctx, names)
		}
		a.publish(func() {
			if stale(&a.keyGen, n) || a.client != cli {
				return
			}
			a.loadingKeys = false
			if err != nil {
				a.status = "SCAN 失败: " + err.Error()
				return
			}
			a.keys = append(a.keys, infos...)
			a.scanCursor = nxt
			a.hasMore = nxt != 0
			a.rebuildTree()
		})
	}()
}

func (a *app) clearNavFocus() {
	a.navRow = -1
	a.selectedNav = ""
}

func (a *app) clearKeyFocus() {
	a.keyRow = -1
	a.keySel.Clear()
	if a.selected == "" {
		return
	}
	a.selected = ""
	a.val = redisx.Value{}
	a.stringEdit = ""
	a.valErr = ""
	a.clearMemberFocus()
}

func (a *app) clearMemberFocus() {
	a.hashRow = -1
	a.listRow = -1
	a.setRow = -1
	a.zsetRow = -1
}

func (a *app) selectNode(id string) {
	if id == a.selected {
		return
	}
	a.selected = id
	a.valErr = ""
	a.showTTL = false
	a.showRename = false
	if !a.tree.IsKey[id] {
		a.val = redisx.Value{}
		a.stringEdit = ""
		return
	}
	a.loadValue(id)
}

func (a *app) loadValue(key string) {
	if !a.connected() || a.client == nil || !a.client.Ready() {
		return
	}
	a.loadingVal = true
	cli := a.client
	n := bump(&a.valGen)
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		v, err := cli.LoadValue(ctx, key)
		a.publish(func() {
			if stale(&a.valGen, n) || a.selected != key {
				return
			}
			a.loadingVal = false
			if err != nil {
				a.valErr = err.Error()
				a.val = redisx.Value{}
				a.stringEdit = ""
				return
			}
			a.applyValue(v)
			a.stringEdit = v.String
			a.valHex = false
			a.stringMode = 0
			if v.Type == "string" && !v.TooLarge {
				switch {
				case format.LooksJSON(v.String):
					if pretty, err := format.PrettyJSON(v.String); err == nil {
						a.stringEdit = pretty
						a.stringMode = 2
					}
				case format.Binary(v.String):
					a.stringEdit = format.ToHex(v.String)
					a.valHex = true
					a.stringMode = 1
				}
			}
			if v.TTL >= 0 {
				a.ttlEdit = strconv.FormatInt(v.TTL/1000, 10)
			} else {
				a.ttlEdit = ""
			}
			a.renameEdit = v.Key
		})
	}()
}

func (a *app) run(fn func(context.Context, *redisx.Client) error, after func()) {
	if !a.connected() || a.client == nil || !a.client.Ready() {
		return
	}
	cli := a.client
	n := bump(&a.valGen)
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		err := fn(ctx, cli)
		a.publish(func() {
			if stale(&a.valGen, n) {
				return
			}
			if err != nil {
				a.valErr = err.Error()
				return
			}
			a.valErr = ""
			if after != nil {
				after()
			}
		})
	}()
}

func (a *app) setStringMode(mode int) {
	if a.stringMode == mode {
		return
	}
	raw := a.stringEdit
	if a.stringMode == 1 {
		decoded, err := format.FromHex(raw)
		if err != nil {
			a.valErr = "Hex 无效"
			return
		}
		raw = decoded
	}
	switch mode {
	case 1:
		a.stringEdit = format.ToHex(raw)
		a.valHex = true
	case 2:
		pretty, err := format.PrettyJSON(raw)
		if err != nil {
			a.valErr = "不是有效 JSON"
			return
		}
		a.stringEdit = pretty
		a.valHex = false
	default:
		a.stringEdit = raw
		a.valHex = false
	}
	a.stringMode = mode
	a.valErr = ""
}

func (a *app) toggleHex(on bool) {
	if on {
		a.setStringMode(1)
	} else {
		a.setStringMode(0)
	}
}

func (a *app) addHashField() {
	key, field, value := a.val.Key, strings.TrimSpace(a.newField), a.newValue
	if field == "" {
		a.valErr = "field 不能为空"
		return
	}
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.HSet(ctx, key, field, value)
	}, func() {
		a.newField, a.newValue = "", ""
		a.loadValue(key)
	})
}

func (a *app) deleteHashField(field string) {
	key := a.val.Key
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.HDel(ctx, key, field)
	}, func() { a.loadValue(key) })
}

func (a *app) saveHashRow(i int) {
	if i < 0 || i >= len(a.val.Hash) {
		return
	}
	orig := a.val.Hash[i].Name
	if i < len(a.hashOrig) {
		orig = a.hashOrig[i]
	}
	name := strings.TrimSpace(a.val.Hash[i].Name)
	if name == "" {
		a.valErr = "field 不能为空"
		return
	}
	value := a.val.Hash[i].Value
	key := a.val.Key
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.HRename(ctx, key, orig, name, value)
	}, func() { a.loadValue(key) })
}

func (a *app) pushList(left bool) {
	key, value := a.val.Key, a.newValue
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		if left {
			return cli.LPush(ctx, key, value)
		}
		return cli.RPush(ctx, key, value)
	}, func() {
		a.newValue = ""
		a.loadValue(key)
	})
}

func (a *app) setListItem(index int64, value string) {
	key := a.val.Key
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.LSet(ctx, key, index, value)
	}, func() { a.loadValue(key) })
}

func (a *app) deleteListItem(index int64) {
	key := a.val.Key
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.LDelIndex(ctx, key, index)
	}, func() { a.loadValue(key) })
}

func (a *app) insertList(before bool) {
	key, value := a.val.Key, a.newValue
	if len(a.val.List) == 0 {
		a.pushList(false)
		return
	}
	idx := a.listRow
	if idx < 0 || idx >= len(a.val.List) {
		if before {
			idx = 0
		} else {
			idx = len(a.val.List) - 1
		}
	}
	pivot := a.val.List[idx]
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.LInsert(ctx, key, before, pivot, value)
	}, func() {
		a.newValue = ""
		a.loadValue(key)
	})
}

func (a *app) addSetMember() {
	key, member := a.val.Key, a.newMember
	if member == "" {
		a.valErr = "member 不能为空"
		return
	}
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.SAdd(ctx, key, member)
	}, func() {
		a.newMember = ""
		a.loadValue(key)
	})
}

func (a *app) deleteSetMember(member string) {
	key := a.val.Key
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.SRem(ctx, key, member)
	}, func() { a.loadValue(key) })
}

func (a *app) saveSetRow(i int) {
	if i < 0 || i >= len(a.val.Set) {
		return
	}
	orig := a.val.Set[i]
	if i < len(a.setOrig) {
		orig = a.setOrig[i]
	}
	to := strings.TrimSpace(a.val.Set[i])
	if to == "" {
		a.valErr = "member 不能为空"
		return
	}
	key := a.val.Key
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.SRename(ctx, key, orig, to)
	}, func() { a.loadValue(key) })
}

func (a *app) addZMember() {
	key, member := a.val.Key, a.newMember
	if member == "" {
		a.valErr = "member 不能为空"
		return
	}
	score, err := strconv.ParseFloat(strings.TrimSpace(a.newScore), 64)
	if err != nil {
		a.valErr = "score 无效"
		return
	}
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.ZAdd(ctx, key, score, member)
	}, func() {
		a.newMember = ""
		a.loadValue(key)
	})
}

func (a *app) deleteZMember(member string) {
	key := a.val.Key
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.ZRem(ctx, key, member)
	}, func() { a.loadValue(key) })
}

func (a *app) saveZSetRow(i int) {
	if i < 0 || i >= len(a.val.ZSet) {
		return
	}
	orig := a.val.ZSet[i].Member
	if i < len(a.zsetOrig) {
		orig = a.zsetOrig[i]
	}
	to := strings.TrimSpace(a.val.ZSet[i].Member)
	if to == "" {
		a.valErr = "member 不能为空"
		return
	}
	scoreText := a.newScore
	if i < len(a.zsetScoreEdits) {
		scoreText = a.zsetScoreEdits[i]
	}
	score, err := strconv.ParseFloat(strings.TrimSpace(scoreText), 64)
	if err != nil {
		a.valErr = "score 无效"
		return
	}
	key := a.val.Key
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.ZUpdate(ctx, key, orig, to, score)
	}, func() { a.loadValue(key) })
}

func (a *app) saveString() {
	key := a.val.Key
	text := a.stringEdit
	if a.stringMode == 1 || a.valHex {
		decoded, err := format.FromHex(text)
		if err != nil {
			a.valErr = "Hex 无效: " + err.Error()
			return
		}
		text = decoded
	}
	if a.stringMode == 2 {
		if !format.LooksJSON(text) {
			a.valErr = "不是有效 JSON"
			return
		}
	}
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.SetString(ctx, key, text)
	}, func() { a.loadValue(key) })
}

func (a *app) deleteSelectedKey() {
	if a.confirm == confirmDeleteKeys {
		a.deleteKeys(strings.Split(a.confirmID, "\n"))
		return
	}
	key := a.confirmID
	if key == "" {
		key = a.selected
	}
	if key == "" || !a.tree.IsKey[key] {
		return
	}
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.Delete(ctx, key)
	}, func() {
		a.dropKey(key)
		a.selected = ""
		a.val = redisx.Value{}
		a.stringEdit = ""
		a.keySel.Clear()
	})
}

func (a *app) chosenKeyNames() []string {
	var keys []string
	for id := range a.keySel.All() {
		if a.tree.IsKey[id] {
			keys = append(keys, id)
		}
	}
	if len(keys) == 0 && a.selected != "" && a.tree.IsKey[a.selected] {
		keys = []string{a.selected}
	}
	return keys
}

func (a *app) askDeleteKeys(keys []string) {
	if len(keys) == 0 {
		return
	}
	if len(keys) == 1 {
		a.askConfirm(confirmDeleteKey, keys[0])
		return
	}
	a.confirm = confirmDeleteKeys
	a.confirmID = strings.Join(keys, "\n")
	a.confirmOpen = true
}

func (a *app) deleteKeys(keys []string) {
	if len(keys) == 0 {
		return
	}
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.DeleteKeys(ctx, keys...)
	}, func() {
		for _, key := range keys {
			a.dropKey(key)
		}
		a.selected = ""
		a.val = redisx.Value{}
		a.stringEdit = ""
		a.keySel.Clear()
		a.status = fmt.Sprintf("已删除 %d 个 key", len(keys))
	})
}

func (a *app) dropKey(key string) {
	out := a.keys[:0]
	for _, k := range a.keys {
		if k.Key != key {
			out = append(out, k)
		}
	}
	a.keys = out
	a.rebuildTree()
}

func (a *app) renameSelected() {
	src := a.selected
	dest := strings.TrimSpace(a.renameEdit)
	if src == "" || dest == "" || dest == src {
		a.showRename = false
		return
	}
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.Rename(ctx, src, dest)
	}, func() {
		a.showRename = false
		for i := range a.keys {
			if a.keys[i].Key == src {
				a.keys[i].Key = dest
			}
		}
		a.rebuildTree()
		a.selected = dest
		a.loadValue(dest)
	})
}

func (a *app) applyTTL() {
	key := a.selected
	sec := int64(0)
	if strings.TrimSpace(a.ttlEdit) != "" {
		n, err := strconv.ParseInt(strings.TrimSpace(a.ttlEdit), 10, 64)
		if err != nil || n < 0 {
			a.valErr = "TTL 必须是非负整数秒，空则永久"
			return
		}
		sec = n
	}
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.Expire(ctx, key, sec)
	}, func() {
		a.showTTL = false
		a.loadValue(key)
	})
}

func (a *app) submitCLI() {
	line := strings.TrimSpace(a.cliInput)
	if line == "" || !a.connected() {
		return
	}
	args, err := cliparse.Split(line)
	if err != nil {
		a.cliOutput = "(error) " + err.Error()
		return
	}
	if cliparse.Dangerous(args) {
		a.pendingCLI = args
		a.askConfirm(confirmDangerCLI, line)
		return
	}
	a.execCLI(args, line)
}

func (a *app) execCLI(args []string, line string) {
	if len(a.cliHist) == 0 || a.cliHist[len(a.cliHist)-1] != line {
		a.cliHist = append(a.cliHist, line)
		if len(a.cliHist) > 50 {
			a.cliHist = a.cliHist[len(a.cliHist)-50:]
		}
	}
	a.cliHistIdx = len(a.cliHist)
	cli := a.client
	raw := make([]any, len(args))
	for i, s := range args {
		raw[i] = s
	}
	n := bump(&a.cliGen)
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		res, err := cli.Do(ctx, raw...)
		a.publish(func() {
			if stale(&a.cliGen, n) {
				return
			}
			if err != nil {
				a.cliOutput = "(error) " + err.Error()
				return
			}
			a.cliOutput = cliparse.FormatReply(res)
			a.cliInput = ""
			upper := strings.ToUpper(args[0])
			if upper == "DEL" || upper == "RENAME" || upper == "FLUSHDB" || upper == "FLUSHALL" || upper == "SET" || upper == "HSET" || upper == "SELECT" {
				if a.inDB() {
					a.refreshKeys(true)
				}
				if upper == "FLUSHALL" || upper == "FLUSHDB" || !a.inDB() {
					a.refreshDatabases()
				}
			}
		})
	}()
}

func (a *app) cliHistory(delta int) {
	if len(a.cliHist) == 0 {
		return
	}
	a.cliHistIdx += delta
	if a.cliHistIdx < 0 {
		a.cliHistIdx = 0
	}
	if a.cliHistIdx >= len(a.cliHist) {
		a.cliHistIdx = len(a.cliHist)
		a.cliInput = ""
		return
	}
	a.cliInput = a.cliHist[a.cliHistIdx]
}

func (a *app) copyText(s string, toast string) {
	if s == "" {
		return
	}
	mygo.Clipboard.WriteText(s)
	a.status = toast
}

func (a *app) openAddKey() {
	if !a.inDB() {
		return
	}
	a.newKeyName = ""
	if a.selected != "" && !a.tree.IsKey[a.selected] {
		sep := a.sep
		if sep == "" {
			sep = ":"
		}
		a.newKeyName = a.selected + sep
	}
	if a.newKeyType == "" {
		a.newKeyType = "string"
	}
	a.newKeyValue = ""
	a.newKeyField = ""
	a.newKeyMember = ""
	a.newKeyScore = "0"
	a.newKeyTTL = ""
	a.newKeyErr = ""
	a.showAddKey = true
}

func (a *app) addKeySpec() (redisx.CreateSpec, string) {
	key := strings.TrimSpace(a.newKeyName)
	if key == "" {
		return redisx.CreateSpec{}, "Key 不能为空"
	}
	typ := a.newKeyType
	if typ == "" {
		typ = "string"
	}
	spec := redisx.CreateSpec{
		Key:    key,
		Type:   typ,
		Value:  a.newKeyValue,
		Field:  strings.TrimSpace(a.newKeyField),
		Member: a.newKeyMember,
	}
	switch typ {
	case "string", "list":
		// empty values are valid
	case "hash", "stream":
		if spec.Field == "" {
			return redisx.CreateSpec{}, "field 不能为空"
		}
	case "set":
		if spec.Member == "" {
			return redisx.CreateSpec{}, "member 不能为空"
		}
	case "zset":
		if spec.Member == "" {
			return redisx.CreateSpec{}, "member 不能为空"
		}
		score, err := strconv.ParseFloat(strings.TrimSpace(a.newKeyScore), 64)
		if err != nil {
			return redisx.CreateSpec{}, "score 无效"
		}
		spec.Score = score
	default:
		return redisx.CreateSpec{}, "不支持的类型"
	}
	if ttl := strings.TrimSpace(a.newKeyTTL); ttl != "" {
		n, err := strconv.ParseInt(ttl, 10, 64)
		if err != nil || n < 0 {
			return redisx.CreateSpec{}, "TTL 必须是非负整数秒，空则永久"
		}
		spec.TTL = n
	}
	return spec, ""
}

func (a *app) saveAddKey() bool {
	spec, err := a.addKeySpec()
	if err != "" {
		a.newKeyErr = err
		return false
	}
	if !a.inDB() || a.client == nil || !a.client.Ready() {
		a.newKeyErr = "未打开数据库"
		return false
	}
	a.newKeyErr = ""
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.Create(ctx, spec)
	}, func() {
		a.showAddKey = false
		if a.typeFilter != "全部" && a.typeFilter != spec.Type {
			a.typeFilter = "全部"
		}
		ttl := int64(-1)
		if spec.TTL > 0 {
			ttl = spec.TTL * 1000
		}
		a.keys = append(a.keys, redisx.KeyInfo{Key: spec.Key, Type: spec.Type, TTL: ttl, Len: 1})
		a.rebuildTree()
		a.selected = spec.Key
		a.status = "已添加 " + spec.Key
		a.loadValue(spec.Key)
	})
	return true
}

func (a *app) setLastDB(index int) {
	for i := range a.connections {
		if a.connections[i].ID == a.connectedID {
			db := index
			a.connections[i].LastDB = &db
			break
		}
	}
	a.persist()
}

func (a *app) setSeparator(sep string) {
	if sep == "" {
		a.sep = ":"
	}
	a.rebuildTree()
	a.persist()
}

func (a *app) loadMoreValue() {
	if !a.connected() || !a.val.Truncated || a.val.Key == "" {
		return
	}
	cli := a.client
	prev := a.val
	n := bump(&a.valGen)
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		v, err := cli.LoadMore(ctx, prev)
		a.publish(func() {
			if stale(&a.valGen, n) || a.val.Key != prev.Key {
				return
			}
			if err != nil {
				a.valErr = err.Error()
				return
			}
			a.applyValue(v)
		})
	}()
}

func (a *app) applyValue(v redisx.Value) {
	a.val = v
	a.syncMemberEdits()
}

func (a *app) syncMemberEdits() {
	a.hashOrig = make([]string, len(a.val.Hash))
	for i, f := range a.val.Hash {
		a.hashOrig[i] = f.Name
	}
	a.setOrig = append([]string(nil), a.val.Set...)
	a.zsetOrig = make([]string, len(a.val.ZSet))
	a.zsetScoreEdits = make([]string, len(a.val.ZSet))
	for i, z := range a.val.ZSet {
		a.zsetOrig[i] = z.Member
		a.zsetScoreEdits[i] = strconv.FormatFloat(z.Score, 'f', -1, 64)
	}
}

func (a *app) copyValue() {
	text := a.valueText()
	if text == "" {
		return
	}
	a.copyText(text, "已复制 Value")
}

func (a *app) copyCommand() {
	cmd := redisx.AsCommand(a.val)
	if cmd == "" {
		return
	}
	a.copyText(cmd, "已复制命令")
}

func (a *app) valueText() string {
	switch a.val.Type {
	case "string":
		if a.stringMode == 1 {
			if s, err := format.FromHex(a.stringEdit); err == nil {
				return s
			}
		}
		return a.stringEdit
	case "hash":
		var b strings.Builder
		for i, f := range a.val.Hash {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(f.Name + "=" + f.Value)
		}
		return b.String()
	case "list":
		return strings.Join(a.val.List, "\n")
	case "set":
		return strings.Join(a.val.Set, "\n")
	case "zset":
		var b strings.Builder
		for i, z := range a.val.ZSet {
			if i > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "%v %s", z.Score, z.Member)
		}
		return b.String()
	case "stream":
		var b strings.Builder
		for i, e := range a.val.Stream {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(e.ID)
		}
		return b.String()
	}
	return ""
}

func (a *app) importCurrentDB() {
	if !a.inDB() || a.client == nil || !a.client.Ready() {
		return
	}
	a.startImport(a.client)
}

func (a *app) importDatabase(connID string, db int) {
	if a.connectedID == connID && a.activeDB == db && a.client != nil && a.client.Ready() {
		a.startImport(a.client)
		return
	}
	c, ok := a.connByID(connID)
	if !ok || !a.isLive(connID) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
		defer cancel()
		cli, err := redisx.Dial(ctx, a.redisConfig(c, db))
		if err != nil {
			a.publish(func() { a.status = "导入失败: " + err.Error() })
			return
		}
		defer cli.Close()
		a.runImport(ctx, cli)
	}()
}

func (a *app) startImport(cli *redisx.Client) {
	if cli == nil || !cli.Ready() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
		defer cancel()
		a.runImport(ctx, cli)
	}()
}

func (a *app) runImport(ctx context.Context, cli *redisx.Client) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:  a.win,
		Title:   "导入 Key",
		Filters: []mygo.FileFilter{{Name: "Redis", Extensions: []string{"redis", "txt"}}},
	})
	if err != nil || len(paths) == 0 {
		a.publish(func() {
			if strings.HasPrefix(a.status, "正在导入") {
				a.status = "已取消导入"
			}
		})
		return
	}
	a.beginImport()
	data, err := os.ReadFile(paths[0])
	if err != nil {
		a.failImport("读取失败: " + err.Error())
		return
	}
	last := time.Time{}
	res, err := cli.Import(ctx, string(data), func(done, total int) {
		now := time.Now()
		if done < total && !last.IsZero() && now.Sub(last) < 80*time.Millisecond {
			return
		}
		last = now
		a.publish(func() {
			a.importDone = done
			a.importTotal = total
			a.status = fmt.Sprintf("正在导入 %d / %d", done, total)
			a.setWindowProgress(done, total)
		})
	})
	if err != nil && res.OK == 0 && res.Failed == 0 {
		a.failImport("导入失败: " + err.Error())
		return
	}
	msg := importStatus(res)
	a.publish(func() {
		a.clearImportProgress()
		a.importResult = res
		a.showImportDone = true
		a.status = msg
		a.refreshAfterWrite()
	})
}

func (a *app) beginImport() {
	a.publish(func() {
		a.importing = true
		a.importDone = 0
		a.importTotal = 0
		a.showImportDone = false
		a.status = "正在导入…"
		a.setWindowProgress(0, 0)
	})
}

func (a *app) failImport(msg string) {
	a.publish(func() {
		a.clearImportProgress()
		a.status = msg
	})
}

func (a *app) clearImportProgress() {
	a.importing = false
	a.importDone = 0
	a.importTotal = 0
	if a.win != nil {
		a.win.SetProgressBar(mygo.ProgressBar{})
	}
}

func importDoneMessage(res redisx.ImportResult) string {
	msg := fmt.Sprintf("已导入 %d 个 key。", res.OK)
	if res.Failed > 0 {
		msg += fmt.Sprintf("\n失败 %d", res.Failed)
		if res.LastErr != nil {
			msg += "：" + res.LastErr.Error()
		}
	}
	if res.Skipped > 0 {
		msg += fmt.Sprintf("\n跳过 %d 条命令", res.Skipped)
	}
	return msg
}

func importStatus(res redisx.ImportResult) string {
	msg := fmt.Sprintf("已导入 %d 个", res.OK)
	if res.Failed > 0 {
		msg += fmt.Sprintf("，失败 %d", res.Failed)
	}
	if res.Skipped > 0 {
		msg += fmt.Sprintf("，跳过 %d", res.Skipped)
	}
	return msg
}

func (a *app) exportFromKeysPane() {
	if names := a.chosenKeyNames(); len(names) > 0 {
		a.exportKeys(names)
		return
	}
	if a.selected != "" && !a.tree.IsKey[a.selected] {
		a.exportPrefix(a.selected)
		return
	}
	if a.inDB() {
		a.exportDatabase(a.connectedID, a.activeDB)
	}
}

func (a *app) exportKeys(keys []string) {
	a.startExport("keys.redis", func(ctx context.Context, cli *redisx.Client) ([]string, error) {
		return keys, nil
	}, a.client)
}

func (a *app) deletePrefix(prefix string) {
	if prefix == "" || !a.inDB() || a.client == nil || !a.client.Ready() {
		return
	}
	cli := a.client
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
		defer cancel()
		keys, err := a.scanPrefixKeys(ctx, cli, prefix)
		if err != nil {
			a.publish(func() { a.status = "删除失败: " + err.Error() })
			return
		}
		if len(keys) == 0 {
			a.publish(func() { a.status = "没有可删除的 key" })
			return
		}
		if err := cli.DeleteKeys(ctx, keys...); err != nil {
			a.publish(func() { a.status = "删除失败: " + err.Error() })
			return
		}
		a.publish(func() {
			a.status = fmt.Sprintf("已删除 %d 个 key", len(keys))
			if a.client == cli && a.inDB() {
				a.refreshKeys(true)
			}
		})
	}()
}

func (a *app) scanPrefixKeys(ctx context.Context, cli *redisx.Client, prefix string) ([]string, error) {
	keys, err := cli.ScanAll(ctx, cliparse.PrefixMatch(prefix, a.sep))
	if err != nil {
		return nil, err
	}
	ok, err := cli.Exists(ctx, prefix)
	if err == nil && ok && !containsString(keys, prefix) {
		keys = append([]string{prefix}, keys...)
	}
	return keys, nil
}

func (a *app) exportPrefix(prefix string) {
	if !a.inDB() || a.client == nil || !a.client.Ready() {
		return
	}
	cli := a.client
	a.startExport(exportFileName(prefix), func(ctx context.Context, cli *redisx.Client) ([]string, error) {
		return a.scanPrefixKeys(ctx, cli, prefix)
	}, cli)
}

func (a *app) exportDatabase(connID string, db int) {
	c, ok := a.connByID(connID)
	if !ok {
		return
	}
	name := fmt.Sprintf("db%d.redis", db)
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
		defer cancel()
		cli, err := redisx.Dial(ctx, a.redisConfig(c, db))
		if err != nil {
			a.publish(func() { a.status = "导出失败: " + err.Error() })
			return
		}
		defer cli.Close()
		a.writeExport(ctx, cli, name, func(ctx context.Context, cli *redisx.Client) ([]string, error) {
			return cli.ScanAll(ctx, "*")
		})
	}()
}

func (a *app) startExport(filename string, list func(context.Context, *redisx.Client) ([]string, error), cli *redisx.Client) {
	if cli == nil || !cli.Ready() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
		defer cancel()
		a.writeExport(ctx, cli, filename, list)
	}()
}

func (a *app) writeExport(ctx context.Context, cli *redisx.Client, filename string, list func(context.Context, *redisx.Client) ([]string, error)) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.win,
		Title:       "导出 Key",
		DefaultPath: filename,
		Filters:     []mygo.FileFilter{{Name: "Redis", Extensions: []string{"redis", "txt"}}},
	})
	if err != nil || path == "" {
		a.publish(func() {
			if strings.HasPrefix(a.status, "正在导出") {
				a.status = "已取消导出"
			}
		})
		return
	}
	a.beginExport()
	keys, err := list(ctx, cli)
	if err != nil {
		a.failExport("导出失败: " + err.Error())
		return
	}
	if len(keys) == 0 {
		a.failExport("没有可导出的 key")
		return
	}
	a.publish(func() {
		a.exportTotal = len(keys)
		a.status = fmt.Sprintf("正在导出 0 / %d", len(keys))
		a.setWindowProgress(0, len(keys))
	})
	last := time.Time{}
	text, err := cli.Export(ctx, keys, func(done, total int) {
		now := time.Now()
		if done < total && !last.IsZero() && now.Sub(last) < 80*time.Millisecond {
			return
		}
		last = now
		a.publish(func() {
			a.exportDone = done
			a.exportTotal = total
			a.status = fmt.Sprintf("正在导出 %d / %d", done, total)
			a.setWindowProgress(done, total)
		})
	})
	if err != nil {
		a.failExport("导出失败: " + err.Error())
		return
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		a.failExport("写出失败: " + err.Error())
		return
	}
	a.publish(func() {
		a.clearExportProgress()
		a.exportPath = path
		a.exportCount = len(keys)
		a.showExportDone = true
		a.status = fmt.Sprintf("已导出 %d 个 key", len(keys))
	})
}

func (a *app) beginExport() {
	a.publish(func() {
		a.exporting = true
		a.exportDone = 0
		a.exportTotal = 0
		a.showExportDone = false
		a.status = "正在导出…"
		a.setWindowProgress(0, 0)
	})
}

func (a *app) failExport(msg string) {
	a.publish(func() {
		a.clearExportProgress()
		a.status = msg
	})
}

func (a *app) clearExportProgress() {
	a.exporting = false
	a.exportDone = 0
	a.exportTotal = 0
	if a.win != nil {
		a.win.SetProgressBar(mygo.ProgressBar{})
	}
}

func (a *app) setWindowProgress(done, total int) {
	if a.win == nil {
		return
	}
	if total <= 0 {
		a.win.SetProgressBar(mygo.ProgressBar{State: mygo.ProgressIndeterminate})
		return
	}
	a.win.SetProgressBar(mygo.ProgressBar{Value: float64(done) / float64(total)})
}

func (a *app) revealExport() {
	if a.exportPath == "" {
		return
	}
	if a.win != nil {
		mygo.Shell.ShowItemInFolder(a.exportPath)
	}
}

func exportFileName(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" {
		return "keys.redis"
	}
	return name + ".redis"
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (a *app) refreshSlowlog() {
	if !a.connected() || a.client == nil || !a.client.Ready() {
		return
	}
	cli := a.client
	ctx, cancel := timeout(a.ctx)
	go func() {
		defer cancel()
		items, err := cli.Slowlog(ctx, 32)
		a.publish(func() {
			if a.client != cli {
				return
			}
			if err != nil {
				a.status = "读取慢日志失败: " + err.Error()
				return
			}
			a.slowlog = items
		})
	}()
}

func (a *app) addStreamEntry() {
	key, field, value := a.val.Key, strings.TrimSpace(a.newField), a.newValue
	if field == "" {
		a.valErr = "field 不能为空"
		return
	}
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.XAdd(ctx, key, field, value)
	}, func() {
		a.newField, a.newValue = "", ""
		a.loadValue(key)
	})
}

func (a *app) deleteStreamEntry(id string) {
	key := a.val.Key
	a.run(func(ctx context.Context, cli *redisx.Client) error {
		return cli.XDel(ctx, key, id)
	}, func() { a.loadValue(key) })
}

func validateAddKey(name, typ, value, field, member, score, ttl string) string {
	_, err := (&app{
		newKeyName:   name,
		newKeyType:   typ,
		newKeyValue:  value,
		newKeyField:  field,
		newKeyMember: member,
		newKeyScore:  score,
		newKeyTTL:    ttl,
	}).addKeySpec()
	return err
}

func validateConnForm(name, host, port string) string {
	_, err := (&app{
		formName: name, formHost: host, formPort: port,
	}).formConnection()
	return err
}
