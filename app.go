package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"redis/internal/keytree"
	"redis/internal/redisx"
	"redis/internal/store"
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmDeleteKey
	confirmDeleteKeys
	confirmDeletePrefix
	confirmDeleteConn
	confirmDangerCLI
)

type app struct {
	win *mygo.Window

	connections []store.Connection
	lastConnID  string
	sep         string
	storeDir    string

	navRow      int
	selectedNav string
	connOutline ui.OutlineState[string]

	showConnForm bool
	editingID    string
	formName     string
	formHost     string
	formPort     string
	formUser     string
	formPass      string
	formSSH       bool
	formSSHHost   string
	formSSHPort   string
	formSSHUser   string
	formSSHAuth   int // 0 密码, 1 私钥
	formSSHPass   string
	formSSHKey    string
	formSSHKeyPEM string
	formSSHPhrase string
	formProxy       bool
	formProxyType   int // 0 SOCKS5, 1 HTTP
	formProxyHost   string
	formProxyPort   string
	formProxyUser   string
	formProxyPass     string
	formShowPass      bool
	formShowSSHPass   bool
	formShowSSHPhrase bool
	formShowProxyPass bool
	formError     string
	formHint      string
	formTesting   bool

	client      *redisx.Client
	connectedID string
	parked      map[string]*live
	status      string
	connecting  bool
	info        redisx.Info
	slowlog     []redisx.SlowEntry

	databases  []redisx.Database
	activeDB   int // -1 = 已连接但尚未打开某个库
	loadingDBs bool

	keyMode     int // 0 tree, 1 flat
	keyPattern  string
	typeFilter  string
	keys        []redisx.KeyInfo
	keyByName   map[string]redisx.KeyInfo
	tree        keytree.Tree
	scanCursor  uint64
	hasMore     bool
	loadingKeys bool
	keyRow      int
	keysOutline ui.OutlineState[string]
	keysList    ui.ListState
	keySel      ui.Selection[string]
	keySort     ui.SortOrder
	selected    string

	val          redisx.Value
	valHex       bool
	stringMode   int // 0 text, 1 hex, 2 json
	stringEdit   string
	ttlEdit      string
	renameEdit   string
	showTTL      bool
	showRename   bool
	showAddKey   bool
	newKeyName   string
	newKeyType   string
	newKeyValue  string
	newKeyField  string
	newKeyMember string
	newKeyScore  string
	newKeyTTL    string
	newKeyErr    string
	valErr       string
	loadingVal   bool
	newField     string
	newValue     string
	newMember    string
	newScore     string
	hashList     ui.ListState
	listList     ui.ListState
	setList      ui.ListState
	zsetList     ui.ListState
	hashRow         int
	listRow         int
	setRow          int
	zsetRow         int
	hashOrig        []string
	setOrig         []string
	zsetOrig        []string
	zsetScoreEdits  []string

	showCLI    bool
	cliInput   string
	cliOutput  string
	cliHist    []string
	cliHistIdx int
	pendingCLI []string

	sidebarW  float32
	keysPct   float32
	editorPct float32

	confirm     confirmKind
	confirmOpen bool
	confirmID   string

	exporting      bool
	exportDone     int
	exportTotal    int
	exportPath     string
	exportCount    int
	showExportDone bool

	importing      bool
	importDone     int
	importTotal    int
	importResult   redisx.ImportResult
	showImportDone bool

	showSync     bool
	syncSrcConn  string
	syncSrcDB    int
	syncMode     int // 0 全部, 1 前缀, 2 指定
	syncPrefix   string
	syncKeys     string
	syncDestConn string
	syncDestDB   string
	syncErr      string
	syncing      bool
	syncDone     int
	syncTotal    int
	syncResult   redisx.ImportResult
	showSyncDone bool

	connGen atomic.Uint64
	keyGen  atomic.Uint64
	valGen  atomic.Uint64
	cliGen  atomic.Uint64
	ctx     context.Context
}

