package format

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// MaxStringBytes is the largest string value loaded into the editor.
	MaxStringBytes = 1 << 20
	// MaxCollection is how many members of a hash/list/set/zset are shown per page.
	MaxCollection = 200
	// ScanCount is SCAN COUNT.
	ScanCount = 200
)

// TTL renders PTTL milliseconds.
func TTL(ms int64) string {
	switch {
	case ms < 0:
		return "永久"
	case ms < 1000:
		return fmt.Sprintf("%d ms", ms)
	case ms < 60_000:
		return fmt.Sprintf("%d s", ms/1000)
	case ms < 3_600_000:
		return fmt.Sprintf("%d min", ms/60_000)
	case ms < 86_400_000:
		return fmt.Sprintf("%.1f h", float64(ms)/3_600_000)
	default:
		return fmt.Sprintf("%.1f d", float64(ms)/86_400_000)
	}
}

// Bytes renders a byte count.
func Bytes(n int64) string {
	switch {
	case n <= 0:
		return "-"
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/1024/1024)
	}
}

// LooksJSON reports text that can be shown as JSON.
func LooksJSON(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if s[0] != '{' && s[0] != '[' {
		return false
	}
	return json.Valid([]byte(s))
}

// PrettyJSON indents JSON text.
func PrettyJSON(s string) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Size renders a type-specific length.
func Size(n int64) string {
	switch {
	case n < 0:
		return "-"
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
}

// Binary reports values that should default to the hex view.
func Binary(s string) bool {
	if strings.IndexByte(s, 0) >= 0 {
		return true
	}
	return !utf8.ValidString(s)
}

// ToHex dumps bytes as spaced hex.
func ToHex(s string) string {
	if s == "" {
		return ""
	}
	h := hex.EncodeToString([]byte(s))
	var b strings.Builder
	for i := 0; i < len(h); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(h[i : i+2])
	}
	return b.String()
}

// FromHex parses spaced or raw hex into bytes.
func FromHex(s string) (string, error) {
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' {
			return -1
		}
		return r
	}, s)
	b, err := hex.DecodeString(clean)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
