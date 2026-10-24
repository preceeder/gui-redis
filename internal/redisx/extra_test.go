package redisx

import "testing"

func TestAsCommand(t *testing.T) {
	got := AsCommand(Value{KeyInfo: KeyInfo{Key: "k", Type: "string"}, String: "v"})
	if got != `SET "k" "v"` {
		t.Fatal(got)
	}
	got = AsCommand(Value{KeyInfo: KeyInfo{Key: "h", Type: "hash"}, Hash: []Field{{Name: "f", Value: "1"}}})
	if got != `HSET "h" "f" "1"` {
		t.Fatal(got)
	}
}

func TestQuoteArg(t *testing.T) {
	if quoteArg("a b") != `"a b"` {
		t.Fatal(quoteArg("a b"))
	}
}

func TestParseScriptRestore(t *testing.T) {
	dump := string([]byte{0, 1, 2, 255, '"', '\\', '\n'})
	text := "RESTORE " + quoteArg("user:1") + " 0 " + quoteArg(dump) + " REPLACE\n"
	text += "# comment\n"
	text += "SET " + quoteArg("plain") + " " + quoteArg("v") + "\n"
	cmds, err := ParseScript(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 2 {
		t.Fatalf("cmds %d", len(cmds))
	}
	if cmds[0][0] != "RESTORE" || cmds[0][1] != "user:1" || cmds[0][2] != "0" || cmds[0][3] != dump || cmds[0][4] != "REPLACE" {
		t.Fatalf("%q", cmds[0])
	}
	if cmds[1][0] != "SET" || cmds[1][1] != "plain" || cmds[1][2] != "v" {
		t.Fatalf("%q", cmds[1])
	}
}

func TestParseKeyList(t *testing.T) {
	got := ParseKeyList(" a \n\nb\nb\n c ")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("%q", got)
	}
	if ParseKeyList("  \n ") != nil {
		t.Fatal("empty")
	}
}

func TestSkipImport(t *testing.T) {
	if !skipImport([]string{"SELECT", "1"}) || !skipImport([]string{"FLUSHALL"}) {
		t.Fatal("should skip")
	}
	if skipImport([]string{"RESTORE", "k", "0", "x", "REPLACE"}) {
		t.Fatal("restore is safe")
	}
}