func newApp() *app {
	a := &app{
		status:     "未连接",
		typeFilter: "全部",
		newKeyType: "string",
		newScore:   "0",
		sep:        ":",
		formHost:   "127.0.0.1",
		formPort:   "6379",
		navRow:     -1,
		activeDB:   -1,
		keyRow:     -1,
		hashRow:    -1,
		listRow:    -1,
		setRow:     -1,
		zsetRow:    -1,
		sidebarW:   240,
		keysPct:    46,
		editorPct:  62,
		keySort:    ui.SortOrder{Column: "Key"},
		keyByName:  map[string]redisx.KeyInfo{},
		parked:     map[string]*live{},
		ctx:        context.Background(),
	}
	return a
}

func (a *app) publish(fn func()) {
	if a.win != nil {
		a.win.Update(fn)
		return
	}
	fn()
}

func (a *app) selectedConn() (store.Connection, bool) {
	return a.connByID(navConnID(a.selectedNav))
}

func (a *app) navRoots() []string {
	out := make([]string, len(a.connections))
	for i, c := range a.connections {
		out[i] = c.ID
	}
	return out
}

func (a *app) navChildren(id string) []string {
	if _, _, ok := parseDBNode(id); ok {
		return nil
	}
	dbs := a.databasesOf(id)
	if len(dbs) == 0 {
		return nil
	}
	out := make([]string, len(dbs))
	for i, db := range dbs {
		out[i] = dbNodeID(id, db.Index)
	}
	return out
}

func (a *app) connByID(id string) (store.Connection, bool) {
	for _, c := range a.connections {
		if c.ID == id {
			return c, true
		}
	}
	return store.Connection{}, false
}

func (a *app) visibleKeys() []redisx.KeyInfo {
	if a.typeFilter == "全部" || a.typeFilter == "" {
		return a.keys
	}
	out := make([]redisx.KeyInfo, 0, len(a.keys))
	for _, k := range a.keys {
		if k.Type == a.typeFilter {
			out = append(out, k)
		}
	}
	return out
}

func (a *app) rebuildTree() {
	vis := a.visibleKeys()
	names := make([]string, 0, len(vis))
	a.keyByName = make(map[string]redisx.KeyInfo, len(vis))
	for _, k := range vis {
		names = append(names, k.Key)
		a.keyByName[k.Key] = k
	}
	a.tree = keytree.Build(names, a.sep)
}

func (a *app) connected() bool {
	return a.client != nil && a.connectedID != ""
}

func (a *app) inDB() bool {
	return a.connected() && a.activeDB >= 0
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (a *app) persist() {
	if a.storeDir == "" {
		return
	}
	last := a.lastConnID
	if c, ok := a.selectedConn(); ok {
		last = c.ID
	}
	_ = store.Save(a.storeDir, store.File{
		LastConnID:  last,
		Separator:   a.sep,
		Connections: a.connections,
	})
}

func (a *app) loadStore() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		a.status = "无法读取配置目录: " + err.Error()
		return
	}
	a.storeDir = dir
	f, err := store.Load(dir)
	if err != nil {
		a.status = "读取连接失败: " + err.Error()
		return
	}
	a.connections = f.Connections
	a.lastConnID = f.LastConnID
	if f.Separator != "" {
		a.sep = f.Separator
	}
	if a.migrateSecretsToStore() || len(a.connections) == 0 {
		if len(a.connections) == 0 {
			a.connections = []store.Connection{store.DefaultLocal()}
		}
		a.persist()
	}
	a.selectedNav = a.connections[0].ID
	for _, c := range a.connections {
		if c.ID == a.lastConnID {
			a.selectedNav = c.ID
			break
		}
	}
}

func (a *app) askConfirm(kind confirmKind, id string) {
	a.confirm = kind
	a.confirmID = id
	a.confirmOpen = true
}
