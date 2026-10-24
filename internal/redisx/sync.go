package redisx

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// ParseKeyList splits specified keys, one per line, skipping blanks.
func ParseKeyList(text string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, line := range strings.Split(text, "\n") {
		k := strings.TrimSpace(line)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// SyncTo copies keys from this client onto dest with RESTORE REPLACE,
// keeping TTL. Missing keys are skipped.
func (c *Client) SyncTo(ctx context.Context, dest *Client, keys []string, progress func(done, total int)) (ImportResult, error) {
	if !c.Ready() {
		return ImportResult{}, fmt.Errorf("源未连接")
	}
	if dest == nil || !dest.Ready() {
		return ImportResult{}, fmt.Errorf("目标未连接")
	}
	if len(keys) == 0 {
		return ImportResult{}, fmt.Errorf("没有可同步的 key")
	}
	var res ImportResult
	total := len(keys)
	if progress != nil {
		progress(0, total)
	}
	for i, key := range keys {
		if err := ctx.Err(); err != nil {
			res.Failed += total - i
			res.LastErr = err
			break
		}
		dump, err := c.rdb.Dump(ctx, key).Bytes()
		switch {
		case err == redis.Nil:
			res.Skipped++
		case err != nil:
			res.Failed++
			res.LastErr = err
		default:
			ttl, err := c.rdb.PTTL(ctx, key).Result()
			ms := int64(0)
			if err == nil && ttl >= 0 {
				ms = ttl.Milliseconds()
			}
			if err := dest.rdb.Do(ctx, "RESTORE", key, ms, dump, "REPLACE").Err(); err != nil {
				res.Failed++
				res.LastErr = err
			} else {
				res.OK++
			}
		}
		if progress != nil {
			progress(i+1, total)
		}
	}
	return res, nil
}
