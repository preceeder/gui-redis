package redisx

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"redis/internal/format"
)

// Config is a standalone Redis endpoint.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	DB       int
	SSH      SSHConfig
	Proxy    ProxyConfig
}

// KeyInfo is metadata for one key.
type KeyInfo struct {
	Key    string
	Type   string
	TTL    int64 // PTTL milliseconds; -1 means no expire
	Len    int64
	Memory int64 // MEMORY USAGE bytes; 0 if unknown
}

// Field is a hash field.
type Field struct {
	Name  string
	Value string
}

// ZMember is a sorted-set member.
type ZMember struct {
	Score  float64
	Member string
}

// Value is the loaded contents of a key.
type Value struct {
	KeyInfo
	TooLarge  bool
	Truncated bool
	Total     int64
	String    string
	Hash      []Field
	List      []string
	Set       []string
	ZSet      []ZMember
	Stream    []StreamEntry
	Cursor    uint64 // next HSCAN/SSCAN cursor, or next list/zset offset
	StreamID  string // last loaded stream ID
	Note      string
}

// StreamEntry is one XRANGE item.
type StreamEntry struct {
	ID     string
	Fields []Field
}

// Info is a short INFO snapshot.
type InfoField struct {
	Label string
	Value string
}

type InfoSection struct {
	Title  string
	Fields []InfoField
}

type Info struct {
	Version  string
	Role     string
	Memory   string
	Clients  string
	OS       string
	Uptime   string
	DBSize   int64
	Sections []InfoSection
}

// Database is one logical Redis DB (SELECT n).
type Database struct {
	Index   int
	Keys    int64
	Expires int64
}

// Client wraps a go-redis standalone client.
type Client struct {
	rdb *redis.Client
	cfg Config
	ssh sshCloser
}

type sshCloser interface{ Close() error }

// Dial connects and PINGs.
func Dial(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Port <= 0 {
		cfg.Port = 6379
	}
	opt := &redis.Options{
		Addr:         net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  8 * time.Second,
		WriteTimeout: 8 * time.Second,
	}
	baseDial, err := cfg.Proxy.dialer()
	if err != nil {
		return nil, err
	}
	var tun sshCloser
	if cfg.SSH.Enabled {
		cli, err := dialSSH(ctx, cfg.SSH, baseDial)
		if err != nil {
			return nil, err
		}
		tun = cli
		opt.Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialSSHTunnel(ctx, cli, network, addr)
		}
		opt.DialTimeout = 8 * time.Second
	} else if cfg.Proxy.Enabled {
		opt.Dialer = baseDial
		opt.DialTimeout = 8 * time.Second
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		if tun != nil {
			_ = tun.Close()
		}
		return nil, err
	}
	return &Client{rdb: rdb, cfg: cfg, ssh: tun}, nil
}

// Close shuts the client down.
func (c *Client) Close() {
	if c == nil {
		return
	}
	if c.rdb != nil {
		_ = c.rdb.Close()
		c.rdb = nil
	}
	if c.ssh != nil {
		_ = c.ssh.Close()
		c.ssh = nil
	}
}

// Ping checks the connection.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Scan returns the next SCAN page.
func (c *Client) Scan(ctx context.Context, cursor uint64, match string, count int64) (keys []string, next uint64, err error) {
	if count <= 0 {
		count = format.ScanCount
	}
	return c.rdb.Scan(ctx, cursor, match, count).Result()
}

