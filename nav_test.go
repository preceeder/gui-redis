package main

import "testing"

func TestParseDBNode(t *testing.T) {
	conn, db, ok := parseDBNode(dbNodeID("local", 3))
	if !ok || conn != "local" || db != 3 {
		t.Fatalf("%s %d %v", conn, db, ok)
	}
	if _, _, ok := parseDBNode("local"); ok {
		t.Fatal("connection id parsed as db")
	}
	if navConnID(dbNodeID("abc", 0)) != "abc" || navConnID("abc") != "abc" {
		t.Fatal(navConnID(dbNodeID("abc", 0)))
	}
}
