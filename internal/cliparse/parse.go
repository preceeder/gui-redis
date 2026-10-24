package cliparse

import (
	"fmt"
	"strings"
	"unicode"
)

// Split breaks a Redis CLI line into arguments, honoring single and double quotes.
func Split(line string) ([]string, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	var (
		args   []string
		cur    strings.Builder
		quote  rune
		escape bool
	)
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		args = append(args, cur.String())
		cur.Reset()
	}
	for _, r := range line {
		switch {
		case escape:
			cur.WriteRune(r)
			escape = false
		case r == '\\' && quote != '\'':
			escape = true
		case quote != 0:
			if r == quote {
				quote = 0
				break
			}
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
		case unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if escape {
		return nil, fmt.Errorf("未结束的转义")
	}
	if quote != 0 {
		return nil, fmt.Errorf("未结束的引号")
	}
	flush()
	return args, nil
}

// Dangerous reports commands that need a second confirmation.
func Dangerous(args []string) bool {
	if len(args) == 0 {
		return false
	}
	cmd := strings.ToUpper(args[0])
	switch cmd {
	case "FLUSHDB", "FLUSHALL", "KEYS", "DEBUG", "SHUTDOWN", "REPLICAOF", "SLAVEOF", "SWAPDB":
		return true
	case "CONFIG":
		return len(args) > 1 && strings.EqualFold(args[1], "SET")
	default:
		return false
	}
}

// ScanMatch turns a search box value into a SCAN MATCH pattern.
func ScanMatch(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return "*"
	}
	if strings.ContainsAny(q, "*?[") {
		return q
	}
	return q + "*"
}

// FormatReply renders a go-redis Do() result as CLI text.
func FormatReply(v any) string {
	return formatReply(v, 0)
}

func formatReply(v any, depth int) string {
	indent := strings.Repeat("  ", depth)
	switch x := v.(type) {
	case nil:
		return indent + "(nil)"
	case string:
		return indent + x
	case []byte:
		return indent + string(x)
	case int64:
		return indent + fmt.Sprintf("(integer) %d", x)
	case float64:
		return indent + fmt.Sprintf("(double) %v", x)
	case bool:
		if x {
			return indent + "(integer) 1"
		}
		return indent + "(integer) 0"
	case error:
		return indent + "(error) " + x.Error()
	case []any:
		if len(x) == 0 {
			return indent + "(empty array)"
		}
		var b strings.Builder
		for i, item := range x {
			if i > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "%s%d) %s", indent, i+1, strings.TrimLeft(formatReply(item, depth+1), " "))
		}
		return b.String()
	case map[any]any:
		var b strings.Builder
		i := 1
		for k, val := range x {
			if i > 1 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "%s%d) %s\n%s", indent, i, formatReply(k, 0), formatReply(val, depth+1))
			i++
		}
		if i == 1 {
			return indent + "(empty map)"
		}
		return b.String()
	default:
		return indent + fmt.Sprint(v)
	}
}