// KeyInfos loads TYPE, PTTL and a type-specific length for each key.
func (c *Client) KeyInfos(ctx context.Context, keys []string) ([]KeyInfo, error) {
	out := make([]KeyInfo, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	pipe := c.rdb.Pipeline()
	types := make([]*redis.StatusCmd, len(keys))
	ttls := make([]*redis.DurationCmd, len(keys))
	for i, k := range keys {
		out[i].Key = k
		types[i] = pipe.Type(ctx, k)
		ttls[i] = pipe.PTTL(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	lenPipe := c.rdb.Pipeline()
	type lenCmd struct {
		i int
		n *redis.IntCmd
	}
	var lens []lenCmd
	for i, k := range keys {
		t, err := types[i].Result()
		if err != nil {
			t = "none"
		}
		out[i].Type = t
		if d, err := ttls[i].Result(); err == nil {
			out[i].TTL = int64(d / time.Millisecond)
		} else {
			out[i].TTL = -1
		}
		switch t {
		case "string":
			lens = append(lens, lenCmd{i, lenPipe.StrLen(ctx, k)})
		case "hash":
			lens = append(lens, lenCmd{i, lenPipe.HLen(ctx, k)})
		case "list":
			lens = append(lens, lenCmd{i, lenPipe.LLen(ctx, k)})
		case "set":
			lens = append(lens, lenCmd{i, lenPipe.SCard(ctx, k)})
		case "zset":
			lens = append(lens, lenCmd{i, lenPipe.ZCard(ctx, k)})
		case "stream":
			lens = append(lens, lenCmd{i, lenPipe.XLen(ctx, k)})
		}
	}
	if len(lens) > 0 {
		if _, err := lenPipe.Exec(ctx); err != nil && err != redis.Nil {
			return out, nil
		}
		for _, cmd := range lens {
			if n, err := cmd.n.Result(); err == nil {
				out[cmd.i].Len = n
			}
		}
	}
	memPipe := c.rdb.Pipeline()
	mems := make([]*redis.IntCmd, len(keys))
	for i, k := range keys {
		mems[i] = memPipe.MemoryUsage(ctx, k)
	}
	if _, err := memPipe.Exec(ctx); err == nil {
		for i, cmd := range mems {
			if n, err := cmd.Result(); err == nil {
				out[i].Memory = n
			}
		}
	}
	return out, nil
}

// LoadValue reads a key's contents, capping large values.
func (c *Client) LoadValue(ctx context.Context, key string) (Value, error) {
	infos, err := c.KeyInfos(ctx, []string{key})
	if err != nil {
		return Value{}, err
	}
	if len(infos) == 0 || infos[0].Type == "none" {
		return Value{}, fmt.Errorf("key 不存在")
	}
	info := infos[0]
	v := Value{KeyInfo: info, Total: info.Len}
	switch info.Type {
	case "string":
		if info.Len > format.MaxStringBytes {
			v.TooLarge = true
			return v, nil
		}
		s, err := c.rdb.Get(ctx, key).Result()
		if err != nil {
			return v, err
		}
		v.String = s
	case "hash", "list", "set", "zset", "stream":
		if err := c.loadPage(ctx, &v, true); err != nil {
			return v, err
		}
	default:
		v.Note = "暂不支持编辑 " + info.Type + " 类型"
	}
	return v, nil
}

func (c *Client) LoadMore(ctx context.Context, prev Value) (Value, error) {
	if !prev.Truncated {
		return prev, nil
	}
	if err := c.loadPage(ctx, &prev, false); err != nil {
		return prev, err
	}
	return prev, nil
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

// CreateSpec is the payload for creating a key that does not yet exist.
type CreateSpec struct {
	Key    string
	Type   string
	Value  string
	Field  string
	Member string
	Score  float64
	TTL    int64 // seconds; 0 means persist
}

func (c *Client) Create(ctx context.Context, spec CreateSpec) error {
	ok, err := c.Exists(ctx, spec.Key)
	if err != nil {
		return err
	}
	if ok {
		existing, typErr := c.rdb.Type(ctx, spec.Key).Result()
		if typErr == nil && existing == spec.Type && collectionType(existing) {
			return fmt.Errorf("Key 已存在，请打开后在右侧继续添加")
		}
		return fmt.Errorf("Key 已存在")
	}
	switch spec.Type {
	case "string":
		err = c.rdb.Set(ctx, spec.Key, spec.Value, 0).Err()
	case "hash":
		err = c.HSet(ctx, spec.Key, spec.Field, spec.Value)
	case "list":
		err = c.RPush(ctx, spec.Key, spec.Value)
	case "set":
		err = c.SAdd(ctx, spec.Key, spec.Member)
	case "zset":
		err = c.ZAdd(ctx, spec.Key, spec.Score, spec.Member)
	case "stream":
		err = c.XAdd(ctx, spec.Key, spec.Field, spec.Value)
	default:
		return fmt.Errorf("不支持的类型 %s", spec.Type)
	}
	if err != nil {
		return err
	}
	if spec.TTL > 0 {
		return c.Expire(ctx, spec.Key, spec.TTL)
	}
	return nil
}

func collectionType(typ string) bool {
	switch typ {
	case "hash", "list", "set", "zset", "stream":
		return true
	}
	return false
}

func (c *Client) SetString(ctx context.Context, key, value string) error {
	return c.rdb.Set(ctx, key, value, 0).Err()
}

func (c *Client) HSet(ctx context.Context, key, field, value string) error {
	return c.rdb.HSet(ctx, key, field, value).Err()
}

func (c *Client) HDel(ctx context.Context, key, field string) error {
	return c.rdb.HDel(ctx, key, field).Err()
}

func (c *Client) HRename(ctx context.Context, key, from, to, value string) error {
	if to == "" {
		return fmt.Errorf("field 不能为空")
	}
	if from == to {
		return c.HSet(ctx, key, to, value)
	}
	exists, err := c.rdb.HExists(ctx, key, to).Result()
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("field %s 已存在", to)
	}
	pipe := c.rdb.TxPipeline()
	pipe.HSet(ctx, key, to, value)
	pipe.HDel(ctx, key, from)
	_, err = pipe.Exec(ctx)
	return err
}

func (c *Client) LSet(ctx context.Context, key string, index int64, value string) error {
	return c.rdb.LSet(ctx, key, index, value).Err()
}

func (c *Client) LPush(ctx context.Context, key, value string) error {
	return c.rdb.LPush(ctx, key, value).Err()
}

func (c *Client) RPush(ctx context.Context, key, value string) error {
	return c.rdb.RPush(ctx, key, value).Err()
}

func (c *Client) LInsert(ctx context.Context, key string, before bool, pivot, value string) error {
	where := "AFTER"
	if before {
		where = "BEFORE"
	}
	n, err := c.rdb.LInsert(ctx, key, where, pivot, value).Result()
	if err != nil {
		return err
	}
	if n == -1 {
		return fmt.Errorf("未找到插入位置")
	}
	return nil
}

func (c *Client) LDelIndex(ctx context.Context, key string, index int64) error {
	tomb := fmt.Sprintf("\x00rediscli-del-%d", time.Now().UnixNano())
	pipe := c.rdb.TxPipeline()
	pipe.LSet(ctx, key, index, tomb)
	pipe.LRem(ctx, key, 1, tomb)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Client) SAdd(ctx context.Context, key, member string) error {
	return c.rdb.SAdd(ctx, key, member).Err()
}

func (c *Client) SRem(ctx context.Context, key, member string) error {
	return c.rdb.SRem(ctx, key, member).Err()
}

func (c *Client) SRename(ctx context.Context, key, from, to string) error {
	if to == "" {
		return fmt.Errorf("member 不能为空")
	}
	if from == to {
		return nil
	}
	ok, err := c.rdb.SIsMember(ctx, key, to).Result()
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("member %s 已存在", to)
	}
	pipe := c.rdb.TxPipeline()
	pipe.SAdd(ctx, key, to)
	pipe.SRem(ctx, key, from)
	_, err = pipe.Exec(ctx)
	return err
}

func (c *Client) ZAdd(ctx context.Context, key string, score float64, member string) error {
	return c.rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

func (c *Client) ZRem(ctx context.Context, key, member string) error {
	return c.rdb.ZRem(ctx, key, member).Err()
}

func (c *Client) ZUpdate(ctx context.Context, key, from, to string, score float64) error {
	if to == "" {
		return fmt.Errorf("member 不能为空")
	}
	if from == to {
		return c.ZAdd(ctx, key, score, to)
	}
	if _, err := c.rdb.ZScore(ctx, key, to).Result(); err == nil {
		return fmt.Errorf("member %s 已存在", to)
	} else if err != redis.Nil {
		return err
	}
	pipe := c.rdb.TxPipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: to})
	pipe.ZRem(ctx, key, from)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

func (c *Client) Rename(ctx context.Context, key, dest string) error {
	ok, err := c.rdb.RenameNX(ctx, key, dest).Result()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("目标 key 已存在")
	}
	return nil
}

func (c *Client) Expire(ctx context.Context, key string, seconds int64) error {
	if seconds <= 0 {
		return c.rdb.Persist(ctx, key).Err()
	}
	return c.rdb.Expire(ctx, key, time.Duration(seconds)*time.Second).Err()
}

func (c *Client) Do(ctx context.Context, args ...any) (any, error) {
	return c.rdb.Do(ctx, args...).Result()
}

func (c *Client) CurrentDB() int { return c.cfg.DB }

func (c *Client) Ready() bool { return c != nil && c.rdb != nil }

// SwitchDB reconnects the client onto another logical database.
func (c *Client) SwitchDB(ctx context.Context, db int) error {
	if db < 0 {
		return fmt.Errorf("无效的数据库")
	}
	if c.cfg.DB == db {
		return c.Ping(ctx)
	}
	cfg := c.cfg
	cfg.DB = db
	next, err := Dial(ctx, cfg)
	if err != nil {
		return err
	}
	old, oldSSH := c.rdb, c.ssh
	c.rdb = next.rdb
	c.ssh = next.ssh
	c.cfg = cfg
	_ = old.Close()
	if oldSSH != nil {
		_ = oldSSH.Close()
	}
	return nil
}

// Databases lists logical DBs. CONFIG GET databases 给出总数，INFO keyspace 给出各库 key 数。
func (c *Client) Databases(ctx context.Context) ([]Database, error) {
	count := 16
	if n, err := configDatabaseCount(ctx, c.rdb); err == nil && n > 0 {
		count = n
	}
	byIndex := map[int]Database{}
	if raw, err := c.rdb.Info(ctx, "keyspace").Result(); err == nil {
		for _, db := range ParseKeyspace(raw) {
			byIndex[db.Index] = db
			if db.Index+1 > count {
				count = db.Index + 1
			}
		}
	}
	out := make([]Database, count)
	for i := 0; i < count; i++ {
		if d, ok := byIndex[i]; ok {
			out[i] = d
			continue
		}
		out[i] = Database{Index: i}
	}
	return out, nil
}

func configDatabaseCount(ctx context.Context, rdb *redis.Client) (int, error) {
	m, err := rdb.ConfigGet(ctx, "databases").Result()
	if err != nil {
		return 0, err
	}
	s, ok := m["databases"]
	if !ok {
		return 0, fmt.Errorf("no databases field")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid databases %q", s)
	}
	return n, nil
}

// ParseKeyspace reads INFO keyspace lines such as db0:keys=3,expires=1,avg_ttl=0.
func ParseKeyspace(s string) []Database {
	var out []Database
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "db") {
			continue
		}
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimPrefix(name, "db"))
		if err != nil {
			continue
		}
		d := Database{Index: idx}
		for part := range strings.SplitSeq(rest, ",") {
			k, v, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			n, _ := strconv.ParseInt(v, 10, 64)
			switch k {
			case "keys":
				d.Keys = n
			case "expires":
				d.Expires = n
			}
		}
		out = append(out, d)
	}
	return out
}

func (c *Client) Snapshot(ctx context.Context) (Info, error) {
	raw, err := c.rdb.Info(ctx).Result()
	if err != nil {
		return Info{}, err
	}
	n, _ := c.rdb.DBSize(ctx).Result()
	return BuildInfo(parseInfo(raw), n), nil
}
