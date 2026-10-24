package redisx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ImportResult is how many commands an import applied.
type ImportResult struct {
	OK      int
	Failed  int
	Skipped int
	LastErr error
}

// ParseScript reads an export file: one command a line, arguments quoted
// as Export writes them (strconv.Quote), so DUMP bytes round-trip.
func ParseScript(text string) ([][]string, error) {
	var cmds [][]string
	for i, line := range strings.Split(text, "\n") {
		args, err := splitQuoted(line)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行: %w", i+1, err)
		}
		if len(args) == 0 {
			continue
		}
		cmds = append(cmds, args)
	}
	return cmds, nil
}

// Import runs commands from an export file against the current database.
// REPLACE on RESTORE overwrites keys that already exist. SELECT and other
// server-wide commands are skipped so the chosen DB stays the target.
func (c *Client) Import(ctx context.Context, text string, progress func(done, total int)) (ImportResult, error) {
	if !c.Ready() {
		return ImportResult{}, fmt.Errorf("未连接")
	}
	cmds, err := ParseScript(text)
	if err != nil {
		return ImportResult{}, err
	}
	var res ImportResult
	total := len(cmds)
	if progress != nil {
		progress(0, total)
	}
	for i, args := range cmds {
		if skipImport(args) {
			res.Skipped++
		} else {
			raw := make([]any, len(args))
			for j, a := range args {
				raw[j] = a
			}
			if _, err := c.Do(ctx, raw...); err != nil {
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
	if res.OK == 0 && res.Failed == 0 && res.Skipped == 0 {
		return res, fmt.Errorf("文件里没有可导入的命令")
	}
	return res, nil
}

func skipImport(args []string) bool {
	if len(args) == 0 {
		return true
	}
	cmd := strings.ToUpper(args[0])
	switch cmd {
	case "FLUSHDB", "FLUSHALL", "KEYS", "DEBUG", "SHUTDOWN", "REPLICAOF", "SLAVEOF", "SWAPDB",
		"SELECT", "AUTH", "HELLO", "MODULE", "MIGRATE", "CLUSTER":
		return true
	case "CONFIG":
		return len(args) > 1 && strings.EqualFold(args[1], "SET")
	default:
		return false
	}
}

func splitQuoted(line string) ([]string, error) {
	line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, nil
	}
	var args []string
	i := 0
	for i < len(line) {
		for i < len(line) && unicode.IsSpace(rune(line[i])) {
			i++
		}
		if i >= len(line) {
			break
		}
		if line[i] == '"' {
			s, err := strconv.QuotedPrefix(line[i:])
			if err != nil {
				return nil, err
			}
			unq, err := strconv.Unquote(s)
			if err != nil {
				return nil, err
			}
			args = append(args, unq)
			i += len(s)
			continue
		}
		j := i
		for j < len(line) && !unicode.IsSpace(rune(line[j])) {
			j++
		}
		args = append(args, line[i:j])
		i = j
	}
	return args, nil
}
