package redisx

import (
	"testing"
)

func TestBuildInfoSections(t *testing.T) {
	raw := `# Server
redis_version:7.2.4
redis_mode:standalone
os:Darwin
arch_bits:64
uptime_in_days:3
uptime_in_seconds:280000
# Clients
connected_clients:8
blocked_clients:0
# Memory
used_memory_human:1.50M
used_memory_peak_human:2.00M
maxmemory:0
maxmemory_human:0B
maxmemory_policy:noeviction
# Persistence
rdb_last_save_time:1700000000
aof_enabled:0
# Stats
total_commands_processed:1000
instantaneous_ops_per_sec:12
keyspace_hits:80
keyspace_misses:20
# Replication
role:master
connected_slaves:0
# Cluster
cluster_enabled:0
`
	info := BuildInfo(parseInfo(raw), 42)
	if info.Version != "7.2.4" || info.Role != "master" || info.Memory != "1.50M" || info.DBSize != 42 {
		t.Fatalf("summary %+v", info)
	}
	if info.Uptime != "3 天" {
		t.Fatalf("uptime %q", info.Uptime)
	}
	got := map[string]map[string]string{}
	for _, sec := range info.Sections {
		got[sec.Title] = map[string]string{}
		for _, f := range sec.Fields {
			got[sec.Title][f.Label] = f.Value
		}
	}
	if got["服务器"]["模式"] != "standalone" || got["内存"]["上限"] != "不限制" {
		t.Fatalf("%+v", got)
	}
	if got["统计"]["命中率"] != "80.00%" || got["持久化"]["AOF"] != "否" {
		t.Fatalf("%+v", got)
	}
	if got["集群"]["已启用"] != "否" {
		t.Fatalf("%+v", got)
	}
}

func TestParseKeyspace(t *testing.T) {
	raw := `# Keyspace
db0:keys=3,expires=1,avg_ttl=100
db2:keys=10,expires=0,avg_ttl=0
`
	got := ParseKeyspace(raw)
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	if got[0] != (Database{Index: 0, Keys: 3, Expires: 1}) {
		t.Fatalf("db0 %+v", got[0])
	}
	if got[1] != (Database{Index: 2, Keys: 10}) {
		t.Fatalf("db2 %+v", got[1])
	}
}
