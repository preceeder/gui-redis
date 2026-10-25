# RedisCli

原生桌面 Redis 客户端。用 [MyGo](https://github.com/egoist/mygo) 做界面，不依赖 Electron / WebView。当前只做单机 Redis（Standalone）。

版本 `0.1.1` · 包名 `com.preceeder.rediscli`

## 功能

**连接**

- 保存多条连接，双击连接，再双击数据库进入
- 可同时保持多个连接；切到另一条连接时，原来的连接会挂起，需要时自动重连
- SSH 隧道（密码或私钥）
- 本地 HTTP / SOCKS5 代理，可与 SSH 叠用
- 密码、私钥写在本地 `connections.json`（权限 `0600`），不进系统钥匙串
- 密码输入框自带眼睛图标，只展开当前这一格

**Key**

- 树形 / 列表两种浏览，按前缀展开
- 搜索、按类型过滤（string / hash / list / set / zset / stream）
- 空白处右击添加 Key；前缀或单个 Key 上右击做对应操作
- 多选批量删除
- 导出、导入（DUMP / RESTORE），带进度条；导出完成后可打开所在目录
- 库、前缀、选中 Key 之间同步数据

**值编辑**

- string：文本 / JSON / Hex
- hash / set / zset / list / stream：增删改成员，支持加载更多
- 复制 Key、Value、等价命令；改 TTL、重命名、删除

**运维**

- 双击已连接的节点查看服务信息
- 慢日志
- CLI 默认隐藏，在服务信息里打开后出现在第三列；输入有命令提示

## 环境

- Go 1.27+（本仓库 `go.mod` 为 `1.27.3`）
- [MyGo](https://github.com/egoist/mygo) `v0.6.2`（已声明为 Go tool）
- macOS 或 Windows（无 cgo，可交叉编译）

## 开发

```bash
make dev          # 热重载
make test         # 测试
make run          # 编译当前系统并启动
```

## 打包

```bash
make build        # 当前系统生产包
make mac          # macOS universal（.app / .dmg）
make windows      # Windows amd64 + arm64
make all          # mac + windows

make bin          # 当前系统裸二进制（更快，不走安装包）
make bin-mac
make bin-windows
```

产物在 `build/`。Windows 用 `-H windowsgui`，双击不会弹出黑色控制台。

## 使用

1. 左侧没有连接时点「新建连接」，或在空白处右击。
2. 双击连接，展开数据库；再双击某个 db 进入 Key 列表。
3. 已连接时再双击该连接，查看服务信息；需要命令行时点「CLI」。
4. Key 区域顶部可导出 / 导入 / 同步；添加 Key 走右击菜单。

连接配置在应用用户数据目录下的 `connections.json`，文件里含密码和私钥，不要提交到 git，也不要随手拷给别人。

## 技术

| 部分 | 依赖 |
| --- | --- |
| UI | `github.com/egoist/mygo` |
| Redis | `github.com/redis/go-redis/v9` |
| SSH | `golang.org/x/crypto/ssh` |
| 代理 | `golang.org/x/net/proxy` + HTTP CONNECT |
