package redisx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"redis/internal/format"
)

func (c *Client) loadPage(ctx context.Context, v *Value, first bool) error {
	page := int64(format.MaxCollection)
	switch v.Type {
	case "hash":
		cursor := uint64(0)
		if !first {
			cursor = v.Cursor
		}
		var added []Field
		for uint64(len(added)) < uint64(page) {
			fields, next, err := c.rdb.HScan(ctx, v.Key, cursor, "*", page).Result()
			if err != nil {
				return err
			}
			for i := 0; i+1 < len(fields); i += 2 {
				added = append(added, Field{Name: fields[i], Value: fields[i+1]})
				if int64(len(added)) >= page {
					break
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
			if int64(len(added)) >= page {
				break
			}
		}
		if first {
			v.Hash = added
		} else {
			v.Hash = append(v.Hash, added...)
		}
		v.Cursor = cursor
		v.Truncated = cursor != 0 || int64(len(v.Hash)) < v.Total
	case "list":
		start := int64(0)
		if !first {
			start = int64(len(v.List))
		}
		items, err := c.rdb.LRange(ctx, v.Key, start, start+page-1).Result()
		if err != nil {
			return err
		}
		if first {
			v.List = items
		} else {
			v.List = append(v.List, items...)
		}
		v.Truncated = int64(len(v.List)) < v.Total
	case "set":
		cursor := uint64(0)
		if !first {
			cursor = v.Cursor
		}
		var added []string
		members, next, err := c.rdb.SScan(ctx, v.Key, cursor, "*", page).Result()
		if err != nil {
			return err
		}
		added = members
		if first {
			v.Set = added
		} else {
			v.Set = append(v.Set, added...)
		}
		v.Cursor = next
		v.Truncated = next != 0 || int64(len(v.Set)) < v.Total
	case "zset":
		start := int64(0)
		if !first {
			start = int64(len(v.ZSet))
		}
		zs, err := c.rdb.ZRangeWithScores(ctx, v.Key, start, start+page-1).Result()
		if err != nil {
			return err
		}
		added := make([]ZMember, 0, len(zs))
		for _, z := range zs {
			added = append(added, ZMember{Score: z.Score, Member: fmt.Sprint(z.Member)})
		}
		if first {
			v.ZSet = added
		} else {
			v.ZSet = append(v.ZSet, added...)
		}
		v.Truncated = int64(len(v.ZSet)) < v.Total
	case "stream":
		start := "-"
		if !first && v.StreamID != "" {
			start = "(" + v.StreamID
		}
		arr, err := c.rdb.XRangeN(ctx, v.Key, start, "+", page).Result()
		if err != nil {
			return err
		}
		added := make([]StreamEntry, 0, len(arr))
		for _, m := range arr {
			ent := StreamEntry{ID: m.ID}
			for k, val := range m.Values {
				ent.Fields = append(ent.Fields, Field{Name: k, Value: fmt.Sprint(val)})
			}
			added = append(added, ent)
		}
		if first {
			v.Stream = added
		} else {
			v.Stream = append(v.Stream, added...)
		}
		if n := len(v.Stream); n > 0 {
			v.StreamID = v.Stream[n-1].ID
		}
		v.Truncated = int64(len(v.Stream)) < v.Total
	}
	return nil
}

func (c *Client) XAdd(ctx context.Context, key, field, value string) error {
	return c.rdb.XAdd(ctx, &redis.XAddArgs{Stream: key, Values: map[string]any{field: value}}).Err()
}

func (c *Client) XDel(ctx context.Context, key, id string) error {
	return c.rdb.XDel(ctx, key, id).Err()
}

func (c *Client) DeleteKeys(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// SlowEntry is one SLOWLOG item.
type SlowEntry struct {
	ID       int64
	Time     time.Time
	Duration time.Duration
	Command  string
	Client   string
}

func (c *Client) Slowlog(ctx context.Context, n int64) ([]SlowEntry, error) {
	if n <= 0 {
		n = 32
	}
	raw, err := c.rdb.SlowLogGet(ctx, n).Result()
	if err != nil {
		return nil, err
	}
	out := make([]SlowEntry, 0, len(raw))
	for _, e := range raw {
		out = append(out, SlowEntry{
			ID:       e.ID,
			Time:     e.Time,
			Duration: e.Duration,
			Command:  strings.Join(e.Args, " "),
			Client:   e.ClientAddr,
		})
	}
	return out, nil
}

func (c *Client) ScanAll(ctx context.Context, match string) ([]string, error) {
	if match == "" {
		match = "*"
	}
	var keys []string
	var cursor uint64
	for {
		names, next, err := c.Scan(ctx, cursor, match, format.ScanCount)
		if err != nil {
			return nil, err
		}
		keys = append(keys, names...)
		if next == 0 {
			return keys, nil
		}
		cursor = next
	}
}

func (c *Client) Export(ctx context.Context, keys []string, progress func(done, total int)) (string, error) {
	var b strings.Builder
	total := len(keys)
	if progress != nil {
		progress(0, total)
	}
	for i, key := range keys {
		dump, err := c.rdb.Dump(ctx, key).Bytes()
		if err != nil {
			return "", err
		}
		ttl, err := c.rdb.PTTL(ctx, key).Result()
		if err != nil {
			ttl = -1
		}
		ms := ttl.Milliseconds()
		if ms < 0 {
			ms = 0
		}
		fmt.Fprintf(&b, "RESTORE %s %d %s REPLACE\n", quoteArg(key), ms, quoteArg(string(dump)))
		if progress != nil {
			progress(i+1, total)
		}
	}
	return b.String(), nil
}

// AsCommand rebuilds a Redis command that can recreate v.
func AsCommand(v Value) string {
	switch v.Type {
	case "string":
		return "SET " + quoteArg(v.Key) + " " + quoteArg(v.String)
	case "hash":
		if len(v.Hash) == 0 {
			return "HSET " + quoteArg(v.Key) + " _ \"\""
		}
		var b strings.Builder
		b.WriteString("HSET " + quoteArg(v.Key))
		for _, f := range v.Hash {
			b.WriteString(" " + quoteArg(f.Name) + " " + quoteArg(f.Value))
		}
		return b.String()
	case "list":
		if len(v.List) == 0 {
			return "RPUSH " + quoteArg(v.Key) + " \"\""
		}
		var b strings.Builder
		b.WriteString("RPUSH " + quoteArg(v.Key))
		for _, x := range v.List {
			b.WriteString(" " + quoteArg(x))
		}
		return b.String()
	case "set":
		if len(v.Set) == 0 {
			return "SADD " + quoteArg(v.Key) + " _"
		}
		var b strings.Builder
		b.WriteString("SADD " + quoteArg(v.Key))
		for _, x := range v.Set {
			b.WriteString(" " + quoteArg(x))
		}
		return b.String()
	case "zset":
		if len(v.ZSet) == 0 {
			return "ZADD " + quoteArg(v.Key) + " 0 _"
		}
		var b strings.Builder
		b.WriteString("ZADD " + quoteArg(v.Key))
		for _, z := range v.ZSet {
			b.WriteString(" " + strconv.FormatFloat(z.Score, 'f', -1, 64) + " " + quoteArg(z.Member))
		}
		return b.String()
	case "stream":
		if len(v.Stream) == 0 {
			return "XADD " + quoteArg(v.Key) + " * _ \"\""
		}
		e := v.Stream[0]
		var b strings.Builder
		b.WriteString("XADD " + quoteArg(v.Key) + " " + e.ID)
		for _, f := range e.Fields {
			b.WriteString(" " + quoteArg(f.Name) + " " + quoteArg(f.Value))
		}
		return b.String()
	default:
		return ""
	}
}

func quoteArg(s string) string {
	return strconv.Quote(s)
}
