package cliparse

import (
	"slices"
	"testing"
)

func TestSplit(t *testing.T) {
	got, err := Split(`SET "hello world" 'a b' c\ d`)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"SET", "hello world", "a b", "c d"}) {
		t.Fatalf("%q", got)
	}
}

func TestSplitErrors(t *testing.T) {
	if _, err := Split(`SET "oops`); err == nil {
		t.Fatal("expected quote error")
	}
}

func TestDangerous(t *testing.T) {
	if !Dangerous([]string{"flushdb"}) || !Dangerous([]string{"KEYS", "*"}) {
		t.Fatal("expected dangerous")
	}
	if !Dangerous([]string{"CONFIG", "SET", "requirepass", "x"}) {
		t.Fatal("config set")
	}
	if Dangerous([]string{"CONFIG", "GET", "*"}) || Dangerous([]string{"GET", "k"}) {
		t.Fatal("safe commands marked dangerous")
	}
}

func TestScanMatch(t *testing.T) {
	if ScanMatch("") != "*" || ScanMatch("user") != "user*" || ScanMatch("user:*") != "user:*" {
		t.Fatal(ScanMatch(""), ScanMatch("user"), ScanMatch("user:*"))
	}
}

func TestPrefixMatch(t *testing.T) {
	if PrefixMatch("user", ":") != "user:*" {
		t.Fatal(PrefixMatch("user", ":"))
	}
	if PrefixMatch("user", "/") != "user/*" {
		t.Fatal(PrefixMatch("user", "/"))
	}
	if PrefixMatch("", ":") != "*" {
		t.Fatal(PrefixMatch("", ":"))
	}
}

func TestLookupHint(t *testing.T) {
	h, ok := Lookup("hset user:1 name Ada")
	if !ok || h.Name != "HSET" || h.Example != "HSET user:1 name Ada" {
		t.Fatalf("%+v ok=%v", h, ok)
	}
	if FormatHint(h) != "HSET key field value  ·  写入 hash 字段，例如 HSET user:1 name Ada" {
		t.Fatal(FormatHint(h))
	}
	if _, ok := Lookup("xyz"); ok {
		t.Fatal("unknown")
	}
	if _, ok := Lookup("zr"); ok {
		t.Fatal("zr is not a unique prefix")
	}
	if h, ok := Lookup("hgeta"); !ok || h.Name != "HGETALL" {
		t.Fatalf("prefix %+v ok=%v", h, ok)
	}
	if h, ok := Lookup("ttl"); !ok || h.Name != "TTL" {
		t.Fatalf("ttl %+v ok=%v", h, ok)
	}
	if len(Suggestions()) == 0 || Suggestions()[0] != "GET key" {
		t.Fatalf("%q", Suggestions())
	}
}

func TestFormatReply(t *testing.T) {
	if FormatReply(nil) != "(nil)" {
		t.Fatal(FormatReply(nil))
	}
	if FormatReply(int64(3)) != "(integer) 3" {
		t.Fatal(FormatReply(int64(3)))
	}
}
