package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"redis/internal/cliparse"
	"redis/internal/redisx"
	"redis/internal/store"
)

func TestAppViewEmpty(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("未连接") || !tt.HasText("新建连接") || !tt.HasText("RedisCli") || !tt.HasText("双击左侧连接") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestSaveConnDialogValidatesName(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.Click("新建连接"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if a.formError != "名称不能为空" {
		t.Fatalf("formError %q", a.formError)
	}
	if !a.showConnForm {
		t.Fatal("dialog should stay open")
	}
}

func TestClickBlankClearsConnFocus(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{{ID: "a", Name: "A", Host: "127.0.0.1", Port: 6379}}
	a.selectedNav = "a"
	a.navRow = 0
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.Click("A"); err != nil {
		t.Fatal(err)
	}
	if a.selectedNav != "a" {
		t.Fatalf("row click lost focus: %q", a.selectedNav)
	}
	r, ok := tt.Find("A")
	if !ok {
		t.Fatal("missing connection")
	}
	tt.ClickAt(r.X+8, r.Y+r.H+28)
	if a.navRow != -1 || a.selectedNav != "" {
		t.Fatalf("blank click should clear focus navRow=%d selectedNav=%q", a.navRow, a.selectedNav)
	}
}

func TestClickBlankClearsKeyFocus(t *testing.T) {
	a := inDBApp()
	a.keys = []redisx.KeyInfo{{Key: "alpha", Type: "string"}}
	a.rebuildTree()
	a.selected = "alpha"
	a.keyRow = 0
	tt := ui.NewTester(a.view, 1180, 760)
	r, ok := tt.Find("alpha")
	if !ok {
		t.Fatal("missing key")
	}
	tt.ClickAt(r.X+8, r.Y+r.H+36)
	if a.selected != "" || a.keyRow != -1 {
		t.Fatalf("blank click should clear key focus selected=%q keyRow=%d", a.selected, a.keyRow)
	}
}

func TestClickBlankClearsMemberFocusOnly(t *testing.T) {
	a := inDBApp()
	a.selected = "k"
	a.tree.IsKey = map[string]bool{"k": true}
	a.val = redisx.Value{
		KeyInfo: redisx.KeyInfo{Key: "k", Type: "hash"},
		Hash:    []redisx.Field{{Name: "f", Value: "v"}},
	}
	a.hashRow = 0
	tt := ui.NewTester(a.view, 1180, 760)
	r, ok := tt.Find("保存")
	if !ok {
		t.Fatal("missing save")
	}
	tt.ClickAt(r.X+8, r.Y+r.H+36)
	if a.hashRow != -1 {
		t.Fatalf("blank click should clear member focus hashRow=%d", a.hashRow)
	}
	if a.selected != "k" {
		t.Fatalf("key focus should stay %q", a.selected)
	}
}

func TestSidebarBlankContextNewConn(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{{ID: "a", Name: "A", Host: "127.0.0.1", Port: 6379}}
	a.selectedNav = "a"
	tt := ui.NewTester(a.view, 1180, 760)
	if tt.HasText("还没有连接") {
		t.Fatal("empty-state should hide when connections exist")
	}
	if err := tt.Click("新建连接"); err == nil {
		t.Fatal("empty-state button should hide when connections exist")
	}
	r, ok := tt.Find("A")
	if !ok {
		t.Fatal("missing connection")
	}
	tt.RightClickAt(r.X+8, r.Y+r.H+24)
	menu := tt.Menu()
	if len(menu) != 1 || menu[0] != "新建连接" {
		t.Fatalf("menu %q", menu)
	}
	if err := tt.ChooseMenuItem("新建连接"); err != nil {
		t.Fatal(err)
	}
	if !a.showConnForm {
		t.Fatal("dialog closed")
	}
}

func TestSidebarEmptyContextNewConn(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("还没有连接") {
		t.Fatal("empty-state missing")
	}
	if err := tt.RightClick("还没有连接"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if len(menu) != 1 || menu[0] != "新建连接" {
		t.Fatalf("menu %q", menu)
	}
	if err := tt.ChooseMenuItem("新建连接"); err != nil {
		t.Fatal(err)
	}
	if !a.showConnForm {
		t.Fatal("dialog closed")
	}
}

func TestOpenConnDialog(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.Click("新建连接"); err != nil {
		t.Fatal(err)
	}
	if !a.showConnForm {
		t.Fatal("dialog closed")
	}
	if !tt.HasText("主机") || !tt.HasText("密码") || !tt.HasText("SSH 隧道") || !tt.HasText("本地代理") {
		t.Fatalf("form texts %q", tt.Texts())
	}
	if tt.HasText("数据库") {
		t.Fatal("new connection form should not ask for a database")
	}
	if tt.HasText("SSH 主机") {
		t.Fatal("SSH fields should stay hidden until the tunnel is enabled")
	}
	if tt.HasText("代理主机") {
		t.Fatal("proxy fields should stay hidden until the proxy is enabled")
	}
	if !tt.HasText("显示密码") {
		t.Fatal("password reveal missing")
	}
	if a.formShowPass {
		t.Fatal("password should start hidden")
	}
	if err := tt.Click("显示密码"); err != nil {
		t.Fatal(err)
	}
	if !a.formShowPass {
		t.Fatal("eye should reveal this password")
	}
}

func TestPasswordEyeTogglesOnlyItsField(t *testing.T) {
	a := newApp()
	a.openNewConn()
	a.formSSH = true
	a.formProxy = true
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.Click("显示密码"); err != nil {
		t.Fatal(err)
	}
	if !a.formShowPass {
		t.Fatal("redis password should reveal")
	}
	if a.formShowSSHPass || a.formShowProxyPass {
		t.Fatal("other password fields should stay hidden")
	}
}

func TestSaveConnFormValidation(t *testing.T) {
	if err := validateConnForm("", "127.0.0.1", "6379"); err != "名称不能为空" {
		t.Fatalf("name: %q", err)
	}
	if err := validateConnForm("dev", "", "6379"); err != "主机不能为空" {
		t.Fatalf("host: %q", err)
	}
	if err := validateConnForm("dev", "127.0.0.1", "0"); err != "端口无效" {
		t.Fatalf("port: %q", err)
	}
	if err := validateConnForm("dev", "127.0.0.1", "6379"); err != "" {
		t.Fatalf("ok: %q", err)
	}
}

func TestSSHFormValidation(t *testing.T) {
	a := &app{formName: "dev", formHost: "10.0.0.1", formPort: "6379", formSSH: true}
	if _, err := a.formConnection(); err != "SSH 主机不能为空" {
		t.Fatalf("host: %q", err)
	}
	a.formSSHHost = "bastion"
	if _, err := a.formConnection(); err != "SSH 用户不能为空" {
		t.Fatalf("user: %q", err)
	}
	a.formSSHUser = "ops"
	if _, err := a.formConnection(); err != "SSH 密码不能为空" {
		t.Fatalf("pass: %q", err)
	}
	a.formSSHAuth = 1
	if _, err := a.formConnection(); err != "请选择 SSH 私钥" {
		t.Fatalf("key: %q", err)
	}
	a.formSSHKey = "/tmp/id_rsa"
	a.formSSHKeyPEM = "-----BEGIN OPENSSH PRIVATE KEY-----\nxxx\n"
	c, err := a.formConnection()
	if err != "" || !c.SSH || c.SSHHost != "bastion" || c.SSHAuth != "key" || c.SSHKeyPath != "/tmp/id_rsa" || c.SSHKey == "" {
		t.Fatalf("%+v %q", c, err)
	}
	a.formSSHAuth = 0
	a.formSSHPass = "secret"
	c, err = a.formConnection()
	if err != "" || c.SSHAuth != "password" || c.SSHPassword != "secret" || c.SSHKeyPath != "" || c.SSHKey != "" {
		t.Fatalf("password auth %+v %q", c, err)
	}
}

func TestProxyFormValidation(t *testing.T) {
	a := &app{formName: "dev", formHost: "10.0.0.1", formPort: "6379", formProxy: true}
	if _, err := a.formConnection(); err != "代理主机不能为空" {
		t.Fatalf("host: %q", err)
	}
	a.formProxyHost = "127.0.0.1"
	a.formProxyPort = "0"
	if _, err := a.formConnection(); err != "代理端口无效" {
		t.Fatalf("port: %q", err)
	}
	a.formProxyPort = "7890"
	a.formProxyType = 1
	a.formProxyUser = "u"
	a.formProxyPass = "p"
	c, err := a.formConnection()
	if err != "" || !c.Proxy || c.ProxyType != "http" || c.ProxyHost != "127.0.0.1" || c.ProxyPort != 7890 || c.ProxyPassword != "p" {
		t.Fatalf("%+v %q", c, err)
	}
}

func TestSidebarShowsSSH(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{{ID: "a", Name: "A", Host: "10.0.0.1", Port: 6379, SSH: true}}
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("10.0.0.1:6379 · SSH") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestSidebarShowsProxy(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{{ID: "a", Name: "A", Host: "10.0.0.1", Port: 6379, Proxy: true}}
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("10.0.0.1:6379 · 代理") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestSaveConnFormAddsConnection(t *testing.T) {
	a := newApp()
	a.openNewConn()
	a.formName = "dev"
	a.formHost = "10.0.0.2"
	a.formPort = "6380"
	a.formPass = "redis-secret"
	if !a.saveConnForm() {
		t.Fatalf("save: %s", a.formError)
	}
	if len(a.connections) != 1 || a.connections[0].Host != "10.0.0.2" || a.connections[0].Port != 6380 || a.connections[0].DB != 0 || a.connections[0].Password != "redis-secret" {
		t.Fatalf("%+v", a.connections)
	}
	if a.connections[0].ID == "" {
		t.Fatal("missing id")
	}
}

func TestSaveConnFormWritesJSONSecrets(t *testing.T) {
	dir := t.TempDir()
	a := newApp()
	a.storeDir = dir
	a.openNewConn()
	a.formName = "dev"
	a.formHost = "10.0.0.2"
	a.formPort = "6380"
	a.formPass = "redis-secret"
	a.formSSH = true
	a.formSSHHost = "bastion"
	a.formSSHUser = "ops"
	a.formSSHPass = "ssh-secret"
	if !a.saveConnForm() {
		t.Fatalf("save: %s", a.formError)
	}
	f, err := store.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Connections) != 1 {
		t.Fatalf("%+v", f)
	}
	got := f.Connections[0]
	if got.Password != "redis-secret" || got.SSHPassword != "ssh-secret" || got.SSHHost != "bastion" {
		t.Fatalf("%+v", got)
	}
}

func TestDeleteConn(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{
		{ID: "a", Name: "A", Host: "127.0.0.1", Port: 6379},
		{ID: "b", Name: "B", Host: "127.0.0.1", Port: 6380},
	}
	a.selectedNav = "a"
	a.deleteSelectedConn()
	if len(a.connections) != 1 || a.connections[0].ID != "b" {
		t.Fatalf("%+v", a.connections)
	}
}

func TestCLIShowsCommandHint(t *testing.T) {
	a := newApp()
	a.showCLI = true
	a.cliInput = "HSET"
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("HSET key field value") || !tt.HasText("HSET user:1 name Ada") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestDangerousCLINeedsConfirm(t *testing.T) {
	a := newApp()
	a.cliInput = "FLUSHDB"
	a.submitCLI()
	if a.confirmOpen {
		t.Fatal("should not confirm while disconnected")
	}
	a.askConfirm(confirmDangerCLI, "FLUSHALL")
	if !a.confirmOpen || a.confirm != confirmDangerCLI {
		t.Fatalf("confirm %v open %v", a.confirm, a.confirmOpen)
	}
	if !cliparse.Dangerous([]string{"FLUSHALL"}) {
		t.Fatal("expected dangerous")
	}
}

func TestDatabaseListView(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{store.DefaultLocal()}
	a.connectedID = "local"
	a.client = &redisx.Client{}
	a.databases = []redisx.Database{{Index: 0, Keys: 3}, {Index: 1, Keys: 0}}
	a.activeDB = -1
	a.connOutline.Open.Add("local")
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("db0") || !tt.HasText("db1") || !tt.HasText("双击左侧数据库") || !tt.HasText("服务信息") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if tt.HasText("加载更多") || tt.HasText("添加 Key") {
		t.Fatal("key browser should stay hidden until a db is opened")
	}
}

func TestDoubleClickConnectedShowsServerInfo(t *testing.T) {
	a := inDBApp()
	a.info = redisx.Info{Version: "7.2.4", Role: "master", Memory: "1.0M", Clients: "3"}
	a.keys = []redisx.KeyInfo{{Key: "alpha", Type: "string"}}
	a.selected = "alpha"
	a.rebuildTree()
	a.selectedNav = "local"
	a.submitNav()
	if a.inDB() {
		t.Fatal("double-click connection should leave the database")
	}
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("服务信息") || !tt.HasText("7.2.4") || !tt.HasText("master") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if tt.HasText("添加 Key") {
		t.Fatal("key browser should hide when viewing server info")
	}
	if tt.HasText("执行") || tt.HasText("输入命令可补全") {
		t.Fatal("CLI should stay hidden until opened")
	}
	if !tt.HasText("CLI") {
		t.Fatal("server info should offer a CLI toggle")
	}
}

func TestOpenCLIFromServerInfo(t *testing.T) {
	a := inDBApp()
	a.info = redisx.Info{Version: "7.2.4", Role: "master"}
	tt := ui.NewTester(a.view, 1180, 760)
	if tt.HasText("执行") {
		t.Fatal("CLI should be closed")
	}
	if err := tt.Click("CLI"); err != nil {
		t.Fatal(err)
	}
	if !a.showCLI {
		t.Fatal("CLI should open")
	}
	if !tt.HasText("执行") || !tt.HasText("关闭") {
		t.Fatalf("cli texts %q", tt.Texts())
	}
	if err := tt.Click("关闭"); err != nil {
		t.Fatal(err)
	}
	if a.showCLI {
		t.Fatal("CLI should close")
	}
}

func TestAddKeyDialog(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{store.DefaultLocal()}
	a.connectedID = "local"
	a.client = &redisx.Client{}
	a.activeDB = 0
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("添加 Key") || !tt.HasText("没有匹配的 key") || !tt.HasText("导出") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("添加 Key"); err != nil {
		t.Fatal(err)
	}
	if !a.showAddKey {
		t.Fatal("dialog closed")
	}
	if !tt.HasText("类型") || !tt.HasText("TTL") {
		t.Fatalf("form texts %q", tt.Texts())
	}
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	if a.newKeyErr != "Key 不能为空" {
		t.Fatalf("newKeyErr %q", a.newKeyErr)
	}
	if !a.showAddKey {
		t.Fatal("dialog should stay open")
	}
}

func TestKeysToolbarExportNotAdd(t *testing.T) {
	a := inDBApp()
	a.keys = []redisx.KeyInfo{{Key: "alpha", Type: "string"}}
	a.rebuildTree()
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("导出") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("添加 Key"); err == nil {
		t.Fatal("top 添加 Key should be gone when keys exist")
	}
}

func TestAddKeyBlankContext(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{store.DefaultLocal()}
	a.connectedID = "local"
	a.client = &redisx.Client{}
	a.activeDB = 0
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.RightClick("没有匹配的 key"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if len(menu) == 0 || menu[0] != "添加 Key" {
		t.Fatalf("menu %q", menu)
	}
	if err := tt.ChooseMenuItem("添加 Key"); err != nil {
		t.Fatal(err)
	}
	if !a.showAddKey {
		t.Fatal("dialog closed")
	}
}

func TestRightClickOtherConnUsesRowMenu(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{
		{ID: "a", Name: "AlphaConn", Host: "127.0.0.1", Port: 6379},
		{ID: "b", Name: "BetaConn", Host: "10.0.0.1", Port: 6379},
	}
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.Click("AlphaConn"); err != nil {
		t.Fatal(err)
	}
	if err := tt.RightClick("BetaConn"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if containsMenu(menu, "新建连接") {
		t.Fatalf("focused A, right-click B got blank menu %q", menu)
	}
	if !containsMenu(menu, "连接") || !containsMenu(menu, "删除") {
		t.Fatalf("menu %q", menu)
	}
	tt.CloseMenu()
	r, ok := tt.Find("BetaConn")
	if !ok {
		t.Fatal("missing BetaConn")
	}
	tt.RightClickAt(r.X+r.W-2, r.Y+r.H/2)
	menu = tt.Menu()
	if containsMenu(menu, "新建连接") {
		t.Fatalf("right-click B row body got blank menu %q", menu)
	}
	if !containsMenu(menu, "删除") {
		t.Fatalf("row body menu %q", menu)
	}
	tt.CloseMenu()
	if r.X > 8 {
		tt.RightClickAt(r.X-8, r.Y+r.H/2)
		menu = tt.Menu()
		if containsMenu(menu, "新建连接") {
			t.Fatalf("right-click B indent got blank menu %q", menu)
		}
		if !containsMenu(menu, "删除") {
			t.Fatalf("indent menu %q", menu)
		}
	}
}

func TestRightClickOtherKeyUsesRowMenu(t *testing.T) {
	a := inDBApp()
	a.keys = []redisx.KeyInfo{
		{Key: "alpha", Type: "string"},
		{Key: "beta", Type: "string"},
	}
	a.rebuildTree()
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.Click("alpha"); err != nil {
		t.Fatal(err)
	}
	if err := tt.RightClick("beta"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if containsMenu(menu, "添加 Key") {
		t.Fatalf("focused A, right-click B got blank menu %q", menu)
	}
	if !containsMenu(menu, "复制 Key") || !containsMenu(menu, "删除") {
		t.Fatalf("menu %q", menu)
	}
	tt.CloseMenu()
	r, ok := tt.Find("beta")
	if !ok {
		t.Fatal("missing beta")
	}
	tt.RightClickAt(r.X+r.W-2, r.Y+r.H/2)
	menu = tt.Menu()
	if containsMenu(menu, "添加 Key") {
		t.Fatalf("right-click B row body got blank menu %q", menu)
	}
	if !containsMenu(menu, "删除") {
		t.Fatalf("row body menu %q", menu)
	}
	tt.CloseMenu()
	if r.X > 12 {
		tt.RightClickAt(r.X-12, r.Y+r.H/2)
		menu = tt.Menu()
		if containsMenu(menu, "添加 Key") {
			t.Fatalf("right-click B indent got blank menu %q", menu)
		}
		if !containsMenu(menu, "删除") {
			t.Fatalf("indent menu %q", menu)
		}
	}
}

func TestRightClickOtherKeyInListUsesRowMenu(t *testing.T) {
	a := inDBApp()
	a.keyMode = 1
	a.keys = []redisx.KeyInfo{
		{Key: "alpha", Type: "string"},
		{Key: "beta", Type: "string"},
	}
	a.rebuildTree()
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.Click("alpha"); err != nil {
		t.Fatal(err)
	}
	if err := tt.RightClick("beta"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if containsMenu(menu, "添加 Key") {
		t.Fatalf("focused A, right-click B got blank menu %q", menu)
	}
	if !containsMenu(menu, "复制 Key") || !containsMenu(menu, "删除") {
		t.Fatalf("menu %q", menu)
	}
}

func TestKeyRowContextMenu(t *testing.T) {
	a := inDBApp()
	a.keys = []redisx.KeyInfo{{Key: "alpha", Type: "string"}}
	a.rebuildTree()
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.RightClick("alpha"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if containsMenu(menu, "添加 Key") {
		t.Fatalf("key menu should be key ops, got %q", menu)
	}
	if !containsMenu(menu, "复制 Key") || !containsMenu(menu, "删除") || !containsMenu(menu, "同步") {
		t.Fatalf("menu %q", menu)
	}
}

func TestPrefixRowContextMenu(t *testing.T) {
	a := inDBApp()
	a.keys = []redisx.KeyInfo{{Key: "user:1", Type: "string"}}
	a.rebuildTree()
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.RightClick("user"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if containsMenu(menu, "添加 Key") {
		t.Fatalf("prefix menu should be prefix ops, got %q", menu)
	}
	if !containsMenu(menu, "复制前缀") || !containsMenu(menu, "导出") || !containsMenu(menu, "删除") || !containsMenu(menu, "同步") {
		t.Fatalf("menu %q", menu)
	}
	if err := tt.ChooseMenuItem("删除"); err != nil {
		t.Fatal(err)
	}
	if a.confirm != confirmDeletePrefix || a.confirmID != "user" {
		t.Fatalf("confirm %v id %q", a.confirm, a.confirmID)
	}
}

func TestKeysBlankContextImport(t *testing.T) {
	a := inDBApp()
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.RightClick("没有匹配的 key"); err != nil {
		t.Fatal(err)
	}
	if !containsMenu(tt.Menu(), "导入") || !containsMenu(tt.Menu(), "同步") {
		t.Fatalf("menu %q", tt.Menu())
	}
}

func TestDatabaseContextExport(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{store.DefaultLocal()}
	a.connectedID = "local"
	a.client = &redisx.Client{}
	a.databases = []redisx.Database{{Index: 0, Keys: 3}}
	a.activeDB = -1
	a.connOutline.Open.Add("local")
	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.RightClick("db0"); err != nil {
		t.Fatal(err)
	}
	if !containsMenu(tt.Menu(), "导出") || !containsMenu(tt.Menu(), "导入") || !containsMenu(tt.Menu(), "同步") {
		t.Fatalf("menu %q", tt.Menu())
	}
}

func TestExportProgressDialog(t *testing.T) {
	a := inDBApp()
	a.exporting = true
	a.exportDone = 2
	a.exportTotal = 10
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("正在导出") || !tt.HasText("2 / 10") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestExportDoneDialog(t *testing.T) {
	a := inDBApp()
	a.showExportDone = true
	a.exportCount = 3
	a.exportPath = "/tmp/keys.redis"
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("导出完成") || !tt.HasText("打开目录") || !tt.HasText("完成") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if !tt.HasText("/tmp/keys.redis") {
		t.Fatalf("path missing %q", tt.Texts())
	}
}

func TestImportProgressDialog(t *testing.T) {
	a := inDBApp()
	a.importing = true
	a.importDone = 4
	a.importTotal = 12
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("正在导入") || !tt.HasText("4 / 12") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestImportDoneDialog(t *testing.T) {
	a := inDBApp()
	a.showImportDone = true
	a.importResult = redisx.ImportResult{OK: 5, Failed: 1, Skipped: 2}
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("导入完成") || !tt.HasText("完成") || !tt.HasText("已导入 5 个 key。") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestExportFileName(t *testing.T) {
	if exportFileName("user:1") != "user-1.redis" {
		t.Fatal(exportFileName("user:1"))
	}
}

func inDBApp() *app {
	a := newApp()
	a.connections = []store.Connection{store.DefaultLocal()}
	a.connectedID = "local"
	a.client = &redisx.Client{}
	a.activeDB = 0
	return a
}

func containsMenu(menu []string, item string) bool {
	for _, s := range menu {
		if s == item {
			return true
		}
	}
	return false
}

func TestAddKeyValidation(t *testing.T) {
	if err := validateAddKey("", "string", "", "", "", "", ""); err != "Key 不能为空" {
		t.Fatalf("name: %q", err)
	}
	if err := validateAddKey("k", "hash", "v", "", "", "", ""); err != "field 不能为空" {
		t.Fatalf("hash: %q", err)
	}
	if err := validateAddKey("k", "set", "", "", "", "", ""); err != "member 不能为空" {
		t.Fatalf("set: %q", err)
	}
	if err := validateAddKey("k", "zset", "", "", "m", "x", ""); err != "score 无效" {
		t.Fatalf("score: %q", err)
	}
	if err := validateAddKey("k", "string", "v", "", "", "", "abc"); err != "TTL 必须是非负整数秒，空则永久" {
		t.Fatalf("ttl: %q", err)
	}
	if err := validateAddKey("k", "string", "v", "", "", "", ""); err != "" {
		t.Fatalf("ok: %q", err)
	}
	if err := validateAddKey("k", "zset", "", "", "m", "1.5", "60"); err != "" {
		t.Fatalf("zset: %q", err)
	}
}

func TestOpenAddKeyPrefillsPrefix(t *testing.T) {
	a := newApp()
	a.keys = []redisx.KeyInfo{{Key: "user:1", Type: "string"}}
	a.rebuildTree()
	a.selected = "user"
	a.activeDB = 0
	a.connectedID = "local"
	a.client = &redisx.Client{}
	a.openAddKey()
	if a.newKeyName != "user:" {
		t.Fatalf("prefill %q", a.newKeyName)
	}
}

func TestLeaveDatabase(t *testing.T) {
	a := newApp()
	a.connectedID = "local"
	a.client = &redisx.Client{}
	a.activeDB = 2
	a.keys = []redisx.KeyInfo{{Key: "a"}}
	a.selected = "a"
	a.leaveDatabase()
	if a.inDB() || a.selected != "" || len(a.keys) != 0 {
		t.Fatalf("activeDB %d selected %q keys %d", a.activeDB, a.selected, len(a.keys))
	}
}

func TestCollectionEditorsShowAdd(t *testing.T) {
	cases := []struct {
		typ  string
		want string
	}{
		{"hash", "添加 field"},
		{"list", "添加元素"},
		{"set", "添加 member"},
		{"zset", "添加 member"},
	}
	for _, tc := range cases {
		a := inDBApp()
		a.selected = "k"
		a.tree.IsKey = map[string]bool{"k": true}
		a.val = redisx.Value{KeyInfo: redisx.KeyInfo{Key: "k", Type: tc.typ}}
		tt := ui.NewTester(a.view, 1180, 760)
		if !tt.HasText(tc.want) {
			t.Fatalf("%s: texts %q", tc.typ, tt.Texts())
		}
	}
}

func TestSaveCollectionRowsValidate(t *testing.T) {
	a := inDBApp()
	a.val = redisx.Value{
		KeyInfo: redisx.KeyInfo{Key: "h", Type: "hash"},
		Hash:    []redisx.Field{{Name: "a", Value: "1"}},
	}
	a.syncMemberEdits()
	a.val.Hash[0].Name = ""
	a.saveHashRow(0)
	if a.valErr != "field 不能为空" {
		t.Fatalf("hash: %q", a.valErr)
	}

	a.val = redisx.Value{
		KeyInfo: redisx.KeyInfo{Key: "s", Type: "set"},
		Set:     []string{"old"},
	}
	a.syncMemberEdits()
	a.val.Set[0] = ""
	a.saveSetRow(0)
	if a.valErr != "member 不能为空" {
		t.Fatalf("set: %q", a.valErr)
	}

	a.val = redisx.Value{
		KeyInfo: redisx.KeyInfo{Key: "z", Type: "zset"},
		ZSet:    []redisx.ZMember{{Score: 1, Member: "m"}},
	}
	a.syncMemberEdits()
	a.zsetScoreEdits[0] = "x"
	a.saveZSetRow(0)
	if a.valErr != "score 无效" {
		t.Fatalf("zset: %q", a.valErr)
	}
}

func TestCollectionRowsShowSave(t *testing.T) {
	cases := []struct {
		val  redisx.Value
		want string
	}{
		{
			val: redisx.Value{
				KeyInfo: redisx.KeyInfo{Key: "k", Type: "hash"},
				Hash:    []redisx.Field{{Name: "f", Value: "v"}},
			},
			want: "添加 field",
		},
		{
			val: redisx.Value{
				KeyInfo: redisx.KeyInfo{Key: "k", Type: "set"},
				Set:     []string{"m"},
			},
			want: "添加 member",
		},
		{
			val: redisx.Value{
				KeyInfo: redisx.KeyInfo{Key: "k", Type: "zset"},
				ZSet:    []redisx.ZMember{{Score: 1.5, Member: "m"}},
			},
			want: "添加 member",
		},
	}
	for _, tc := range cases {
		a := inDBApp()
		a.selected = "k"
		a.tree.IsKey = map[string]bool{"k": true}
		a.val = tc.val
		tt := ui.NewTester(a.view, 1180, 760)
		texts := strings.Join(tt.Texts(), "\n")
		if !strings.Contains(texts, "保存") || !strings.Contains(texts, tc.want) {
			t.Fatalf("%s: texts %q", tc.val.Type, tt.Texts())
		}
	}
}

func TestAddZMemberValidatesScore(t *testing.T) {
	a := inDBApp()
	a.val.Key = "z"
	a.newMember = "m"
	a.newScore = "x"
	a.addZMember()
	if a.valErr != "score 无效" {
		t.Fatalf("%q", a.valErr)
	}
}

func TestChosenKeyNames(t *testing.T) {
	a := inDBApp()
	a.keys = []redisx.KeyInfo{{Key: "a"}, {Key: "b"}}
	a.rebuildTree()
	a.keySel.Add("a")
	a.keySel.Add("b")
	got := a.chosenKeyNames()
	if len(got) != 2 {
		t.Fatalf("%q", got)
	}
}

func TestParkFocusKeepsOtherLive(t *testing.T) {
	a := inDBApp()
	a.parkFocus()
	if a.parked["local"] == nil || a.connectedID != "" {
		t.Fatal("expected parked local")
	}
	if a.isLive("local") == false {
		t.Fatal("parked should still be live")
	}
}

func TestSetLastDB(t *testing.T) {
	a := inDBApp()
	a.setLastDB(2)
	if a.connections[0].LastDB == nil || *a.connections[0].LastDB != 2 {
		t.Fatalf("%v", a.connections[0].LastDB)
	}
}

func TestSetStringModeJSON(t *testing.T) {
	a := newApp()
	a.stringEdit = `{"a":1}`
	a.setStringMode(2)
	if a.stringMode != 2 || !strings.Contains(a.stringEdit, "\n") {
		t.Fatalf("mode %d edit %q", a.stringMode, a.stringEdit)
	}
}

func TestOpenOtherConnectionDB(t *testing.T) {
	a := newApp()
	a.connections = []store.Connection{
		{ID: "a", Name: "A", Host: "127.0.0.1", Port: 6379},
		{ID: "b", Name: "B", Host: "127.0.0.1", Port: 6380},
	}
	a.client = &redisx.Client{}
	a.connectedID = "b"
	a.databases = []redisx.Database{{Index: 0}}
	a.activeDB = 0
	a.parked = map[string]*live{
		"a": {client: &redisx.Client{}, databases: []redisx.Database{{Index: 1}}, activeDB: -1},
	}
	a.selectedNav = dbNodeID("a", 1)
	a.submitNav()
	if a.connectedID != "a" {
		t.Fatalf("focus %s", a.connectedID)
	}
	if a.parked["b"] == nil {
		t.Fatal("connection b should stay live")
	}
}

func TestKeySearchHint(t *testing.T) {
	if !strings.Contains(matchHint("user"), "user*") {
		t.Fatal(matchHint("user"))
	}
}

func TestOpenSyncDialog(t *testing.T) {
	a := inDBApp()
	a.connections = append(a.connections, store.Connection{ID: "other", Name: "Other", Host: "10.0.0.1", Port: 6379})
	a.openSync("local", 0, 1, "user", nil)
	if !a.showSync || a.syncMode != 1 || a.syncPrefix != "user" || a.syncDestConn != "other" {
		t.Fatalf("sync %+v dest %s", a, a.syncDestConn)
	}
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("同步数据") || !tt.HasText("开始同步") || !tt.HasText("目标连接") {
		t.Fatalf("texts %q", tt.Texts())
	}
}

func TestValidateSyncSameDB(t *testing.T) {
	a := inDBApp()
	a.openSync("local", 0, 0, "", nil)
	a.syncDestConn = "local"
	a.syncDestDB = "db0"
	if a.validateSync() != "目标和源不能是同一个库" {
		t.Fatal(a.validateSync())
	}
}

func TestParseDBLabel(t *testing.T) {
	n, ok := parseDBLabel("db3")
	if !ok || n != 3 {
		t.Fatal(n, ok)
	}
	if _, ok := parseDBLabel("nope"); ok {
		t.Fatal("expected fail")
	}
}

func TestSyncProgressDialog(t *testing.T) {
	a := inDBApp()
	a.syncing = true
	a.syncDone = 3
	a.syncTotal = 9
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("正在同步") || !tt.HasText("3 / 9") {
		t.Fatalf("texts %q", tt.Texts())
	}
}
