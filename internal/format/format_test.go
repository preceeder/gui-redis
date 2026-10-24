package format

import (
	"strings"
	"testing"
)

func TestTTL(t *testing.T) {
	if TTL(-1) != "永久" {
		t.Fatal(TTL(-1))
	}
	if TTL(2500) != "2 s" {
		t.Fatal(TTL(2500))
	}
}

func TestPrettyJSON(t *testing.T) {
	if !LooksJSON(`{"a":1}`) || LooksJSON("hello") {
		t.Fatal("looks")
	}
	got, err := PrettyJSON(`{"a":1}`)
	if err != nil || !strings.Contains(got, "\n") {
		t.Fatalf("%q %v", got, err)
	}
	if Bytes(0) != "-" || Bytes(2048) != "2.0 KB" {
		t.Fatal(Bytes(0), Bytes(2048))
	}
}

func TestHexRoundTrip(t *testing.T) {
	raw := "hi\x00"
	h := ToHex(raw)
	got, err := FromHex(h)
	if err != nil || got != raw {
		t.Fatalf("%q %v", got, err)
	}
	if !Binary(raw) || Binary("你好") {
		t.Fatal("binary detect")
	}
}
