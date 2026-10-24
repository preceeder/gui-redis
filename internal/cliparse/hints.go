package cliparse

import (
	"strings"
	"unicode"
)

// Hint is a Redis command syntax, example, and short description.
type Hint struct {
	Name    string
	Syntax  string
	Example string
	Desc    string
}

var commands = []Hint{
	{"GET", "GET key", "GET user:1", "读取字符串"},
	{"SET", "SET key value", "SET user:1 Ada", "写入字符串"},
	{"MGET", "MGET key [key ...]", "MGET user:1 user:2", "批量读取字符串"},
	{"MSET", "MSET key value [key value ...]", "MSET user:1 Ada user:2 Bob", "批量写入字符串"},
	{"INCR", "INCR key", "INCR views", "整数加 1"},
	{"DECR", "DECR key", "DECR stock", "整数减 1"},
	{"APPEND", "APPEND key value", "APPEND log :ok", "追加字符串"},
	{"STRLEN", "STRLEN key", "STRLEN user:1", "字符串长度"},
	{"HGET", "HGET key field", "HGET user:1 name", "读取 hash 字段"},
	{"HSET", "HSET key field value", "HSET user:1 name Ada", "写入 hash 字段"},
	{"HGETALL", "HGETALL key", "HGETALL user:1", "读取整个 hash"},
	{"HDEL", "HDEL key field [field ...]", "HDEL user:1 name", "删除 hash 字段"},
	{"HKEYS", "HKEYS key", "HKEYS user:1", "列出 hash 字段名"},
	{"HVALS", "HVALS key", "HVALS user:1", "列出 hash 字段值"},
	{"HLEN", "HLEN key", "HLEN user:1", "hash 字段数量"},
	{"HEXISTS", "HEXISTS key field", "HEXISTS user:1 name", "判断 hash 字段"},
	{"LPUSH", "LPUSH key value [value ...]", "LPUSH queue job1", "从左侧插入列表"},
	{"RPUSH", "RPUSH key value [value ...]", "RPUSH queue job1", "从右侧插入列表"},
	{"LPOP", "LPOP key", "LPOP queue", "从左侧弹出"},
	{"RPOP", "RPOP key", "RPOP queue", "从右侧弹出"},
	{"LRANGE", "LRANGE key start stop", "LRANGE queue 0 -1", "读取列表区间"},
	{"LLEN", "LLEN key", "LLEN queue", "列表长度"},
	{"LINDEX", "LINDEX key index", "LINDEX queue 0", "读取列表下标"},
	{"SADD", "SADD key member [member ...]", "SADD tags go", "添加集合元素"},
	{"SREM", "SREM key member [member ...]", "SREM tags go", "删除集合元素"},
	{"SMEMBERS", "SMEMBERS key", "SMEMBERS tags", "列出集合元素"},
	{"SISMEMBER", "SISMEMBER key member", "SISMEMBER tags go", "判断集合元素"},
	{"SCARD", "SCARD key", "SCARD tags", "集合大小"},
	{"ZADD", "ZADD key score member", "ZADD rank 100 ada", "添加有序集合元素"},
	{"ZREM", "ZREM key member [member ...]", "ZREM rank ada", "删除有序集合元素"},
	{"ZRANGE", "ZRANGE key start stop [WITHSCORES]", "ZRANGE rank 0 -1 WITHSCORES", "按排名读取"},
	{"ZREVRANGE", "ZREVRANGE key start stop [WITHSCORES]", "ZREVRANGE rank 0 9 WITHSCORES", "按排名倒序读取"},
	{"ZSCORE", "ZSCORE key member", "ZSCORE rank ada", "读取分数"},
	{"ZCARD", "ZCARD key", "ZCARD rank", "有序集合大小"},
	{"DEL", "DEL key [key ...]", "DEL user:1", "删除 key"},
	{"EXISTS", "EXISTS key [key ...]", "EXISTS user:1", "判断 key 是否存在"},
	{"TYPE", "TYPE key", "TYPE user:1", "查看 key 类型"},
	{"TTL", "TTL key", "TTL user:1", "剩余过期秒数"},
	{"PTTL", "PTTL key", "PTTL user:1", "剩余过期毫秒"},
	{"EXPIRE", "EXPIRE key seconds", "EXPIRE session 3600", "设置过期秒数"},
	{"PERSIST", "PERSIST key", "PERSIST session", "移除过期时间"},
	{"RENAME", "RENAME key newkey", "RENAME user:1 user:2", "重命名 key"},
	{"SCAN", "SCAN cursor [MATCH pattern] [COUNT n]", "SCAN 0 MATCH user:* COUNT 100", "扫描 key"},
	{"SELECT", "SELECT index", "SELECT 0", "切换数据库"},
	{"DBSIZE", "DBSIZE", "DBSIZE", "当前库 key 数量"},
	{"PING", "PING [message]", "PING", "测试连通"},
	{"INFO", "INFO [section]", "INFO server", "查看服务信息"},
	{"ECHO", "ECHO message", "ECHO hello", "原样返回"},
	{"TIME", "TIME", "TIME", "服务器时间"},
	{"KEYS", "KEYS pattern", "KEYS user:*", "按模式列 key，库大时很慢"},
	{"FLUSHDB", "FLUSHDB", "FLUSHDB", "清空当前库"},
	{"FLUSHALL", "FLUSHALL", "FLUSHALL", "清空全部库"},
	{"CONFIG", "CONFIG GET parameter", "CONFIG GET databases", "读取配置"},
}

// Suggestions are command templates for CLI autocomplete.
func Suggestions() []string {
	out := make([]string, len(commands))
	for i, h := range commands {
		out[i] = h.Syntax
	}
	return out
}

// Lookup finds a command hint from the first token of a CLI line.
func Lookup(line string) (Hint, bool) {
	name := firstToken(line)
	if name == "" {
		return Hint{}, false
	}
	name = strings.ToUpper(name)
	var prefixes []Hint
	for _, h := range commands {
		if h.Name == name {
			return h, true
		}
		if strings.HasPrefix(h.Name, name) {
			prefixes = append(prefixes, h)
		}
	}
	if len(prefixes) == 1 {
		return prefixes[0], true
	}
	return Hint{}, false
}

// FormatHint is the one-line help under the CLI input.
func FormatHint(h Hint) string {
	return h.Syntax + "  ·  " + h.Desc + "，例如 " + h.Example
}

// PrefixMatch is the SCAN pattern for keys under prefix, using sep.
func PrefixMatch(prefix, sep string) string {
	if prefix == "" {
		return "*"
	}
	if sep == "" {
		return prefix + "*"
	}
	return prefix + sep + "*"
}

func firstToken(line string) string {
	line = strings.TrimLeftFunc(line, unicode.IsSpace)
	if line == "" {
		return ""
	}
	i := strings.IndexFunc(line, unicode.IsSpace)
	if i < 0 {
		return line
	}
	return line[:i]
}
