package redisx

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type infoSpec struct {
	title  string
	fields [][2]string // label, redis INFO key
}

var infoSpecs = []infoSpec{
	{"服务器", [][2]string{
		{"版本", "redis_version"},
		{"模式", "redis_mode"},
		{"系统", "os"},
		{"架构", "arch_bits"},
		{"进程", "process_id"},
		{"端口", "tcp_port"},
		{"运行", "uptime_in_seconds"},
		{"可执行文件", "executable"},
		{"配置文件", "config_file"},
		{"Hz", "hz"},
		{"IO 线程", "io_threads_active"},
	}},
	{"客户端", [][2]string{
		{"已连接", "connected_clients"},
		{"阻塞", "blocked_clients"},
		{"跟踪", "tracking_clients"},
		{"最大连接", "maxclients"},
	}},
	{"内存", [][2]string{
		{"已用", "used_memory_human"},
		{"峰值", "used_memory_peak_human"},
		{"RSS", "used_memory_rss_human"},
		{"Lua", "used_memory_lua_human"},
		{"碎片率", "mem_fragmentation_ratio"},
		{"上限", "maxmemory_human"},
		{"淘汰策略", "maxmemory_policy"},
		{"分配器", "mem_allocator"},
	}},
	{"持久化", [][2]string{
		{"上次 RDB", "rdb_last_save_time"},
		{"未保存变更", "rdb_changes_since_last_save"},
		{"上次 BGSAVE", "rdb_last_bgsave_status"},
		{"AOF", "aof_enabled"},
		{"AOF 重写中", "aof_rewrite_in_progress"},
		{"正在载入", "loading"},
	}},
	{"统计", [][2]string{
		{"总连接", "total_connections_received"},
		{"总命令", "total_commands_processed"},
		{"QPS", "instantaneous_ops_per_sec"},
		{"过期 Key", "expired_keys"},
		{"淘汰 Key", "evicted_keys"},
		{"命中", "keyspace_hits"},
		{"未命中", "keyspace_misses"},
		{"命中率", "keyspace_hit_rate"},
		{"拒绝连接", "rejected_connections"},
	}},
	{"复制", [][2]string{
		{"角色", "role"},
		{"从节点", "connected_slaves"},
		{"主库", "master_host"},
		{"主库端口", "master_port"},
		{"主从链路", "master_link_status"},
		{"复制偏移", "master_repl_offset"},
	}},
	{"CPU", [][2]string{
		{"系统", "used_cpu_sys"},
		{"用户", "used_cpu_user"},
	}},
	{"集群", [][2]string{
		{"已启用", "cluster_enabled"},
	}},
}

// BuildInfo maps INFO fields into the toolbar snapshot and grouped sections.
func BuildInfo(fields map[string]string, dbsize int64) Info {
	info := Info{
		Version: fields["redis_version"],
		Role:    fields["role"],
		Memory:  fields["used_memory_human"],
		Clients: fields["connected_clients"],
		OS:      fields["os"],
		Uptime:  formatUptime(fields),
		DBSize:  dbsize,
	}
	for _, spec := range infoSpecs {
		sec := InfoSection{Title: spec.title}
		for _, pair := range spec.fields {
			if v := formatInfoValue(pair[1], fields); v != "" {
				sec.Fields = append(sec.Fields, InfoField{Label: pair[0], Value: v})
			}
		}
		if len(sec.Fields) > 0 {
			info.Sections = append(info.Sections, sec)
		}
	}
	return info
}

func formatInfoValue(key string, fields map[string]string) string {
	switch key {
	case "uptime_in_seconds":
		return formatUptime(fields)
	case "rdb_last_save_time":
		return formatUnix(fields[key])
	case "aof_enabled", "aof_rewrite_in_progress", "loading", "cluster_enabled", "io_threads_active":
		return formatFlag(fields[key])
	case "keyspace_hit_rate":
		return formatHitRate(fields["keyspace_hits"], fields["keyspace_misses"])
	case "arch_bits":
		if fields[key] == "" {
			return ""
		}
		return fields[key] + " bit"
	case "maxmemory_human":
		v := fields[key]
		if v == "" || v == "0" || v == "0B" {
			if fields["maxmemory"] == "0" || fields["maxmemory"] == "" {
				return "不限制"
			}
		}
		return v
	default:
		return strings.TrimSpace(fields[key])
	}
}

func formatUptime(fields map[string]string) string {
	if days := fields["uptime_in_days"]; days != "" && days != "0" {
		return days + " 天"
	}
	sec, err := strconv.ParseInt(fields["uptime_in_seconds"], 10, 64)
	if err != nil || sec <= 0 {
		if fields["uptime_in_days"] == "0" {
			return "不到 1 天"
		}
		return ""
	}
	switch {
	case sec < 60:
		return fmt.Sprintf("%d 秒", sec)
	case sec < 3600:
		return fmt.Sprintf("%d 分钟", sec/60)
	case sec < 86400:
		return fmt.Sprintf("%.1f 小时", float64(sec)/3600)
	default:
		return fmt.Sprintf("%d 天", sec/86400)
	}
}

func formatUnix(s string) string {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return ""
	}
	return time.Unix(n, 0).Local().Format("2006-01-02 15:04:05")
}

func formatFlag(s string) string {
	switch s {
	case "1":
		return "是"
	case "0":
		return "否"
	default:
		return s
	}
}

func formatHitRate(hits, misses string) string {
	h, err1 := strconv.ParseFloat(hits, 64)
	m, err2 := strconv.ParseFloat(misses, 64)
	if err1 != nil || err2 != nil {
		return ""
	}
	total := h + m
	if total <= 0 {
		return ""
	}
	return fmt.Sprintf("%.2f%%", h/total*100)
}

func parseInfo(s string) map[string]string {
	out := map[string]string{}
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if ok {
			out[k] = v
		}
	}
	return out
}
